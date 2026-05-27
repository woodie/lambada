package main

import (
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

const (
	attachmentDir = "./attachments"
	listenAddr    = "0.0.0.0:25"
)

// Backend implements the SMTP server backend.
type Backend struct{}

func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	log.Printf("New connection from %s", c.Hostname())
	return &Session{}, nil
}

// Session holds per-connection state.
type Session struct {
	from string
	to   []string
}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	log.Printf("MAIL FROM: %s", from)
	s.from = from
	return nil
}

func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	log.Printf("RCPT TO: %s", to)
	s.to = append(s.to, to)
	return nil
}

func (s *Session) Data(r io.Reader) error {
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return err
	}

	log.Printf("Receiving message from %s (subject: %q)", s.from, msg.Header.Get("Subject"))

	contentType := msg.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
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
			part.Close()
			continue
		}

		filename := dispParams["filename"]
		if filename == "" {
			filename = fmt.Sprintf("attachment-%d", time.Now().UnixNano())
		}
		filename = filepath.Base(filename) // strip any path components

		destPath := filepath.Join(attachmentDir, filename)
		if _, statErr := os.Stat(destPath); statErr == nil {
			ext := filepath.Ext(filename)
			base := strings.TrimSuffix(filename, ext)
			destPath = filepath.Join(attachmentDir, base+"-"+time.Now().Format("20060102-150405")+ext)
		}

		if err := saveAttachment(part, destPath); err != nil {
			log.Printf("Failed to save attachment %q: %v", filename, err)
		} else {
			log.Printf("Saved attachment: %s", destPath)
		}
		part.Close()
	}

	return nil
}

func (s *Session) Reset() {
	s.from = ""
	s.to = nil
}

func (s *Session) Logout() error {
	return nil
}

func saveAttachment(r io.Reader, destPath string) error {
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	written, err := io.Copy(f, r)
	if err != nil {
		return err
	}
	log.Printf("  Written %d bytes to %s", written, destPath)
	return nil
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

	log.Printf("SMTP open relay listening on %s (attachments -> %s)", listenAddr, attachmentDir)
	if err := s.ListenAndServe(); err != nil {
		log.Fatalf("SMTP server error: %v", err)
	}
}
