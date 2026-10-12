// Package ui embeds the dashboard (dependency-free vanilla HTML/CSS/JS) into the
// binary and serves it. No Node runtime, no CDN — it works offline, which an
// enterprise security tool requires.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets
var assets embed.FS

// Handler serves the embedded dashboard. "/" resolves to assets/index.html.
func Handler() http.Handler {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err) // embedded FS is known at build time; a failure is a build bug
	}
	return http.FileServer(http.FS(sub))
}
