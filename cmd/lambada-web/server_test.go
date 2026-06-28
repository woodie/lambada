package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Server mirrors server.go -- newServer is the constructor these tests
// guard, including the issue #2 regression check.
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
		// the suspected cause of leaked keep-alive connections piling up
		// until new clients couldn't connect at all (see server.go). This
		// just has to fail loudly if a future edit accidentally drops back
		// to a zero-value server.
		It("sets every timeout to a nonzero value", func() {
			srv := newServer("0.0.0.0:9090", newMux())
			Expect(srv.ReadHeaderTimeout).To(BeNumerically(">", 0))
			Expect(srv.ReadTimeout).To(BeNumerically(">", 0))
			Expect(srv.WriteTimeout).To(BeNumerically(">", 0))
			Expect(srv.IdleTimeout).To(BeNumerically(">", 0))
		})
	})
})
