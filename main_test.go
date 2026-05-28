package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Lambada", func() {

	Describe("saveAttachment", func() {
		Context("with a valid path", func() {
			It("writes the content to disk", func() {
				dir := GinkgoT().TempDir()
				dest := filepath.Join(dir, "test.pdf")
				content := "fake pdf content"

				Expect(saveAttachment(strings.NewReader(content), dest)).To(Succeed())

				data, err := os.ReadFile(dest)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(Equal(content))
			})
		})

		Context("with an invalid path", func() {
			It("returns an error", func() {
				err := saveAttachment(strings.NewReader("data"), "/nonexistent/dir/file.pdf")
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("cleanupOldFiles", func() {
		var dir string

		BeforeEach(func() {
			dir = GinkgoT().TempDir()
			attachmentDir = dir
		})

		Context("with a file", func() {
			var path string

			BeforeEach(func() {
				path = filepath.Join(dir, "1234567890.pdf")
				os.WriteFile(path, []byte("data"), 0644)
			})

			Context("created right now", func() {
				It("keeps the file", func() {
					cleanupOldFiles()
					Expect(path).To(BeAnExistingFile())
				})
			})

			Context("created yesterday", func() {
				var old time.Time

				BeforeEach(func() {
					old = time.Now().Add(-25 * time.Hour)
					os.Chtimes(path, old, old)
				})

				It("deletes the file", func() {
					cleanupOldFiles()
					Expect(path).NotTo(BeAnExistingFile())
				})

				Context("named .DS_Store", func() {
					BeforeEach(func() {
						path = filepath.Join(dir, ".DS_Store")
						os.WriteFile(path, []byte("data"), 0644)
						os.Chtimes(path, old, old)
					})

					It("keeps the file", func() {
						cleanupOldFiles()
						Expect(path).To(BeAnExistingFile())
					})
				})

			})
		})
	})
})
