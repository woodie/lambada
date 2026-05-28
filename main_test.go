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
