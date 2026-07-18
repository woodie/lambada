package main

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ScanFiles exercises scanFilesListing/toScansJSON, the Go port of Ruby's ScanFiles#listing/#scans_json.
var _ = Describe("ScanFiles", func() {
	Describe("scanFilesListing", func() {
		var dir string

		BeforeEach(func() {
			dir = GinkgoT().TempDir()
		})

		It("returns an empty slice for an empty directory", func() {
			scans, err := scanFilesListing(dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(scans).To(BeEmpty())
		})

		It("returns one entry per *.pdf file, ignoring other extensions", func() {
			Expect(os.WriteFile(filepath.Join(dir, "1234567890.pdf"), []byte("content"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0o644)).To(Succeed())

			scans, err := scanFilesListing(dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(scans).To(HaveLen(1))
			Expect(scans[0].Name).To(Equal("1234567890.pdf"))
			Expect(scans[0].Size).To(Equal(int64(7)))
		})

		It("sorts newest filename first", func() {
			Expect(os.WriteFile(filepath.Join(dir, "1000000000.pdf"), []byte("a"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "2000000000.pdf"), []byte("b"), 0o644)).To(Succeed())

			scans, err := scanFilesListing(dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(scans).To(HaveLen(2))
			Expect(scans[0].Name).To(Equal("2000000000.pdf"))
			Expect(scans[1].Name).To(Equal("1000000000.pdf"))
		})

		It("returns an error for a malformed glob pattern", func() {
			// "[" is an unterminated bracket expression -- filepath.Glob's only error case (ErrBadPattern).
			_, err := scanFilesListing(filepath.Join(dir, "["))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("scanFilesPath", func() {
		BeforeEach(func() {
			scanDir = GinkgoT().TempDir()
		})

		It("resolves an existing file", func() {
			Expect(os.WriteFile(filepath.Join(scanDir, "1234567890.pdf"), []byte("content"), 0o644)).To(Succeed())

			path, err := scanFilesPath("1234567890.pdf")
			Expect(err).NotTo(HaveOccurred())
			Expect(path).To(Equal(filepath.Join(scanDir, "1234567890.pdf")))
		})

		It("returns os.ErrNotExist for a missing file", func() {
			_, err := scanFilesPath("missing.pdf")
			Expect(errors.Is(err, os.ErrNotExist)).To(BeTrue())
		})

		It("returns os.ErrNotExist for a directory", func() {
			Expect(os.Mkdir(filepath.Join(scanDir, "subdir"), 0o755)).To(Succeed())

			_, err := scanFilesPath("subdir")
			Expect(errors.Is(err, os.ErrNotExist)).To(BeTrue())
		})

		DescribeTable("returns os.ErrNotExist for an invalid or unresolvable filename",
			func(filename string) {
				_, err := scanFilesPath(filename)
				Expect(errors.Is(err, os.ErrNotExist)).To(BeTrue())
			},
			Entry("empty", ""),
			Entry("current dir", "."),
			Entry("parent dir", ".."),
			// filepath.Base already strips any directory component before scanFilesPath sees it, so this just resolves to a nonexistent base name in scanDir.
			Entry("path with a directory prefix", "sub/1234567890.pdf"),
		)

		It("returns a non-ErrNotExist error when the directory can't be searched", func() {
			if os.Geteuid() == 0 {
				Skip("running as root; permission checks don't apply")
			}
			Expect(os.WriteFile(filepath.Join(scanDir, "1234567890.pdf"), []byte("content"), 0o644)).To(Succeed())
			Expect(os.Chmod(scanDir, 0o000)).To(Succeed())
			defer func() {
				Expect(os.Chmod(scanDir, 0o755)).To(Succeed())
			}()

			_, err := scanFilesPath("1234567890.pdf")
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, os.ErrNotExist)).To(BeFalse())
		})
	})

	Describe("toScansJSON", func() {
		It("returns an empty slice for no scans", func() {
			Expect(toScansJSON(nil)).To(BeEmpty())
		})

		It("maps each scan to its API shape", func() {
			when := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
			scans := []scan{{Name: "1234567890.pdf", Time: when, Size: 7}}

			Expect(toScansJSON(scans)).To(Equal([]scanJSON{
				{
					Name: "1234567890.pdf",
					Size: 7,
					Time: when.Format(time.RFC3339),
					Path: "/download/1234567890.pdf",
				},
			}))
		})
	})
})
