package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
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

// postFile performs an in-process multipart POST /backups carrying a
// single "file" form field named name with the given content.
func postFile(mux *http.ServeMux, name, content string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", name)
	Expect(err).NotTo(HaveOccurred())
	_, err = part.Write([]byte(content))
	Expect(err).NotTo(HaveOccurred())
	Expect(w.Close()).To(Succeed())

	req := httptest.NewRequest(http.MethodPost, "/backups", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
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
		// backupsDir() derives from scanDir -- create its "backups"
		// subdirectory up front so /backups tests below don't each need to.
		Expect(os.MkdirAll(backupsDir(), 0o755)).To(Succeed())
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

	Describe("GET /backups", func() {
		Context("with no files", func() {
			It("renders the empty state", func() {
				rec := get(mux, "/backups")
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Body.String()).To(ContainSubstring("Backups"))
				Expect(rec.Body.String()).To(ContainSubstring("No files found in the directory."))
			})
		})

		Context("with a file", func() {
			BeforeEach(func() {
				Expect(os.WriteFile(filepath.Join(backupsDir(), "notes.txt"), []byte("content"), 0o644)).To(Succeed())
			})

			It("renders a download link with the file's name, size, and age", func() {
				rec := get(mux, "/backups")
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Body.String()).To(ContainSubstring("/download/backups/notes.txt"))
				Expect(rec.Body.String()).To(ContainSubstring("notes.txt"))
				Expect(rec.Body.String()).To(ContainSubstring("7 B"))
			})
		})
	})

	// Backup files are served through the same /download/{filename...}
	// route as scans (see handleDownload in main.go), just one path segment
	// deeper -- backupsDir() is scanDir's "backups" subdirectory.
	Describe("GET /download/backups/{filename}", func() {
		Context("when the file is missing", func() {
			It("responds with 404", func() {
				rec := get(mux, "/download/backups/missing.txt")
				Expect(rec.Code).To(Equal(http.StatusNotFound))
			})
		})

		Context("when the file exists", func() {
			BeforeEach(func() {
				Expect(os.WriteFile(filepath.Join(backupsDir(), "notes.txt"), []byte("content"), 0o644)).To(Succeed())
			})

			It("responds with 200 and an attachment header", func() {
				rec := get(mux, "/download/backups/notes.txt")
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Header().Get("Content-Disposition")).To(ContainSubstring("notes.txt"))
				Expect(rec.Body.String()).To(Equal("content"))
			})
		})
	})

	// sanitizeRelPath's own traversal guard can't be exercised through the
	// mux: net/http's ServeMux redirects (301, before any handler runs) any
	// request path containing a ".." segment to its already-cleaned
	// equivalent, so a raw ".." never reaches handleDownload over HTTP --
	// see sanitizeRelPath's comment in main.go. Test it directly instead.
	Describe("sanitizeRelPath", func() {
		It("passes through a plain relative path unchanged", func() {
			rel, ok := sanitizeRelPath("backups/report.pdf")
			Expect(ok).To(BeTrue())
			Expect(rel).To(Equal("backups/report.pdf"))
		})

		It("collapses a traversal attempt to something that can't escape the root", func() {
			rel, ok := sanitizeRelPath("backups/../../../etc/passwd")
			Expect(ok).To(BeTrue())
			Expect(rel).To(Equal("etc/passwd"))
			Expect(rel).NotTo(HavePrefix(".."))
		})

		It("rejects an empty or root-only path", func() {
			_, ok := sanitizeRelPath("")
			Expect(ok).To(BeFalse())

			_, ok = sanitizeRelPath("/")
			Expect(ok).To(BeFalse())
		})
	})

	Describe("POST /backups (upload)", func() {
		It("adds a new file and redirects to /backups", func() {
			rec := postFile(mux, "new.txt", "hello")
			Expect(rec.Code).To(Equal(http.StatusSeeOther))
			Expect(rec.Header().Get("Location")).To(Equal("/backups"))

			data, err := os.ReadFile(filepath.Join(backupsDir(), "new.txt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("hello"))
		})

		It("accepts any extension", func() {
			rec := postFile(mux, "archive.tar.gz", "binary-ish content")
			Expect(rec.Code).To(Equal(http.StatusSeeOther))

			data, err := os.ReadFile(filepath.Join(backupsDir(), "archive.tar.gz"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("binary-ish content"))
		})

		It("replaces a file with the same name", func() {
			Expect(os.WriteFile(filepath.Join(backupsDir(), "existing.txt"), []byte("old"), 0o644)).To(Succeed())

			rec := postFile(mux, "existing.txt", "new content")
			Expect(rec.Code).To(Equal(http.StatusSeeOther))

			data, err := os.ReadFile(filepath.Join(backupsDir(), "existing.txt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("new content"))
		})

		// Go's own mime/multipart parsing already reduces a "filename"
		// disposition parameter to its base name (Part.FileName() does the
		// equivalent of filepath.Base(filepath.Clean("/"+name)) before
		// handleBackupsUpload ever sees header.Filename) -- so this never
		// reaches sanitizeFilename as a raw traversal attempt in the first
		// place. Confirms the safe, expected outcome: the upload succeeds,
		// landing at the base name inside backupsDir(), not anywhere else.
		It("reduces a path-traversal filename to its base name rather than escaping", func() {
			rec := postFile(mux, "../evil.txt", "hello")
			Expect(rec.Code).To(Equal(http.StatusSeeOther))

			data, err := os.ReadFile(filepath.Join(backupsDir(), "evil.txt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("hello"))
		})
	})

	// sanitizeFilename's own rejection cases -- values whose filepath.Base()
	// is empty, ".", or "..", which Go's multipart parsing wouldn't itself
	// produce from a client-supplied traversal attempt (see the upload test
	// above) but which sanitizeFilename still guards against directly,
	// since it's also reachable in principle from any future caller.
	Describe("sanitizeFilename", func() {
		It("accepts a plain filename", func() {
			name, ok := sanitizeFilename("report.pdf")
			Expect(ok).To(BeTrue())
			Expect(name).To(Equal("report.pdf"))
		})

		It("reduces a path to its base name", func() {
			name, ok := sanitizeFilename("some/dir/report.pdf")
			Expect(ok).To(BeTrue())
			Expect(name).To(Equal("report.pdf"))
		})

		It("rejects an empty name", func() {
			_, ok := sanitizeFilename("")
			Expect(ok).To(BeFalse())
		})

		It("rejects a bare \"..\"", func() {
			_, ok := sanitizeFilename("..")
			Expect(ok).To(BeFalse())
		})

		It("rejects a bare \".\"", func() {
			_, ok := sanitizeFilename(".")
			Expect(ok).To(BeFalse())
		})
	})
})
