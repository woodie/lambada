// Server is the http.Server lambada-web runs, with issue #2's timeouts applied.
package main

import (
	"net/http"
	"time"
)

// Nonzero timeouts avoid the zero-value http.Server that could leak idle keep-alive connections indefinitely (suspected cause of issue #2).
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 60 * time.Second
)

// newServer builds lambada-web's http.Server, pulled out of main so it's unit-testable without binding a real listener.
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
