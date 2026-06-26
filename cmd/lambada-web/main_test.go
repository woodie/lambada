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

		// Exact size/age formatting is unit-tested directly against
		// toViewData/humanSize/timeAgoInWords below -- this just checks the
		// page renders a link to the file.
		Context("with a file", func() {
			BeforeEach(writeFile)

			It("renders a download link for the file", func() {
				rec := get(mux, "/")
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Body.String()).To(ContainSubstring("/download/" + file))
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
		// against toScansJSON below -- this just checks the route is wired
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

var _ = Describe("listing", func() {
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

var _ = Describe("newServer", func() {
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

var _ = Describe("toScansJSON", func() {
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

var _ = Describe("toViewData", func() {
	It("returns no scans for an empty listing", func() {
		Expect(toViewData(nil, time.Now()).Scans).To(BeEmpty())
	})

	It("formats each scan's size and age", func() {
		now := time.Now()
		scans := []scan{{Name: "1234567890.pdf", Time: now, Size: 7}}

		Expect(toViewData(scans, now).Scans).To(Equal([]viewScan{
			{Name: "1234567890.pdf", HumanSize: "7 Bytes", TimeAgo: "less than a minute"},
		}))
	})
})

var _ = DescribeTable("humanSize",
	func(size int64, want string) {
		Expect(humanSize(size)).To(Equal(want))
	},
	Entry("zero bytes", int64(0), "0 Bytes"),
	Entry("one byte", int64(1), "1 Byte"),
	Entry("plain bytes", int64(7), "7 Bytes"),
	Entry("just under a KB", int64(1023), "1023 Bytes"),
	Entry("exactly a KB", int64(1024), "1 KB"),
	Entry("fractional KB", int64(1234), "1.21 KB"),
	Entry("exactly a MB", int64(1048576), "1 MB"),
)

var _ = DescribeTable("timeAgoInWords",
	func(ago time.Duration, want string) {
		now := time.Now()
		Expect(timeAgoInWords(now.Add(-ago), now)).To(Equal(want))
	},
	Entry("just now", 0*time.Second, "less than a minute"),
	Entry("20 seconds ago", 20*time.Second, "less than a minute"),
	Entry("90 seconds ago", 90*time.Second, "2 minutes"),
	Entry("5 minutes ago", 5*time.Minute, "5 minutes"),
	Entry("1 hour ago", 60*time.Minute, "about 1 hour"),
	Entry("2 hours ago", 2*time.Hour, "about 2 hours"),
	Entry("1 day ago", 24*time.Hour, "1 day"),
	Entry("3 days ago", 72*time.Hour, "3 days"),
)
