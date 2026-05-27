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

		Context("with a file created right now", func() {
			It("keeps the file", func() {
				path := filepath.Join(dir, "1234567890.pdf")
				Expect(os.WriteFile(path, []byte("data"), 0644)).To(Succeed())

				cleanupOldFiles()

				Expect(path).To(BeAnExistingFile())
			})
		})

		Context("with a file created yesterday", func() {
			It("deletes the file", func() {
				path := filepath.Join(dir, "1234567890.pdf")
				Expect(os.WriteFile(path, []byte("data"), 0644)).To(Succeed())

				old := time.Now().Add(-25 * time.Hour)
				Expect(os.Chtimes(path, old, old)).To(Succeed())

				cleanupOldFiles()

				Expect(path).NotTo(BeAnExistingFile())
			})
		})

		Context("with a subdirectory", func() {
			It("skips directories", func() {
				subdir := filepath.Join(dir, "subdir")
				Expect(os.Mkdir(subdir, 0755)).To(Succeed())

				old := time.Now().Add(-25 * time.Hour)
				Expect(os.Chtimes(subdir, old, old)).To(Succeed())

				cleanupOldFiles()

				Expect(subdir).To(BeADirectory())
			})
		})
	})
})
