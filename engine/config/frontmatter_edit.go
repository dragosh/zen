package config

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// SetNavKey sets (or, with a nil value, removes) one key of the `nav:` mapping
// in a page's frontmatter and returns the new file. value is YAML text of one
// scalar ("10", "true"). Only the frontmatter block changes, and only on the
// lines of that key: the body, line endings, a BOM, other keys and comments stay
// as they are. A page without frontmatter gets a block. Removing a key that is
// not there returns src as it is.
//
// The edit is checked by reading the result back: when the frontmatter is not
// valid YAML, nav is written in a form this cannot edit (an alias, a list, a
// mapping over several lines in flow style), or the result would mean anything
// but the one change, SetNavKey returns an error and the caller writes nothing.
func SetNavKey(src []byte, key string, value *string) ([]byte, error) {
	bom := ""
	rest := string(src)
	if strings.HasPrefix(rest, "\xEF\xBB\xBF") {
		bom, rest = rest[:3], rest[3:]
	}
	if _, _, warn := SplitFrontmatter([]byte(rest)); warn != "" {
		return nil, errors.New("the frontmatter is not valid YAML; edit the page by hand")
	}
	lines := splitLines(rest)
	eol := "\n"
	if len(lines) > 0 && lines[0].end != "" {
		eol = lines[0].end
	}

	closeAt := closingLine(lines)
	var out []fmLine
	switch {
	case closeAt < 0: // no frontmatter
		if value == nil {
			return src, nil
		}
		head := []fmLine{{"---", eol}, {"nav:", eol}, {"  " + key + ": " + *value, eol}, {"---", eol}}
		out = append(head, lines...)
	default:
		var err error
		out, err = editBlock(lines, closeAt, eol, key, value)
		if err != nil {
			return nil, err
		}
		if out == nil {
			return src, nil // nothing to remove
		}
	}
	result := bom + joinLines(out)
	if err := checkEdit(rest, result[len(bom):], key, value); err != nil {
		return nil, err
	}
	return []byte(result), nil
}

type fmLine struct{ text, end string }

func splitLines(s string) []fmLine {
	var out []fmLine
	for len(s) > 0 {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			out = append(out, fmLine{s, ""})
			break
		}
		text, end := s[:i], "\n"
		if strings.HasSuffix(text, "\r") {
			text, end = text[:len(text)-1], "\r\n"
		}
		out = append(out, fmLine{text, end})
		s = s[i+1:]
	}
	return out
}

func joinLines(ls []fmLine) string {
	var b strings.Builder
	for _, l := range ls {
		b.WriteString(l.text)
		b.WriteString(l.end)
	}
	return b.String()
}

// closingLine is the index of the line that closes the frontmatter, -1 when
// the text has none (see SplitFrontmatter).
func closingLine(lines []fmLine) int {
	if len(lines) == 0 || !isDelimiter([]byte(lines[0].text), false) {
		return -1
	}
	size := 0
	for i := 1; i < len(lines); i++ {
		if size > maxFrontmatter {
			return -1
		}
		if isDelimiter([]byte(lines[i].text), true) {
			return i
		}
		size += len(lines[i].text) + len(lines[i].end)
	}
	return -1
}

func indentOf(s string) string { return s[:len(s)-len(strings.TrimLeft(s, " \t"))] }

func isBlankOrComment(s string) bool {
	t := strings.TrimSpace(s)
	return t == "" || strings.HasPrefix(t, "#")
}

// keyAt reports whether the line (without its indent) is `key:` and returns
// the text after the colon.
func keyAt(line, key string) (string, bool) {
	t := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(t, key) {
		return "", false
	}
	t = strings.TrimLeft(t[len(key):], " \t")
	if !strings.HasPrefix(t, ":") {
		return "", false
	}
	t = t[1:]
	if t != "" && t[0] != ' ' && t[0] != '\t' {
		return "", false
	}
	return t, true
}

// splitComment separates a scalar from a trailing "# comment" (with the
// spaces before it), honouring quotes.
func splitComment(s string) (val, comment string) {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	end := len(s)
	if i < len(s) && (s[i] == '"' || s[i] == '\'') {
		q := s[i]
		j := i + 1
		for j < len(s) {
			if q == '"' && s[j] == '\\' {
				j += 2
				continue
			}
			if s[j] == q {
				if q == '\'' && j+1 < len(s) && s[j+1] == '\'' {
					j += 2
					continue
				}
				break
			}
			j++
		}
		if j >= len(s) {
			return s, ""
		}
		end = j + 1
		rest := s[end:]
		if k := strings.Index(rest, "#"); k >= 0 && strings.TrimSpace(rest[:k]) == "" {
			return s[:end], rest
		}
		return s, ""
	}
	for j := i; j < len(s); j++ {
		if s[j] == '#' && (j == i || s[j-1] == ' ' || s[j-1] == '\t') {
			k := j
			for k > 0 && (s[k-1] == ' ' || s[k-1] == '\t') {
				k--
			}
			return s[:k], s[k:]
		}
	}
	return s, ""
}

// editBlock applies the change to the lines of the frontmatter, lines[0] and
// lines[closeAt] being the fences. A nil result means nothing to change.
func editBlock(lines []fmLine, closeAt int, eol, key string, value *string) ([]fmLine, error) {
	navAt := -1
	for i := 1; i < closeAt; i++ {
		if indentOf(lines[i].text) == "" {
			if _, ok := keyAt(lines[i].text, "nav"); ok {
				navAt = i
				break
			}
		}
	}
	cp := func() []fmLine { return append([]fmLine(nil), lines...) }
	if navAt < 0 {
		if value == nil {
			return nil, nil
		}
		ins := []fmLine{{"nav:", eol}, {"  " + key + ": " + *value, eol}}
		out := append(cp()[:closeAt:closeAt], ins...)
		return append(out, lines[closeAt:]...), nil
	}

	_, tail, _ := strings.Cut(lines[navAt].text, ":")
	val, comment := splitComment(tail)
	switch v := strings.TrimSpace(val); {
	case v == "":
		return editNavBlock(lines, navAt, closeAt, eol, key, value, comment)
	case strings.HasPrefix(v, "{") && strings.HasSuffix(v, "}"):
		text, remove, err := editFlow(v, key, value)
		if err != nil {
			return nil, err
		}
		if text == "" && !remove {
			return nil, nil // removing a key that is not there
		}
		out := cp()
		if remove {
			return append(out[:navAt], out[navAt+1:]...), nil
		}
		out[navAt].text = "nav: " + text + comment
		return out, nil
	}
	return nil, errors.New("nav is written in a form zen cannot edit; edit the page by hand")
}

// editNavBlock handles `nav:` followed by an indented mapping.
func editNavBlock(lines []fmLine, navAt, closeAt int, eol, key string, value *string, comment string) ([]fmLine, error) {
	end := closeAt // the nav block ends at the next top-level key
	for i := navAt + 1; i < closeAt; i++ {
		if indentOf(lines[i].text) == "" && !isBlankOrComment(lines[i].text) {
			end = i
			break
		}
	}
	indent, last, keyAt0, children := "  ", navAt, -1, 0
	first := true
	for i := navAt + 1; i < end; i++ {
		if isBlankOrComment(lines[i].text) {
			continue
		}
		children++
		if first {
			indent, first = indentOf(lines[i].text), false
		}
		last = i
		if indentOf(lines[i].text) == indent {
			if _, ok := keyAt(lines[i].text, key); ok {
				keyAt0 = i
			}
		}
	}
	out := append([]fmLine(nil), lines...)
	switch {
	case keyAt0 >= 0 && value == nil:
		if children == 1 { // the nav mapping would be empty: drop it
			return append(out[:navAt], out[keyAt0+1:]...), nil
		}
		return append(out[:keyAt0], out[keyAt0+1:]...), nil
	case keyAt0 >= 0:
		_, tail, _ := strings.Cut(strings.TrimLeft(out[keyAt0].text, " \t")[len(key):], ":")
		_, cmt := splitComment(tail)
		out[keyAt0].text = indent + key + ": " + *value + cmt
		return out, nil
	case value == nil:
		return nil, nil
	}
	ins := fmLine{indent + key + ": " + *value, eol}
	out = append(out[:last+1:last+1], append([]fmLine{ins}, lines[last+1:]...)...)
	return out, nil
}

// editFlow edits `{ a: 1, b: x }`; remove reports that the mapping is empty now, and out is "" with
// remove false when there was nothing to remove.
func editFlow(text, key string, value *string) (out string, remove bool, err error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return "", false, errors.New("nav is written in a form zen cannot edit; edit the page by hand")
	}
	m := doc.Content[0]
	var parts []string
	found := false
	for i := 0; i+1 < len(m.Content); i += 2 {
		k, v := m.Content[i], m.Content[i+1]
		if k.Kind != yaml.ScalarNode || v.Kind != yaml.ScalarNode {
			return "", false, errors.New("nav is written in a form zen cannot edit; edit the page by hand")
		}
		kt, err := yaml.Marshal(k)
		if err != nil {
			return "", false, err
		}
		vt, err := yaml.Marshal(v)
		if err != nil {
			return "", false, err
		}
		ks, vs := strings.TrimRight(string(kt), "\n"), strings.TrimRight(string(vt), "\n")
		if k.Value == key {
			found = true
			if value == nil {
				continue
			}
			vs = *value
		}
		parts = append(parts, ks+": "+vs)
	}
	if !found && value != nil {
		parts = append(parts, key+": "+*value)
	}
	if len(parts) == 0 {
		return "", found, nil // found and removed: the mapping is empty now
	}
	return "{ " + strings.Join(parts, ", ") + " }", false, nil
}

// checkEdit reads the result back: same data as before except the one key,
// same body.
func checkEdit(before, after, key string, value *string) error {
	bm, bbody, _ := SplitFrontmatter([]byte(before))
	am, abody, warn := SplitFrontmatter([]byte(after))
	bad := errors.New("the page's frontmatter is written in a way zen cannot edit safely; edit the page by hand")
	if warn != "" || string(bbody) != string(abody) {
		return bad
	}
	want := map[string]any{}
	for k, v := range bm {
		want[k] = v
	}
	nav := map[string]any{}
	switch n := bm["nav"].(type) {
	case nil:
	case map[string]any:
		for k, v := range n {
			nav[k] = v
		}
	default:
		return bad
	}
	if value == nil {
		delete(nav, key)
	} else {
		var v any
		if err := yaml.Unmarshal([]byte(*value), &v); err != nil {
			return fmt.Errorf("%q is not a YAML value", *value)
		}
		nav[key] = v
	}
	if len(nav) == 0 {
		delete(want, "nav")
	} else {
		want["nav"] = nav
	}
	if !reflect.DeepEqual(want, am) {
		return bad
	}
	return nil
}
