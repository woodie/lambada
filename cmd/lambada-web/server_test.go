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
	spec.Run(t, "Server", func(t *testing.T, describe spec.G, it spec.S) {

		describe("newServer", func() {
			it("sets the address and handler", func() {
				mux := newMux()
				srv := newServer("0.0.0.0:9090", mux)
				expect(srv.Addr, t).To(Equal("0.0.0.0:9090"))
				expect(srv.Handler, t).To(BeIdenticalTo[http.Handler](mux))
			})

			it("sets every timeout to a nonzero value", func() {
				srv := newServer("0.0.0.0:9090", newMux())
				expect(srv.ReadHeaderTimeout, t).To(BeNumerically[time.Duration](">", 0))
				expect(srv.ReadTimeout, t).To(BeNumerically[time.Duration](">", 0))
				expect(srv.WriteTimeout, t).To(BeNumerically[time.Duration](">", 0))
				expect(srv.IdleTimeout, t).To(BeNumerically[time.Duration](">", 0))
			})
		})
	})
}
