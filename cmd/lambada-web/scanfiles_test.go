package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sclevine/spec"
	"github.com/woodie/expect"
)

// TestScanFiles exercises scanFilesListing/toScansJSON, the Go port of Ruby's ScanFiles#listing/#scans_json.
func TestScanFiles(t *testing.T) {
	spec.Run(t, "ScanFiles", func(t *testing.T, describe spec.Describe, it spec.S) {
		describe("scanFilesListing", func() {
			var dir string
			it.Before(func() { dir = it.T().TempDir() })

			it("returns an empty slice for an empty directory", func() {
				scans, err := scanFilesListing(dir)
				expect.That(t, err).To(expect.Succeed())
				expect.That(t, len(scans)).To(expect.Equal(0))
			})

			it("returns one entry per *.pdf file, ignoring other extensions", func() {
				expect.That(t, os.WriteFile(filepath.Join(dir, "1234567890.pdf"), []byte("content"), 0o644)).To(expect.Succeed())
				expect.That(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0o644)).To(expect.Succeed())

				scans, err := scanFilesListing(dir)
				expect.That(t, err).To(expect.Succeed())
				expect.That(t, len(scans)).To(expect.Equal(1))
				expect.That(t, scans[0].Name).To(expect.Equal("1234567890.pdf"))
				expect.That(t, scans[0].Size).To(expect.Equal(int64(7)))
			})

			it("sorts newest filename first", func() {
				expect.That(t, os.WriteFile(filepath.Join(dir, "1000000000.pdf"), []byte("a"), 0o644)).To(expect.Succeed())
				expect.That(t, os.WriteFile(filepath.Join(dir, "2000000000.pdf"), []byte("b"), 0o644)).To(expect.Succeed())

				scans, err := scanFilesListing(dir)
				expect.That(t, err).To(expect.Succeed())
				expect.That(t, len(scans)).To(expect.Equal(2))
				expect.That(t, scans[0].Name).To(expect.Equal("2000000000.pdf"))
				expect.That(t, scans[1].Name).To(expect.Equal("1000000000.pdf"))
			})

			it("returns an error for a malformed glob pattern", func() {
				// "[" is an unterminated bracket expression -- filepath.Glob's only error case (ErrBadPattern).
				_, err := scanFilesListing(filepath.Join(dir, "["))
				expect.That(t, err).To(expect.HaveOccurred())
			})
		})

		describe("scanFilesPath", func() {
			it.Before(func() { scanDir = it.T().TempDir() })

			it("resolves an existing file", func() {
				expect.That(t, os.WriteFile(filepath.Join(scanDir, "1234567890.pdf"), []byte("content"), 0o644)).To(expect.Succeed())

				path, err := scanFilesPath("1234567890.pdf")
				expect.That(t, err).To(expect.Succeed())
				expect.That(t, path).To(expect.Equal(filepath.Join(scanDir, "1234567890.pdf")))
			})

			it("returns os.ErrNotExist for a missing file", func() {
				_, err := scanFilesPath("missing.pdf")
				expect.That(t, errors.Is(err, os.ErrNotExist)).To(expect.BeTrue())
			})

			it("returns os.ErrNotExist for a directory", func() {
				expect.That(t, os.Mkdir(filepath.Join(scanDir, "subdir"), 0o755)).To(expect.Succeed())

				_, err := scanFilesPath("subdir")
				expect.That(t, errors.Is(err, os.ErrNotExist)).To(expect.BeTrue())
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
						expect.That(t, errors.Is(err, os.ErrNotExist)).To(expect.BeTrue())
					})
				}
			})

			it("returns a non-ErrNotExist error when the directory can't be searched", func() {
				if os.Geteuid() == 0 {
					it.T().Skip("running as root; permission checks don't apply")
				}
				expect.That(t, os.WriteFile(filepath.Join(scanDir, "1234567890.pdf"), []byte("content"), 0o644)).To(expect.Succeed())
				expect.That(t, os.Chmod(scanDir, 0o000)).To(expect.Succeed())
				defer func() {
					expect.That(t, os.Chmod(scanDir, 0o755)).To(expect.Succeed())
				}()

				_, err := scanFilesPath("1234567890.pdf")
				expect.That(t, err).To(expect.HaveOccurred())
				expect.That(t, errors.Is(err, os.ErrNotExist)).To(expect.BeFalse())
			})
		})

		describe("toScansJSON", func() {
			it("returns an empty slice for no scans", func() {
				expect.That(t, len(toScansJSON(nil))).To(expect.Equal(0))
			})

			it("maps each scan to its API shape", func() {
				when := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
				scans := []scan{{Name: "1234567890.pdf", Time: when, Size: 7}}

				expect.That(t, toScansJSON(scans)).To(expect.DeepEqual([]scanJSON{
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
