// ScanFiles reads the scan directory and shapes the result for callers.
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

// listing returns every *.pdf file in dir, newest filename first (epoch filenames sort lexicographically).
func listing(dir string) ([]scan, error) {
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
