package plugins

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Span is one directive found in markdown source. Start and End are byte
// offsets (End exclusive) into the scanned input.
//
// For a container, the span runs from the start of the opening `:::name` line
// to the end of the closing `:::` line (line terminator excluded) and Body is
// the raw markdown between those two lines. For an inline directive the span
// covers `:name[text]{attrs}` and Body is the text. Attrs is never nil.
type Span struct {
	Start, End int
	Name, Body string
	Attrs      map[string]string
}

// ParseAttrs parses an attribute list including its braces:
// `{key=value key2="two words" flag}`. A bare flag is "true". Keys match
// [A-Za-z][A-Za-z0-9_-]*; unquoted values run to the next whitespace and may
// not contain quotes or braces; quoted values support \" and \\. A repeated
// key keeps its last value. Malformed input returns an error and callers
// treat the text as literal.
func ParseAttrs(s string) (map[string]string, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return nil, errors.New("plugins: attributes must be enclosed in { }")
	}
	s = s[1 : len(s)-1]
	out := map[string]string{}
	i := 0
	for {
		for i < len(s) && isSpace(s[i]) {
			i++
		}
		if i == len(s) {
			return out, nil
		}
		start := i
		for i < len(s) && isKeyByte(s[i], i == start) {
			i++
		}
		key := s[start:i]
		if key == "" {
			return nil, fmt.Errorf("plugins: invalid attribute name at %q", s[start:])
		}
		if i == len(s) || isSpace(s[i]) {
			out[key] = "true"
			continue
		}
		if s[i] != '=' {
			return nil, fmt.Errorf("plugins: invalid attribute name at %q", s[start:])
		}
		i++
		if i < len(s) && s[i] == '"' {
			i++
			var b strings.Builder
			closed := false
			for i < len(s) {
				c := s[i]
				if c == '\\' && i+1 < len(s) && (s[i+1] == '"' || s[i+1] == '\\') {
					b.WriteByte(s[i+1])
					i += 2
					continue
				}
				i++
				if c == '"' {
					closed = true
					break
				}
				b.WriteByte(c)
			}
			if !closed {
				return nil, fmt.Errorf("plugins: unterminated quote in attribute %q", key)
			}
			if i < len(s) && !isSpace(s[i]) {
				return nil, fmt.Errorf("plugins: unexpected text after attribute %q", key)
			}
			out[key] = b.String()
			continue
		}
		start = i
		for i < len(s) && !isSpace(s[i]) {
			if c := s[i]; c == '"' || c == '{' || c == '}' {
				return nil, fmt.Errorf("plugins: invalid character %q in attribute %q", c, key)
			}
			i++
		}
		if i == start {
			return nil, fmt.Errorf("plugins: attribute %q has no value", key)
		}
		out[key] = s[start:i]
	}
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func isKeyByte(c byte, first bool) bool {
	if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
		return true
	}
	return !first && (c >= '0' && c <= '9' || c == '_' || c == '-')
}

// scanName returns the end of the directive name ([a-z][a-z0-9-]*) starting
// at b[i], or i when there is none.
func scanName(b []byte, i int) int {
	if i >= len(b) || b[i] < 'a' || b[i] > 'z' {
		return i
	}
	j := i + 1
	for j < len(b) && (b[j] >= 'a' && b[j] <= 'z' || b[j] >= '0' && b[j] <= '9' || b[j] == '-') {
		j++
	}
	return j
}

// attrsEnd returns the index just past the `}` closing the attribute list
// that opens at b[i] == '{', honouring quoted values, or -1.
func attrsEnd(b []byte, i int) int {
	quoted := false
	for j := i + 1; j < len(b); j++ {
		switch c := b[j]; {
		case quoted && c == '\\':
			j++
		case c == '"':
			quoted = !quoted
		case !quoted && c == '}':
			return j + 1
		case c == '\n':
			return -1
		}
	}
	return -1
}

// ScanContainers finds `:::name{attrs}` … `:::` container blocks in src.
//
// The opening fence starts at the beginning of a line and is followed only by
// the optional attributes; the closing fence is `:::` alone on a line
// (trailing spaces allowed). Containers nest by matching depth and every
// closed container is returned, nested ones included, ordered by Start (an
// outer container comes before the containers in its Body). Unclosed
// containers, openers with malformed attributes and anything inside a fenced
// code block are not containers.
func ScanContainers(src []byte) []Span {
	type open struct {
		start, bodyStart int
		name             string
		attrs            map[string]string
	}
	var (
		out       []Span
		stack     []open
		fenceChar byte // non-zero while inside a fenced code block
		fenceLen  int
	)
	for pos := 0; pos < len(src); {
		lineStart := pos
		next := len(src)
		if nl := bytes.IndexByte(src[pos:], '\n'); nl >= 0 {
			next = pos + nl + 1
		}
		pos = next
		line := bytes.TrimRight(src[lineStart:next], " \t\r\n")
		lineEnd := lineStart + len(bytes.TrimRight(src[lineStart:next], "\r\n"))

		if c, n, rest := codeFence(line); n > 0 {
			if fenceChar == 0 {
				// a backtick fence may not have backticks in its info string
				if c != '`' || bytes.IndexByte(rest, '`') < 0 {
					fenceChar, fenceLen = c, n
					continue
				}
			} else if c == fenceChar && n >= fenceLen && len(rest) == 0 {
				fenceChar = 0
				continue
			}
		}
		if fenceChar != 0 || !bytes.HasPrefix(line, []byte(":::")) {
			continue
		}

		if len(line) == 3 {
			if len(stack) == 0 {
				continue // stray closer
			}
			o := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			out = append(out, Span{
				Start: o.start, End: lineEnd,
				Name: o.name, Body: string(src[o.bodyStart:lineStart]),
				Attrs: o.attrs,
			})
			continue
		}

		nameEnd := scanName(line, 3)
		if nameEnd == 3 {
			continue
		}
		attrs := map[string]string{}
		if rest := bytes.TrimLeft(line[nameEnd:], " \t"); len(rest) > 0 {
			if rest[0] != '{' || attrsEnd(rest, 0) != len(rest) {
				continue
			}
			parsed, err := ParseAttrs(string(rest))
			if err != nil {
				continue
			}
			attrs = parsed
		}
		stack = append(stack, open{start: lineStart, bodyStart: next, name: string(line[3:nameEnd]), attrs: attrs})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

// codeFence reports whether line (right-trimmed) starts a code fence: up to
// three spaces, then at least three backticks or tildes. It returns the fence
// character, its length and the text after it; n is 0 when it is no fence.
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

// ScanInlines finds inline directives `:name[text]{attrs}` (attrs optional)
// in one line of markdown. A directive is reported only when known(name) is
// true, it is not inside a code span, and its colon is not escaped (`\:`) or
// preceded by a letter, digit or another colon — so prose such as
// `note: text` or `10:30[am]` is never affected. Text may contain balanced
// brackets, or `\[` / `\]`. A directive with malformed attributes is left as
// literal text.
func ScanInlines(line []byte, known func(string) bool) []Span {
	var out []Span
	if known == nil {
		return out
	}
	for i := 0; i < len(line); {
		switch line[i] {
		case '\\':
			i += 2
			continue
		case '`':
			n := 1
			for i+n < len(line) && line[i+n] == '`' {
				n++
			}
			i = codeSpanEnd(line, i, n)
			continue
		case ':':
			if i > 0 && (isAlnum(line[i-1]) || line[i-1] == ':') {
				break
			}
			if sp, ok := inlineAt(line, i, known); ok {
				out = append(out, sp)
				i = sp.End
				continue
			}
		}
		i++
	}
	return out
}

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// codeSpanEnd returns the index after the code span opened by the run of n
// backticks at line[i], or after the run itself when no equal run closes it.
func codeSpanEnd(line []byte, i, n int) int {
	for j := i + n; j < len(line); {
		if line[j] != '`' {
			j++
			continue
		}
		m := 1
		for j+m < len(line) && line[j+m] == '`' {
			m++
		}
		if m == n {
			return j + m
		}
		j += m
	}
	return i + n
}

// inlineAt parses a directive whose colon is at line[i].
func inlineAt(line []byte, i int, known func(string) bool) (Span, bool) {
	nameEnd := scanName(line, i+1)
	if nameEnd == i+1 || nameEnd >= len(line) || line[nameEnd] != '[' {
		return Span{}, false
	}
	name := string(line[i+1 : nameEnd])
	if !known(name) {
		return Span{}, false
	}
	var text []byte
	end := -1
	depth := 1
scan:
	for j := nameEnd + 1; j < len(line); j++ {
		c := line[j]
		switch {
		case c == '\\' && j+1 < len(line) && (line[j+1] == '[' || line[j+1] == ']'):
			j++
			c = line[j]
		case c == '[':
			depth++
		case c == ']':
			depth--
			if depth == 0 {
				end = j + 1
				break scan
			}
		}
		text = append(text, c)
	}
	if end < 0 {
		return Span{}, false
	}
	attrs := map[string]string{}
	if end < len(line) && line[end] == '{' {
		ae := attrsEnd(line, end)
		if ae < 0 {
			return Span{}, false
		}
		parsed, err := ParseAttrs(string(line[end:ae]))
		if err != nil {
			return Span{}, false
		}
		attrs, end = parsed, ae
	}
	return Span{Start: i, End: end, Name: name, Body: string(text), Attrs: attrs}, true
}
