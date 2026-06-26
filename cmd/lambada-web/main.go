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
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	scanDir    = "./attachments"
	listenAddr = "0.0.0.0:8080"
)

//go:embed templates/listing.html.tmpl
var templatesFS embed.FS

//go:embed static/style.css
var styleCSS []byte

var listingTemplate = template.Must(template.ParseFS(templatesFS, "templates/listing.html.tmpl"))

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

// humanSize renders a byte count the way Rails' ActionView
// number_to_human_size does by default: "7 Bytes", "1 Byte", "1.21 KB",
// "5 GB", etc. -- rounded to three significant digits with trailing zeros
// trimmed.
func humanSize(size int64) string {
	if size < 1024 {
		if size == 1 {
			return "1 Byte"
		}
		return fmt.Sprintf("%d Bytes", size)
	}

	units := []string{"KB", "MB", "GB", "TB", "PB", "EB"}
	value := float64(size)
	idx := 0
	for value >= 1024 && idx < len(units)-1 {
		value /= 1024
		idx++
	}

	return fmt.Sprintf("%s %s", formatSignificant(value, 3), units[idx-1])
}

// formatSignificant formats v to the given number of significant digits,
// trimming any trailing zeros (and a trailing decimal point).
func formatSignificant(v float64, sig int) string {
	if v <= 0 {
		return "0"
	}
	digitsBeforePoint := int(math.Floor(math.Log10(v))) + 1
	decimals := sig - digitsBeforePoint
	if decimals < 0 {
		decimals = 0
	}
	s := strconv.FormatFloat(v, 'f', decimals, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

// timeAgoInWords mirrors Rails' ActionView time_ago_in_words(from), called
// without `include_seconds: true` -- the same way listing.erb calls it.
// That means anything under 30 seconds reads "less than a minute" rather
// than dropping down to seconds-level granularity.
func timeAgoInWords(from, to time.Time) string {
	minutes := int(math.Round(to.Sub(from).Minutes()))
	if minutes < 0 {
		minutes = -minutes
	}

	switch {
	case minutes == 0:
		return "less than a minute"
	case minutes == 1:
		return "1 minute"
	case minutes < 45:
		return fmt.Sprintf("%d minutes", minutes)
	case minutes < 90:
		return "about 1 hour"
	case minutes < 1440:
		return fmt.Sprintf("about %d hours", roundDiv(minutes, 60))
	case minutes < 2520:
		return "1 day"
	case minutes < 43200:
		return fmt.Sprintf("%d days", roundDiv(minutes, 1440))
	case minutes < 86400:
		return fmt.Sprintf("about %d months", roundDiv(minutes, 43200))
	case minutes < 525600:
		return fmt.Sprintf("%d months", roundDiv(minutes, 43200))
	default:
		// Simplified vs. Rails' leap-year-aware "about/over/almost X years"
		// -- a home scanner's listing realistically never gets this stale.
		return fmt.Sprintf("about %d years", roundDiv(minutes, 525600))
	}
}

func roundDiv(n, d int) int {
	return int(math.Round(float64(n) / float64(d)))
}

type viewScan struct {
	Name      string
	HumanSize string
	TimeAgo   string
}

type viewData struct {
	Scans []viewScan
}

// toViewData converts a raw scan listing into the shape listing.html.tmpl
// renders, formatting each scan's size and age relative to now -- pulled
// out of handleIndex so it's unit-testable without going through
// net/http/httptest.
func toViewData(scans []scan, now time.Time) viewData {
	data := viewData{Scans: make([]viewScan, 0, len(scans))}
	for _, s := range scans {
		data.Scans = append(data.Scans, viewScan{
			Name:      s.Name,
			HumanSize: humanSize(s.Size),
			TimeAgo:   timeAgoInWords(s.Time, now),
		})
	}
	return data
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
	if err := listingTemplate.Execute(w, toViewData(scans, time.Now())); err != nil {
		log.Printf("template error: %v", err)
	}
}

func handleStyle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	_, _ = w.Write(styleCSS)
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
	if err := http.ListenAndServe(listenAddr, newMux()); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}
