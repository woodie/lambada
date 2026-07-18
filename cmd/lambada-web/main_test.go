package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sclevine/spec"
	. "github.com/woodie/expect"
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

// TestLambadaWeb exercises the HTTP routes (scanfiles_test.go/server_test.go have their own test files).
func TestLambadaWeb(t *testing.T) {
	spec.RunAliased(t, "Lambada WEB", func(t *testing.T, describe, context spec.Describe, it spec.S, before, after func(func())) {
		var mux *http.ServeMux
		var file string

		before(func() {
			scanDir = it.T().TempDir()
			mux = newMux()
			file = "1234567890.pdf"
		})

		writeFile := func() {
			content := strings.Repeat("content.", 9999)
			Expect(t, os.WriteFile(filepath.Join(scanDir, file), []byte(content), 0o644)).To(Succeed())
		}

		describe("GET index", func() {
			context("with no files", func() {
				it("renders the empty state", func() {
					rec := get(mux, "/")
					Expect(t, rec.Code).To(Equal(http.StatusOK))
					Expect(t, rec.Body.String()).To(Contain("Available Scans"))
					Expect(t, rec.Body.String()).To(Contain("No files found in the directory."))
				})
			})

			context("with a file", func() {
				before(writeFile)

				it("renders a download link with the file's size and age", func() {
					rec := get(mux, "/")
					Expect(t, rec.Code).To(Equal(http.StatusOK))
					Expect(t, rec.Body.String()).To(Contain("/download/" + file))
					Expect(t, rec.Body.String()).To(Contain("📄 80 KB"))
				})

				context("when files can be older", func() {
					setFileAge := func(age time.Duration) {
						when := time.Now().Add(-age)
						Expect(t, os.Chtimes(filepath.Join(scanDir, file), when, when)).To(Succeed())
					}

					context("just now", func() {
						before(func() { setFileAge(0) })

						it("displays less than a minute ago", func() {
							rec := get(mux, "/")
							Expect(t, rec.Body.String()).To(Contain("less than a minute ago"))
						})
					})
				})

				context("when files can be newer", func() {
					before(func() {
						when := time.Now().Add(3 * time.Minute)
						Expect(t, os.Chtimes(filepath.Join(scanDir, file), when, when)).To(Succeed())
					})

					it("displays in 3 minutes", func() {
						rec := get(mux, "/")
						Expect(t, rec.Body.String()).To(Contain("in 3 minutes"))
						Expect(t, rec.Body.String()).NotTo(Contain("ago"))
					})
				})

				it("wires the delete confirm dialog with the full message", func() {
					rec := get(mux, "/")
					Expect(t, rec.Body.String()).To(Contain("Delete this scan from less than a minute ago?"))
				})
			})
		})

		describe("GET download by filename", func() {
			context("when the file is missing", func() {
				it("responds with 404", func() {
					rec := get(mux, "/download/"+file)
					Expect(t, rec.Code).To(Equal(http.StatusNotFound))
				})
			})

			context("when the file exists", func() {
				before(writeFile)

				it("responds with 200 and an attachment header", func() {
					rec := get(mux, "/download/"+file)
					Expect(t, rec.Code).To(Equal(http.StatusOK))
					Expect(t, rec.Header().Get("Content-Disposition")).To(Contain(file))
				})
			})

			context("when the directory can't be searched (permission error)", func() {
				before(func() {
					if os.Geteuid() == 0 {
						it.T().Skip("running as root; permission checks don't apply")
					}
					writeFile()
					Expect(t, os.Chmod(scanDir, 0o000)).To(Succeed())
				})
				after(func() {
					Expect(t, os.Chmod(scanDir, 0o755)).To(Succeed())
				})

				it("responds with 500", func() {
					rec := get(mux, "/download/"+file)
					Expect(t, rec.Code).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		// DELETE /download/{filename} is the RESTful counterpart to GET on the same route, not a separate "/delete" route.
		describe("DELETE download by filename", func() {
			context("when the file is missing", func() {
				it("responds with 404", func() {
					rec := del(mux, "/download/"+file)
					Expect(t, rec.Code).To(Equal(http.StatusNotFound))
				})
			})

			context("when the file exists", func() {
				before(writeFile)

				it("responds with 204 and removes the file", func() {
					rec := del(mux, "/download/"+file)
					Expect(t, rec.Code).To(Equal(http.StatusNoContent))
					Expect(t, filepath.Join(scanDir, file)).NotTo(BeAnExistingFile())
				})

				it("leaves the file gone for a subsequent GET", func() {
					del(mux, "/download/"+file)
					rec := get(mux, "/download/"+file)
					Expect(t, rec.Code).To(Equal(http.StatusNotFound))
				})
			})

			context("when the directory can't be searched (permission error)", func() {
				before(func() {
					if os.Geteuid() == 0 {
						it.T().Skip("running as root; permission checks don't apply")
					}
					writeFile()
					Expect(t, os.Chmod(scanDir, 0o000)).To(Succeed())
				})
				after(func() {
					Expect(t, os.Chmod(scanDir, 0o755)).To(Succeed())
				})

				it("responds with 500", func() {
					rec := del(mux, "/download/"+file)
					Expect(t, rec.Code).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		describe("GET files.json", func() {
			context("with no files", func() {
				it("returns an empty array", func() {
					rec := get(mux, "/files.json")
					Expect(t, rec.Code).To(Equal(http.StatusOK))
					Expect(t, rec.Header().Get("Content-Type")).To(Equal("application/json"))

					var entries []map[string]any
					Expect(t, json.Unmarshal(rec.Body.Bytes(), &entries)).To(Succeed())
					Expect(t, len(entries)).To(Equal(0))
				})
			})

			// The exact JSON shape is unit-tested in scanfiles_test.go; this just checks the route is wired up.
			context("with a file", func() {
				before(writeFile)

				it("returns one entry", func() {
					rec := get(mux, "/files.json")
					Expect(t, rec.Code).To(Equal(http.StatusOK))

					var entries []map[string]any
					Expect(t, json.Unmarshal(rec.Body.Bytes(), &entries)).To(Succeed())
					Expect(t, len(entries)).To(Equal(1))
				})
			})
		})

		describe("GET style.css", func() {
			it("serves the embedded stylesheet", func() {
				rec := get(mux, "/style.css")
				Expect(t, rec.Code).To(Equal(http.StatusOK))
				Expect(t, rec.Header().Get("Content-Type")).To(Contain("text/css"))
				Expect(t, rec.Body.String()).To(Contain("font-family"))
			})
		})

		describe("GET script.js", func() {
			it("serves the embedded script", func() {
				rec := get(mux, "/script.js")
				Expect(t, rec.Code).To(Equal(http.StatusOK))
				Expect(t, rec.Header().Get("Content-Type")).To(Contain("javascript"))
				Expect(t, rec.Body.String()).To(Contain("function deleteFile"))
			})
		})
	})
}
