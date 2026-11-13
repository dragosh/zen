package plugins

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseAttrs(t *testing.T) {
	good := []struct {
		in   string
		want map[string]string
	}{
		{`{a=1 b="two words" flag}`, map[string]string{"a": "1", "b": "two words", "flag": "true"}},
		{`{}`, map[string]string{}},
		{`{  }`, map[string]string{}},
		{` {theme=neutral} `, map[string]string{"theme": "neutral"}},
		{`{transition=fade}`, map[string]string{"transition": "fade"}},
		{`{expr="a=b" url="x?y=1&z=2"}`, map[string]string{"expr": "a=b", "url": "x?y=1&z=2"}},
		{`{q="say \"hi\" \\ there"}`, map[string]string{"q": `say "hi" \ there`}},
		{`{empty=""}`, map[string]string{"empty": ""}},
		{`{brace="{x}"}`, map[string]string{"brace": "{x}"}},
		{"{a=1\tb=2}", map[string]string{"a": "1", "b": "2"}},
		{`{a=1 a=2}`, map[string]string{"a": "2"}},
		{`{data-id=x_1 Key2=v}`, map[string]string{"data-id": "x_1", "Key2": "v"}},
		{`{flag other}`, map[string]string{"flag": "true", "other": "true"}},
	}
	for _, tc := range good {
		got, err := ParseAttrs(tc.in)
		if err != nil {
			t.Errorf("ParseAttrs(%q): %v", tc.in, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ParseAttrs(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	bad := []string{
		``,
		`a=1`,
		`{a=1`,
		`a=1}`,
		`{a=}`,
		`{=1}`,
		`{a="unterminated}`,
		`{a="x"y}`,
		`{a=x"y}`,
		`{1a=2}`,
		`{a b=}`,
		`{a=1} trailing}`,
		`{a={x}}`,
		`{a,b}`,
	}
	for _, in := range bad {
		if got, err := ParseAttrs(in); err == nil {
			t.Errorf("ParseAttrs(%q) = %v, want error", in, got)
		}
	}
}

func TestScanContainers(t *testing.T) {
	check := func(name, src string, want []Span) {
		t.Helper()
		got := ScanContainers([]byte(src))
		if len(got) != len(want) {
			t.Errorf("%s: got %d spans %+v, want %d", name, len(got), got, len(want))
			return
		}
		for i := range want {
			if want[i].Attrs == nil {
				want[i].Attrs = map[string]string{}
			}
			if got[i].Name != want[i].Name || got[i].Body != want[i].Body ||
				!reflect.DeepEqual(got[i].Attrs, want[i].Attrs) {
				t.Errorf("%s: span %d = %+v, want %+v", name, i, got[i], want[i])
			}
			if want[i].End != 0 && (got[i].Start != want[i].Start || got[i].End != want[i].End) {
				t.Errorf("%s: span %d at [%d,%d), want [%d,%d)", name, i, got[i].Start, got[i].End, want[i].Start, want[i].End)
			}
		}
	}

	src := "intro\n:::slides{transition=fade}\n# First\n---\n# Second\n:::\noutro\n"
	check("basic", src, []Span{{
		Start: 6, End: strings.Index(src, "\noutro"),
		Name: "slides", Body: "# First\n---\n# Second\n",
		Attrs: map[string]string{"transition": "fade"},
	}})
	sp := ScanContainers([]byte(src))[0]
	if got := src[sp.Start:sp.End]; got != ":::slides{transition=fade}\n# First\n---\n# Second\n:::" {
		t.Errorf("span text = %q", got)
	}

	check("no attrs, at EOF without newline", ":::note\nhi\n:::", []Span{{Start: 0, End: 14, Name: "note", Body: "hi\n"}})
	check("empty body", ":::note\n:::\n", []Span{{Start: 0, End: 11, Name: "note", Body: ""}})
	check("crlf", ":::note\r\nhi\r\n:::\r\nx", []Span{{Start: 0, End: 16, Name: "note", Body: "hi\r\n"}})
	check("closing with trailing spaces", ":::note\nhi\n:::  \t\nafter\n", []Span{{Start: 0, End: 16, Name: "note", Body: "hi\n"}})
	check("space before attrs, quoted, trailing spaces", ":::poll {title=\"a = b\" open}  \n- x\n:::\n",
		[]Span{{Name: "poll", Body: "- x\n", Attrs: map[string]string{"title": "a = b", "open": "true"}}})
	check("two in a row", ":::a\n1\n:::\n:::b\n2\n:::\n", []Span{{Name: "a", Body: "1\n"}, {Name: "b", Body: "2\n"}})

	// nesting: outer first, inner reported too
	nested := ":::outer\nbefore\n:::inner{x=1}\ndeep\n:::\nafter\n:::\n"
	check("nested", nested, []Span{
		{Start: 0, End: len(nested) - 1, Name: "outer", Body: "before\n:::inner{x=1}\ndeep\n:::\nafter\n"},
		{Start: 16, End: 38, Name: "inner", Body: "deep\n", Attrs: map[string]string{"x": "1"}},
	})

	check("unclosed", ":::note\nhi\n", nil)
	check("unclosed outer keeps closed inner", ":::outer\n:::inner\nx\n:::\n", []Span{{Name: "inner", Body: "x\n"}})
	check("stray closer", "text\n:::\nmore\n", nil)
	check("indented opener", " :::note\nhi\n:::\n", nil)
	check("not at line start", "see :::note\nhi\n:::\n", nil)
	check("no name", "::: note\nhi\n:::\n", nil)
	check("bad name", ":::Note\nhi\n:::\n", nil)
	check("four colons", "::::note\nhi\n:::\n", nil)
	check("junk after name", ":::note text\nhi\n:::\n", nil)
	check("malformed attrs", ":::note{a=}\nhi\n:::\n", nil)
	check("unterminated attrs", ":::note{a=1\nhi\n:::\n", nil)
	check("indented closer does not close", ":::note\nhi\n :::\n", nil)

	// fenced code blocks hide directives
	check("inside backtick fence", "```\n:::note\nhi\n:::\n```\n", nil)
	check("inside tilde fence with info", "~~~markdown\n:::note\nhi\n:::\n~~~\n", nil)
	check("inside indented, longer fence", "  ````md\n:::note\n```\n:::\n````\n", nil)
	check("unclosed fence hides the rest", "```\n:::note\nhi\n:::\n", nil)
	check("after fence", "```\n:::x\n```\n:::note\nhi\n:::\n", []Span{{Name: "note", Body: "hi\n"}})
	check("fence inside container", ":::note\n```\n:::\n```\nend\n:::\n", []Span{{Name: "note", Body: "```\n:::\n```\nend\n"}})
	check("tilde does not close backtick fence", "```\n~~~\n:::note\nhi\n:::\n```\n", nil)
	check("empty", "", nil)
}

func TestScanInlines(t *testing.T) {
	known := func(n string) bool { return n == "badge" || n == "icon" || n == "am" }
	check := func(name, line string, want []Span) {
		t.Helper()
		got := ScanInlines([]byte(line), known)
		if len(got) != len(want) {
			t.Errorf("%s: got %d spans %+v, want %d", name, len(got), got, len(want))
			return
		}
		for i := range want {
			if want[i].Attrs == nil {
				want[i].Attrs = map[string]string{}
			}
			if !reflect.DeepEqual(got[i], want[i]) {
				t.Errorf("%s: span %d = %+v, want %+v", name, i, got[i], want[i])
			}
		}
	}

	line := "Status: :badge[stable]{color=gray}"
	check("doc example", line, []Span{{Start: 8, End: len(line), Name: "badge", Body: "stable", Attrs: map[string]string{"color": "gray"}}})
	if sp := ScanInlines([]byte(line), known)[0]; line[sp.Start:sp.End] != ":badge[stable]{color=gray}" {
		t.Errorf("span text = %q", line[sp.Start:sp.End])
	}
	check("no attrs", ":icon[home] rest", []Span{{Start: 0, End: 11, Name: "icon", Body: "home"}})
	check("empty text", "x :icon[] y", []Span{{Start: 2, End: 9, Name: "icon", Body: ""}})
	check("two", ":icon[a] and (:badge[b c]{k=\"v w\" f})", []Span{
		{Start: 0, End: 8, Name: "icon", Body: "a"},
		{Start: 14, End: 36, Name: "badge", Body: "b c", Attrs: map[string]string{"k": "v w", "f": "true"}},
	})
	check("nested brackets", ":badge[a [b] c]", []Span{{Start: 0, End: 15, Name: "badge", Body: "a [b] c"}})
	check("escaped bracket", `:badge[a \] b]`, []Span{{Start: 0, End: 14, Name: "badge", Body: "a ] b"}})
	check("brace in quoted attr", `:badge[x]{t="a } b"}!`, []Span{{Start: 0, End: 20, Name: "badge", Body: "x", Attrs: map[string]string{"t": "a } b"}}})
	check("attrs must touch", ":badge[x] {k=v}", []Span{{Start: 0, End: 9, Name: "badge", Body: "x"}})

	check("unknown name", "see :unknown[x]{a=1} here", nil)
	check("prose colon", "note: text and more", nil)
	check("prose colon before brackets", "note: [link](url)", nil)
	check("time", "at 10:30[am] sharp", nil)
	check("word before colon", "foo:badge[x]", nil)
	check("double colon", "::badge[x]", nil)
	check("escaped colon", `\:badge[x]`, nil)
	check("no brackets", ":badge and :icon", nil)
	check("space before bracket", ":badge [x]", nil)
	check("unclosed text", ":badge[x", nil)
	check("uppercase name", ":Badge[x]", nil)
	check("malformed attrs are literal", ":badge[x]{a=}", nil)
	check("unterminated attrs are literal", ":badge[x]{a=1", nil)
	check("nil known", "", nil)
	if got := ScanInlines([]byte(":badge[x]"), nil); len(got) != 0 {
		t.Errorf("nil known matched: %+v", got)
	}

	// code spans
	check("in code span", "use `:badge[x]` here", nil)
	check("in double backtick span", "use ``a ` :badge[x]`` here", nil)
	check("after code span", "`code` :badge[x]", []Span{{Start: 7, End: 16, Name: "badge", Body: "x"}})
	check("unmatched backtick is literal", "a ` b :badge[x]", []Span{{Start: 6, End: 15, Name: "badge", Body: "x"}})
	check("mismatched run is literal", "`` a ` :badge[x]", []Span{{Start: 7, End: 16, Name: "badge", Body: "x"}})
}
