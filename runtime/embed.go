// Package runtime embeds the built browser runtime: zen.js (a classic script
// that mounts plugin and layout scripts on a built page) and zen.css. Both
// are built from runtime/src by `npm run build` (the Taskfile does it before
// any Go build); runtime/dist is git-ignored. The package imports only the
// standard library.
package runtime

import (
	"embed"
	"io/fs"
)

//go:embed dist
var dist embed.FS

// FS is the built runtime: zen.js and zen.css at its root.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist is embedded above: cannot fail
	}
	return sub
}
