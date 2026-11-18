// Package render turns a markdown file into the sanitised HTML page the
// reader shows, running the plugin pipeline described in docs/PLAN.md:
// frontmatter, source hooks, parse, block/inline hooks, links, HTML hooks,
// sanitise. Sanitising is last and unconditional.
//
// A program that calls render directly instead of going through
// engine/session must keep that guarantee: show only the HTML Render
// returns, never the markdown or a plugin's output on its own, and never
// disable or reorder the sanitising step. session also decides which of a
// folder's own plugins and layouts may run; a bare Renderer trusts the
// registry it is given.
package render

import (
	"bytes"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/dragosh/zenmd/engine/config"
	"github.com/dragosh/zenmd/engine/layouts"
	"github.com/dragosh/zenmd/engine/plugins"
	"github.com/dragosh/zenmd/engine/workspace"
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
	Layout            LayoutRef // the layout the page was laid out with
}

// LayoutRef names the layout a page uses: its id and where it comes from
// (builtin, folder or user). "article" and builtin when the page chose none.
type LayoutRef struct{ Name, Source string }

// Renderer renders pages. It holds no mutable state and is safe for
// concurrent use.
type Renderer struct {
	reg   *plugins.Registry
	cfg   *config.Config
	links Resolver

	layouts *layouts.Registry // nil: only the built-in article exists
	site    layouts.Site      // where pages sit in the folder, for template layouts
}

// WithLayouts returns a renderer that also knows the layouts of reg, and site
// for the navigation data of template layouts (nil for none).
func (r *Renderer) WithLayouts(reg *layouts.Registry, site layouts.Site) *Renderer {
	c := *r
	c.layouts, c.site = reg, site
	return &c
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
		err := guard(func() (e error) { out, e = h.Source(st.ctx(id), body); return })
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
		err := guard(func() (e error) { res, e = h.HTML(st.ctx(id), out); return })
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
	lay := r.applyLayout(st, pagePath, title, meta, settings, out)
	out = lay.html
	for _, id := range st.denied {
		st.warnings = append(st.warnings, fmt.Sprintf("plugin %q is not allowed (Settings › Plugins)", id))
	}

	// 7 sanitise
	policy := basePolicy
	if lay.template {
		policy = layoutPolicy
	}
	out = addInternalHref(scopedSanitiseWith(out, set.AllowByPlugin(), policy))

	assets := usedAssets(set.Assets(), st.used)
	assets.CSS = append(assets.CSS, lay.assets.CSS...)
	assets.JS = append(assets.JS, lay.assets.JS...)

	return Page{
		Path:        pagePath,
		Title:       title,
		HTML:        out,
		Frontmatter: meta,
		Assets:      assets,
		Warnings:    st.warnings,
		Layout:      lay.ref,
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

// PluginSet returns the plugins enabled for the page whose source (with its
// frontmatter) is src, and their effective options, as Render sees them.
func (r *Renderer) PluginSet(src []byte) *plugins.Set {
	meta, _, _ := config.SplitFrontmatter(src)
	settings := r.cfg.ForPage(meta)
	set, _ := r.reg.Enabled(settings.Plugin, frontmatterPluginIDs(meta))
	return set
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

// laidOut is the page after its layout.
type laidOut struct {
	html     string
	ref      LayoutRef
	assets   plugins.Assets
	template bool // a template drew part of the page
}

// applyLayout puts the page in the layout it chose (frontmatter, then the
// config's patterns, then the config's default, then the article). Problems
// become warnings and the page falls back to the article, which adds nothing.
func (r *Renderer) applyLayout(st *state, pagePath, title string, meta map[string]any, settings config.Settings, out string) laidOut {
	article := laidOut{html: out, ref: LayoutRef{Name: layouts.Default, Source: layouts.SourceBuiltin}}
	choice := layouts.Resolve(pagePath, meta, settings.Layouts())
	st.warnings = append(st.warnings, choice.Warnings...)
	var l *layouts.Layout
	if r.layouts != nil {
		l = r.layouts.Get(choice.Name)
	}
	switch {
	case l == nil && choice.Name == layouts.Default:
		return article
	case l == nil && r.layouts != nil && r.layouts.Problem(choice.Name) != "":
		st.warnings = append(st.warnings, fmt.Sprintf("layout %q is invalid: %s", choice.Name, r.layouts.Problem(choice.Name)))
		return article
	case l == nil && r.layouts != nil && r.layouts.Denied(choice.Name):
		st.warnings = append(st.warnings, fmt.Sprintf("layout %q is not allowed (Settings › Layouts)", choice.Name))
		return article
	case l == nil:
		st.warnings = append(st.warnings, fmt.Sprintf("unknown layout %q", choice.Name))
		return article
	case l.Changed():
		st.warnings = append(st.warnings, fmt.Sprintf("layout %q changed; allow it again (Settings › Layouts)", choice.Name))
		return article
	}
	if l.Manifest().ID == layouts.Default && !l.HasTemplate() && len(l.Manifest().CSS)+len(l.Manifest().JS) == 0 {
		return article
	}
	in := layouts.Input{
		Title: title, Path: pagePath, Content: out, Frontmatter: meta, Options: choice.Options, Site: r.site,
		Markdown: st.markdown,
		Image: func(v string) (string, bool) {
			if lk := r.resolve(pagePath, v); lk.Kind == "asset" {
				return wsURL(lk.Path), true
			}
			if u, err := url.Parse(v); err == nil && strings.EqualFold(u.Scheme, "https") && u.Host != "" {
				return v, true
			}
			return "", false
		},
	}
	if st.titleOnly {
		return article
	}
	var res layouts.Result
	err := guard(func() (e error) { res, e = l.Apply(in); return })
	if err != nil {
		st.warnings = append(st.warnings, fmt.Sprintf("layout %q: %v; using the article layout", choice.Name, err))
		return article
	}
	st.warnings = append(st.warnings, res.Warnings...)
	ver := ""
	if r.layouts != nil {
		ver = r.layouts.AssetVersion()
	}
	return laidOut{html: res.HTML, ref: LayoutRef{Name: l.Manifest().ID, Source: l.Source()}, assets: l.URLs(ver), template: l.HasTemplate()}
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
