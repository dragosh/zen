package render

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"strings"

	"github.com/dragosh/zen/internal/core/plugins"
)

// state is the per-render mutable state; one goroutine uses it.
type state struct {
	set      *plugins.Set
	path     string
	nonce    string
	frags    []string // placeholder index -> HTML
	texts    []string // placeholder index -> plain text (for the title)
	used     map[string]bool
	warnings []string
}

func newNonce() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (st *state) warn(id string, err error) {
	st.warnings = append(st.warnings, fmt.Sprintf("plugin %q: %v", id, err))
}

func (st *state) token(i int) string { return fmt.Sprintf("zenph%sn%dz", st.nonce, i) }

// placeholder stores fragment HTML and returns its token.
func (st *state) placeholder(html, plain string) string {
	st.frags = append(st.frags, html)
	st.texts = append(st.texts, plain)
	return st.token(len(st.frags) - 1)
}

// block runs a block hook and returns the wrapped HTML.
func (st *state) block(p plugins.Plugin, b plugins.Block) string {
	id := p.Manifest().ID
	var out string
	err := fmt.Errorf("plugin does not render blocks")
	if h, ok := p.(plugins.BlockHook); ok {
		err = guard(func() (e error) { out, e = h.RenderBlock(st.set.Context(id, st.path), b); return })
	}
	if err != nil {
		st.warn(id, err)
		out = plugins.ErrorBox(id, err)
	} else {
		st.used[id] = true
	}
	return `<div data-zen-plugin="` + html.EscapeString(id) + `">` + out + "</div>"
}

func (st *state) inline(p plugins.Plugin, in plugins.Inline) string {
	id := p.Manifest().ID
	var out string
	err := fmt.Errorf("plugin does not render inlines")
	if h, ok := p.(plugins.InlineHook); ok {
		err = guard(func() (e error) { out, e = h.RenderInline(st.set.Context(id, st.path), in); return })
	}
	if err != nil {
		st.warn(id, err)
		out = plugins.ErrorBox(id, err)
	} else {
		st.used[id] = true
	}
	return `<span data-zen-plugin="` + html.EscapeString(id) + `">` + out + "</span>"
}

// replaceContainers renders the outermost containers owned by an enabled
// plugin and puts a placeholder paragraph in their place.
func (st *state) replaceContainers(src []byte) []byte {
	if !bytes.Contains(src, []byte(":::")) {
		return src
	}
	var out bytes.Buffer
	last := 0
	for _, sp := range plugins.ScanContainers(src) {
		if sp.Start < last {
			continue // nested in a replaced container
		}
		p := st.set.Block(sp.Name)
		if p == nil {
			continue
		}
		h := st.block(p, plugins.Block{Name: sp.Name, Body: sp.Body, Attrs: sp.Attrs})
		out.Write(src[last:sp.Start])
		out.WriteString("\n" + st.placeholder(h, "") + "\n\n")
		last = sp.End
	}
	if last == 0 {
		return src
	}
	out.Write(src[last:])
	return out.Bytes()
}

// replaceInlines replaces inline directives of enabled plugins outside
// fenced code blocks with placeholder tokens.
func (st *state) replaceInlines(src []byte) []byte {
	known := func(n string) bool { return st.set.Inline(n) != nil }
	if bytes.IndexByte(src, ':') < 0 {
		return src
	}
	var out bytes.Buffer
	var fenceChar byte
	fenceLen := 0
	for pos := 0; pos < len(src); {
		next := len(src)
		if nl := bytes.IndexByte(src[pos:], '\n'); nl >= 0 {
			next = pos + nl + 1
		}
		line := src[pos:next]
		pos = next
		if c, n, rest := codeFence(bytes.TrimRight(line, " \t\r\n")); n > 0 {
			if fenceChar == 0 && (c != '`' || !bytes.Contains(rest, []byte("`"))) {
				fenceChar, fenceLen = c, n
			} else if fenceChar == c && n >= fenceLen && len(rest) == 0 {
				fenceChar = 0
			}
			out.Write(line)
			continue
		}
		if fenceChar != 0 {
			out.Write(line)
			continue
		}
		at := 0
		for _, sp := range plugins.ScanInlines(line, known) {
			h := st.inline(st.set.Inline(sp.Name), plugins.Inline{Name: sp.Name, Text: sp.Body, Attrs: sp.Attrs})
			out.Write(line[at:sp.Start])
			out.WriteString(st.placeholder(h, sp.Body))
			at = sp.End
		}
		out.Write(line[at:])
	}
	return out.Bytes()
}

// codeFence mirrors plugins' fence detection: up to three spaces, then three
// or more backticks or tildes.
func codeFence(line []byte) (c byte, n int, rest []byte) {
	i := 0
	for i < len(line) && i < 3 && line[i] == ' ' {
		i++
	}
	if i >= len(line) || (line[i] != '`' && line[i] != '~') {
		return 0, 0, nil
	}
	c = line[i]
	j := i
	for j < len(line) && line[j] == c {
		j++
	}
	if j-i < 3 {
		return 0, 0, nil
	}
	return c, j - i, bytes.TrimSpace(line[j:])
}

// fill swaps placeholder tokens for their fragments.
func (st *state) fill(s string) string {
	if len(st.frags) == 0 {
		return s
	}
	pairs := make([]string, 0, 4*len(st.frags))
	for i, f := range st.frags {
		t := st.token(i)
		pairs = append(pairs, "<p>"+t+"</p>", f, t, f)
	}
	return strings.NewReplacer(pairs...).Replace(s)
}

// plain swaps placeholder tokens for their plain text.
func (st *state) plain(s string) string {
	if len(st.frags) == 0 {
		return s
	}
	pairs := make([]string, 0, 2*len(st.frags))
	for i, t := range st.texts {
		pairs = append(pairs, st.token(i), t)
	}
	return strings.NewReplacer(pairs...).Replace(s)
}
