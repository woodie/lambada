// ScanFiles reads the scan directory and shapes the result for callers.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// scan describes one file available for download.
type scan struct {
	Name string
	Time time.Time
	Size int64
}

// Resolve filename to a path within scanDir; a not-found name/file/directory returns os.ErrNotExist, any other stat failure is logged and returned as-is.
func scanFilesPath(filename string) (path string, err error) {
	name := filepath.Base(filename)
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, filepath.Separator) {
		return "", os.ErrNotExist
	}

	path = filepath.Join(scanDir, name)
	info, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("stat %s: %v", path, err)
		}
		return "", err
	}
	if info.IsDir() {
		return "", os.ErrNotExist
	}

	return path, nil
}

// scanFilesListing returns every *.pdf file in dir, newest filename first (epoch filenames sort lexicographically).
func scanFilesListing(dir string) ([]scan, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.pdf"))
	scans := make([]scan, 0, len(matches))
	if err != nil {
		err = fmt.Errorf("glob %s: %w", dir, err)
		log.Printf("%v", err)
		return scans, err
	}

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

	slices.SortFunc(scans, func(a, b scan) int { return strings.Compare(b.Name, a.Name) })
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
