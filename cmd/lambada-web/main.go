// Command lambada-web serves a listing of the scans Lambada (or its Ruby
// predecessor, scandalous) has saved, plus a small JSON API for the zouk
// Mac client. It is the Go port of scandalous's web.rb + lib/scan_files.rb,
// split the same way: the "work" lives in scanfiles.go (ScanFiles) and
// server.go (Server), and this file is just the HTTP wiring (web.rb).
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/justincampbell/timeago"
)

// scanDir defaults to a relative path so a plain `go build &&
// ./lambada-web` from a checkout just works with no setup. Under systemd,
// LAMBADA_ATTACHMENTS_DIR overrides it to the shared production location
// (/srv/lambada/attachments) -- lambada-mta honors the same variable for
// the same directory, since both binaries have to agree on it.
//
// listenAddr defaults to 0.0.0.0:8080, the same direct-expose setup
// lambada-web has always used -- nginx (service/lambada-web.nginx.conf)
// is optional, not assumed. Set LAMBADA_WEB_LISTEN_ADDR=127.0.0.1:8080
// without a rebuild to switch to loopback-only once nginx is actually the
// thing facing the LAN on port 80, proxying to here over a stable local
// connection -- see docs/COWORK.md for the motivation (the suspected
// culprit behind an intermittent zouk connect hang, issue #5) and
// docs/DEVELOPMENT.md's "Reverse proxy (nginx)" section for setup/rollback.
var (
	scanDir    = envOr("LAMBADA_ATTACHMENTS_DIR", "./attachments")
	listenAddr = envOr("LAMBADA_WEB_LISTEN_ADDR", "0.0.0.0:8080")
)

// backupsSubdirName is where /backups keeps its files: a subdirectory of
// scanDir rather than a directory of its own. That gets three things for
// free instead of needing a third LAMBADA_BACKUPS_DIR env var: it inherits
// scanDir's relative-dev-default/absolute-production-path behavior with no
// separate override, `make install` doesn't need to provision or chown a
// second directory (it's created under the one it already provisions), and
// /download/{filename...} (below) can serve backup files directly at
// /download/backups/<name> without a second download route. lambada-mta's
// cleanupOldFiles skips it (entry.IsDir() and, as of this session, a *.pdf-
// only filter -- see cmd/lambada-mta/attachments.go) and listing()'s
// Glob("*.pdf") here never descends into it, so scans and backups can't
// collide even though they share a parent directory.
const backupsSubdirName = "backups"

// backupsDir derives from the current scanDir rather than being its own
// var, so tests that reassign scanDir per-example (see main_test.go) don't
// need a second reassignment to stay in sync.
func backupsDir() string { return filepath.Join(scanDir, backupsSubdirName) }

// envOr returns the value of the named environment variable, or fallback
// if it's unset or empty. The one and only knob lambada-web exposes outside
// of editing main.go directly -- see the var block above.
func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

//go:embed templates/listing.html.tmpl templates/backups.html.tmpl
var templatesFS embed.FS

//go:embed static/style.css
var styleCSS []byte

// templateFuncs is shared by listingTemplate and backupsTemplate -- both
// pages render a file listing with the same humanSize/timeAgo shape, just
// against a different directory.
var templateFuncs = template.FuncMap{
	"humanSize": func(size int64) string { return humanize.Bytes(uint64(size)) },
	"timeAgo":   func(t, now time.Time) string { return timeago.FromDuration(now.Sub(t).Abs()) },
	"timeNow":   time.Now,
}

// listingTemplate renders listing.html.tmpl, calling humanSize/timeAgo
// directly from the template -- same shape as listing.erb calling
// number_to_human_size/time_ago_in_words inline.
var listingTemplate = template.Must(
	template.New("listing.html.tmpl").
		Funcs(templateFuncs).
		ParseFS(templatesFS, "templates/listing.html.tmpl"),
)

// backupsTemplate renders backups.html.tmpl -- same page shape as
// listingTemplate (a file listing using the same style.css), plus an
// upload form. See handleBackupsIndex/handleBackupsUpload below.
var backupsTemplate = template.Must(
	template.New("backups.html.tmpl").
		Funcs(templateFuncs).
		ParseFS(templatesFS, "templates/backups.html.tmpl"),
)

// listingData is what listing.html.tmpl renders: just the raw scan
// listing -- the template fetches the current time itself via timeNow to
// compute each scan's age via timeAgo.
type listingData struct {
	Scans []scan
}

// backupsData is what backups.html.tmpl renders -- the raw backupsDir()
// listing, same shape as listingData.
type backupsData struct {
	Files []backupFile
}

// handleIndex is registered under the "GET /{$}" pattern, which (unlike a
// bare "/") only matches the exact root path -- everything else falls
// through to a 404, matching Sinatra's behavior for unmatched routes.
func handleIndex(w http.ResponseWriter, r *http.Request) {
	scans, err := listing(scanDir)
	if err != nil {
		log.Printf("listing error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := listingData{Scans: scans}
	if err := listingTemplate.Execute(w, data); err != nil {
		log.Printf("template error: %v", err)
	}
}

func handleStyle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	_, _ = w.Write(styleCSS)
}

func handleScansJSON(w http.ResponseWriter, r *http.Request) {
	scans, err := listing(scanDir)
	if err != nil {
		log.Printf("listing error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(toScansJSON(scans)); err != nil {
		log.Printf("json encode error: %v", err)
	}
}

// sanitizeFilename reduces raw to its base name and rejects anything that
// isn't a plain, single-segment filename -- used for the upload path only
// (handleBackupsUpload), where an uploaded name is never supposed to
// contain a directory component.
func sanitizeFilename(raw string) (string, bool) {
	name := filepath.Base(raw)
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, filepath.Separator) {
		return "", false
	}
	return name, true
}

// sanitizeRelPath cleans raw (URL-path-style, forward-slash-separated) into
// a path relative to scanDir, collapsing any ".." segments so the result
// can never point above scanDir -- the same Clean("/"+raw) trick net/http's
// own file-serving code uses (a leading "/" makes ".." segments that would
// otherwise escape the root simply get dropped instead). Needed because
// serveFile now has two shapes to handle: flat scan filenames
// ("1234567890.pdf") and one-level-nested backup files
// ("backups/report.pdf", since backupsDir lives inside scanDir -- see its
// comment above). net/http's ServeMux already redirects requests
// containing ".." path segments to their cleaned equivalent before any
// handler runs; this is a second, defense-in-depth layer in case that ever
// changes (e.g. a future router swap).
func sanitizeRelPath(raw string) (string, bool) {
	clean := strings.TrimPrefix(path.Clean("/"+raw), "/")
	if clean == "" || clean == "." {
		return "", false
	}
	return clean, true
}

// handleDownload serves a single file out of scanDir at the relative path
// given by the "filename" wildcard -- registered as
// "GET /download/{filename...}" so it matches both flat scan filenames
// (/download/1234567890.pdf) and backup files one level down
// (/download/backups/report.pdf), since backupsDir is just scanDir's
// "backups" subdirectory. One route/handler for both instead of a second
// /backups/download/{filename} -- see backupsDir's comment above.
func handleDownload(w http.ResponseWriter, r *http.Request) {
	rel, ok := sanitizeRelPath(r.PathValue("filename"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	fsPath := filepath.Join(scanDir, filepath.FromSlash(rel))
	info, err := os.Stat(fsPath)
	if err != nil || info.IsDir() {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("File not found"))
		return
	}

	name := path.Base(rel)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	http.ServeFile(w, r, fsPath)
}

// handleBackupsIndex renders backups.html.tmpl -- same shape as
// handleIndex, against backupsDir() instead of scanDir.
func handleBackupsIndex(w http.ResponseWriter, r *http.Request) {
	files, err := listBackups(backupsDir())
	if err != nil {
		log.Printf("listBackups error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := backupsData{Files: files}
	if err := backupsTemplate.Execute(w, data); err != nil {
		log.Printf("template error: %v", err)
	}
}

// maxUploadBytes bounds a single /backups upload -- generous enough for
// photos/archives/backups of any extension, but not unbounded (an
// http.Server with no cap would let a slow/huge upload hold a goroutine
// open indefinitely, the same class of problem newServer's timeouts guard
// against elsewhere -- see server.go).
const maxUploadBytes = 200 << 20 // 200 MiB

// handleBackupsUpload accepts a POST of a single "file" form field and
// writes it into backupsDir() under its own filename, any extension
// accepted. Uploading a name that already exists in backupsDir() replaces
// it -- add and replace are the same code path (O_TRUNC), keyed only on
// whether that name already exists on disk.
func handleBackupsUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Upload too large or malformed", http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	name, ok := sanitizeFilename(header.Filename)
	if !ok {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	dest, err := os.OpenFile(filepath.Join(backupsDir(), name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		log.Printf("backup upload error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer dest.Close()

	if _, err := io.Copy(dest, file); err != nil {
		log.Printf("backup upload error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/backups", http.StatusSeeOther)
}

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handleIndex)
	mux.HandleFunc("GET /style.css", handleStyle)
	mux.HandleFunc("GET /files.json", handleScansJSON)
	mux.HandleFunc("GET /download/{filename...}", handleDownload)
	mux.HandleFunc("GET /backups", handleBackupsIndex)
	mux.HandleFunc("POST /backups", handleBackupsUpload)
	return mux
}

func main() {
	if err := os.MkdirAll(scanDir, 0755); err != nil {
		log.Fatalf("Cannot create scan directory: %v", err)
	}
	if err := os.MkdirAll(backupsDir(), 0755); err != nil {
		log.Fatalf("Cannot create backups directory: %v", err)
	}

	log.Printf("lambada-web listening on %s, serving %s (scans) and %s (backups)", listenAddr, scanDir, backupsDir())
	if err := newServer(listenAddr, newMux()).ListenAndServe(); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}
