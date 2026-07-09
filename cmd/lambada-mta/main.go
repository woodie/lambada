// Command lambada-mta is a tiny open-relay SMTP server: any mail sent to
// it gets its attachments saved to disk by attachments.go and is otherwise
// discarded. It is the Go port of scandalous's mta.rb, split the same way:
// the "work" lives in attachments.go, and this file is just the SMTP
// wiring (mta.rb's MidiSmtpServer subclass).
package main

import (
	"io"
	"log"
	"net/mail"
	"os"
	"time"

	"github.com/emersion/go-smtp"
)

var listenAddr = "0.0.0.0:2525"

// LAMBADA_QUIET silences all logging (log.Printf/Fatalf) when set to any
// non-empty value -- both binaries honor it the same way. Useful for
// keeping `ginkgo -r`'s output focused on pass/fail dots rather than every
// handler's log lines (see `check` in package.json), without editing every
// log call individually.
func init() {
	if os.Getenv("LAMBADA_QUIET") != "" {
		log.SetOutput(io.Discard)
	}
}

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
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return err
	}
	cleanupOldFiles()
	return processAttachments(msg)
}

func main() {
	checkAttachmentDir()

	s := smtp.NewServer(&Backend{})
	s.Addr = listenAddr
	s.Domain = "localhost"
	s.ReadTimeout = 60 * time.Second
	s.WriteTimeout = 60 * time.Second
	s.MaxMessageBytes = 25 * 1024 * 1024
	s.MaxRecipients = 100

	log.Printf("SMTP open relay listening on %s", listenAddr)
	if err := s.ListenAndServe(); err != nil {
		log.Fatalf("SMTP server error: %v", err)
	}
}
