package plugins

import (
	"strings"
	"testing"
)

func TestCheckHelp(t *testing.T) {
	ok := Help{Summary: "Does a thing.", Snippets: []Snippet{{Title: "Basic", Markdown: "```x\nhi\n```"}}}
	if err := CheckHelp(ok); err != nil {
		t.Fatalf("valid help rejected: %v", err)
	}
	if err := CheckHelp(Help{}); err != nil {
		t.Fatalf("empty help is allowed (it is optional): %v", err)
	}
	many := make([]Snippet, 9)
	for i := range many {
		many[i] = Snippet{Title: "t", Markdown: "m"}
	}
	eight := many[:8]
	if err := CheckHelp(Help{Snippets: eight}); err != nil {
		t.Fatalf("8 snippets: %v", err)
	}
	cases := []struct {
		name string
		h    Help
		want string
	}{
		{"summary too long", Help{Summary: strings.Repeat("a", 501)}, "summary"},
		{"too many snippets", Help{Snippets: many}, "8"},
		{"snippet too big", Help{Snippets: []Snippet{{Title: "t", Markdown: strings.Repeat("a", 4097)}}}, "4 KB"},
		{"no markdown", Help{Snippets: []Snippet{{Title: "t"}}}, "markdown"},
		{"no title", Help{Snippets: []Snippet{{Markdown: "m"}}}, "title"},
	}
	for _, c := range cases {
		err := CheckHelp(c.h)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want it to mention %q", c.name, err, c.want)
		}
	}
	if err := CheckHelp(Help{Summary: strings.Repeat("é", 500), Snippets: []Snippet{{Title: "t", Markdown: strings.Repeat("a", 4096)}}}); err != nil {
		t.Errorf("limits are inclusive (summary counts characters): %v", err)
	}
}
