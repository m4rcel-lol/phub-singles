// Package web embeds the compiled Angular bundle so the backend ships as a
// single binary. The Docker build drops the production build into ./dist
// before compiling; the checked-in placeholder keeps `go build` working in a
// bare checkout.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// Dist returns the built frontend rooted at its index.html.
func Dist() (fs.FS, error) {
	return fs.Sub(embedded, "dist")
}
