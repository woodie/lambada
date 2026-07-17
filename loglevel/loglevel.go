// Package loglevel lets any Go program silence its standard logger's output
// via the LOG_LEVEL environment variable, the same convention widely used
// across other languages' tooling (Node, Python, Ruby, many CLIs).
package loglevel

import (
	"io"
	"log"
	"os"
)

// Apply discards the standard logger's output when LOG_LEVEL is set to "OFF".
func Apply() {
	if os.Getenv("LOG_LEVEL") == "OFF" {
		log.SetOutput(io.Discard)
	}
}
