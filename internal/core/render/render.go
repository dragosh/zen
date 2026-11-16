// Package render turns a markdown file into the sanitised HTML page the
// reader shows, running the plugin pipeline described in docs/PLAN.md:
// frontmatter, source hooks, parse, block/inline hooks, links, HTML hooks,
// sanitise. Sanitising is last and unconditional.
package render

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"path"
	"sort"
	"strings"

	"github.com/dragosh/zen/internal/core/config"
	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/core/workspace"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Resolver resolves links found in a page. *workspace.Workspace satisfies it.
type Resolver interface {
	ResolveLink(from, href string) workspace.Link
}

// Page is a rendered markdown file.
type Page struct {
	Path, Title, HTML string
	Frontmatter       map[string]any
	Assets            plugins.Assets
	Warnings          []string
	Layout            string // layout plugin name applied to the page, "" when none
}

// Renderer renders pages. It holds no mutable state and is safe for
// concurrent use.
type Renderer struct {
	reg   *plugins.Registry
	cfg   *config.Config
	links Resolver
}

// New returns a renderer. reg, cfg and links may be nil: no plugins, the
// default config and only http(s)/mailto links resolving (as external).
func New(reg *plugins.Registry, cfg *config.Config, links Resolver) *Renderer {
	if reg == nil {
		reg = plugins.NewRegistry()
	}
	if cfg == nil {
		cfg = config.Default()
	}
	return &Renderer{reg: reg, cfg: cfg, links: links}
}

func (r *Renderer) resolve(from, href string) workspace.Link {
	if r.links == nil {
		if safeExternal(href) {
			return workspace.Link{Path: href, Kind: "external"}
		}
		return workspace.Link{Kind: "broken"}
	}
	return r.links.ResolveLink(from, href)
}

// parserOptions are shared by rendering and link extraction.
func newMarkdown(extra ...goldmark.Option) goldmark.Markdown {
	opts := append([]goldmark.Option{
		goldmark.WithExtensions(extension.GFM, extension.Footnote),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		// Raw HTML passes goldmark and is sanitised at the end.
		goldmark.WithRendererOptions(html.WithUnsafe()),
	}, extra...)
	return goldmark.New(opts...)
}

// Render runs the pipeline on src, the content of the page at path
// (workspace-relative).
func (r *Renderer) Render(pagePath string, src []byte) (Page, error) {
	// 1 frontmatter and effective config
	meta, body, fmWarn := config.SplitFrontmatter(src)
	settings := r.cfg.ForPage(meta)
	set, regWarn := r.reg.Enabled(settings.Plugin, frontmatterPluginIDs(meta))

	st := &state{r: r, set: set, path: pagePath, used: map[string]bool{}, nonce: newNonce()}
	st.warnings = append(st.warnings, settings.Warnings...)
	if fmWarn != "" {
		st.warnings = append(st.warnings, fmWarn)
	}
	st.warnings = append(st.warnings, regWarn...)

	// 2 source hooks
	for _, p := range set.Plugins() {
		h, ok := plugins.SourceHookOf(p)
		if !ok {
			continue
		}
		id := p.Manifest().ID
		var out []byte
		err := guard(func() (e error) { out, e = h.Source(set.Context(id, pagePath), body); return })
		if err != nil {
			st.warn(id, err)
			continue
		}
		body = out
	}

	// 3+4 directives become placeholders, then goldmark parses
	body = st.replaceContainers(body)
	body = st.replaceInlines(body)

	md := newMarkdown(goldmark.WithRendererOptions(
		renderer.WithNodeRenderers(util.Prioritized(&nodeRenderer{st: st, r: r}, 100)),
	))
	doc := md.Parser().Parse(text.NewReader(body))

	title := pageTitle(st, meta, doc, body, pagePath)

	// 4+5 render, with fenced plugin blocks and link rewriting
	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, body, doc); err != nil {
		return Page{}, err
	}
	out := st.fill(buf.String())

	// 6 HTML hooks
	for _, p := range set.Plugins() {
		h, ok := plugins.HTMLHookOf(p)
		if !ok {
			continue
		}
		id := p.Manifest().ID
		var res string
		err := guard(func() (e error) { res, e = h.HTML(set.Context(id, pagePath), out); return })
		if err != nil {
			st.warn(id, err)
			continue
		}
		if res != out {
			st.used[id] = true // its assets (e.g. CSS) are needed on this page
		}
		out = res
	}

	// 6b layout
	layout := ""
	if name, wrapped := r.layout(st, meta, out); wrapped != "" {
		layout, out = name, wrapped
	}
	for _, id := range st.denied {
		st.warnings = append(st.warnings, fmt.Sprintf("plugin %q is not allowed (Settings › Plugins)", id))
	}

	// 7 sanitise
	out = addInternalHref(policyFor(set.Allow()).Sanitize(out))

	return Page{
		Path:        pagePath,
		Title:       title,
		HTML:        out,
		Frontmatter: meta,
		Assets:      usedAssets(set.Assets(), st.used),
		Warnings:    st.warnings,
		Layout:      layout,
	}, nil
}

// pageTitle is the frontmatter title, else the first h1, else the file name.
func pageTitle(st *state, meta map[string]any, doc ast.Node, body []byte, pagePath string) string {
	title := ""
	if t, ok := meta["title"].(string); ok && strings.TrimSpace(t) != "" {
		title = strings.TrimSpace(t)
	} else {
		title = st.plain(firstH1(doc, body))
	}
	if title == "" {
		base := path.Base(pagePath)
		title = strings.TrimSuffix(base, path.Ext(base))
	}
	return title
}

// Title returns the title Render would give the page without running any
// plugin hook (so indexing never starts an external program).
func (r *Renderer) Title(pagePath string, src []byte) string {
	meta, body, _ := config.SplitFrontmatter(src)
	settings := r.cfg.ForPage(meta)
	set, _ := r.reg.Enabled(settings.Plugin, frontmatterPluginIDs(meta))
	st := &state{r: r, set: set, path: pagePath, used: map[string]bool{}, nonce: newNonce(), titleOnly: true}
	body = st.replaceContainers(body)
	body = st.replaceInlines(body)
	doc := newMarkdown().Parser().Parse(text.NewReader(body))
	return pageTitle(st, meta, doc, body, pagePath)
}

// layout wraps the page in its layout plugin's element when the frontmatter
// asks for one. It returns the layout name and the wrapped HTML, or "" when
// the page has no (usable) layout; problems become warnings.
func (r *Renderer) layout(st *state, meta map[string]any, out string) (string, string) {
	raw, present := meta["layout"]
	if !present {
		return "", ""
	}
	name, ok := raw.(string)
	if !ok || !plugins.ValidName(name) {
		st.warnings = append(st.warnings, "layout must be a name such as slides")
		return "", ""
	}
	p := st.set.Layout(name)
	if p == nil {
		switch {
		case st.set.Denied("layout", name) != "":
			st.deny(st.set.Denied("layout", name))
		case r.reg.Layout(name) != nil:
			st.warnings = append(st.warnings, fmt.Sprintf("layout %q belongs to plugin %q, which is switched off", name, r.reg.Layout(name).Manifest().ID))
		default:
			st.warnings = append(st.warnings, fmt.Sprintf("unknown layout %q", name))
		}
		return "", ""
	}
	attrs := map[string]string{}
	switch opts := meta["layoutOptions"].(type) {
	case nil:
	case map[string]any:
		keys := make([]string, 0, len(opts))
		for k := range opts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if s, ok := opts[k].(string); ok {
				attrs[k] = s
			} else {
				st.warnings = append(st.warnings, fmt.Sprintf("layoutOptions.%s must be text; ignored", k))
			}
		}
	default:
		st.warnings = append(st.warnings, "layoutOptions must be a mapping; ignored")
	}
	id := p.Manifest().ID
	st.used[id] = true
	return name, `<div data-zen-plugin="` + stdhtml.EscapeString(id) + `" data-zen-layout="` + stdhtml.EscapeString(name) + `"` +
		attrsJSON(attrs) + ">" + out + "</div>"
}

// Links returns the resolved links and images of the page in document order.
func (r *Renderer) Links(pagePath string, src []byte) []workspace.Link {
	_, body, _ := config.SplitFrontmatter(src)
	doc := newMarkdown().Parser().Parse(text.NewReader(body))
	var out []workspace.Link
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Link:
			out = append(out, r.resolve(pagePath, string(n.Destination)))
		case *ast.Image:
			out = append(out, r.resolve(pagePath, string(n.Destination)))
		case *ast.AutoLink:
			out = append(out, r.resolve(pagePath, autoLinkURL(n, body)))
		}
		return ast.WalkContinue, nil
	})
	return out
}

// HTML renders markdown without plugins or a workspace: external links are
// kept, relative ones render as broken. Kept for callers that predate Renderer.
func HTML(src []byte) (string, error) {
	p, err := New(nil, nil, nil).Render("", src)
	return p.HTML, err
}

func frontmatterPluginIDs(meta map[string]any) []string {
	m, ok := meta["plugins"].(map[string]any)
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	return ids
}

func usedAssets(all plugins.Assets, used map[string]bool) plugins.Assets {
	keep := func(urls []string) []string {
		var out []string
		for _, u := range urls {
			rest := strings.TrimPrefix(u, "/_plugins/")
			if id, _, ok := strings.Cut(rest, "/"); ok && used[id] {
				out = append(out, u)
			}
		}
		return out
	}
	return plugins.Assets{CSS: keep(all.CSS), JS: keep(all.JS)}
}

// guard runs f, turning a panic into an error.
func guard(f func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
	}()
	return f()
}

// firstH1 returns the text of the first top-level h1.
func firstH1(doc ast.Node, src []byte) string {
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		if h, ok := c.(*ast.Heading); ok && h.Level == 1 {
			return strings.TrimSpace(nodeText(h, src))
		}
	}
	return ""
}

func nodeText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch c := c.(type) {
		case *ast.Text:
			// Titles are plain text shown via textContent: resolve backslash
			// escapes and entities here instead of leaving \< or &lt; behind.
			v := c.Segment.Value(src)
			if !c.IsRaw() {
				v = util.UnescapePunctuations(v)
				v = util.ResolveNumericReferences(v)
				v = util.ResolveEntityNames(v)
			}
			b.Write(v)
			if c.SoftLineBreak() || c.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(c.Value)
		case *ast.AutoLink:
			b.Write(c.Label(src))
		case *ast.RawHTML:
			// Keep "<T>" in "List<T>" as literal text; the UI escapes it.
			for i := 0; i < c.Segments.Len(); i++ {
				seg := c.Segments.At(i)
				b.Write(seg.Value(src))
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}
