package main

import (
	"encoding/base64"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sclevine/spec"
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
	spec.Run(t, "Attachments", func(t *testing.T, describe spec.G, it spec.S) {
		context, before := describe, it.Before

		before(func() { attachmentDir = t.TempDir() }) // stub implementation

		describe("checkAttachmentDir", func() {
			context("when the path is missing", func() {
				it("creates the directory", func() {
					attachmentDir = filepath.Join(t.TempDir(), "newdir")
					checkAttachmentDir()
					expect(attachmentDir, t).To(BeADirectory())
				})
			})

			context("when the path is a real directory", func() {
				it("does not error", func() {
					attachmentDir = t.TempDir()
					expect(func() { checkAttachmentDir() }, t).NotTo(Panic())
				})
			})

			context("when the path is a symlink", func() {
				it("does not error", func() {
					target := t.TempDir()
					attachmentDir = filepath.Join(t.TempDir(), "link")
					_ = os.Symlink(target, attachmentDir)
					expect(func() { checkAttachmentDir() }, t).NotTo(Panic())
				})
			})
		})

		describe("cleanupOldFiles", func() {
			var pdf, dss, dir string

			before(func() {
				pdf = filepath.Join(attachmentDir, "1234567890.pdf")
				_ = os.WriteFile(pdf, []byte("data"), 0644)
				dss = filepath.Join(attachmentDir, ".DS_Store")
				_ = os.WriteFile(dss, []byte("data"), 0644)
				dir = filepath.Join(attachmentDir, "subdir")
				_ = os.Mkdir(dir, 0755)
			})

			context("when entries are recent", func() {
				it("keeps the PDF file", func() {
					cleanupOldFiles()
					expect(pdf, t).To(BeAnExistingFile())
				})
			})

			context("when entries are older", func() {
				before(func() {
					old := time.Now().Add(-25 * time.Hour)
					_ = os.Chtimes(pdf, old, old)
					_ = os.Chtimes(dir, old, old)
					_ = os.Chtimes(dss, old, old)
				})

				it("deletes the PDF file", func() {
					cleanupOldFiles()
					expect(pdf, t).NotTo(BeAnExistingFile())
				})
				it("keeps the .DS_Store", func() {
					cleanupOldFiles()
					expect(dss, t).To(BeAnExistingFile())
				})
				it("keeps the directory", func() {
					cleanupOldFiles()
					expect(dir, t).To(BeADirectory())
				})
			})
		})

		describe("processAttachments", func() {
			var err error

			processMessage := func(raw string) {
				msg, msgErr := mail.ReadMessage(strings.NewReader(raw))
				expect(msgErr, t).To(Succeed())
				err = processAttachments(msg)
			}

			context("when the message is not multipart", func() {
				before(func() { processMessage(plainMessage) })

				it("returns no error", func() { expect(err, t).To(Succeed()) })
				it("saves no files", func() {
					entries, _ := os.ReadDir(attachmentDir)
					expect(len(entries), t).To(Equal(0))
				})
			})

			context("when the message has only inline parts", func() {
				before(func() { processMessage(inlineMessage) })

				it("returns no error", func() { expect(err, t).To(Succeed()) })
				it("saves no files", func() {
					entries, _ := os.ReadDir(attachmentDir)
					expect(len(entries), t).To(Equal(0))
				})
			})

			context("when the message has an attachment", func() {
				before(func() { processMessage(multipartMessage) })

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
				before(func() { processMessage(base64PdfMessage) })

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

			context("when the path is valid", func() {
				before(func() { path = filepath.Join(attachmentDir, "test.pdf") })

				it("writes the content to disk", func() {
					expect(saveAttachment(strings.NewReader("fake pdf content"), path), t).To(Succeed())
					data, err := os.ReadFile(path)
					expect(err, t).To(Succeed())
					expect(string(data), t).To(Equal("fake pdf content"))
				})
			})

			context("when the path is invalid", func() {
				before(func() { path = "/nonexistent/dir/file.pdf" })

				it("returns an error", func() {
					expect(saveAttachment(strings.NewReader("data"), path), t).To(HaveOccurred())
				})
			})
		})
	})
}
