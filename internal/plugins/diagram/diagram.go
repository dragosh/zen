// Package diagram renders ```mermaid and :::diagram blocks as diagrams. Go
// emits an escaped placeholder; assets/diagram.js turns it into an SVG with
// the vendored mermaid bundle (see VENDOR.md).
package diagram

import (
	"embed"
	"html"
	"io/fs"

	"github.com/dragosh/zen/internal/core/plugins"
)

//go:embed assets/diagram.js assets/diagram.css assets/mermaid.min.js
var assets embed.FS

var themes = map[string]bool{"neutral": true, "dark": true, "default": true}

type plugin struct{}

// New returns the diagram plugin.
func New() plugins.Plugin { return plugin{} }

func (plugin) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID:          "diagram",
		Version:     "0.1.0",
		APIVersion:  plugins.APIVersion,
		Description: "Mermaid diagrams",
		Default:     true,
		Blocks:      []string{"mermaid", "diagram"},
		Allow:       []plugins.AllowRule{{Element: "div", Attrs: []string{"data-theme"}}},
		Config: map[string]plugins.Option{
			"theme": {Type: "string", Default: "neutral", Description: "mermaid theme: neutral, dark or default"},
		},
	}
}

func (plugin) RenderBlock(ctx plugins.Context, b plugins.Block) (string, error) {
	theme, _ := ctx.Config["theme"].(string)
	if !themes[theme] {
		theme = "neutral"
	}
	return `<div data-zen-plugin="diagram" class="zen-diagram" data-theme="` + theme + `"><pre>` +
		html.EscapeString(b.Body) + `</pre></div>`, nil
}

func (plugin) Assets() fs.FS {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err)
	}
	return sub
}

func (plugin) Entry() plugins.Assets {
	return plugins.Assets{CSS: []string{"diagram.css"}, JS: []string{"diagram.js"}}
}
