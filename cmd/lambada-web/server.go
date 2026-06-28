// Server -- the http.Server lambada-web actually runs, and the timeouts it
// needs to avoid issue #2 (see docs/COWORK.md).
package main

import (
	"net/http"
	"time"
)

// Connection timeouts for the http.Server lambada-web runs. The bare
// http.ListenAndServe(addr, handler) helper used previously builds a
// zero-value http.Server, and every one of ReadTimeout, ReadHeaderTimeout,
// WriteTimeout, and IdleTimeout defaults to 0 there -- i.e. "wait forever."
// A client that opens a keep-alive connection and then goes quiet (a
// laptop sleeping mid-request, a flaky Wi-Fi hop, zouk reconnecting
// without cleanly closing the old socket) would tie up a goroutine and a
// file descriptor on the Pi for as long as the process has been running.
// This is the suspected -- not confirmed, see docs/COWORK.md -- cause
// behind https://github.com/woodie/lambada/issues/2, and these timeouts
// are the fix either way: a server that actually times out idle
// connections can't leak them indefinitely.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 60 * time.Second
)

// newServer builds the http.Server lambada-web actually runs, with the
// timeouts above applied -- pulled out of main so they're unit-testable
// without binding a real listener.
func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}
