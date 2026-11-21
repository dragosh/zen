package quiz

import (
	"os"
	"strings"
	"testing"
)

// The demo page renders: its working quiz has five questions, the results
// block is there and the key stays out of the live part of the page.
func TestDemoPage(t *testing.T) {
	src, err := os.ReadFile("../../../examples/showcase/plugins/quiz.md")
	if err != nil {
		t.Skip(err)
	}
	p, err := newRenderer(t, New()).Render("plugins/quiz.md", src)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(p.HTML, `class="zq-q `) != 5 || !strings.Contains(p.HTML, `class="zq-results"`) || strings.Contains(p.HTML, "zen-plugin-error") {
		t.Errorf("%s", p.HTML)
	}
	live := strings.SplitN(p.HTML, `data-zen-plugin="quiz"`, 2)[1]
	for _, leak := range []string{"(x)", "[x]", "Because", "Every plugin folder needs one"} {
		if strings.Contains(live, leak) {
			t.Errorf("live quiz leaks %q", leak)
		}
	}
}
