// Server -- the http.Server lambada-web actually runs, and the timeouts it
// needs to avoid issue #2 (see docs/COWORK.md).
package main

import (
	"net/http"
	"time"
)

// Timeouts for the http.Server lambada-web runs. A bare http.ListenAndServe
// defaults all four to 0 ("wait forever") -- the suspected, unconfirmed
// cause of issue #2. See docs/COWORK.md.
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
