package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// get performs an in-process GET against newMux() without binding a real
// listener -- mirrors how the Ruby suite uses Rack::Test against WebApp.
func get(mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// ScanFiles mirrors scandalous's ScanFiles class -- listing/toScansJSON are
// the Go port of its #listing/#scans_json, kept as their own top-level
// group, same split as the Ruby specs and the same order as main.go.
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
					URL:  "/download/1234567890.pdf",
				},
			}))
		})
	})
})

// Server mirrors main.go's Server section -- newServer is the constructor
// these tests guard, including the issue #2 regression check.
var _ = Describe("Server", func() {
	Describe("newServer", func() {
		It("sets the address and handler", func() {
			mux := newMux()
			srv := newServer("0.0.0.0:9090", mux)
			Expect(srv.Addr).To(Equal("0.0.0.0:9090"))
			Expect(srv.Handler).To(BeIdenticalTo(mux))
		})

		// Regression test for https://github.com/woodie/lambada/issues/2: a
		// zero-value http.Server (what the old http.ListenAndServe(addr,
		// handler) helper built) leaves every timeout at 0, i.e. "never" --
		// which is how leaked keep-alive connections piled up until new
		// clients couldn't connect at all. This just has to fail loudly if a
		// future edit accidentally drops back to a zero-value server.
		It("sets every timeout to a nonzero value", func() {
			srv := newServer("0.0.0.0:9090", newMux())
			Expect(srv.ReadHeaderTimeout).To(BeNumerically(">", 0))
			Expect(srv.ReadTimeout).To(BeNumerically(">", 0))
			Expect(srv.WriteTimeout).To(BeNumerically(">", 0))
			Expect(srv.IdleTimeout).To(BeNumerically(">", 0))
		})
	})
})

var _ = Describe("Lambada WEB", func() {
	var (
		mux  *http.ServeMux
		file string
	)

	BeforeEach(func() {
		scanDir = GinkgoT().TempDir() // stub implementation, mirrors lambada-mta's tests
		mux = newMux()
		file = "1234567890.pdf"
	})

	writeFile := func() {
		Expect(os.WriteFile(filepath.Join(scanDir, file), []byte("content"), 0o644)).To(Succeed())
	}

	Describe("GET /", func() {
		Context("with no files", func() {
			It("renders the empty state", func() {
				rec := get(mux, "/")
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Body.String()).To(ContainSubstring("Available Scans"))
				Expect(rec.Body.String()).To(ContainSubstring("No files found in the directory."))
			})
		})

		// Mirrors scandalous/spec/web_spec.rb's "file description" test --
		// asserts the rendered size/age text directly rather than
		// unit-testing the formatting separately.
		Context("with a file", func() {
			BeforeEach(writeFile)

			It("renders a download link with the file's size and age", func() {
				rec := get(mux, "/")
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Body.String()).To(ContainSubstring("/download/" + file))
				Expect(rec.Body.String()).To(ContainSubstring("7 B"))
				Expect(rec.Body.String()).To(ContainSubstring("less than a minute ago"))
			})
		})
	})

	Describe("GET /download/{filename}", func() {
		Context("when the file is missing", func() {
			It("responds with 404", func() {
				rec := get(mux, "/download/"+file)
				Expect(rec.Code).To(Equal(http.StatusNotFound))
			})
		})

		Context("when the file exists", func() {
			BeforeEach(writeFile)

			It("responds with 200 and an attachment header", func() {
				rec := get(mux, "/download/"+file)
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Header().Get("Content-Disposition")).To(ContainSubstring(file))
				Expect(rec.Body.String()).To(Equal("content"))
			})
		})
	})

	Describe("GET /scans.json", func() {
		Context("with no files", func() {
			It("returns an empty array", func() {
				rec := get(mux, "/scans.json")
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Header().Get("Content-Type")).To(Equal("application/json"))

				var entries []map[string]any
				Expect(json.Unmarshal(rec.Body.Bytes(), &entries)).To(Succeed())
				Expect(entries).To(BeEmpty())
			})
		})

		// The exact JSON shape (name/size/time/url) is unit-tested directly
		// against toScansJSON above -- this just checks the route is wired
		// up to it.
		Context("with a file", func() {
			BeforeEach(writeFile)

			It("returns one entry", func() {
				rec := get(mux, "/scans.json")
				Expect(rec.Code).To(Equal(http.StatusOK))

				var entries []map[string]any
				Expect(json.Unmarshal(rec.Body.Bytes(), &entries)).To(Succeed())
				Expect(entries).To(HaveLen(1))
			})
		})
	})

	Describe("GET /style.css", func() {
		It("serves the embedded stylesheet", func() {
			rec := get(mux, "/style.css")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Header().Get("Content-Type")).To(ContainSubstring("text/css"))
			Expect(rec.Body.String()).To(ContainSubstring("font-family"))
		})
	})
})
