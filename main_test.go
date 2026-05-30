package main

import (
	"encoding/base64"
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

var _ = Describe("Lambada", func() {

	BeforeEach(func() { attachmentDir = GinkgoT().TempDir() }) // stub implementation

	Describe("cleanupOldFiles", func() {
		var pdf, dss, dir string

		BeforeEach(func() {
			pdf = filepath.Join(attachmentDir, "1234567890.pdf")
			os.WriteFile(pdf, []byte("data"), 0644)
			dss = filepath.Join(attachmentDir, ".DS_Store")
			os.WriteFile(dss, []byte("data"), 0644)
			dir = filepath.Join(attachmentDir, "subdir")
			os.Mkdir(dir, 0755)
		})

		Context("when entries are recent", func() {
			It("keeps the PDF file", func() {
				cleanupOldFiles()
				Expect(pdf).To(BeAnExistingFile())
			})
		})

		Context("when entries are older", func() {
			BeforeEach(func() {
				old := time.Now().Add(-25 * time.Hour)
				os.Chtimes(pdf, old, old)
				os.Chtimes(dir, old, old)
				os.Chtimes(dss, old, old)
			})

			It("deletes the PDF file", func() {
				cleanupOldFiles()
				Expect(pdf).NotTo(BeAnExistingFile())
			})
			It("keeps the .DS_Store", func() {
				cleanupOldFiles()
				Expect(dss).To(BeAnExistingFile())
			})
			It("keeps the directory", func() {
				cleanupOldFiles()
				Expect(dir).To(BeADirectory())
			})
		})
	})

	Describe("processAttachments", func() {
		var err error

		processMessage := func(raw string) {
			msg, msgErr := mail.ReadMessage(strings.NewReader(raw))
			Expect(msgErr).To(BeNil())
			err = processAttachments(msg)
		}

		Context("when the message is not multipart", func() {
			BeforeEach(func() { processMessage(plainMessage) })

			It("returns no error", func() { Expect(err).To(BeNil()) })
			It("saves no files", func() {
				entries, _ := os.ReadDir(attachmentDir)
				Expect(entries).To(BeEmpty())
			})
		})

		Context("when the message has only inline parts", func() {
			BeforeEach(func() { processMessage(inlineMessage) })

			It("returns no error", func() { Expect(err).To(BeNil()) })
			It("saves no files", func() {
				entries, _ := os.ReadDir(attachmentDir)
				Expect(entries).To(BeEmpty())
			})
		})

		Context("when the message has an attachment", func() {
			BeforeEach(func() { processMessage(multipartMessage) })

			It("returns no error", func() { Expect(err).To(BeNil()) })
			It("saves one file", func() {
				entries, _ := os.ReadDir(attachmentDir)
				Expect(entries).To(HaveLen(1))
			})
			It("preserves the file extension", func() {
				entries, _ := os.ReadDir(attachmentDir)
				Expect(filepath.Ext(entries[0].Name())).To(Equal(".txt"))
			})
			It("preserves the file content", func() {
				entries, _ := os.ReadDir(attachmentDir)
				data, _ := os.ReadFile(filepath.Join(attachmentDir, entries[0].Name()))
				Expect(string(data)).To(Equal("file content"))
			})
		})

		Context("when the message has a base64-encoded PDF attachment", func() {
			BeforeEach(func() { processMessage(base64PdfMessage) })

			It("returns no error", func() { Expect(err).To(BeNil()) })
			It("saves one file", func() {
				entries, _ := os.ReadDir(attachmentDir)
				Expect(entries).To(HaveLen(1))
			})
			It("preserves the file extension", func() {
				entries, _ := os.ReadDir(attachmentDir)
				Expect(filepath.Ext(entries[0].Name())).To(Equal(".pdf"))
			})
			It("decodes the base64 content correctly", func() {
				entries, _ := os.ReadDir(attachmentDir)
				data, _ := os.ReadFile(filepath.Join(attachmentDir, entries[0].Name()))
				Expect(string(data)).To(Equal("fake pdf content"))
			})
		})
	})

	Describe("saveAttachment", func() {
		var path string

		Context("when the path is valid", func() {
			BeforeEach(func() { path = filepath.Join(attachmentDir, "test.pdf") })

			It("writes the content to disk", func() {
				Expect(saveAttachment(strings.NewReader("fake pdf content"), path)).To(Succeed())
				data, err := os.ReadFile(path)
				Expect(err).To(BeNil())
				Expect(string(data)).To(Equal("fake pdf content"))
			})
		})

		Context("when the path is invalid", func() {
			BeforeEach(func() { path = "/nonexistent/dir/file.pdf" })

			It("returns an error", func() {
				Expect(saveAttachment(strings.NewReader("data"), path)).To(HaveOccurred())
			})
		})
	})
})
