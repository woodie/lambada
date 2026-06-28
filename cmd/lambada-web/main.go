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
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/justincampbell/timeago"
)

// listenAddr defaults to 0.0.0.0:8080, the same direct-expose setup
// lambada-web has always used -- nginx (service/lambada-web.nginx.conf)
// is optional, not assumed. Set LAMBADA_WEB_LISTEN_ADDR=127.0.0.1:8080
// without a rebuild to switch to loopback-only once nginx is actually the
// thing facing the LAN on port 80, proxying to here over a stable local
// connection -- see docs/COWORK.md for the motivation (the suspected
// culprit behind an intermittent zouk connect hang, issue #5) and
// docs/DEVELOPMENT.md's "Reverse proxy (nginx)" section for setup/rollback.
var (
	scanDir    = "./attachments"
	listenAddr = envOr("LAMBADA_WEB_LISTEN_ADDR", "0.0.0.0:8080")
)

// envOr returns the value of the named environment variable, or fallback
// if it's unset or empty. The one and only knob lambada-web exposes outside
// of editing main.go directly -- see the var block above.
func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

//go:embed templates/listing.html.tmpl
var templatesFS embed.FS

//go:embed static/style.css
var styleCSS []byte

// listingTemplate renders listing.html.tmpl, calling humanSize/timeAgo
// directly from the template -- same shape as listing.erb calling
// number_to_human_size/time_ago_in_words inline.
var listingTemplate = template.Must(
	template.New("listing.html.tmpl").
		Funcs(template.FuncMap{
			"humanSize": func(size int64) string { return humanize.Bytes(uint64(size)) },
			"timeAgo":   func(t, now time.Time) string { return timeago.FromDuration(now.Sub(t).Abs()) },
			"timeNow":   time.Now,
		}).
		ParseFS(templatesFS, "templates/listing.html.tmpl"),
)

// listingData is what listing.html.tmpl renders: just the raw scan
// listing -- the template fetches the current time itself via timeNow to
// compute each scan's age via timeAgo.
type listingData struct {
	Scans []scan
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

// handleDownload serves a single file out of scanDir. net/http's ServeMux
// already redirects requests containing ".." path segments to their cleaned
// equivalent before this handler ever runs; the checks below are a second,
// defense-in-depth layer in case that ever changes (e.g. a future router
// swap) or a filename containing "/" or ".." ends up on disk some other way.
func handleDownload(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.PathValue("filename"))
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, filepath.Separator) {
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

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handleIndex)
	mux.HandleFunc("GET /style.css", handleStyle)
	mux.HandleFunc("GET /scans.json", handleScansJSON)
	mux.HandleFunc("GET /download/{filename}", handleDownload)
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
