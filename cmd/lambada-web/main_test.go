package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

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

// Lambada WEB -- the HTTP routes and the page/JSON they serve (the
// scanfiles.go/server.go "work" they call into has its own test files).
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

	Describe("GET /files.json", func() {
		Context("with no files", func() {
			It("returns an empty array", func() {
				rec := get(mux, "/files.json")
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Header().Get("Content-Type")).To(Equal("application/json"))

				var entries []map[string]any
				Expect(json.Unmarshal(rec.Body.Bytes(), &entries)).To(Succeed())
				Expect(entries).To(BeEmpty())
			})
		})

		// The exact JSON shape (name/size/time/path) is unit-tested directly
		// against toScansJSON in scanfiles_test.go -- this just checks the
		// route is wired up to it.
		Context("with a file", func() {
			BeforeEach(writeFile)

			It("returns one entry", func() {
				rec := get(mux, "/files.json")
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
