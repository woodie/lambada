// Command lambada-web serves a listing of scans plus a JSON API for zouk
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/woodie/humane"
)

// silence logging for `npm run check`
func init() {
	if os.Getenv("LAMBADA_QUIET") != "" {
		log.SetOutput(io.Discard)
	}
}

var (
	scanDir    = envOr("LAMBADA_ATTACHMENTS_DIR", "./attachments")
	listenAddr = envOr("LAMBADA_WEB_LISTEN_ADDR", "0.0.0.0:8080")
)

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

type listingData struct { Listing []scan }

//go:embed views/listing.html.tmpl
var viewsFS embed.FS
//go:embed static
var staticFS embed.FS

// render listing.html.tmpl exposing timeAgo and humanSize
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

// shared by the index and files.json routes: fetch the scan listing or fail the request
func scanListingOrFail(w http.ResponseWriter) ([]scan, bool) {
	scans, err := listing(scanDir)
	if err != nil {
		log.Printf("listing error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return nil, false
	}
	return scans, true
}

func writeFileNotFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte("File not found"))
}

// Resolve filename to a path within scanDir, writing 404 on error
func scanFilePathOr404(w http.ResponseWriter, filename string) (path string, ok bool) {
	name := filepath.Base(filename)
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, filepath.Separator) {
		writeFileNotFound(w)
		return "", false
	}

	path = filepath.Join(scanDir, name)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		writeFileNotFound(w)
		return "", false
	}

	return path, true
}

func newMux() *http.ServeMux {
	mux := http.NewServeMux()

	// Route to list all available files
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		scans, ok := scanListingOrFail(w)
		if !ok { return }

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := listingTemplate.Execute(w, listingData{Listing: scans}); err != nil {
			log.Printf("template error: %v", err)
		}
	})

	// Route to JSON list of files (for zouk client)
	mux.HandleFunc("GET /files.json", func(w http.ResponseWriter, r *http.Request) {
		scans, ok := scanListingOrFail(w)
		if !ok { return }

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(toScansJSON(scans)); err != nil {
			log.Printf("json encode error: %v", err)
		}
	})

	// Route to download a specific file
	mux.HandleFunc("GET /download/{filename}", func(w http.ResponseWriter, r *http.Request) {
		path, ok := scanFilePathOr404(w, r.PathValue("filename"))
		if !ok { return }

		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(path)))
		http.ServeFile(w, r, path)
	})

	// Route to delete a specific file
	mux.HandleFunc("DELETE /download/{filename}", func(w http.ResponseWriter, r *http.Request) {
		path, ok := scanFilePathOr404(w, r.PathValue("filename"))
		if !ok { return }

		if err := os.Remove(path); err != nil {
			log.Printf("delete error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	// Route to serve static files
	mux.Handle("GET /", func() http.Handler {
		static, err := fs.Sub(staticFS, "static")
		if err != nil {
			log.Fatalf("static assets: %v", err)
		}
		return http.FileServerFS(static)
	}())

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
