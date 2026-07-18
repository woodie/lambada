package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/sclevine/spec"
	. "github.com/woodie/expect"
)

// TestServer exercises newServer, the constructor server.go defines.
func TestServer(t *testing.T) {
	spec.Run(t, "Server", func(t *testing.T, describe spec.Describe, it spec.S) {
		describe("newServer", func() {
			it("sets the address and handler", func() {
				mux := newMux()
				srv := newServer("0.0.0.0:9090", mux)
				Expect(t, srv.Addr).To(Equal("0.0.0.0:9090"))
				Expect(t, srv.Handler).To(BeIdenticalTo[http.Handler](mux))
			})

			it("sets every timeout to a nonzero value", func() {
				srv := newServer("0.0.0.0:9090", newMux())
				Expect(t, srv.ReadHeaderTimeout).To(BeNumerically[time.Duration](">", 0))
				Expect(t, srv.ReadTimeout).To(BeNumerically[time.Duration](">", 0))
				Expect(t, srv.WriteTimeout).To(BeNumerically[time.Duration](">", 0))
				Expect(t, srv.IdleTimeout).To(BeNumerically[time.Duration](">", 0))
			})
		})
	})
}
