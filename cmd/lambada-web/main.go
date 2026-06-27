// Command lambada-web serves a listing of the scans Lambada (or its Ruby
// predecessor, scandalous) has saved, plus a small JSON API for the zouk
// Mac client. It is the Go port of scandalous's web.rb + lib/scan_files.rb.
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
	"sort"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/justincampbell/timeago"
)

var (
	scanDir    = "./attachments"
	listenAddr = "0.0.0.0:8080"
)

//go:embed templates/listing.html.tmpl
var templatesFS embed.FS

//go:embed static/style.css
var styleCSS []byte

// ScanFiles -- reads the scan directory and shapes the result for callers.
// Go port of scandalous's ScanFiles class; mirrored in main_test.go as its
// own top-level group, same split as the Ruby specs.

// scan describes one file available for download.
type scan struct {
	Name string
	Time time.Time
	Size int64
}

// listing returns every *.pdf file in dir, newest filename first. Scan
// filenames are an epoch timestamp (e.g. 1779867473.pdf), so a descending
// lexicographic sort on the name is equivalent to newest-first -- this
// matches ScanFiles.listing's `sort_by { |h| h[:name] }.reverse` in the
// Ruby version.
func listing(dir string) ([]scan, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.pdf"))
	if err != nil {
		return nil, err
	}

	scans := make([]scan, 0, len(matches))
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			// File may have been removed/renamed between Glob and Stat
			// (e.g. lambada-mta's cleanup running concurrently) -- skip it.
			continue
		}
		scans = append(scans, scan{
			Name: filepath.Base(path),
			Time: info.ModTime(),
			Size: info.Size(),
		})
	}

	sort.Slice(scans, func(i, j int) bool { return scans[i].Name > scans[j].Name })
	return scans, nil
}

type scanJSON struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Time string `json:"time"`
	URL  string `json:"url"`
}

// toScansJSON converts a raw scan listing into the shape served at
// /scans.json (and consumed by the zouk Mac client) -- pulled out of
// handleScansJSON so it's unit-testable without going through
// net/http/httptest. Mirrors Ruby's ScanFiles.scans_json.
func toScansJSON(scans []scan) []scanJSON {
	out := make([]scanJSON, 0, len(scans))
	for _, s := range scans {
		out = append(out, scanJSON{
			Name: s.Name,
			Size: s.Size,
			Time: s.Time.Format(time.RFC3339),
			URL:  "/download/" + s.Name,
		})
	}
	return out
}

// Server -- the http.Server lambada-web actually runs, and the timeouts it
// needs to avoid issue #2 (see docs/COWORK.md). Mirrored in main_test.go as
// its own top-level group, Server > newServer.

// Connection timeouts for the http.Server lambada-web runs. The bare
// http.ListenAndServe(addr, handler) helper used previously builds a
// zero-value http.Server, and every one of ReadTimeout, ReadHeaderTimeout,
// WriteTimeout, and IdleTimeout defaults to 0 there -- i.e. "wait forever."
// A client that opens a keep-alive connection and then goes quiet (a
// laptop sleeping mid-request, a flaky Wi-Fi hop, zouk reconnecting
// without cleanly closing the old socket) ties up a goroutine and a file
// descriptor on the Pi forever. lambada-web.service's Restart=always never
// fires to clear this because the process never actually crashes -- it
// just silently accumulates leaked connections for as long as it's been
// running, until new clients can't get in at all even though systemd
// still reports it "active". This was the root cause behind
// https://github.com/woodie/lambada/issues/2: restarting the process by
// hand "fixed" it only because that reset the leak count to zero, not
// because manually backgrounding it is mechanically different from
// systemd's Type=simple -- it isn't. See docs/COWORK.md.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 60 * time.Second
)

// newServer builds the http.Server lambada-web actually runs, with the
// timeouts above applied -- pulled out of main so they're unit-testable
// without binding a real listener.
func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

// Lambada WEB -- the HTTP routes and the page/JSON they serve. Mirrored in
// main_test.go as the Lambada WEB group (GET /, GET /download/{filename},
// GET /scans.json, GET /style.css).

// listingTemplate renders listing.html.tmpl, calling humanSize/timeAgo
// directly from the template -- same shape as listing.erb calling
// number_to_human_size/time_ago_in_words inline.
var listingTemplate = template.Must(
	template.New("listing.html.tmpl").
		Funcs(template.FuncMap{
			"humanSize": func(size int64) string { return humanize.Bytes(uint64(size)) },
			"timeAgo":   func(t, now time.Time) string { return timeago.FromDuration(now.Sub(t).Abs()) },
		}).
		ParseFS(templatesFS, "templates/listing.html.tmpl"),
)

// listingData is what listing.html.tmpl renders: the raw scan listing plus
// the request time, since the template needs both to compute each scan's
// age via the timeAgo func.
type listingData struct {
	Scans []scan
	Now   time.Time
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
	data := listingData{Scans: scans, Now: time.Now()}
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
