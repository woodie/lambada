// ScanFiles -- reads the scan directory and shapes the result for callers.
// Go port of scandalous's lib/scan_files.rb (the ScanFiles class), kept in
// its own file/test file the same way: main.go's HTTP handlers call into
// this, but nothing here knows about net/http.
package main

import (
	"os"
	"path/filepath"
	"sort"
	"time"
)

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

// scanJSON is the shape served at /files.json (and consumed by the zouk
// Mac client). Path is a server-relative download path, not a URL --
// previously misnamed "url" in this field and in scandalous's matching
// Ruby shape; both were renamed together as part of the /files.json
// rename so the field name actually describes what it holds.
type scanJSON struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Time string `json:"time"`
	Path string `json:"path"`
}

// toScansJSON converts a raw scan listing into its API shape -- pulled out
// of handleScansJSON so it's unit-testable without going through
// net/http/httptest. Mirrors Ruby's ScanFiles.scans_json.
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
