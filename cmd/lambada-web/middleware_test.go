package main

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Middleware exercises withLogging, the request-logging wrapper middleware.go defines.
var _ = Describe("Middleware", func() {
	Describe("withLogging", func() {
		It("passes through the wrapped handler's status and body unchanged", func() {
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "nope", http.StatusNotFound)
			})

			req := httptest.NewRequest(http.MethodGet, "/missing", nil)
			rec := httptest.NewRecorder()
			withLogging(inner).ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNotFound))
			Expect(rec.Body.String()).To(ContainSubstring("nope"))
		})

		It("defaults to 200 when the handler never calls WriteHeader", func() {
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			withLogging(inner).ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
		})
	})
})
