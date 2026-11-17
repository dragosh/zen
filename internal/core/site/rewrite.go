package site

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// rewriter turns the app's link conventions in a page's HTML into static ones.
// It works on the tokenizer's output: attribute values are decoded, checked
// and written back escaped, never concatenated into markup. Tags it does not
// change are copied byte for byte.
type rewriter struct {
	from    string // workspace path of the page
	fromOut string // its output file
	edit    string // build.editURL, "" for none

	page func(p string) (out string, ok bool)   // a page that is written
	slug func(s string) (string, bool)          // frontmatter slug -> page path
	file func(p string) (out string, err error) // copies a workspace file; returns its output path

	warns   []string
	needsJS bool // the page has something zen.js must handle (print links)
}

func (r *rewriter) warn(format string, args ...any) {
	r.warns = append(r.warns, fmt.Sprintf(format, args...))
}

const wsPrefix = "/_ws/"

// html rewrites a page body.
func (r *rewriter) html(in string) string {
	z := html.NewTokenizer(strings.NewReader(in))
	var b strings.Builder
	skipAnchor := false // inside a removed <a>
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return b.String()
		}
		raw := string(z.Raw())
		if skipAnchor {
			if tt == html.EndTagToken {
				if name, _ := z.TagName(); string(name) == "a" {
					skipAnchor = false
				}
			}
			continue
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			b.WriteString(raw)
			continue
		}
		tok := z.Token()
		attrs := tok.Attr
		changed := false
		drop := false
		if tok.Data == "a" {
			attrs, changed, drop = r.anchor(attrs)
			if drop {
				skipAnchor = tt == html.StartTagToken
				continue
			}
		}
		for i := 0; i < len(attrs); i++ {
			a := attrs[i]
			if a.Namespace != "" || !strings.HasPrefix(a.Val, wsPrefix) {
				continue
			}
			changed = true
			if v, ok := r.workspaceURL(a.Val); ok {
				attrs[i].Val = v
			} else {
				attrs = append(attrs[:i], attrs[i+1:]...)
				i--
			}
		}
		if !changed {
			b.WriteString(raw)
			continue
		}
		b.WriteString(startTag(tok.Data, attrs, tt == html.SelfClosingTagToken))
	}
}

// startTag serialises a start tag with every value escaped.
func startTag(name string, attrs []html.Attribute, self bool) string {
	var b strings.Builder
	b.WriteString("<" + name)
	for _, a := range attrs {
		b.WriteString(" " + a.Key + `="` + html.EscapeString(a.Val) + `"`)
	}
	if self {
		b.WriteString("/")
	}
	b.WriteString(">")
	return b.String()
}

func attr(attrs []html.Attribute, key string) (string, bool) {
	for _, a := range attrs {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func setAttr(attrs []html.Attribute, key, val string) []html.Attribute {
	for i, a := range attrs {
		if a.Key == key {
			attrs[i].Val = val
			return attrs
		}
	}
	return append(attrs, html.Attribute{Key: key, Val: val})
}

func delAttrs(attrs []html.Attribute, keys ...string) []html.Attribute {
	out := attrs[:0:0]
	for _, a := range attrs {
		drop := false
		for _, k := range keys {
			drop = drop || a.Key == k
		}
		if !drop {
			out = append(out, a)
		}
	}
	return out
}

// anchor rewrites the attributes of an <a>. drop reports that the whole
// element goes (an Edit link without a build.editURL).
func (r *rewriter) anchor(attrs []html.Attribute) (out []html.Attribute, changed, drop bool) {
	if action, ok := attr(attrs, "data-zen-action"); ok {
		switch action {
		case "edit":
			if r.edit == "" {
				return nil, true, true
			}
			u := strings.ReplaceAll(r.edit, "{path}", escapePath(r.from))
			attrs = delAttrs(attrs, "href", "target", "rel")
			attrs = append(attrs, html.Attribute{Key: "href", Val: u}, html.Attribute{Key: "target", Val: "_blank"}, html.Attribute{Key: "rel", Val: "noopener noreferrer"})
			return attrs, true, false
		case "print":
			r.needsJS = true
			attrs = setAttr(delAttrs(attrs, "href"), "href", "#")
			return setAttr(attrs, "role", "button"), true, false
		}
		// Unknown actions do nothing here.
		return delAttrs(attrs, "href", "data-zen-action"), true, false
	}
	kind, _ := attr(attrs, "data-zen-link")
	switch kind {
	case "internal":
		p, _ := attr(attrs, "data-path")
		anchor, _ := attr(attrs, "data-anchor")
		target, ok := r.page(p)
		if !ok {
			if sp, found := r.slug(p); found {
				target, ok = r.page(sp)
			}
		}
		if !ok {
			return r.broken(attrs), true, false
		}
		href := escapePath(relFile(r.fromOut, target))
		if anchor != "" {
			href += (&url.URL{Fragment: anchor}).String()
		}
		return setAttr(attrs, "href", href), true, false
	case "external":
		href, _ := attr(attrs, "href")
		if !safeExternal(href) {
			return r.broken(attrs), true, false
		}
		attrs = delAttrs(attrs, "target", "rel")
		return append(attrs, html.Attribute{Key: "target", Val: "_blank"}, html.Attribute{Key: "rel", Val: "noopener noreferrer"}), true, false
	case "broken":
		if _, has := attr(attrs, "href"); has {
			return r.broken(attrs), true, false
		}
		return attrs, false, false
	}
	// No convention: keep anchors and relative links, drop dangerous schemes.
	href, has := attr(attrs, "href")
	if has && !strings.HasPrefix(href, wsPrefix) && !safeHref(href) {
		return delAttrs(attrs, "href"), true, false
	}
	return attrs, false, false
}

// broken marks a link as broken: no href, nothing of the target left.
func (r *rewriter) broken(attrs []html.Attribute) []html.Attribute {
	attrs = delAttrs(attrs, "href", "data-path", "data-anchor", "target", "rel")
	return setAttr(attrs, "data-zen-link", "broken")
}

// scheme returns the lower-cased URL scheme of v ("" for none), after
// removing what browsers ignore in front of and inside it.
func scheme(v string) string {
	v = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == ' ' {
			return -1
		}
		return r
	}, v)
	i := strings.IndexAny(v, ":/?#")
	if i < 0 || v[i] != ':' {
		return ""
	}
	return strings.ToLower(v[:i])
}

func safeExternal(href string) bool {
	switch scheme(href) {
	case "http", "https", "mailto":
		return true
	}
	return false
}

// safeHref accepts relative links, fragments and the external schemes.
func safeHref(href string) bool {
	s := scheme(href)
	return s == "" || safeExternal(href)
}

// workspaceURL maps a /_ws/<path> URL to the relative URL of the copied file.
func (r *rewriter) workspaceURL(v string) (string, bool) {
	rest := strings.TrimPrefix(v, wsPrefix)
	suffix := ""
	if i := strings.IndexAny(rest, "?#"); i >= 0 {
		rest, suffix = rest[:i], rest[i:]
	}
	p, err := url.PathUnescape(rest)
	if err != nil {
		r.warn("%s: not a valid file name", shorten(v))
		return "", false
	}
	p, ok := cleanRel(p)
	if !ok {
		r.warn("%s: outside the folder; left out", shorten(v))
		return "", false
	}
	out, err := r.file(p)
	if err != nil {
		r.warn("%s: %v; left out", p, err)
		return "", false
	}
	return escapePath(relFile(r.fromOut, out)) + suffix, true
}

func shorten(s string) string {
	if r := []rune(s); len(r) > 80 {
		return string(r[:80]) + "…"
	}
	return s
}
