// Attachments -- parses incoming mail, saves attachments to disk, and
// cleans up old ones. Go port of scandalous's lib/scan_files.rb (the
// cleanup/detach side of ScanFiles) plus the MIME parsing mta.rb gets for
// free from the mail gem -- kept in its own file/test file the same way:
// main.go's SMTP Backend/Session call into this, but nothing here knows
// about net/smtp.
package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// attachmentDir is where attachments get saved, and maxFileAge is how long
// cleanupOldFiles lets them sit before deleting them. Go port of
// scandalous's ScanFiles::SCAN_FOLDER / ScanFiles::ONE_DAY_AGO.
//
// attachmentDir defaults to a relative path so a plain `go build &&
// ./lambada-mta` from a checkout just works with no setup. Under systemd,
// LAMBADA_ATTACHMENTS_DIR overrides it to the shared production location
// (/srv/lambada/attachments -- see service/lambada-mta.service and
// docs/DEVELOPMENT.md's Configuration section). lambada-web honors the same
// variable for the same directory, since both binaries have to agree on it.
var (
	attachmentDir = envOr("LAMBADA_ATTACHMENTS_DIR", "./attachments")
	maxFileAge    = 24 * time.Hour
)

// envOr returns the value of the named environment variable, or fallback if
// it's unset or empty. Same helper as lambada-web's -- duplicated rather
// than shared, since the two binaries don't have a common internal package.
func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func processAttachments(msg *mail.Message) error {
	log.Println("Receiving message")

	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		log.Printf("Failed to parse Content-Type: %v", err)
		return nil
	}

	if !strings.HasPrefix(mediaType, "multipart/") {
		log.Println("Message is not multipart, no attachments to save.")
		return nil
	}

	mr := multipart.NewReader(msg.Body, params["boundary"])
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error reading part: %v", err)
			break
		}

		disposition, dispParams, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
		if err != nil || disposition != "attachment" {
			_ = part.Close()
			continue
		}

		ext := filepath.Ext(dispParams["filename"])
		filename := fmt.Sprintf("%d%s", time.Now().Unix(), ext)
		destPath := filepath.Join(attachmentDir, filename)

		var reader io.Reader = part
		if strings.EqualFold(part.Header.Get("Content-Transfer-Encoding"), "base64") {
			reader = base64.NewDecoder(base64.StdEncoding, part)
		}

		if err := saveAttachment(reader, destPath); err != nil {
			log.Printf("Failed to save attachment: %v", err)
		} else {
			log.Printf("Saved attachment: %s", destPath)
		}
		_ = part.Close()
	}
	return nil
}

func saveAttachment(r io.Reader, destPath string) (err error) {
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}()

	bytes, err := io.Copy(f, r)
	if err != nil {
		return err
	}
	log.Printf("Write %d bytes to %s", bytes, destPath)
	return nil
}

func cleanupOldFiles() {
	cutoff := time.Now().Add(-maxFileAge)
	entries, err := os.ReadDir(attachmentDir)
	if err != nil {
		log.Printf("Cleanup error reading dir: %v", err)
		return
	}
	for _, entry := range entries {
		// Only *.pdf files (case-sensitive, matching lambada-web's
		// scanfiles.go Glob("*.pdf") -- both binaries agree on what counts
		// as a scan) are lambada-mta's concern. This one check covers
		// .DS_Store (no .pdf extension, no longer needs its own name
		// check) and also leaves lambada-web's backups/ subdirectory --
		// and anything else a user or another process drops in
		// attachmentDir -- alone, on top of the entry.IsDir() check
		// already skipping directories regardless of name.
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".pdf" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(attachmentDir, entry.Name())
			if err := os.Remove(path); err != nil {
				log.Printf("Cleanup failed to remove %s: %v", path, err)
			} else {
				log.Printf("Cleanup removed old file: %s", path)
			}
		}
	}
}

func checkAttachmentDir() {
	if err := os.MkdirAll(attachmentDir, 0755); err != nil {
		log.Fatalf("Cannot create attachment directory: %v", err)
	}
	if info, err := os.Lstat(attachmentDir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			if target, err := os.Readlink(attachmentDir); err == nil {
				log.Printf("Attachment symlink: %s -> %s", attachmentDir, target)
			}
		}
	}
}
