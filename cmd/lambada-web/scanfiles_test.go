package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sclevine/spec"
	. "github.com/woodie/expect"
)

// TestScanFiles exercises scanFilesListing/toScansJSON, the Go port of Ruby's ScanFiles#listing/#scans_json.
func TestScanFiles(t *testing.T) {
	spec.Run(t, "ScanFiles", func(t *testing.T, describe spec.G, it spec.S) {
		before := it.Before

		describe("scanFilesListing", func() {
			var dir string
			before(func() { dir = t.TempDir() })

			it("returns an empty slice for an empty directory", func() {
				scans, err := scanFilesListing(dir)
				expect(err, t).To(Succeed())
				expect(len(scans), t).To(Equal(0))
			})

			it("returns one entry per *.pdf file, ignoring other extensions", func() {
				expect(os.WriteFile(filepath.Join(dir, "1234567890.pdf"), []byte("content"), 0o644), t).To(Succeed())
				expect(os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0o644), t).To(Succeed())

				scans, err := scanFilesListing(dir)
				expect(err, t).To(Succeed())
				expect(len(scans), t).To(Equal(1))
				expect(scans[0].Name, t).To(Equal("1234567890.pdf"))
				expect(scans[0].Size, t).To(Equal(int64(7)))
			})

			it("sorts newest filename first", func() {
				expect(os.WriteFile(filepath.Join(dir, "1000000000.pdf"), []byte("a"), 0o644), t).To(Succeed())
				expect(os.WriteFile(filepath.Join(dir, "2000000000.pdf"), []byte("b"), 0o644), t).To(Succeed())

				scans, err := scanFilesListing(dir)
				expect(err, t).To(Succeed())
				expect(len(scans), t).To(Equal(2))
				expect(scans[0].Name, t).To(Equal("2000000000.pdf"))
				expect(scans[1].Name, t).To(Equal("1000000000.pdf"))
			})

			it("returns an error for a malformed glob pattern", func() {
				// "[" is an unterminated bracket expression -- filepath.Glob's only error case (ErrBadPattern).
				_, err := scanFilesListing(filepath.Join(dir, "["))
				expect(err, t).To(HaveOccurred())
			})
		})

		describe("scanFilesPath", func() {
			before(func() { scanDir = t.TempDir() }) // stub implementation

			it("resolves an existing file", func() {
				expect(os.WriteFile(filepath.Join(scanDir, "1234567890.pdf"), []byte("content"), 0o644), t).To(Succeed())

				path, err := scanFilesPath("1234567890.pdf")
				expect(err, t).To(Succeed())
				expect(path, t).To(Equal(filepath.Join(scanDir, "1234567890.pdf")))
			})

			it("returns os.ErrNotExist for a missing file", func() {
				_, err := scanFilesPath("missing.pdf")
				expect(errors.Is(err, os.ErrNotExist), t).To(BeTrue())
			})

			it("returns os.ErrNotExist for a directory", func() {
				expect(os.Mkdir(filepath.Join(scanDir, "subdir"), 0o755), t).To(Succeed())

				_, err := scanFilesPath("subdir")
				expect(errors.Is(err, os.ErrNotExist), t).To(BeTrue())
			})

			describe("returns os.ErrNotExist for an invalid or unresolvable filename", func() {
				cases := []struct{ name, filename string }{
					{"empty", ""},
					{"current dir", "."},
					{"parent dir", ".."},
					// filepath.Base already strips any directory component before scanFilesPath sees it, so this just resolves to a nonexistent base name in scanDir.
					{"path with a directory prefix", "sub/1234567890.pdf"},
				}
				for _, tc := range cases {
					it(tc.name, func() {
						_, err := scanFilesPath(tc.filename)
						expect(errors.Is(err, os.ErrNotExist), t).To(BeTrue())
					})
				}
			})

			it("returns a non-ErrNotExist error when the directory can't be searched", func() {
				if os.Geteuid() == 0 {
					t.Skip("running as root; permission checks don't apply")
				}
				expect(os.WriteFile(filepath.Join(scanDir, "1234567890.pdf"), []byte("content"), 0o644), t).To(Succeed())
				expect(os.Chmod(scanDir, 0o000), t).To(Succeed())
				defer func() {
					expect(os.Chmod(scanDir, 0o755), t).To(Succeed())
				}()

				_, err := scanFilesPath("1234567890.pdf")
				expect(err, t).To(HaveOccurred())
				expect(errors.Is(err, os.ErrNotExist), t).To(BeFalse())
			})
		})

		describe("toScansJSON", func() {
			it("returns an empty slice for no scans", func() {
				expect(len(toScansJSON(nil)), t).To(Equal(0))
			})

			it("maps each scan to its API shape", func() {
				when := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
				scans := []scan{{Name: "1234567890.pdf", Time: when, Size: 7}}

				expect(toScansJSON(scans), t).To(DeepEqual([]scanJSON{
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
}
