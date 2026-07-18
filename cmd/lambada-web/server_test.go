package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/sclevine/spec"
	"github.com/woodie/expect"
)

// TestServer exercises newServer, the constructor server.go defines.
func TestServer(t *testing.T) {
	spec.Run(t, "Server", func(t *testing.T, describe spec.Describe, it spec.S) {
		describe("newServer", func() {
			it("sets the address and handler", func() {
				mux := newMux()
				srv := newServer("0.0.0.0:9090", mux)
				expect.That(t, srv.Addr).To(expect.Equal("0.0.0.0:9090"))
				expect.That(t, srv.Handler).To(expect.BeIdenticalTo[http.Handler](mux))
			})

			it("sets every timeout to a nonzero value", func() {
				srv := newServer("0.0.0.0:9090", newMux())
				expect.That(t, srv.ReadHeaderTimeout).To(expect.BeNumerically[time.Duration](">", 0))
				expect.That(t, srv.ReadTimeout).To(expect.BeNumerically[time.Duration](">", 0))
				expect.That(t, srv.WriteTimeout).To(expect.BeNumerically[time.Duration](">", 0))
				expect.That(t, srv.IdleTimeout).To(expect.BeNumerically[time.Duration](">", 0))
			})
		})
	})
}
