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

// get performs an in-process GET against newMux() without binding a real listener.
func get(mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// del performs an in-process DELETE against newMux(); named del, not delete, to avoid shadowing the builtin.
func del(mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// Lambada WEB exercises the HTTP routes (scanfiles.go/server.go have their own test files).
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

	// DELETE /download/{filename} is the RESTful counterpart to GET on the same route, not a separate "/delete" route.
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

		// The exact JSON shape is unit-tested in scanfiles_test.go; this just checks the route is wired up.
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

	Describe("GET /script.js", func() {
		It("serves the embedded script", func() {
			rec := get(mux, "/script.js")
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Header().Get("Content-Type")).To(ContainSubstring("javascript"))
			Expect(rec.Body.String()).To(ContainSubstring("function deleteFile"))
		})
	})
})
