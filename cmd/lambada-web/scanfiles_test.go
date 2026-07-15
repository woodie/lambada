package main

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ScanFiles exercises listing/toScansJSON, the Go port of Ruby's ScanFiles#listing/#scans_json.
var _ = Describe("ScanFiles", func() {
	Describe("listing", func() {
		var dir string

		BeforeEach(func() {
			dir = GinkgoT().TempDir()
		})

		It("returns an empty slice for an empty directory", func() {
			scans, err := listing(dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(scans).To(BeEmpty())
		})

		It("returns one entry per *.pdf file, ignoring other extensions", func() {
			Expect(os.WriteFile(filepath.Join(dir, "1234567890.pdf"), []byte("content"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0o644)).To(Succeed())

			scans, err := listing(dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(scans).To(HaveLen(1))
			Expect(scans[0].Name).To(Equal("1234567890.pdf"))
			Expect(scans[0].Size).To(Equal(int64(7)))
		})

		It("sorts newest filename first", func() {
			Expect(os.WriteFile(filepath.Join(dir, "1000000000.pdf"), []byte("a"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "2000000000.pdf"), []byte("b"), 0o644)).To(Succeed())

			scans, err := listing(dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(scans).To(HaveLen(2))
			Expect(scans[0].Name).To(Equal("2000000000.pdf"))
			Expect(scans[1].Name).To(Equal("1000000000.pdf"))
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
