package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Server exercises newServer, the constructor server.go defines.
var _ = Describe("Server", func() {
	Describe("newServer", func() {
		It("sets the address and handler", func() {
			mux := newMux()
			srv := newServer("0.0.0.0:9090", mux)
			Expect(srv.Addr).To(Equal("0.0.0.0:9090"))
			Expect(srv.Handler).To(BeIdenticalTo(mux))
		})

		// Regression test for issue #2: guards against newServer reverting to a zero-value (all-timeouts-0) http.Server.
		It("sets every timeout to a nonzero value", func() {
			srv := newServer("0.0.0.0:9090", newMux())
			Expect(srv.ReadHeaderTimeout).To(BeNumerically(">", 0))
			Expect(srv.ReadTimeout).To(BeNumerically(">", 0))
			Expect(srv.WriteTimeout).To(BeNumerically(">", 0))
			Expect(srv.IdleTimeout).To(BeNumerically(">", 0))
		})
	})
})
