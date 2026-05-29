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

var _ = Describe("Lambada", func() {

	Describe("processAttachments", func() {
		var (
			destDir string
			err     error
		)

		BeforeEach(func() {
			destDir = GinkgoT().TempDir()
		})

		processMessage := func(raw string) {
			msg, msgErr := mail.ReadMessage(strings.NewReader(raw))
			Expect(msgErr).To(BeNil())
			err = processAttachments(msg, destDir)
		}

		Context("when the message is not multipart", func() {
			BeforeEach(func() {
				processMessage("From: sender@example.com\r\n" +
					"Content-Type: text/plain\r\n" +
					"\r\nHello world")
			})

			It("returns no error", func() { Expect(err).To(BeNil()) })
			It("saves no files", func() {
				entries, _ := os.ReadDir(destDir)
				Expect(entries).To(BeEmpty())
			})
		})

		Context("when the message has an attachment", func() {
			BeforeEach(func() {
				processMessage("From: sender@example.com\r\n" +
					"Content-Type: multipart/mixed; boundary=boundary\r\n" +
					"\r\n" +
					"--boundary\r\n" +
					"Content-Disposition: attachment; filename=\"test.txt\"\r\n" +
					"\r\nfile content\r\n" +
					"--boundary--\r\n")
			})

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
			BeforeEach(func() {
				processMessage("From: sender@example.com\r\n" +
					"Content-Type: multipart/mixed; boundary=boundary\r\n" +
					"\r\n" +
					"--boundary\r\n" +
					"Content-Type: text/plain\r\n" +
					"\r\njust body text\r\n" +
					"--boundary--\r\n")
			})

			It("returns no error", func() { Expect(err).To(BeNil()) })
			It("saves no files", func() {
				entries, _ := os.ReadDir(destDir)
				Expect(entries).To(BeEmpty())
			})
		})
	})

	Describe("saveAttachment", func() {
		var path string

		Context("with a valid path", func() {
			BeforeEach(func() { path = filepath.Join(GinkgoT().TempDir(), "1234567890.pdf") })

			It("writes the content to disk", func() {
				Expect(saveAttachment(strings.NewReader("content"), path)).To(Succeed())
				data, err := os.ReadFile(path)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(Equal("content"))
			})
		})

		Context("with an invalid path", func() {
			BeforeEach(func() { path = "/nonexistent/dir/file.pdf" })

			It("returns an error", func() {
				err := saveAttachment(strings.NewReader("content"), path)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("cleanupOldFiles", func() {
		Context("with files", func() {
			var pdf string
			var dss string

			BeforeEach(func() {
				attachmentDir = GinkgoT().TempDir() // stub implementation
				pdf = filepath.Join(attachmentDir, "1234567890.pdf")
				os.WriteFile(pdf, []byte("data"), 0644)
				dss = filepath.Join(attachmentDir, ".DS_Store")
				os.WriteFile(dss, []byte("data"), 0644)
			})

			Context("created right now", func() {
				It("keeps both PDF and .DS_Store", func() {
					cleanupOldFiles()
					Expect(pdf).To(BeAnExistingFile())
					Expect(dss).To(BeAnExistingFile())
				})
			})

			Context("created yesterday", func() {
				BeforeEach(func() {
					old := time.Now().Add(-25 * time.Hour)
					os.Chtimes(pdf, old, old)
					os.Chtimes(dss, old, old)
				})

				It("deletes PDF but keeps .DS_Store", func() {
					cleanupOldFiles()
					Expect(pdf).NotTo(BeAnExistingFile())
					Expect(dss).To(BeAnExistingFile())
				})
			})
		})
	})
})
