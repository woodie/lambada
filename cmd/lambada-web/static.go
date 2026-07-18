// Static serves the embedded static/ directory (style.css, script.js) via http.FileServerFS.
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
)

//go:embed static
var staticFS embed.FS

// staticHandler builds the http.Handler newMux's catch-all route serves static/ through.
func staticHandler() http.Handler {
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("static assets: %v", err)
	}
	return http.FileServerFS(static)
}
