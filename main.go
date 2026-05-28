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

	"github.com/emersion/go-smtp"
)

var (
	attachmentDir = "./attachments"
	listenAddr    = "0.0.0.0:2525"
	maxFileAge    = 24 * time.Hour
)

type Backend struct{}

func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	log.Printf("New connection from %s", c.Hostname())
	return &Session{}, nil
}

type Session struct{}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error { return nil }

func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error { return nil }

func (s *Session) Logout() error { return nil }

func (s *Session) Reset() {}

func (s *Session) Data(r io.Reader) error {
	cleanupOldFiles()

	msg, err := mail.ReadMessage(r)
	if err != nil {
		return err
	}

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
		if entry.IsDir() {
			continue
		}
		if entry.Name() == ".DS_Store" {
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

func main() {
	if err := os.MkdirAll(attachmentDir, 0755); err != nil {
		log.Fatalf("Cannot create attachment directory: %v", err)
	}

	s := smtp.NewServer(&Backend{})
	s.Addr = listenAddr
	s.Domain = "localhost"
	s.ReadTimeout = 60 * time.Second
	s.WriteTimeout = 60 * time.Second
	s.MaxMessageBytes = 25 * 1024 * 1024 // 25 MB
	s.MaxRecipients = 100

	log.Printf("SMTP open relay listening on %s", listenAddr)
	if err := s.ListenAndServe(); err != nil {
		log.Fatalf("SMTP server error: %v", err)
	}
}
