package quiz

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []Question // Kind, Text, Options, Correct, Answer, Pattern, Explanation compared
		err  string
	}{
		{
			name: "single choice with explanation",
			body: "1. Which method is idempotent?\n   - ( ) POST\n   - (x) PUT\n   - ( ) PATCH\n   > PUT replaces the resource.\n   > Repeating it changes nothing.\n",
			want: []Question{{Kind: Single, Text: "Which method is idempotent?", Options: []string{"POST", "PUT", "PATCH"}, Correct: []int{1}, Explanation: "PUT replaces the resource. Repeating it changes nothing."}},
		},
		{
			name: "multiple choice",
			body: "1. Select the 2xx codes.\n   - [x] 200\n   - [x] 204\n   - [ ] 301\n",
			want: []Question{{Kind: Multi, Text: "Select the 2xx codes.", Options: []string{"200", "204", "301"}, Correct: []int{0, 1}}},
		},
		{
			name: "free text is normalised",
			body: "1. What does TLS stand for? =   Transport   LAYER Security \n",
			want: []Question{{Kind: Text, Text: "What does TLS stand for?", Answer: "transport layer security"}},
		},
		{
			name: "regex answer",
			body: "1. Name a verb = /^(get|post)$/\n",
			want: []Question{{Kind: Text, Text: "Name a verb", Pattern: "^(get|post)$"}},
		},
		{
			name: "equals sign inside the question",
			body: "1. What is x = 1 plus 1? = 2\n",
			want: []Question{{Kind: Text, Text: "What is x = 1 plus 1?", Answer: "2"}},
		},
		{
			name: "star bullets, paren numbers, blank lines, continuation",
			body: "\n1) First line\n   continues here\n\n   * ( ) a\n   * (X) b\n\n2) Second = yes\n",
			want: []Question{
				{Kind: Single, Text: "First line continues here", Options: []string{"a", "b"}, Correct: []int{1}},
				{Kind: Text, Text: "Second", Answer: "yes"},
			},
		},
		{name: "empty body", body: "  \n", err: "no questions"},
		{name: "text before first question", body: "hello\n1. Q = a\n", err: "before the first question"},
		{name: "single without a right answer", body: "1. Q\n - ( ) a\n - ( ) b\n", err: "exactly one"},
		{name: "single with two right answers", body: "1. Q\n - (x) a\n - (x) b\n", err: "exactly one"},
		{name: "multiple without a right answer", body: "1. Q\n - [ ] a\n - [ ] b\n", err: "at least one"},
		{name: "mixed option kinds", body: "1. Q\n - (x) a\n - [x] b\n", err: "mixes"},
		{name: "question with nothing to answer", body: "1. Just a statement\n", err: "no options"},
		{name: "options and an answer", body: "1. Q = a\n - (x) a\n - ( ) b\n", err: "both"},
		{name: "single option", body: "1. Q\n - (x) a\n", err: "two options"},
		{name: "bad regex", body: "1. Q = /(/\n", err: "pattern"},
		{name: "regex too long", body: "1. Q = /" + strings.Repeat("a", 201) + "/\n", err: "200"},
		{name: "empty answer", body: "1. Q = \n", err: "no options"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q, err := Parse(c.body)
			if c.err != "" {
				if err == nil || !strings.Contains(err.Error(), c.err) {
					t.Fatalf("err %v, want containing %q", err, c.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(q.Questions) != len(c.want) {
				t.Fatalf("got %d questions, want %d: %+v", len(q.Questions), len(c.want), q.Questions)
			}
			for i, w := range c.want {
				g := q.Questions[i]
				if g.Kind != w.Kind || g.Text != w.Text || g.Answer != w.Answer || g.Pattern != w.Pattern || g.Explanation != w.Explanation ||
					strings.Join(g.Options, "|") != strings.Join(w.Options, "|") || len(g.Correct) != len(w.Correct) {
					t.Fatalf("q%d got %+v\nwant %+v", i, g, w)
				}
				for j := range w.Correct {
					if g.Correct[j] != w.Correct[j] {
						t.Fatalf("q%d correct %v want %v", i, g.Correct, w.Correct)
					}
				}
			}
		})
	}
}

func TestGrade(t *testing.T) {
	q, err := Parse("1. A\n - ( ) a\n - (x) b\n - ( ) c\n2. B\n - [x] a\n - [ ] b\n - [x] c\n3. C = Transport Layer Security\n4. D = /^(get|post)\\d*$/\n")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		q    int
		a    Answer
		want bool
	}{
		{0, Answer{Choices: []int{1}}, true}, {0, Answer{Choices: []int{0}}, false}, {0, Answer{Choices: []int{1, 2}}, false}, {0, Answer{}, false},
		{1, Answer{Choices: []int{2, 0}}, true}, {1, Answer{Choices: []int{0}}, false}, {1, Answer{Choices: []int{0, 1, 2}}, false}, {1, Answer{Choices: []int{0, 0, 2}}, true},
		{1, Answer{Choices: []int{9}}, false},
		{2, Answer{Text: "  transport LAYER   security"}, true}, {2, Answer{Text: "transport security"}, false}, {2, Answer{}, false},
		{3, Answer{Text: "POST12"}, true}, {3, Answer{Text: "put"}, false}, {3, Answer{Text: "xget"}, false},
	}
	for _, c := range cases {
		if got := q.Questions[c.q].Grade(c.a); got != c.want {
			t.Errorf("q%d %+v: %v, want %v", c.q, c.a, got, c.want)
		}
	}
}

func TestRegexIsLinear(t *testing.T) {
	q, err := Parse("1. Q = /(a+)+$/\n")
	if err != nil {
		t.Fatal(err)
	}
	if q.Questions[0].Grade(Answer{Text: strings.Repeat("a", 5000) + "b"}) {
		t.Error("should not match")
	}
}
