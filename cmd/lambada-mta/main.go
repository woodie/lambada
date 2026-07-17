// Command lambada-mta is a tiny open-relay SMTP server that saves attachments to disk.
package main

import (
	"io"
	"log"
	"net/mail"
	"time"

	"lambada/loglevel"

	"github.com/emersion/go-smtp"
)

var listenAddr = "0.0.0.0:2525"

// LOG_LEVEL=OFF silences all logging (see package.json's check script).
func init() {
	loglevel.Apply()
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
