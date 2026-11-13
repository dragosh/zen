package render

import (
	"strings"
	"testing"
)

func TestHTML(t *testing.T) {
	cases := []struct {
		name, in   string
		want, deny []string
	}{
		{"heading", "# Title", []string{`<h1 id="title">Title</h1>`}, nil},
		{"bold", "**b**", []string{"<strong>b</strong>"}, nil},
		{"table", "|a|b|\n|-|-|\n|1|2|", []string{"<table>", "<td>1</td>"}, nil},
		{"tasks", "- [x] done", []string{`type="checkbox"`, "checked"}, nil},
		{"strike", "~~x~~", []string{"<del>x</del>"}, nil},
		{"code class", "```go\nx\n```", []string{`class="language-go"`}, nil},
		{"script", "<script>alert(1)</script>ok", nil, []string{"<script", "alert(1)"}},
		{"handler", `<img src="x" onerror="alert(1)">`, nil, []string{"onerror"}},
		{"js link", "[x](javascript:alert(1))", nil, []string{"javascript:"}},
		{"iframe", `<iframe src="https://e.com"></iframe>`, nil, []string{"<iframe"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := HTML([]byte(c.in))
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range c.want {
				if !strings.Contains(got, w) {
					t.Errorf("missing %q in %q", w, got)
				}
			}
			for _, d := range c.deny {
				if strings.Contains(got, d) {
					t.Errorf("unexpected %q in %q", d, got)
				}
			}
		})
	}
}
