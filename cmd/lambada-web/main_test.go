package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"
	"strings"

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

// del performs an in-process DELETE against newMux() without binding a
// real listener. Named del, not delete, so it doesn't shadow the builtin.
func del(mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, path, nil)
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
	  content := strings.Repeat("content.", 9999)
		Expect(os.WriteFile(filepath.Join(scanDir, file), []byte(content), 0o644)).To(Succeed())
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

		Context("with a file", func() {
			BeforeEach(writeFile)

			It("renders a download link with the file's size and age", func() {
				rec := get(mux, "/")
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Body.String()).To(ContainSubstring("/download/" + file))
				Expect(rec.Body.String()).To(ContainSubstring("📄 80 KB"))
			})

			Context("when files can be older", func() {
				setFileAge := func(age time.Duration) {
					when := time.Now().Add(-age)
					Expect(os.Chtimes(filepath.Join(scanDir, file), when, when)).To(Succeed())
				}

				Context("just now", func() {
					BeforeEach(func() { setFileAge(0) })

					It("displays less than a minute ago", func() {
						rec := get(mux, "/")
						Expect(rec.Body.String()).To(ContainSubstring("less than a minute ago"))
					})
				})

				Context("three minutes ago", func() {
					BeforeEach(func() { setFileAge(3 * time.Minute) })

					It("displays 3 minutes ago", func() {
						rec := get(mux, "/")
						Expect(rec.Body.String()).To(ContainSubstring("3 minutes ago"))
					})
				})

				Context("fifteen hours ago", func() {
					BeforeEach(func() { setFileAge(15 * time.Hour) })

					It("displays 15 hours ago", func() {
						rec := get(mux, "/")
						Expect(rec.Body.String()).To(ContainSubstring("15 hours ago"))
					})
				})

				Context("thirty hours ago", func() {
					BeforeEach(func() { setFileAge(30 * time.Hour) })

					It("displays 1 day ago", func() {
						rec := get(mux, "/")
						Expect(rec.Body.String()).To(ContainSubstring("1 day ago"))
					})
				})
			})

			Context("when files can be newer", func() {
				BeforeEach(func() {
					when := time.Now().Add(3 * time.Minute)
					Expect(os.Chtimes(filepath.Join(scanDir, file), when, when)).To(Succeed())
				})

				It("displays in 3 minutes", func() {
					rec := get(mux, "/")
					Expect(rec.Body.String()).To(ContainSubstring("in 3 minutes"))
					Expect(rec.Body.String()).NotTo(ContainSubstring("ago"))
				})
			})

			It("wires the delete confirm dialog with the full message", func() {
				rec := get(mux, "/")
				Expect(rec.Body.String()).To(ContainSubstring("Delete this scan from less than a minute ago?"))
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
			})
		})
	})

	// The RESTful DELETE counterpart to GET /download/{filename} -- same
	// resource path, different verb, rather than a separate
	// "/delete/{filename}" route. See handleDelete in main.go.
	Describe("DELETE /download/{filename}", func() {
		Context("when the file is missing", func() {
			It("responds with 404", func() {
				rec := del(mux, "/download/"+file)
				Expect(rec.Code).To(Equal(http.StatusNotFound))
			})
		})

		Context("when the file exists", func() {
			BeforeEach(writeFile)

			It("responds with 204 and removes the file", func() {
				rec := del(mux, "/download/"+file)
				Expect(rec.Code).To(Equal(http.StatusNoContent))
				Expect(filepath.Join(scanDir, file)).NotTo(BeAnExistingFile())
			})

			It("leaves the file gone for a subsequent GET", func() {
				del(mux, "/download/"+file)
				rec := get(mux, "/download/"+file)
				Expect(rec.Code).To(Equal(http.StatusNotFound))
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
