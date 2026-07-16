// Command lambada-web serves a listing of scans plus a JSON API for the zouk Mac client.
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

// scanDir and listenAddr are overridden via LAMBADA_ATTACHMENTS_DIR and LAMBADA_WEB_LISTEN_ADDR; see docs/DEVELOPMENT.md.
var (
	scanDir    = envOr("LAMBADA_ATTACHMENTS_DIR", "./attachments")
	listenAddr = envOr("LAMBADA_WEB_LISTEN_ADDR", "0.0.0.0:8080")
)

// LAMBADA_QUIET, if set, silences all logging (see package.json's check script).
func init() {
	if os.Getenv("LAMBADA_QUIET") != "" {
		log.SetOutput(io.Discard)
	}
}

// envOr returns the named environment variable, or fallback if unset/empty.
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

// renders listing.html.tmpl, exposing timeAgo humanSize.
var listingTemplate = template.Must(
	template.New("listing.html.tmpl").
		Funcs(template.FuncMap{
			"humanSize": humane.HumanSize,
			"timeAgo": func(t time.Time) string {
				return humane.TimeAgo(&t)
			},
		}).
		ParseFS(viewsFS, "views/listing.html.tmpl"),
)

// listingData is what listing.html.tmpl renders.
type listingData struct {
	Listing []scan
}

// The scan listing at the exact root path (registered as "GET /{$}", not "GET /").
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

// sanitizeFilename defends against path traversal; ServeMux already blocks "..", this is a second layer.
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

// handleDelete is the DELETE counterpart to handleDownload, on the same "/download/{filename}" route.
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
