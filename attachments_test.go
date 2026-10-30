package main

import (
	"encoding/base64"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodie/spec"
	. "github.com/woodie/expect"
)

var plainMessage = "From: sender@example.com\r\n" +
	"Content-Type: text/plain\r\n" +
	"\r\nHello world"

var inlineMessage = "From: sender@example.com\r\n" +
	"Content-Type: multipart/mixed; boundary=boundary\r\n" +
	"\r\n--boundary\r\n" +
	"Content-Type: text/plain\r\n" +
	"\r\njust body text\r\n" +
	"--boundary--\r\n"

var multipartMessage = "From: sender@example.com\r\n" +
	"Content-Type: multipart/mixed; boundary=boundary\r\n" +
	"\r\n--boundary\r\n" +
	"Content-Disposition: attachment; filename=\"test.txt\"\r\n" +
	"\r\nfile content\r\n" +
	"--boundary--\r\n"

var base64PdfMessage = "From: sender@example.com\r\n" +
	"Content-Type: multipart/mixed; boundary=boundary\r\n" +
	"\r\n--boundary\r\n" +
	"Content-Type: application/pdf\r\n" +
	"Content-Disposition: attachment; filename=\"test.pdf\"\r\n" +
	"Content-Transfer-Encoding: base64\r\n" +
	"\r\n" + base64.StdEncoding.EncodeToString([]byte("fake pdf content")) + "\r\n" +
	"--boundary--\r\n"

func TestAttachments(t *testing.T) {
	spec.Run(t, "Attachments", func(t *testing.T, context spec.G, it spec.S) {
		describe, beforeEach, justBeforeEach := context, it.BeforeEach, it.JustBeforeEach

		beforeEach(func() { attachmentDir = t.TempDir() }) // stub implementation

		describe("checkAttachmentDir", func() {
			justBeforeEach(checkAttachmentDir)

			context("when the path is missing", func() {
				beforeEach(func() { attachmentDir = filepath.Join(t.TempDir(), "newdir") })

				it("creates the directory", func() {
					expect(attachmentDir, t).To(BeADirectory())
				})
			})

			context("when the path is a real directory", func() {
				it("keeps the directory", func() {
					expect(attachmentDir, t).To(BeADirectory())
				})
			})

			context("when the path is a symlink", func() {
				beforeEach(func() {
					target := t.TempDir()
					attachmentDir = filepath.Join(t.TempDir(), "link")
					_ = os.Symlink(target, attachmentDir)
				})

				it("keeps the link", func() {
					expect(attachmentDir, t).To(BeADirectory())
				})
			})
		})

		// Only scans, named like processAttachments names them (^[0-9]{10}\.pdf$), are ever
		// deleted. Everything else in the folder is skipped, whatever its age.
		describe("cleanupOldFiles", func() {
			var scan, dir string
			var others []string

			beforeEach(func() {
				scan = filepath.Join(attachmentDir, "1234567890.pdf")
				_ = os.WriteFile(scan, []byte("data"), 0644)
				others = nil
				for _, name := range []string{"report.pdf", "1234567890.jpg", ".DS_Store"} {
					path := filepath.Join(attachmentDir, name)
					_ = os.WriteFile(path, []byte("data"), 0644)
					others = append(others, path)
				}
				dir = filepath.Join(attachmentDir, "subdir")
				_ = os.Mkdir(dir, 0755)
			})

			justBeforeEach(cleanupOldFiles)

			context("when the scan is recent", func() {
				it("keeps it", func() {
					expect(scan, t).To(BeAnExistingFile())
				})
			})

			context("when everything is older than maxFileAge", func() {
				beforeEach(func() {
					old := time.Now().Add(-maxFileAge - time.Hour)
					for _, path := range append([]string{scan, dir}, others...) {
						_ = os.Chtimes(path, old, old)
					}
				})

				it("deletes the scan", func() {
					expect(scan, t).NotTo(BeAnExistingFile())
				})
				it("skips files not named like a scan", func() {
					for _, path := range others {
						expect(path, t).To(BeAnExistingFile())
					}
				})
				it("skips directories", func() {
					expect(dir, t).To(BeADirectory())
				})
			})
		})

		describe("processAttachments", func() {
			var raw string
			var err error

			justBeforeEach(func() {
				msg, msgErr := mail.ReadMessage(strings.NewReader(raw))
				expect(msgErr, t).To(Succeed())
				err = processAttachments(msg)
			})

			context("when the message is not multipart", func() {
				beforeEach(func() { raw = plainMessage })

				it("returns no error", func() { expect(err, t).To(Succeed()) })
				it("saves no files", func() {
					entries, _ := os.ReadDir(attachmentDir)
					expect(len(entries), t).To(Equal(0))
				})
			})

			context("when the message has only inline parts", func() {
				beforeEach(func() { raw = inlineMessage })

				it("returns no error", func() { expect(err, t).To(Succeed()) })
				it("saves no files", func() {
					entries, _ := os.ReadDir(attachmentDir)
					expect(len(entries), t).To(Equal(0))
				})
			})

			context("when the message has an attachment", func() {
				beforeEach(func() { raw = multipartMessage })

				it("returns no error", func() { expect(err, t).To(Succeed()) })
				it("saves one file", func() {
					entries, _ := os.ReadDir(attachmentDir)
					expect(len(entries), t).To(Equal(1))
				})
				it("preserves the file extension", func() {
					entries, _ := os.ReadDir(attachmentDir)
					expect(filepath.Ext(entries[0].Name()), t).To(Equal(".txt"))
				})
				it("preserves the file content", func() {
					entries, _ := os.ReadDir(attachmentDir)
					data, _ := os.ReadFile(filepath.Join(attachmentDir, entries[0].Name()))
					expect(string(data), t).To(Equal("file content"))
				})
			})

			context("when the message has a base64-encoded PDF attachment", func() {
				beforeEach(func() { raw = base64PdfMessage })

				it("returns no error", func() { expect(err, t).To(Succeed()) })
				it("saves one file", func() {
					entries, _ := os.ReadDir(attachmentDir)
					expect(len(entries), t).To(Equal(1))
				})
				it("preserves the file extension", func() {
					entries, _ := os.ReadDir(attachmentDir)
					expect(filepath.Ext(entries[0].Name()), t).To(Equal(".pdf"))
				})
				it("decodes the base64 content correctly", func() {
					entries, _ := os.ReadDir(attachmentDir)
					data, _ := os.ReadFile(filepath.Join(attachmentDir, entries[0].Name()))
					expect(string(data), t).To(Equal("fake pdf content"))
				})
			})
		})

		describe("saveAttachment", func() {
			var path string
			var err error

			justBeforeEach(func() { err = saveAttachment(strings.NewReader("fake pdf content"), path) })

			context("when the path is valid", func() {
				beforeEach(func() { path = filepath.Join(attachmentDir, "test.pdf") })

				it("returns no error", func() { expect(err, t).To(Succeed()) })
				it("writes the content to disk", func() {
					data, readErr := os.ReadFile(path)
					expect(readErr, t).To(Succeed())
					expect(string(data), t).To(Equal("fake pdf content"))
				})
			})

			context("when the path is invalid", func() {
				beforeEach(func() { path = "/nonexistent/dir/file.pdf" })

				it("returns an error", func() { expect(err, t).To(HaveOccurred()) })
			})
		})
	})
}
