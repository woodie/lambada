// ScanFiles reads the scan directory and shapes the result for callers.
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// scan describes one file available for download.
type scan struct {
	Name string
	Time time.Time
	Size int64
}

// Resolve filename to a path within scanDir
func scanFilesPath(filename string) (path string, ok bool) {
	name := filepath.Base(filename)
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, filepath.Separator) {
		return "", false
	}

	path = filepath.Join(scanDir, name)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", false
	}

	return path, true
}

// shared by the index and files.json routes: fetch the scan listing or fail the request
func scanFilesListingOrFail(w http.ResponseWriter) ([]scan, bool) {
	scans, err := scanFilesListing(scanDir)
	if err != nil {
		log.Printf("listing error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return nil, false
	}
	return scans, true
}

// scanFilesListing returns every *.pdf file in dir, newest filename first (epoch filenames sort lexicographically).
func scanFilesListing(dir string) ([]scan, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.pdf"))
	if err != nil {
		return nil, err
	}

	scans := make([]scan, 0, len(matches))
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			// File may have vanished between Glob and Stat (e.g. concurrent cleanup) -- skip it.
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

// scanJSON is the /files.json (and zouk) wire shape; Path is a server-relative path, not a URL.
type scanJSON struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Time string `json:"time"`
	Path string `json:"path"`
}

// toScansJSON converts a raw listing to its API shape, pulled out for unit testing without net/http/httptest.
func toScansJSON(scans []scan) []scanJSON {
	out := make([]scanJSON, 0, len(scans))
	for _, s := range scans {
		out = append(out, scanJSON{
			Name: s.Name,
			Size: s.Size,
			Time: s.Time.Format(time.RFC3339),
			Path: "/download/" + s.Name,
		})
	}
	return out
}
