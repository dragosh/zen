// Package codeblock is the code block plugin: an HTML hook that wraps every
// <pre><code> block in a [data-zen-plugin="codeblock"] wrapper and replaces
// blocks whose language chroma knows with chroma output using CSS classes
// only. Colours live in codeblock.css; codeblock.js adds the Copy and Wrap
// buttons inside each block.
package codeblock

import (
	"embed"
	"errors"
	"html"
	"io/fs"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/dragosh/zenmd/engine/plugins"
)

// maxBlock is the largest code block that is highlighted.
const maxBlock = 2 << 20

//go:embed codeblock.css codeblock.js
var assets embed.FS

type plugin struct{}

// New returns the codeblock plugin.
func New() plugins.Plugin { return plugin{} }

func (plugin) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID:          "codeblock",
		Version:     "0.1.0",
		APIVersion:  plugins.APIVersion,
		Description: "Code blocks: syntax colours (chroma, CSS classes only) and Copy and Wrap buttons.",
		Default:     true,
		Help: plugins.Help{
			Summary: "Colours fenced code blocks by language and gives each block a Copy and a Wrap button. A block without a language, or with one chroma does not know, stays plain.",
			Snippets: []plugins.Snippet{
				{Title: "A coloured block", Markdown: plugins.Fences("~~~go\nfunc Hello(name string) string {\n\treturn \"hello, \" + name\n}\n~~~")},
				{Title: "Line numbers and wrapping for one page", Markdown: plugins.Fences("---\nplugins:\n  codeblock: { lineNumbers: true, wrap: true }\n---\n# A page\n\n~~~sh\nzenmd edit README.md\n~~~")},
			},
		},
		Allow: []plugins.AllowRule{
			{Element: "span", Attrs: []string{"class"}},
			{Element: "pre", Attrs: []string{"class"}},
			{Element: "code", Attrs: []string{"class"}},
			{Element: "div", Attrs: []string{"class"}},
		},
		Config: map[string]plugins.Option{
			"style":       {Type: "string", Default: "zen", Description: "Highlight style (only \"zen\" is shipped)."},
			"lineNumbers": {Type: "bool", Default: false, Description: "Show a line-number gutter."},
			"wrap":        {Type: "bool", Default: false, Description: "Wrap long lines by default (the Wrap button toggles it per block)."},
		},
	}
}

func (plugin) Assets() fs.FS { return assets }
func (plugin) Entry() plugins.Assets {
	return plugins.Assets{CSS: []string{"codeblock.css"}, JS: []string{"codeblock.js"}}
}

var (
	// blockRE matches goldmark's code block opening tag, with or without a language.
	blockRE = regexp.MustCompile(`<pre><code(?: class="language-([^"\s]+)")?>`)
	// tagRE matches div/span tags, to track other plugins' elements.
	tagRE = regexp.MustCompile(`<(/?)(div|span)\b([^>]*)>`)
)

const closing = "</code></pre>"

// HTML wraps every code block outside other plugins' elements and highlights
// those whose language chroma knows. A page with a block over 2 MB is returned
// unchanged with an error (a render warning).
func (plugin) HTML(ctx plugins.Context, in string) (string, error) {
	lineNumbers, _ := ctx.Config["lineNumbers"].(bool)
	open := `<div data-zen-plugin="codeblock" class="zen-codeblock">`
	if wrap, _ := ctx.Config["wrap"].(bool); wrap {
		open = `<div data-zen-plugin="codeblock" class="zen-codeblock zen-codeblock-wrap">`
	}
	tags := tagRE.FindAllStringSubmatchIndex(in, -1)

	var b strings.Builder
	last, ti := 0, 0
	var stack []bool // open div/span: does it carry data-zen-plugin?
	inPlugin := 0
	for _, m := range blockRE.FindAllStringSubmatchIndex(in, -1) {
		if m[0] < last {
			continue
		}
		for ; ti < len(tags) && tags[ti][0] < m[0]; ti++ {
			t := tags[ti]
			if t[3] > t[2] { // closing tag
				if n := len(stack); n > 0 {
					if stack[n-1] {
						inPlugin--
					}
					stack = stack[:n-1]
				}
				continue
			}
			attrs := in[t[6]:t[7]]
			if strings.HasSuffix(attrs, "/") {
				continue
			}
			p := strings.Contains(attrs, "data-zen-plugin")
			if p {
				inPlugin++
			}
			stack = append(stack, p)
		}
		end := strings.Index(in[m[1]:], closing)
		if end < 0 || inPlugin > 0 {
			continue
		}
		bodyEnd := m[1] + end
		if bodyEnd-m[1] > maxBlock {
			return in, errors.New("code block over 2 MB not highlighted")
		}
		block := in[m[0] : bodyEnd+len(closing)]
		if m[2] >= 0 {
			if lexer := lexers.Get(in[m[2]:m[3]]); lexer != nil {
				if out, err := format(lexer, in[m[2]:m[3]], html.UnescapeString(in[m[1]:bodyEnd]), lineNumbers); err == nil {
					block = out
				}
			}
		}
		b.WriteString(in[last:m[0]])
		b.WriteString(open)
		b.WriteString(block)
		b.WriteString("</div>")
		last = bodyEnd + len(closing)
	}
	if last == 0 {
		return in, nil
	}
	b.WriteString(in[last:])
	return b.String(), nil
}

type wrapper struct{ lang string }

func (w wrapper) Start(code bool, _ string) string {
	return `<pre class="chroma"><code class="language-` + html.EscapeString(w.lang) + `">`
}
func (wrapper) End(bool) string { return closing }

func format(lexer chroma.Lexer, lang, code string, lineNumbers bool) (string, error) {
	it, err := tokenise(lexer, code)
	if err != nil {
		return "", err
	}
	f := chromahtml.New(
		chromahtml.WithClasses(true),
		chromahtml.WithAllClasses(true),
		chromahtml.WithLineNumbers(lineNumbers),
		chromahtml.WithPreWrapper(wrapper{lang}),
	)
	var b strings.Builder
	if err := f.Format(&b, styles.Fallback, it); err != nil {
		return "", err
	}
	return b.String(), nil
}

// parallelOver is the block size above which a block is split at line
// boundaries and the parts are tokenised concurrently. Constructs spanning a
// split (a long multi-line comment, say) may then be coloured imperfectly;
// the text is never altered.
const parallelOver = 64 << 10

func tokenise(lexer chroma.Lexer, code string) (chroma.Iterator, error) {
	n := runtime.NumCPU()
	if len(code) <= parallelOver || n < 2 {
		return lexer.Tokenise(nil, code)
	}
	if per := len(code) / n; per < parallelOver/2 {
		n = len(code)/(parallelOver/2) + 1
	}
	var parts []string
	for rest := code; rest != ""; {
		cut := len(code)/n + 1
		if cut >= len(rest) {
			parts = append(parts, rest)
			break
		}
		if i := strings.IndexByte(rest[cut:], '\n'); i >= 0 {
			cut += i + 1
		} else {
			cut = len(rest)
		}
		parts = append(parts, rest[:cut])
		rest = rest[cut:]
	}
	toks := make([][]chroma.Token, len(parts))
	errs := make([]error, len(parts))
	var wg sync.WaitGroup
	for i, part := range parts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			it, err := lexer.Tokenise(nil, part)
			if err != nil {
				errs[i] = err
				return
			}
			toks[i] = it.Tokens()
		}()
	}
	wg.Wait()
	var all []chroma.Token
	for i := range parts {
		if errs[i] != nil {
			return nil, errs[i]
		}
		all = append(all, toks[i]...)
	}
	return chroma.Literator(all...), nil
}
