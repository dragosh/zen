package myst

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/zenmark-org/zenmd/engine/dialects"
)

func scan(src string) []dialects.Span { return New().Scan([]byte(src)) }

func one(t *testing.T, src string) dialects.Span {
	t.Helper()
	sp := scan(src)
	if len(sp) != 1 {
		t.Fatalf("%q: %d spans, want 1: %+v", src, len(sp), sp)
	}
	return sp[0]
}

func TestFence(t *testing.T) {
	src := "intro\n\n```{note} Remember\nBody text.\n```\n\nafter\n"
	sp := one(t, src)
	if sp.Name != "note" || sp.Body != "Body text.\n" || sp.Code {
		t.Errorf("span: %+v", sp)
	}
	if got := src[sp.Start:sp.End]; got != "```{note} Remember\nBody text.\n```" {
		t.Errorf("range %q", got)
	}
	tilde := one(t, "~~~{tip}\nx\n~~~\n")
	if tilde.Name != "tip" || tilde.Body != "x\n" {
		t.Errorf("tilde: %+v", tilde)
	}
	// the closer is the same character, at least as long, with nothing after it
	sp = one(t, "```{note}\na\n~~~\n``` x\n```\n")
	if sp.Body != "a\n~~~\n``` x\n" {
		t.Errorf("closer: %q", sp.Body)
	}
	// an unclosed fence runs to the end of the page
	sp = one(t, "```{note}\nno end\nstill body")
	if sp.Body != "no end\nstill body" || sp.End != len("```{note}\nno end\nstill body") {
		t.Errorf("unclosed: %+v", sp)
	}
	// 1 to 3 spaces of indent
	if sp := one(t, "   ```{note}\nx\n   ```\n"); sp.Name != "note" {
		t.Errorf("indented: %+v", sp)
	}
	if len(scan("    ```{note}\nx\n    ```\n")) != 0 {
		t.Error("4 spaces is an indented code block")
	}
	// CRLF
	sp = one(t, "```{note} T\r\nbody\r\n```\r\nafter\r\n")
	if sp.Attrs["title"] != "T" || sp.Body != "body\r\n" {
		t.Errorf("crlf: %+v", sp)
	}
	// not directives
	for _, s := range []string{"```{ note }\nx\n```\n", "```note\nx\n```\n", "```{}\nx\n```\n", "```{Note}\nx\n```\n", "``{note}\nx\n``\n", "```{note} `a`\nx\n```\n"} {
		if got := scan(s); len(got) != 0 {
			t.Errorf("%q: %+v", s, got)
		}
	}
}

func TestArgument(t *testing.T) {
	sp := one(t, "```{note} Remember me\nx\n```\n")
	if sp.Attrs["title"] != "Remember me" || sp.Attrs["arg"] != "Remember me" {
		t.Errorf("attrs: %v", sp.Attrs)
	}
	sp = one(t, "```{note} Remember\n:title: Explicit\nx\n```\n")
	if sp.Attrs["title"] != "Explicit" || sp.Attrs["arg"] != "Remember" {
		t.Errorf("option title wins: %v", sp.Attrs)
	}
	sp = one(t, "```{note}\nx\n```\n")
	if len(sp.Attrs) != 0 || sp.Attrs == nil {
		t.Errorf("no argument: %#v", sp.Attrs)
	}
}

func TestNesting(t *testing.T) {
	src := "````{note} Outer\n```{tip} Inner\ninner body\n```\n````\n\n```{warning}\nw\n```\n"
	sp := scan(src)
	if len(sp) != 2 || sp[0].Name != "note" || sp[1].Name != "warning" {
		t.Fatalf("spans: %+v", sp)
	}
	if sp[0].Body != "```{tip} Inner\ninner body\n```\n" {
		t.Errorf("outer body %q", sp[0].Body)
	}
	inner := one(t, sp[0].Body)
	if inner.Name != "tip" || inner.Body != "inner body\n" {
		t.Errorf("inner: %+v", inner)
	}
}

func TestColonFence(t *testing.T) {
	src := ":::{note} Title\nbody\n:::\n"
	sp := one(t, src)
	if sp.Name != "note" || sp.Attrs["title"] != "Title" || sp.Body != "body\n" || src[sp.Start:sp.End] != ":::{note} Title\nbody\n:::" {
		t.Errorf("colon fence: %+v", sp)
	}
	// closes on at least as many colons; fewer colons are body
	sp = one(t, "::::{note}\n:::{tip}\nx\n:::\n::::::\n")
	if sp.Name != "note" || sp.Body != ":::{tip}\nx\n:::\n" {
		t.Errorf("long colon fence: %+v", sp)
	}
	// zen's own syntax is not this dialect's
	for _, s := range []string{":::note\nx\n:::\n", ":::note{title=\"T\"}\nx\n:::\n", ":::\nx\n:::\n"} {
		if got := scan(s); len(got) != 0 {
			t.Errorf("%q: %+v", s, got)
		}
	}
	// a code fence inside a colon fence keeps its colons
	sp = one(t, ":::{note}\n```\n:::\n```\nafter\n:::\n")
	if sp.Body != "```\n:::\n```\nafter\n" {
		t.Errorf("code inside: %q", sp.Body)
	}
}

func TestOptions(t *testing.T) {
	src := "```{note} T\n:class: wide\n:open:\n:1bad: x\n:title: Mine\n\n:later: x\nbody\n```\n"
	sp := one(t, src)
	want := map[string]string{"title": "Mine", "arg": "T", "class": "wide", "open": "true"}
	if !reflect.DeepEqual(sp.Attrs, want) {
		t.Errorf("attrs %v, want %v", sp.Attrs, want)
	}
	if sp.Body != "\n:later: x\nbody\n" {
		t.Errorf("body %q", sp.Body)
	}
	if len(sp.Warnings) != 1 || !strings.Contains(sp.Warnings[0], "1bad") {
		t.Errorf("warnings %v", sp.Warnings)
	}
	// a value keeps its colons and spaces
	sp = one(t, ":::{note}\n:link: a: b c\nx\n:::\n")
	if sp.Attrs["link"] != "a: b c" || sp.Body != "x\n" {
		t.Errorf("value: %+v", sp)
	}
	// text that merely starts with a colon is body
	sp = one(t, ":::{note}\n:smile: is an emoji\nx\n:::\n")
	if sp.Attrs["smile"] != "is an emoji" {
		t.Errorf("emoji-like: %+v", sp)
	}
	sp = one(t, "```{note}\n:not an option\nx\n```\n")
	if sp.Body != ":not an option\nx\n" {
		t.Errorf("not an option: %q", sp.Body)
	}
}

func TestCodeDirectives(t *testing.T) {
	for _, name := range []string{"code-block", "code", "code-cell"} {
		sp := one(t, "```{"+name+"} python\nprint(1)\n```\n")
		if !sp.Code || sp.Lang != "python" || sp.Body != "print(1)\n" || sp.Name != name {
			t.Errorf("%s: %+v", name, sp)
		}
	}
	// options are not code; no language is allowed
	sp := one(t, "```{code-block} go\n:linenos:\nx := 1\n```\n")
	if sp.Body != "x := 1\n" || sp.Lang != "go" {
		t.Errorf("options: %+v", sp)
	}
	if sp := one(t, "```{code}\nx\n```\n"); !sp.Code || sp.Lang != "" {
		t.Errorf("no lang: %+v", sp)
	}
	// the language is a single safe word
	if sp := one(t, "```{code-block} py<script> x\ny\n```\n"); sp.Lang != "" {
		t.Errorf("unsafe lang %q", sp.Lang)
	}
	// a fence inside a code directive is code, not scanned
	sp = one(t, "````{code-block} md\n```{note}\nx\n```\n````\n")
	if sp.Code != true || sp.Body != "```{note}\nx\n```\n" {
		t.Errorf("fence in code: %+v", sp)
	}
}

func TestInsideCode(t *testing.T) {
	for _, src := range []string{
		"````\n```{note}\nx\n```\n````\n",
		"~~~\n```{note}\nx\n```\n~~~\n",
		"```python\n:::{note}\nx\n:::\n```\n",
		"```\n```{note}\n",
	} {
		if got := scan(src); len(got) != 0 {
			t.Errorf("%q: %+v", src, got)
		}
	}
	// after the code fence closes, directives count again
	sp := one(t, "```\ncode\n```\n```{note}\nx\n```\n")
	if sp.Name != "note" {
		t.Errorf("after code: %+v", sp)
	}
}

func TestLinear(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 10000; i++ {
		fmt.Fprintf(&b, "```{note} n%d\n:class: c\nbody %d\n```\n\n:::{tip}\nx\n:::\n\n", i, i)
	}
	start := time.Now()
	if got := scan(b.String()); len(got) != 20000 {
		t.Fatalf("%d spans", len(got))
	}
	// openers that never close, and long runs of options and colons
	bad := strings.Repeat("```{note}\n", 20000) + strings.Repeat(":::{x}\n", 20000)
	scan(bad)
	scan(strings.Repeat(":a: b\n", 50000))
	scan(strings.Repeat("```{a} b\n:k: v\n", 20000))
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("scanning took %v", d)
	}
}
