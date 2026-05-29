package main

import (
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var plainMessage = "From: sender@example.com\r\n" +
	"Content-Type: text/plain\r\n" +
	"\r\nHello world"

var multipartMessage = "From: sender@example.com\r\n" +
	"Content-Type: multipart/mixed; boundary=boundary\r\n" +
	"\r\n" +
	"--boundary\r\n" +
	"Content-Disposition: attachment; filename=\"test.txt\"\r\n" +
	"\r\nfile content\r\n" +
	"--boundary--\r\n"

var inlineMessage = "From: sender@example.com\r\n" +
	"Content-Type: multipart/mixed; boundary=boundary\r\n" +
	"\r\n" +
	"--boundary\r\n" +
	"Content-Type: text/plain\r\n" +
	"\r\njust body text\r\n" +
	"--boundary--\r\n"

var _ = Describe("Lambada", func() {

	Describe("saveAttachment", func() {
		var path string

		Context("when the path is valid", func() {
			BeforeEach(func() { path = GinkgoT().TempDir() })

			It("writes the content to disk", func() {
				dest := filepath.Join(path, "test.pdf")
				Expect(saveAttachment(strings.NewReader("fake pdf content"), dest)).To(Succeed())
				data, err := os.ReadFile(dest)
				Expect(err).To(BeNil())
				Expect(string(data)).To(Equal("fake pdf content"))
			})
		})

		Context("when the path is invalid", func() {
			It("returns an error", func() {
				Expect(saveAttachment(strings.NewReader("data"), "/nonexistent/dir/file.pdf")).To(HaveOccurred())
			})
		})
	})

	Describe("cleanupOldFiles", func() {
		var path string
		var old time.Time

		BeforeEach(func() {
			attachmentDir = GinkgoT().TempDir() // stub implementation
			old = time.Now().Add(-25 * time.Hour)
			path = filepath.Join(attachmentDir, "1234567890.pdf")
			os.WriteFile(path, []byte("data"), 0644)
		})

		Context("when a file is recent", func() {
			It("keeps the file", func() {
				cleanupOldFiles()
				Expect(path).To(BeAnExistingFile())
			})
		})

		Context("when a file is older than maxFileAge", func() {
			BeforeEach(func() { os.Chtimes(path, old, old) })

			It("deletes the file", func() {
				cleanupOldFiles()
				Expect(path).NotTo(BeAnExistingFile())
			})
		})

		Context("when an entry is a directory", func() {
			BeforeEach(func() {
				path = filepath.Join(attachmentDir, "subdir")
				os.Mkdir(path, 0755)
				os.Chtimes(path, old, old)
			})

			It("skips it", func() {
				cleanupOldFiles()
				Expect(path).To(BeADirectory())
			})
		})
	})

	Describe("processAttachments", func() {
		var destDir string
		var err error

		BeforeEach(func() { destDir = GinkgoT().TempDir() })

		processMessage := func(raw string) {
			msg, msgErr := mail.ReadMessage(strings.NewReader(raw))
			Expect(msgErr).To(BeNil())
			err = processAttachments(msg, destDir)
		}

		Context("when the message is not multipart", func() {
			BeforeEach(func() { processMessage(plainMessage) })

			It("returns no error", func() { Expect(err).To(BeNil()) })
			It("saves no files", func() {
				entries, _ := os.ReadDir(destDir)
				Expect(entries).To(BeEmpty())
			})
		})

		Context("when the message has an attachment", func() {
			BeforeEach(func() { processMessage(multipartMessage) })

			It("returns no error", func() { Expect(err).To(BeNil()) })
			It("saves one file", func() {
				entries, _ := os.ReadDir(destDir)
				Expect(entries).To(HaveLen(1))
			})
			It("preserves the file extension", func() {
				entries, _ := os.ReadDir(destDir)
				Expect(filepath.Ext(entries[0].Name())).To(Equal(".txt"))
			})
			It("preserves the file content", func() {
				entries, _ := os.ReadDir(destDir)
				data, _ := os.ReadFile(filepath.Join(destDir, entries[0].Name()))
				Expect(string(data)).To(Equal("file content"))
			})
		})

		Context("when the message has only inline parts", func() {
			BeforeEach(func() { processMessage(inlineMessage) })

			It("returns no error", func() { Expect(err).To(BeNil()) })
			It("saves no files", func() {
				entries, _ := os.ReadDir(destDir)
				Expect(entries).To(BeEmpty())
			})
		})
	})
})
