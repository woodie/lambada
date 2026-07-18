package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sclevine/spec"
	"github.com/woodie/expect"
)

// TestMiddleware exercises withLogging, the request-logging wrapper middleware.go defines.
func TestMiddleware(t *testing.T) {
	spec.Run(t, "Middleware", func(t *testing.T, describe spec.Describe, it spec.S) {
		describe("withLogging", func() {
			it("passes through the wrapped handler's status and body unchanged", func() {
				inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Error(w, "nope", http.StatusNotFound)
				})

				req := httptest.NewRequest(http.MethodGet, "/missing", nil)
				rec := httptest.NewRecorder()
				withLogging(inner).ServeHTTP(rec, req)

				expect.That(t, rec.Code).To(expect.Equal(http.StatusNotFound))
				expect.That(t, rec.Body.String()).To(expect.Contain("nope"))
			})

			it("defaults to 200 when the handler never calls WriteHeader", func() {
				inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

				req := httptest.NewRequest(http.MethodGet, "/", nil)
				rec := httptest.NewRecorder()
				withLogging(inner).ServeHTTP(rec, req)

				expect.That(t, rec.Code).To(expect.Equal(http.StatusOK))
			})
		})
	})
}
