package render

import "testing"

func TestTitleKeepsAngleBracketsAndDecodes(t *testing.T) {
	cases := map[string]string{
		"# The <div> tag":             "The <div> tag",
		"# Generics List<T>":          "Generics List<T>",
		"# Use `<div>` here":          "Use <div> here",
		"# Escaped \\<b\\> text":      "Escaped <b> text",
		"# A &lt;b&gt; &amp; more":    "A <b> & more",
		"# <script>alert(1)</script>": "<script>alert(1)</script>",
	}
	for src, want := range cases {
		p, err := New(nil, nil, nil).Render("x.md", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		if p.Title != want {
			t.Errorf("%q: title %q, want %q", src, p.Title, want)
		}
	}
}
