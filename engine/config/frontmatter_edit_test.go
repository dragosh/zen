package config

import (
	"reflect"
	"strings"
	"testing"
)

func sp(s string) *string { return &s }

func TestSetNavKey(t *testing.T) {
	cases := []struct {
		name, src, key string
		val            *string
		want           string
	}{
		{"no frontmatter", "# Hi\n\ntext\n", "order", sp("10"),
			"---\nnav:\n  order: 10\n---\n# Hi\n\ntext\n"},
		{"no frontmatter, empty file", "", "order", sp("10"),
			"---\nnav:\n  order: 10\n---\n"},
		{"no frontmatter, CRLF", "# Hi\r\n\r\ntext\r\n", "order", sp("20"),
			"---\r\nnav:\r\n  order: 20\r\n---\r\n# Hi\r\n\r\ntext\r\n"},
		{"no frontmatter, BOM", "\xEF\xBB\xBF# Hi\n", "hidden", sp("true"),
			"\xEF\xBB\xBF---\nnav:\n  hidden: true\n---\n# Hi\n"},
		{"unclosed fence is not frontmatter", "---\ntitle: x\n", "order", sp("10"),
			"---\nnav:\n  order: 10\n---\n---\ntitle: x\n"},
		{"frontmatter without nav", "---\ntitle: Guide # the title\nslug: g\n---\n# Guide\n", "order", sp("30"),
			"---\ntitle: Guide # the title\nslug: g\nnav:\n  order: 30\n---\n# Guide\n"},
		{"empty block", "---\n---\nbody\n", "order", sp("10"),
			"---\nnav:\n  order: 10\n---\nbody\n"},
		{"dots close the block", "---\ntitle: x\n...\nbody\n", "order", sp("10"),
			"---\ntitle: x\nnav:\n  order: 10\n...\nbody\n"},
		{"CRLF frontmatter", "---\r\ntitle: x\r\n---\r\nbody\r\n", "order", sp("10"),
			"---\r\ntitle: x\r\nnav:\r\n  order: 10\r\n---\r\nbody\r\n"},
		{"nav block without the key", "---\ntitle: x\nnav:\n  title: Setup\nplugins:\n  codeblock: false\n---\n", "order", sp("20"),
			"---\ntitle: x\nnav:\n  title: Setup\n  order: 20\nplugins:\n  codeblock: false\n---\n"},
		{"nav block keeps four-space indent", "---\nnav:\n    title: Setup\n---\n", "order", sp("20"),
			"---\nnav:\n    title: Setup\n    order: 20\n---\n"},
		{"insert goes after the last child, before comments and blank lines", "---\nnav:\n  title: A\n\n# about plugins\nplugins: {}\n---\n", "order", sp("10"),
			"---\nnav:\n  title: A\n  order: 10\n\n# about plugins\nplugins: {}\n---\n"},
		{"replace in place keeps comment", "---\nnav:\n  order: 5   # first\n  title: A\n---\n", "order", sp("10"),
			"---\nnav:\n  order: 10   # first\n  title: A\n---\n"},
		{"replace a quoted value", "---\nnav:\n  order: \"5\" # q\n---\n", "order", sp("10"),
			"---\nnav:\n  order: 10 # q\n---\n"},
		{"empty nav with comment", "---\nnav: # sidebar\ntitle: x\n---\n", "order", sp("10"),
			"---\nnav: # sidebar\n  order: 10\ntitle: x\n---\n"},
		{"nav as a one-line mapping", "---\nnav: { title: Setup }  # c\n---\n", "order", sp("10"),
			"---\nnav: { title: Setup, order: 10 }  # c\n---\n"},
		{"nav one-line mapping, replace", "---\nnav: { order: 2, title: \"A: b\" }\n---\n", "order", sp("10"),
			"---\nnav: { order: 10, title: \"A: b\" }\n---\n"},
		{"empty one-line mapping", "---\nnav: {}\n---\n", "hidden", sp("true"),
			"---\nnav: { hidden: true }\n---\n"},
		{"remove a key", "---\nnav:\n  hidden: true\n  order: 3\n---\n", "hidden", nil,
			"---\nnav:\n  order: 3\n---\n"},
		{"removing the last key removes nav", "---\ntitle: x\nnav:\n  hidden: true # why\nslug: s\n---\nbody\n", "hidden", nil,
			"---\ntitle: x\nslug: s\n---\nbody\n"},
		{"removing the last key of a one-line mapping removes nav", "---\ntitle: x\nnav: { hidden: true }\n---\n", "hidden", nil,
			"---\ntitle: x\n---\n"},
		{"removing a missing key changes nothing", "---\ntitle: x\n---\nbody\n", "hidden", nil,
			"---\ntitle: x\n---\nbody\n"},
		{"removing from a page without frontmatter changes nothing", "# T\n", "hidden", nil, "# T\n"},
		{"body that looks like frontmatter is untouched", "---\ntitle: x\n---\nbody\n\n---\nnav:\n  order: 1\n---\n", "order", sp("10"),
			"---\ntitle: x\nnav:\n  order: 10\n---\nbody\n\n---\nnav:\n  order: 1\n---\n"},
		{"unicode and long values", "---\ntitle: Über — 日本語\n---\n", "order", sp("10"),
			"---\ntitle: Über — 日本語\nnav:\n  order: 10\n---\n"},
	}
	for _, c := range cases {
		got, err := SetNavKey([]byte(c.src), c.key, c.val)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if string(got) != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

// The meaning of the file is the same apart from the one key.
func TestSetNavKeyKeepsTheRest(t *testing.T) {
	src := "---\ntitle: T\nlist: [1, 2]\nanchors: &a {x: 1}\nuse: *a\nnav:\n  title: A\n  parent: guides/index.md\n---\n# Body\n\n- a\n"
	got, err := SetNavKey([]byte(src), "order", sp("40"))
	if err != nil {
		t.Fatal(err)
	}
	before, _, _ := SplitFrontmatter([]byte(src))
	after, body, warn := SplitFrontmatter(got)
	if warn != "" || string(body) != "# Body\n\n- a\n" {
		t.Fatalf("body %q warn %q", body, warn)
	}
	nav := before["nav"].(map[string]any)
	nav["order"] = 40
	if !reflect.DeepEqual(before, after) {
		t.Errorf("before %v\nafter %v", before, after)
	}
}

func TestSetNavKeyRefuses(t *testing.T) {
	for name, src := range map[string]string{
		"invalid yaml":        "---\ntitle: [unclosed\n---\nbody\n",
		"not a mapping":       "---\n- a\n- b\n---\nbody\n",
		"nav is a scalar":     "---\nnav: 5\n---\n",
		"nav is a list":       "---\nnav:\n  - a\n---\n",
		"nav is an alias":     "---\nbase: &b {order: 1}\nnav: *b\n---\n",
		"nested flow value":   "---\nnav: { title: [a, b] }\n---\n",
		"nav key twice":       "---\nnav:\n  order: 1\nnav:\n  order: 2\n---\n",
		"order is a mapping":  "---\nnav:\n  order:\n    x: 1\n---\n",
		"multi-line flow nav": "---\nnav: {\n  title: A }\n---\n",
	} {
		if got, err := SetNavKey([]byte(src), "order", sp("10")); err == nil {
			t.Errorf("%s: no error, got %q", name, got)
		}
	}
}

func TestSetNavKeyDoesNotModifyItsInput(t *testing.T) {
	src := []byte("---\ntitle: x\n---\nbody\n")
	keep := string(src)
	if _, err := SetNavKey(src, "order", sp("10")); err != nil || string(src) != keep {
		t.Fatalf("%v %q", err, src)
	}
}

func TestSetNavKeyIsIdempotent(t *testing.T) {
	once, err := SetNavKey([]byte("# T\n"), "order", sp("10"))
	if err != nil {
		t.Fatal(err)
	}
	twice, err := SetNavKey(once, "order", sp("10"))
	if err != nil || string(once) != string(twice) {
		t.Fatalf("%v\n%q\n%q", err, once, twice)
	}
	if !strings.HasPrefix(string(once), "---\nnav:\n  order: 10\n---\n") {
		t.Fatalf("%q", once)
	}
}

func TestSetNavKeyRemoveFromEmptyFlow(t *testing.T) {
	src := "---\nnav: {}\n---\n"
	got, err := SetNavKey([]byte(src), "hidden", nil)
	if err != nil || string(got) != src {
		t.Fatalf("%q %v", got, err)
	}
}

func TestSetNavKeyRemovingTheOnlyKeyRemovesTheBlock(t *testing.T) {
	got, err := SetNavKey([]byte("---\nnav:\n  hidden: true\n---\n# T\n"), "hidden", nil)
	if err != nil || string(got) != "# T\n" {
		t.Fatalf("%q %v", got, err)
	}
	// A block that was empty to begin with is left as it is.
	got, err = SetNavKey([]byte("---\n---\n# T\n"), "hidden", nil)
	if err != nil || string(got) != "---\n---\n# T\n" {
		t.Fatalf("%q %v", got, err)
	}
}
