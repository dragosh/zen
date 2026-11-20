package config

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var dialectIDRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// SetDialects sets markdown.dialects in the text of a .zen.yml to ids and
// returns the new file ("" in, a new file out). Only the lines of the
// markdown: block change: comments, other keys, their order, line endings and a
// BOM stay. An empty ids removes the key, and the markdown: block when nothing
// else is in it; with nothing to remove src comes back as it is.
//
// The edit is checked by reading the result back. A file this cannot edit
// (invalid YAML, a top level that is not a mapping, markdown written as an
// alias or anchor, a flow list over several lines) is an error and the caller
// writes nothing.
func SetDialects(src []byte, ids []string) ([]byte, error) {
	var want []string
	for _, id := range ids {
		if !dialectIDRE.MatchString(id) {
			return nil, fmt.Errorf("%q is not a dialect id", id)
		}
		if !contains(want, id) {
			want = append(want, id)
		}
	}
	bom := ""
	text := string(src)
	if strings.HasPrefix(text, "\xEF\xBB\xBF") {
		bom, text = text[:3], text[3:]
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		return nil, errors.New(".zen.yml is not valid YAML; edit it by hand")
	}
	lines := splitLines(text)
	eol := "\n"
	for _, l := range lines {
		if l.end != "" {
			eol = l.end
			break
		}
	}
	block := func(indent string) []fmLine {
		return []fmLine{{indent + "dialects: [" + strings.Join(want, ", ") + "]", eol}}
	}
	appendLines := func(ls []fmLine, add []fmLine) []fmLine {
		if n := len(ls); n > 0 && ls[n-1].end == "" {
			ls[n-1].end = eol
		}
		return append(ls, add...)
	}

	var out []fmLine
	switch {
	case len(doc.Content) == 0: // empty, or only comments
		if len(want) == 0 {
			return src, nil
		}
		out = appendLines(append([]fmLine(nil), lines...), append([]fmLine{{"markdown:", eol}}, block("  ")...))
	case doc.Content[0].Kind != yaml.MappingNode:
		return nil, errors.New(".zen.yml must be a mapping of settings; edit it by hand")
	default:
		var err error
		out, err = editMarkdown(lines, doc.Content[0], want, eol, block, appendLines)
		if err != nil {
			return nil, err
		}
		if out == nil {
			return src, nil
		}
	}
	result := bom + joinLines(out)
	if err := checkDialectEdit(text, result[len(bom):], want); err != nil {
		return nil, err
	}
	return []byte(result), nil
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// editMarkdown returns the new lines, or nil when nothing changes.
func editMarkdown(lines []fmLine, top *yaml.Node, want []string, eol string,
	block func(string) []fmLine, appendLines func([]fmLine, []fmLine) []fmLine) ([]fmLine, error) {

	cp := func() []fmLine { return append([]fmLine(nil), lines...) }
	var key, val *yaml.Node
	for i := 0; i+1 < len(top.Content); i += 2 {
		if top.Content[i].Value == "markdown" {
			key, val = top.Content[i], top.Content[i+1]
		}
	}
	if key == nil {
		if len(want) == 0 {
			return nil, nil
		}
		return appendLines(cp(), append([]fmLine{{"markdown:", eol}}, block("  ")...)), nil
	}
	if val.Kind == yaml.AliasNode || val.Anchor != "" || key.Anchor != "" {
		return nil, errors.New("markdown in .zen.yml uses an alias; edit it by hand")
	}
	at := key.Line - 1 // the markdown: line
	if at < 0 || at >= len(lines) {
		return nil, errors.New("could not place markdown in .zen.yml; edit it by hand")
	}

	switch {
	case val.Kind == yaml.ScalarNode && val.Tag == "!!null":
		if len(want) == 0 {
			return nil, nil
		}
		out := cp()
		out[at] = fmLine{"markdown:", lines[at].end}
		if out[at].end == "" {
			out[at].end = eol
		}
		return splice(out, at+1, at+1, block("  ")), nil

	case val.Kind == yaml.MappingNode && val.Style&yaml.FlowStyle != 0:
		l := lines[at].text
		if strings.Count(l, "{") != strings.Count(l, "}") || strings.Count(l, "[") != strings.Count(l, "]") {
			return nil, errors.New("markdown in .zen.yml spans several lines; edit it by hand")
		}
		pairs := append([]*yaml.Node(nil), val.Content...)
		var kept []*yaml.Node
		for i := 0; i+1 < len(pairs); i += 2 {
			if pairs[i].Value != "dialects" {
				kept = append(kept, pairs[i], pairs[i+1])
			}
		}
		if len(want) > 0 {
			seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
			for _, id := range want {
				seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: id})
			}
			kept = append([]*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!!str", Value: "dialects"}, seq}, kept...)
		}
		out := cp()
		if len(kept) == 0 {
			return splice(out, at, at+1, nil), nil
		}
		flow := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Style: yaml.FlowStyle, Content: kept}
		b, err := yaml.Marshal(flow)
		if err != nil {
			return nil, err
		}
		nl := lines[at].end
		if nl == "" {
			nl = eol
		}
		out[at] = fmLine{"markdown: " + string(bytes.TrimSpace(b)), nl}
		return out, nil

	case val.Kind == yaml.MappingNode:
		indent := "  "
		if len(val.Content) > 0 {
			indent = strings.Repeat(" ", val.Content[0].Column-1)
		}
		var dk, dv *yaml.Node
		for i := 0; i+1 < len(val.Content); i += 2 {
			if val.Content[i].Value == "dialects" {
				dk, dv = val.Content[i], val.Content[i+1]
			}
		}
		if dk == nil {
			if len(want) == 0 {
				return nil, nil
			}
			return splice(cp(), at+1, at+1, block(indent)), nil
		}
		if dv.Kind == yaml.AliasNode || dv.Anchor != "" {
			return nil, errors.New("markdown.dialects in .zen.yml uses an alias; edit it by hand")
		}
		var cur []string
		if err := dv.Decode(&cur); err == nil && reflect.DeepEqual(cur, want) && len(want) > 0 {
			return nil, nil
		}
		d := dk.Line - 1
		if d < 0 || d >= len(lines) {
			return nil, errors.New("could not place markdown.dialects in .zen.yml; edit it by hand")
		}
		end := d + 1
		if dv.Kind == yaml.SequenceNode && dv.Style&yaml.FlowStyle == 0 || dv.Kind == yaml.ScalarNode && dv.Tag == "!!null" && dv.Line != dk.Line {
			for end < len(lines) {
				t := strings.TrimSpace(lines[end].text)
				if t == "-" || strings.HasPrefix(t, "- ") {
					end++
					continue
				}
				break
			}
		}
		out := cp()
		if len(want) == 0 {
			if len(val.Content) == 2 { // dialects was all the block had
				return splice(out, at, end, nil), nil
			}
			return splice(out, d, end, nil), nil
		}
		repl := block(indent)
		if end == d+1 { // one line: keep a trailing comment
			if _, rest, ok := strings.Cut(lines[d].text, ":"); ok {
				_, comment := splitComment(rest)
				repl[0].text += comment
			}
			if lines[d].end != "" {
				repl[0].end = lines[d].end
			}
		}
		return splice(out, d, end, repl), nil
	}
	return nil, errors.New("markdown in .zen.yml must be a mapping; edit it by hand")
}

// splice replaces lines[from:to] with add.
func splice(lines []fmLine, from, to int, add []fmLine) []fmLine {
	out := append([]fmLine(nil), lines[:from]...)
	out = append(out, add...)
	return append(out, lines[to:]...)
}

// checkDialectEdit decodes both texts and requires the only difference to be
// markdown.dialects.
func checkDialectEdit(before, after string, want []string) error {
	b, err := decodeMap([]byte(before))
	if err != nil {
		return errors.New(".zen.yml is not valid YAML; edit it by hand")
	}
	a, err := decodeMap([]byte(after))
	if err != nil {
		return errors.New("the change would make .zen.yml invalid; edit it by hand")
	}
	strip := func(m map[string]any) []string {
		var got []string
		if md, ok := m["markdown"].(map[string]any); ok {
			if l, ok := md["dialects"].([]any); ok {
				for _, x := range l {
					got = append(got, fmt.Sprint(x))
				}
			}
			delete(md, "dialects")
			if len(md) == 0 {
				delete(m, "markdown")
			}
		} else if m["markdown"] == nil {
			delete(m, "markdown")
		}
		return got
	}
	if b == nil {
		b = map[string]any{}
	}
	if a == nil {
		a = map[string]any{}
	}
	strip(b)
	got := strip(a)
	if len(want) == 0 {
		got = nil
	}
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(b, a) {
		return errors.New("the change to .zen.yml could not be made safely; edit it by hand")
	}
	return nil
}
