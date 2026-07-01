// Backups -- backupsDir() (main.go) is a subdirectory of scanDir, but
// otherwise managed entirely separately from scans: through /backups (see
// handleBackupsIndex/handleBackupsUpload in main.go), not lambada-mta. Any
// filename/extension is accepted; there's no *.pdf filter like
// scanfiles.go's listing(). Kept in its own file/test file the same way
// scanfiles.go is, so main.go's handlers stay thin HTTP wiring.
package main

import (
	"os"
	"sort"
	"time"
)

// backupFile describes one file available in backupsDir.
type backupFile struct {
	Name string
	Time time.Time
	Size int64
}

// listBackups returns every regular file directly inside dir (no
// recursion), alphabetical by name -- unlike listing()'s newest-filename-
// first sort, backup filenames are whatever the uploader named them, not
// epoch timestamps, so there's no equivalent "newest" ordering to exploit.
func listBackups(dir string) ([]backupFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := make([]backupFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			// Removed between ReadDir and Info -- skip it, same
			// race-tolerance as listing().
			continue
		}
		files = append(files, backupFile{
			Name: entry.Name(),
			Time: info.ModTime(),
			Size: info.Size(),
		})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}
