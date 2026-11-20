package config

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func dialectsOf(t *testing.T, src []byte) []string {
	t.Helper()
	var doc struct {
		Markdown struct {
			Dialects []string `yaml:"dialects"`
		} `yaml:"markdown"`
	}
	if err := yaml.Unmarshal(src, &doc); err != nil {
		t.Fatalf("result is not YAML: %v\n%s", err, src)
	}
	return doc.Markdown.Dialects
}

func TestSetDialects(t *testing.T) {
	set := func(src string, ids ...string) string {
		t.Helper()
		out, err := SetDialects([]byte(src), ids)
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		return string(out)
	}

	// a missing or empty file is created with the block
	if got := set("", "myst"); got != "markdown:\n  dialects: [myst]\n" {
		t.Errorf("empty: %q", got)
	}
	// comments, other keys and their order stay; the block is added at the end
	src := "# my folder\nversion: 1\n\nplugins:\n  lint: false   # no Vale here\n# trailing comment\n"
	got := set(src, "myst")
	if !strings.HasPrefix(got, src) || !strings.HasSuffix(got, "markdown:\n  dialects: [myst]\n") {
		t.Errorf("appended:\n%s", got)
	}
	// no trailing newline
	if got := set("version: 1", "myst"); got != "version: 1\nmarkdown:\n  dialects: [myst]\n" {
		t.Errorf("no final newline: %q", got)
	}

	// an existing markdown block keeps its other keys and comments
	src = "version: 1\nmarkdown:\n  # what to read\n  other: 1\nplugins: {}\n"
	got = set(src, "myst")
	if got != "version: 1\nmarkdown:\n  dialects: [myst]\n  # what to read\n  other: 1\nplugins: {}\n" {
		t.Errorf("inserted:\n%s", got)
	}
	// dialects there already: replaced in place, the comment kept
	src = "markdown:\n  dialects: [mkdocs]   # old\n  other: 1\n"
	got = set(src, "mkdocs", "myst")
	if got != "markdown:\n  dialects: [mkdocs, myst]   # old\n  other: 1\n" {
		t.Errorf("replaced:\n%s", got)
	}
	// a block list becomes a flow list
	src = "markdown:\n  dialects:\n    - mkdocs\n    - myst\n  other: 1\nz: 1\n"
	got = set(src, "myst")
	if got != "markdown:\n  dialects: [myst]\n  other: 1\nz: 1\n" {
		t.Errorf("block list:\n%s", got)
	}
	// flow mapping
	got = set("markdown: { dialects: [] }\nz: 1\n", "myst")
	if !reflect.DeepEqual(dialectsOf(t, []byte(got)), []string{"myst"}) || !strings.Contains(got, "z: 1\n") {
		t.Errorf("flow mapping:\n%s", got)
	}
	got = set("markdown: { dialects: [myst], other: 1 }\n", "myst", "mkdocs")
	if !reflect.DeepEqual(dialectsOf(t, []byte(got)), []string{"myst", "mkdocs"}) || !strings.Contains(got, "other") {
		t.Errorf("flow mapping with other key:\n%s", got)
	}
	// a null markdown:
	got = set("markdown:\nz: 1\n", "myst")
	if got != "markdown:\n  dialects: [myst]\nz: 1\n" {
		t.Errorf("null markdown:\n%s", got)
	}

	// turning the last one off removes the key, and the block when it is empty
	if got := set("version: 1\nmarkdown:\n  dialects: [myst]\nz: 1\n"); got != "version: 1\nz: 1\n" {
		t.Errorf("removed:\n%s", got)
	}
	if got := set("markdown:\n  dialects: [myst]\n  other: 1\n"); got != "markdown:\n  other: 1\n" {
		t.Errorf("removed, other kept:\n%s", got)
	}
	if got := set("markdown:\n  dialects: [myst]\n"); got != "" {
		t.Errorf("removed, nothing left: %q", got)
	}
	// nothing to remove: unchanged
	if got := set("version: 1\n"); got != "version: 1\n" {
		t.Errorf("nothing to remove: %q", got)
	}
	if got := set(""); got != "" {
		t.Errorf("empty, nothing to set: %q", got)
	}
	// setting what is there changes nothing
	src = "markdown:\n  dialects: [myst]\n"
	if got := set(src, "myst"); got != src {
		t.Errorf("same: %q", got)
	}

	// CRLF and a BOM
	got = set("\xEF\xBB\xBFversion: 1\r\nz: 2\r\n", "myst")
	if got != "\xEF\xBB\xBFversion: 1\r\nz: 2\r\nmarkdown:\r\n  dialects: [myst]\r\n" {
		t.Errorf("crlf: %q", got)
	}

	// what cannot be edited safely is an error and nothing is written
	for _, bad := range []string{
		"markdown: [unclosed\n",
		"- a list\n- not a mapping\n",
		"x: &a 1\nmarkdown: *a\n",
		"markdown:\n  dialects: &d [myst]\nother: *d\n",
	} {
		if out, err := SetDialects([]byte(bad), []string{"myst"}); err == nil {
			t.Errorf("%q: no error, got %q", bad, out)
		}
	}
	if _, err := SetDialects(nil, []string{"Bad Id"}); err == nil {
		t.Error("invalid id accepted")
	}
}
