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
	"path/filepath"
	"strings"
	"time"

	"github.com/woodie/humane"
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

// LAMBADA_QUIET silences all logging (log.Printf/Fatalf) when set to any
// non-empty value -- both binaries honor it the same way. Useful for
// keeping `ginkgo -r`'s output focused on pass/fail dots rather than every
// handler's log lines (see `check` in package.json), without editing every
// log call individually.
func init() {
	if os.Getenv("LAMBADA_QUIET") != "" {
		log.SetOutput(io.Discard)
	}
}

// envOr returns the value of the named environment variable, or fallback
// if it's unset or empty. The one and only knob lambada-web exposes outside
// of editing main.go directly -- see the var block above.
func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

//go:embed views/listing.html.tmpl
var viewsFS embed.FS

//go:embed static/style.css
var styleCSS []byte

//go:embed static/script.js
var scriptJS []byte

// listingTemplate renders listing.html.tmpl, calling humanSize/timeAgo
// directly from the template -- same shape as listing.erb calling
// human_size/time_ago_in_words inline. humane.TimeAgo defaults to
// Approximate: true (matching ActionView's own always-on-past-the-hour
// behavior, see github.com/woodie/humane v0.9.0), and is direction-aware
// (renders "in 3 minutes" for a future time instead of requiring the
// caller to normalize the sign, which would collapse future and past into
// the same "3 minutes ago" text -- see
// https://github.com/woodie/lambada/issues/15); it already appends its own
// "ago"/"in " affix, so the template doesn't add one.
var listingTemplate = template.Must(
	template.New("listing.html.tmpl").
		Funcs(template.FuncMap{
			"humanSize": humane.HumanSize,
			"timeAgo": func(t time.Time) string {
				return humane.TimeAgo(&t, time.Now())
			},
		}).
		ParseFS(viewsFS, "views/listing.html.tmpl"),
)

// listingData is what listing.html.tmpl renders: just the raw scan
// listing -- timeAgo reaches for the real clock itself (time.Now()) to
// compute each scan's age, the same way Ruby's time_ago_in_words(from_time)
// defaults its to_time to Time.now rather than taking it as an argument.
type listingData struct {
	Listing []scan
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
	data := listingData{Listing: scans}
	if err := listingTemplate.Execute(w, data); err != nil {
		log.Printf("template error: %v", err)
	}
}

func handleStyle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	_, _ = w.Write(styleCSS)
}

func handleScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = w.Write(scriptJS)
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
// isn't a plain, single-segment filename. net/http's ServeMux already
// redirects requests containing ".." path segments to their cleaned
// equivalent before any handler runs; this is a second, defense-in-depth
// layer in case that ever changes (e.g. a future router swap) or a
// filename containing "/" or ".." ends up on disk some other way. Shared by
// handleDownload and handleDelete, so there's one place guarding against
// directory traversal rather than two copies of the same check.
func sanitizeFilename(raw string) (string, bool) {
	name := filepath.Base(raw)
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, filepath.Separator) {
		return "", false
	}
	return name, true
}

// handleDownload serves a single file out of scanDir.
func handleDownload(w http.ResponseWriter, r *http.Request) {
	name, ok := sanitizeFilename(r.PathValue("filename"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	path := filepath.Join(scanDir, name)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("File not found"))
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	http.ServeFile(w, r, path)
}

// handleDelete removes a single file out of scanDir -- the DELETE
// counterpart to handleDownload, registered on the exact same
// "/download/{filename}" resource path (GET fetches it, DELETE removes
// it: same resource, different verb, rather than a separate RPC-style
// "/delete/{filename}" route). Browsers can't submit an HTML form with
// method="DELETE", so the trash icon in listing.html.tmpl calls this via
// a small inline fetch(), not a <form> post.
func handleDelete(w http.ResponseWriter, r *http.Request) {
	name, ok := sanitizeFilename(r.PathValue("filename"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	path := filepath.Join(scanDir, name)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("File not found"))
		return
	}

	if err := os.Remove(path); err != nil {
		log.Printf("delete error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handleIndex)
	mux.HandleFunc("GET /style.css", handleStyle)
	mux.HandleFunc("GET /script.js", handleScript)
	mux.HandleFunc("GET /files.json", handleScansJSON)
	mux.HandleFunc("GET /download/{filename}", handleDownload)
	mux.HandleFunc("DELETE /download/{filename}", handleDelete)
	return mux
}

func main() {
	if err := os.MkdirAll(scanDir, 0755); err != nil {
		log.Fatalf("Cannot create scan directory: %v", err)
	}

	log.Printf("lambada-web listening on %s, serving %s", listenAddr, scanDir)
	if err := newServer(listenAddr, newMux()).ListenAndServe(); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}
