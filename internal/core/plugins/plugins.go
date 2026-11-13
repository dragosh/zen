// Package plugins defines the zen plugin API (version 1), the registry that
// holds plugins and the parser for the directive syntax. It only defines
// types and lookups; core/render drives the hooks. See docs/PLUGINS.md.
package plugins

import (
	"html"
	"io/fs"
)

// APIVersion is the plugin API version a manifest must declare.
const APIVersion = 1

// Manifest describes a plugin.
type Manifest struct {
	ID          string            // "diagram"; [a-z][a-z0-9-]*
	Version     string            // SemVer of the plugin
	APIVersion  int               // must equal APIVersion
	Description string            //
	Default     bool              // enabled when no config mentions it
	Blocks      []string          // block names it owns: "mermaid", "diagram"
	Inlines     []string          // inline names it owns
	Allow       []AllowRule       // extra sanitiser rules: element + attributes
	Config      map[string]Option // option name -> type, default, description
}

// Option describes one plugin option. Type is "string", "bool" or "int".
type Option struct {
	Type        string
	Default     any
	Description string
}

// AllowRule is an extra sanitiser rule: an element and its allowed attributes.
type AllowRule struct {
	Element string
	Attrs   []string
}

// Assets lists CSS and JS files. In AssetProvider.Entry the entries are paths
// inside the plugin's fs.FS; in Set.Assets they are URLs.
type Assets struct{ CSS, JS []string }

// Plugin is the only interface every plugin implements; hooks are optional.
type Plugin interface{ Manifest() Manifest }

// Context is what a hook learns about the page being rendered.
type Context struct {
	Path   string         // workspace-relative path of the page
	Config map[string]any // this plugin's effective options for this page
}

// Block is a named fenced or container block handed to a BlockHook.
type Block struct {
	Name, Body string
	Attrs      map[string]string
	Fenced     bool
}

// Inline is a named inline directive handed to an InlineHook.
type Inline struct {
	Name, Text string
	Attrs      map[string]string
}

// SourceHook rewrites markdown before parsing.
type SourceHook interface {
	Source(ctx Context, src []byte) ([]byte, error)
}

// BlockHook renders a named fenced or container block.
type BlockHook interface {
	RenderBlock(ctx Context, b Block) (string, error)
}

// InlineHook renders a named inline directive.
type InlineHook interface {
	RenderInline(ctx Context, i Inline) (string, error)
}

// HTMLHook rewrites the final HTML before sanitising.
type HTMLHook interface {
	HTML(ctx Context, html string) (string, error)
}

// AssetProvider ships CSS/JS files served at /_plugins/<id>/<file>.
type AssetProvider interface {
	Assets() fs.FS
	Entry() Assets
}

// ErrorBox returns the HTML shown in place of a block whose hook failed. The
// plugin id and the message are HTML-escaped.
func ErrorBox(pluginID string, err error) string {
	msg := "unknown error"
	if err != nil {
		msg = err.Error()
	}
	return `<div class="zen-plugin-error"><strong>` + html.EscapeString(pluginID) +
		`</strong>: ` + html.EscapeString(msg) + `</div>`
}
