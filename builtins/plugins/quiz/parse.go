package quiz

import (
	"fmt"
	"regexp"
	"strings"
)

// Kind is the type of a question.
type Kind string

const (
	Single Kind = "single" // one right option: - ( ) / - (x)
	Multi  Kind = "multi"  // several right options: - [ ] / - [x]
	Text   Kind = "text"   // free text: question = answer
)

// MaxPattern is the longest regex answer, in characters.
const MaxPattern = 200

// Question is one parsed question with its key. Everything but Kind, Text
// and Options is the answer key and must never reach the page.
type Question struct {
	Kind        Kind     `json:"kind"`
	Text        string   `json:"text"`
	Options     []string `json:"options,omitempty"`
	Correct     []int    `json:"correct,omitempty"` // indexes of the right options
	Answer      string   `json:"answer,omitempty"`  // normalised free-text answer
	Pattern     string   `json:"pattern,omitempty"` // regex answer, matched against the normalised reply
	Explanation string   `json:"explanation,omitempty"`

	re *regexp.Regexp
}

// Quiz is the parsed body of a :::quiz container.
type Quiz struct {
	Questions []Question `json:"questions"`
}

// Answer is a reader's reply to one question.
type Answer struct {
	Choices []int  // chosen option indexes (single and multiple choice)
	Text    string // free text
}

var (
	questionRE = regexp.MustCompile(`^\s{0,3}(\d{1,4})[.)]\s+(.*\S)\s*$`)
	optionRE   = regexp.MustCompile(`^\s*[-*+]\s+(\(([ xX])\)|\[([ xX])\])\s+(.*\S)\s*$`)
	quoteRE    = regexp.MustCompile(`^\s*>\s?(.*)$`)
	answerRE   = regexp.MustCompile(`^(.*\S)\s+=\s+(.*\S)\s*$`)
	spaceRE    = regexp.MustCompile(`\s+`)
)

// splitAnswer applies a trailing "= answer" of text to q; it reports whether there was one.
func (q *Question) splitAnswer(text string) bool {
	a := answerRE.FindStringSubmatch(text)
	if a == nil {
		return false
	}
	q.Text = a[1]
	ans := strings.TrimSpace(a[2])
	if len(ans) >= 2 && strings.HasPrefix(ans, "/") && strings.HasSuffix(ans, "/") {
		q.Pattern = ans[1 : len(ans)-1]
	} else {
		q.Answer = normalise(ans)
	}
	q.Kind = Text
	return true
}

func normalise(s string) string {
	return strings.ToLower(spaceRE.ReplaceAllString(strings.TrimSpace(s), " "))
}

// compile prepares the regex of a pattern question; it is also called after a
// key is loaded from disk.
func (q *Question) compile() error {
	if q.Pattern == "" {
		return nil
	}
	if len([]rune(q.Pattern)) > MaxPattern {
		return fmt.Errorf("pattern longer than %d characters", MaxPattern)
	}
	// Anchored: the whole reply must match. Go's regexp runs in linear time.
	re, err := regexp.Compile(`(?i)^(?:` + q.Pattern + `)$`)
	if err != nil {
		return fmt.Errorf("pattern %q: %v", q.Pattern, err)
	}
	q.re = re
	return nil
}

// Parse reads the numbered questions of a quiz body.
func Parse(body string) (*Quiz, error) {
	quiz := &Quiz{}
	var cur *Question
	var quote []string
	flush := func() error {
		if cur == nil {
			return nil
		}
		cur.Explanation = strings.TrimSpace(strings.Join(quote, " "))
		quote = nil
		n := len(quiz.Questions) + 1
		q := *cur
		cur = nil
		if err := q.validate(); err != nil {
			return fmt.Errorf("question %d: %v", n, err)
		}
		quiz.Questions = append(quiz.Questions, q)
		return nil
	}
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := questionRE.FindStringSubmatch(line); m != nil {
			if err := flush(); err != nil {
				return nil, err
			}
			cur = &Question{Text: m[2]}
			cur.splitAnswer(m[2])
			continue
		}
		if cur == nil {
			return nil, fmt.Errorf("text before the first question: %q", strings.TrimSpace(line))
		}
		if m := optionRE.FindStringSubmatch(line); m != nil {
			kind, mark := Single, m[2]
			if m[3] != "" {
				kind, mark = Multi, m[3]
			}
			if len(cur.Options) > 0 && cur.Kind != kind {
				return nil, fmt.Errorf("question %d mixes ( ) and [ ] options", len(quiz.Questions)+1)
			}
			if cur.Kind == Text {
				return nil, fmt.Errorf("question %d has both options and an = answer", len(quiz.Questions)+1)
			}
			cur.Kind = kind
			if mark != " " {
				cur.Correct = append(cur.Correct, len(cur.Options))
			}
			cur.Options = append(cur.Options, m[4])
			continue
		}
		if m := quoteRE.FindStringSubmatch(line); m != nil {
			if t := strings.TrimSpace(m[1]); t != "" {
				quote = append(quote, t)
			}
			continue
		}
		if len(cur.Options) == 0 && cur.Kind != Text && len(quote) == 0 {
			cur.Text += " " + strings.TrimSpace(line) // a wrapped question
			cur.splitAnswer(cur.Text)
			continue
		}
		return nil, fmt.Errorf("question %d: cannot read %q", len(quiz.Questions)+1, strings.TrimSpace(line))
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if len(quiz.Questions) == 0 {
		return nil, fmt.Errorf("no questions: number them as 1. Question")
	}
	return quiz, nil
}

func (q *Question) validate() error {
	switch {
	case q.Kind == "":
		return fmt.Errorf("no options and no = answer")
	case q.Kind == Text:
		if q.Pattern == "" && q.Answer == "" {
			return fmt.Errorf("no options and no = answer")
		}
		return q.compile()
	case len(q.Options) < 2:
		return fmt.Errorf("needs at least two options")
	case q.Kind == Single && len(q.Correct) != 1:
		return fmt.Errorf("a ( ) question needs exactly one (x) option")
	case q.Kind == Multi && len(q.Correct) == 0:
		return fmt.Errorf("a [ ] question needs at least one [x] option")
	}
	return nil
}

// Grade reports whether a is the right answer.
func (q *Question) Grade(a Answer) bool {
	switch q.Kind {
	case Single, Multi:
		got := map[int]bool{}
		for _, c := range a.Choices {
			if c < 0 || c >= len(q.Options) {
				return false
			}
			got[c] = true
		}
		if len(got) != len(q.Correct) {
			return false
		}
		for _, c := range q.Correct {
			if !got[c] {
				return false
			}
		}
		return true
	case Text:
		t := normalise(a.Text)
		if t == "" {
			return false
		}
		if q.Pattern != "" {
			if q.re == nil && q.compile() != nil {
				return false
			}
			return q.re.MatchString(t)
		}
		return t == q.Answer
	}
	return false
}
