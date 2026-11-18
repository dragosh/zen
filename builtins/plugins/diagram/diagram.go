// Package diagram renders ```mermaid and :::diagram blocks as diagrams. Go
// emits an escaped placeholder; assets/diagram.js turns it into an SVG with
// the vendored mermaid bundle (see VENDOR.md).
package diagram

import (
	"embed"
	"html"
	"io/fs"

	"github.com/zenmark-org/zenmd/engine/plugins"
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
		Help: plugins.Help{
			Summary: "Draws mermaid diagrams from a mermaid (or diagram) block. Diagrams follow the light or dark theme. A wrong diagram keeps its source visible, with mermaid's error under it.",
			Snippets: []plugins.Snippet{
				{Title: "A flowchart", Markdown: plugins.Fences("~~~mermaid\ngraph LR\n  A[Open folder] --> B{README.md?}\n  B -- yes --> C[Show README]\n  B -- no --> D[Pick a page]\n~~~")},
				{Title: "A sequence diagram", Markdown: plugins.Fences("~~~mermaid\nsequenceDiagram\n  You->>Editor: type\n  You->>Editor: save\n  Editor-->>You: page reloads\n~~~")},
				{Title: "Another theme for one page", Markdown: plugins.Fences("---\nplugins:\n  diagram: { theme: dark }\n---\n# A page\n\n~~~mermaid\ngraph TD\n  A --> B\n~~~")},
			},
		},
		Blocks: []string{"mermaid", "diagram"},
		Allow:  []plugins.AllowRule{{Element: "div", Attrs: []string{"data-theme"}}},
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
	return `<div class="zen-diagram" data-theme="` + theme + `"><pre>` +
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
