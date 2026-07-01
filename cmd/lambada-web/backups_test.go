package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Backups mirrors scanfiles_test.go's shape -- listBackups is the "work"
// backup.go does, kept in its own file/test file the same way scanfiles.go
// is, so main_test.go's HTTP route tests stay about wiring, not listing
// logic.
var _ = Describe("Backups", func() {
	Describe("listBackups", func() {
		var dir string

		BeforeEach(func() {
			dir = GinkgoT().TempDir()
		})

		It("returns an empty slice for an empty directory", func() {
			files, err := listBackups(dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(BeEmpty())
		})

		It("returns one entry per file, regardless of extension", func() {
			Expect(os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("content"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "archive.tar.gz"), []byte("gz"), 0o644)).To(Succeed())

			files, err := listBackups(dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveLen(2))
		})

		It("skips subdirectories", func() {
			Expect(os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("content"), 0o644)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(dir, "subdir"), 0o755)).To(Succeed())

			files, err := listBackups(dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveLen(1))
			Expect(files[0].Name).To(Equal("notes.txt"))
			Expect(files[0].Size).To(Equal(int64(7)))
		})

		It("sorts alphabetically by name", func() {
			Expect(os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)).To(Succeed())

			files, err := listBackups(dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveLen(2))
			Expect(files[0].Name).To(Equal("a.txt"))
			Expect(files[1].Name).To(Equal("b.txt"))
		})
	})
})
