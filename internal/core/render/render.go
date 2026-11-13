// Package render turns markdown into sanitised HTML.
package render

import (
	"bytes"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var (
	md = goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Footnote),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		// Raw HTML is allowed through goldmark and then sanitised below.
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)
	policy = newPolicy()
)

func newPolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("id").OnElements("h1", "h2", "h3", "h4", "h5", "h6", "li", "sup")
	p.AllowAttrs("class").Matching(bluemonday.SpaceSeparatedTokens).OnElements("code", "pre", "div", "span", "li", "ul", "ol", "section", "sup", "a")
	p.AllowAttrs("type", "checked", "disabled").OnElements("input")
	p.AllowElements("input", "section")
	return p
}

// HTML renders markdown source to sanitised HTML.
func HTML(source []byte) (string, error) {
	var buf bytes.Buffer
	if err := md.Convert(source, &buf); err != nil {
		return "", err
	}
	return policy.Sanitize(buf.String()), nil
}
