// Package web embeds and serves the statically-exported Next.js frontend.
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

// The "all:" prefix is required: without it, go:embed silently excludes any
// file or directory whose name starts with "_" or "." — which would drop
// Next.js's entire _next/ asset directory from the embedded build.
//
//go:embed all:static
var staticFS embed.FS

// Handler returns an http.Handler serving the embedded frontend build at
// "/". It strips the leading "static" path component embed.FS always adds.
func Handler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// static/index.html is committed as a placeholder specifically so
		// this can never happen — embed.FS fails to compile on a missing
		// or empty directory.
		panic(err)
	}
	return http.FileServerFS(sub)
}
