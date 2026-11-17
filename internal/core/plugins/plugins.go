// Package plugins defines the zen plugin API (version 1), the registry that
// holds plugins and the parser for the directive syntax. It only defines
// types and lookups; core/render drives the hooks. See docs/PLUGINS.md.
package plugins

import (
	"context"
	"errors"
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
	Help        Help              // summary and snippets for Settings › Plugins
}

// Option describes one plugin option. Type is "string", "bool", "int", "map"
// (string to string, a map[string]string) or "list" (strings, a []string).
// Scope is "" (config, folder and page may set it) or "user": only the global
// config.yml may set it; the same option in .zen.yml or frontmatter is
// ignored with a warning.
type Option struct {
	Type        string
	Default     any
	Description string
	Scope       string
}

// ScopeUser marks an option only the user's global config may set.
const ScopeUser = "user"

// MaxStreamOutput is the most bytes one stream may deliver; past it the
// stream is cut and ends with ErrOutputLimit.
const MaxStreamOutput = 4 << 20

// ErrOutputLimit ends a stream that produced more than MaxStreamOutput.
var ErrOutputLimit = errors.New("output limit")

// AllowRule is an extra sanitiser rule: an element and its allowed attributes.
type AllowRule struct {
	Element string
	Attrs   []string
}

// Assets lists CSS and JS files. In AssetProvider.Entry the entries are paths
// inside the plugin's fs.FS; in Set.Assets they are URLs.
type Assets struct{ CSS, JS []string }

// hasHook is implemented by plugins that declare which hooks they really
// provide (external plugins). A plugin without it provides every hook whose
// method it has.
type hasHook interface{ HasHook(name string) bool }

func declares(p Plugin, hook string) bool {
	if h, ok := p.(hasHook); ok {
		return h.HasHook(hook)
	}
	return true
}

// BlockHookOf returns p's block hook when it has one and declares it.
func BlockHookOf(p Plugin) (BlockHook, bool) {
	h, ok := p.(BlockHook)
	return h, ok && declares(p, "block")
}

// InlineHookOf returns p's inline hook when it has one and declares it.
func InlineHookOf(p Plugin) (InlineHook, bool) {
	h, ok := p.(InlineHook)
	return h, ok && declares(p, "inline")
}

// SourceHookOf returns p's source hook when it has one and declares it.
func SourceHookOf(p Plugin) (SourceHook, bool) {
	h, ok := p.(SourceHook)
	return h, ok && declares(p, "source")
}

// HTMLHookOf returns p's html hook when it has one and declares it.
func HTMLHookOf(p Plugin) (HTMLHook, bool) {
	h, ok := p.(HTMLHook)
	return h, ok && declares(p, "html")
}

// ActionHookOf returns p's action hook when it has one and declares it.
func ActionHookOf(p Plugin) (ActionHook, bool) {
	h, ok := p.(ActionHook)
	return h, ok && declares(p, "action")
}

// StreamHookOf returns p's stream hook when it has one and declares it.
func StreamHookOf(p Plugin) (StreamHook, bool) {
	h, ok := p.(StreamHook)
	return h, ok && declares(p, "stream")
}

// ValidName reports whether s is a valid plugin, block, inline or layout name.
func ValidName(s string) bool { return nameRE.MatchString(s) }

// ValidSemVer reports whether s is a valid SemVer version.
func ValidSemVer(s string) bool { return semverRE.MatchString(s) }

// Plugin is the only interface every plugin implements; hooks are optional.
type Plugin interface{ Manifest() Manifest }

// Context is what a hook learns about the page being rendered.
type Context struct {
	Path   string         // workspace-relative path of the page
	Config map[string]any // this plugin's effective options for this page
	// DataDir is the plugin's writable folder (the same place as an external
	// plugin's ZEN_DATA_DIR) and Workspace the open folder's root. They are
	// set for actions and streams; "" when there is no folder or settings dir.
	DataDir   string
	Workspace string
	// Markdown renders markdown through the page's own pipeline (nested
	// plugins included, with a depth limit) and returns HTML that is
	// sanitised with the rest of the page. It is nil where there is no page
	// pipeline to call.
	Markdown func(src string) (string, error)
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

// ActionHook answers a request from the plugin's frontend (ctx.invoke):
// JSON in, JSON out. A nil answer is "null".
type ActionHook interface {
	Action(ctx Context, payload []byte) ([]byte, error)
}

// StreamHook produces output over time for the plugin's frontend (ctx.stream).
// It calls emit for each chunk, in order, from the calling goroutine, and
// returns when done. It must return soon after c is cancelled (the error is
// then ignored) and must not call emit after it returned. More than
// MaxStreamOutput bytes in total are dropped and end the stream.
type StreamHook interface {
	Stream(c context.Context, ctx Context, payload []byte, emit func(chunk []byte)) error
}

// AssetProvider ships CSS/JS files served at /_plugins/<id>/<file>.
type AssetProvider interface {
	Assets() fs.FS
	Entry() Assets
}

// AssetVersioner is implemented by plugins whose asset URLs carry their own
// version (?v=<AssetVersion>): external plugins use the first 12 hex digits
// of their approved hash, so a changed plugin loads under a new URL. Plugins
// without it use the registry's version (SetAssetVersion, the app version
// for built-ins).
type AssetVersioner interface{ AssetVersion() string }

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
