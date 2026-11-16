package render

import (
	"html"
	"net/url"
	"strings"

	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// nodeRenderer overrides goldmark's rendering of fenced code blocks (plugin
// blocks), links, autolinks and images (link rewriting).
type nodeRenderer struct {
	st *state
	r  *Renderer
}

func (n *nodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, n.fenced)
	reg.Register(ast.KindLink, n.link)
	reg.Register(ast.KindAutoLink, n.autoLink)
	reg.Register(ast.KindImage, n.image)
}

func (n *nodeRenderer) fenced(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	fb := node.(*ast.FencedCodeBlock)
	var body strings.Builder
	lines := fb.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		body.Write(seg.Value(src))
	}
	lang := string(fb.Language(src))
	if id := n.st.set.Denied("block", lang); lang != "" && id != "" && n.st.set.Block(lang) == nil {
		n.st.deny(id)
	}
	if p := n.st.set.Block(lang); lang != "" && p != nil {
		attrs := map[string]string{}
		info := strings.TrimSpace(string(fb.Info.Segment.Value(src)))
		if rest := strings.TrimSpace(strings.TrimPrefix(info, lang)); strings.HasPrefix(rest, "{") {
			if a, err := plugins.ParseAttrs(rest); err == nil {
				attrs = a
			}
		}
		_, _ = w.WriteString(n.st.block(p, plugins.Block{Name: lang, Body: body.String(), Attrs: attrs, Fenced: true}))
		_ = w.WriteByte('\n')
		return ast.WalkSkipChildren, nil
	}
	_, _ = w.WriteString("<pre><code")
	if lang != "" {
		_, _ = w.WriteString(` class="language-` + html.EscapeString(lang) + `"`)
	}
	_, _ = w.WriteString(">" + html.EscapeString(body.String()) + "</code></pre>\n")
	return ast.WalkSkipChildren, nil
}

// openLink writes the opening <a> tag for href, rewritten per its kind.
func (n *nodeRenderer) openLink(w util.BufWriter, href, title string) {
	b := &strings.Builder{}
	b.WriteString("<a")
	attr := func(k, v string) { b.WriteString(" " + k + `="` + html.EscapeString(v) + `"`) }
	if strings.HasPrefix(href, "#") {
		attr("href", href)
	} else {
		l := n.r.resolve(n.st.path, href)
		switch l.Kind {
		case "internal":
			attr("href", "#")
			attr("data-zen-link", "internal")
			attr("data-path", l.Path)
			if l.Anchor != "" {
				attr("data-anchor", l.Anchor)
			}
		case "asset":
			attr("href", wsURL(l.Path))
		case "external":
			if safeExternal(href) {
				attr("href", href)
				attr("data-zen-link", "external")
				attr("rel", "noopener noreferrer")
			} else {
				attr("data-zen-link", "broken")
			}
		default:
			attr("data-zen-link", "broken")
		}
	}
	if title != "" {
		attr("title", title)
	}
	b.WriteString(">")
	_, _ = w.WriteString(b.String())
}

func (n *nodeRenderer) link(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	l := node.(*ast.Link)
	if entering {
		n.openLink(w, string(l.Destination), string(l.Title))
	} else {
		_, _ = w.WriteString("</a>")
	}
	return ast.WalkContinue, nil
}

func (n *nodeRenderer) autoLink(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	l := node.(*ast.AutoLink)
	n.openLink(w, autoLinkURL(l, src), "")
	_, _ = w.WriteString(html.EscapeString(string(l.Label(src))) + "</a>")
	return ast.WalkSkipChildren, nil
}

func (n *nodeRenderer) image(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	img := node.(*ast.Image)
	alt := nodeText(img, src)
	dest := string(img.Destination)
	srcURL := ""
	if l := n.r.resolve(n.st.path, dest); l.Kind == "asset" {
		srcURL = wsURL(l.Path)
	} else if u, err := url.Parse(dest); err == nil && strings.EqualFold(u.Scheme, "https") && u.Host != "" {
		srcURL = dest
	}
	if srcURL == "" {
		_, _ = w.WriteString(html.EscapeString(alt))
		return ast.WalkSkipChildren, nil
	}
	_, _ = w.WriteString(`<img src="` + html.EscapeString(srcURL) + `" alt="` + html.EscapeString(alt) + `"`)
	if len(img.Title) > 0 {
		_, _ = w.WriteString(` title="` + html.EscapeString(string(img.Title)) + `"`)
	}
	_, _ = w.WriteString(">")
	return ast.WalkSkipChildren, nil
}

func autoLinkURL(l *ast.AutoLink, src []byte) string {
	u := string(l.URL(src))
	if l.AutoLinkType == ast.AutoLinkEmail && !strings.HasPrefix(strings.ToLower(u), "mailto:") {
		u = "mailto:" + u
	}
	return u
}

// safeExternal reports whether an external href may be kept.
func safeExternal(href string) bool {
	u, err := url.Parse(href)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return u.Host != ""
	case "mailto":
		return true
	}
	return false
}

// wsURL is the URL the app serves workspace file p at.
func wsURL(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return "/_ws/" + strings.Join(parts, "/")
}
