// Command lambada-web serves a listing of scans plus a JSON API for zouk
package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"lambada/internal/envutil"
	"lambada/loglevel"

	"github.com/woodie/humane"
)

var (
	scanDir    = envutil.Or("LAMBADA_ATTACHMENTS_DIR", "./attachments")
	listenAddr = envutil.Or("LAMBADA_WEB_LISTEN_ADDR", "0.0.0.0:8080")
)

type listingData struct { Listing []scan }

//go:embed views/listing.html.tmpl
var viewsFS embed.FS
//go:embed static
var staticFS embed.FS

// render listing.html.tmpl with timeAgo and humanSize
var listingTemplate = template.Must(
	template.New("listing.html.tmpl").
		Funcs(template.FuncMap{
			"humanSize": humane.HumanSize,
			"timeAgo":   humane.TimeAgo,
		}).
		ParseFS(viewsFS, "views/listing.html.tmpl"),
)

func newMux() *http.ServeMux {
	mux := http.NewServeMux()

	// Route to list all available files -- a listing error is treated as no files, not a failed request
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		scans, _ := scanFilesListing(scanDir)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := listingTemplate.Execute(w, listingData{Listing: scans}); err != nil {
			log.Printf("template error: %v", err)
		}
	})

	// Route to JSON list of files (for zouk client) -- a listing error is treated as no files, not a failed request
	mux.HandleFunc("GET /files.json", func(w http.ResponseWriter, r *http.Request) {
		scans, _ := scanFilesListing(scanDir)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(toScansJSON(scans)); err != nil {
			log.Printf("json encode error: %v", err)
		}
	})

	// Route to download a specific file
	mux.HandleFunc("GET /download/{filename}", func(w http.ResponseWriter, r *http.Request) {
		path, err := scanFilesPath(r.PathValue("filename"))
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "File not found", http.StatusNotFound) // 404
		} else if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError) // 500
		} else {
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(path)))
			http.ServeFile(w, r, path)
		}
	})

	// Route to delete a specific file
	mux.HandleFunc("DELETE /download/{filename}", func(w http.ResponseWriter, r *http.Request) {
		path, err := scanFilesPath(r.PathValue("filename"))
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "File not found", http.StatusNotFound) // 404
		} else if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError) // 500
		} else if err := os.Remove(path); err != nil {
			log.Printf("delete error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError) // 500
		} else {
			w.WriteHeader(http.StatusNoContent) // 204
		}
	})

	// Catch-all pattern to fetch static content
	mux.Handle("GET /", func() http.Handler {
		static, err := fs.Sub(staticFS, "static")
		if err != nil {
			log.Fatalf("static assets: %v", err)
		}
		return http.FileServerFS(static)
	}())

	return mux
}

// LOG_LEVEL=OFF silences logging
func init() {
	loglevel.Apply()
}

func main() {
	if err := os.MkdirAll(scanDir, 0755); err != nil {
		log.Fatalf("Cannot create scan directory: %v", err)
	}

	log.Printf("lambada-web listening on %s, serving %s", listenAddr, scanDir)
	if err := newServer(listenAddr, withLogging(newMux())).ListenAndServe(); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}
