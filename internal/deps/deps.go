// Package deps pins the Go modules the planned features use, so feature
// branches never need to edit go.mod (see specs/README.md). The lead deletes
// this file once wave 3 is merged.
package deps

import (
	_ "github.com/alecthomas/chroma/v2"
	_ "github.com/fsnotify/fsnotify"
	_ "github.com/microcosm-cc/bluemonday"
	_ "github.com/yuin/goldmark"
	_ "gopkg.in/yaml.v3"
)
