// Package quiz renders :::quiz containers as quizzes whose answer key stays in
// Go: the page gets questions and options only, and the "check" action grades
// the reader's answers. :::quiz-results lists every stored quiz. The frontend
// is assets/quiz.js; scores are kept by it in ctx.storage under "quiz-<id>".
package quiz

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/dragosh/zen/internal/core/plugins"
)

//go:embed assets/quiz.js assets/quiz.css
var assets embed.FS

const maxKeys = 2000 // quizzes kept in memory

var idRE = regexp.MustCompile(`^[A-Za-z0-9._-]{1,59}$`)

// record is the answer key of one quiz on one page.
type record struct {
	Page string `json:"page"`
	ID   string `json:"id"`
	Pass int    `json:"pass"`
	Quiz
}

type plugin struct {
	mu   sync.Mutex
	keys map[string]*record
}

// New returns the quiz plugin.
func New() plugins.Plugin { return &plugin{keys: map[string]*record{}} }

func (*plugin) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID:          "quiz",
		Version:     "0.1.0",
		APIVersion:  plugins.APIVersion,
		Description: "Quizzes with stored answers and scores",
		Default:     true,
		Blocks:      []string{"quiz", "quiz-results"},
	}
}

func recordKey(page, id string) string { return page + "\x00" + id }

func keyFile(dir, page, id string) string {
	sum := sha256.Sum256([]byte(recordKey(page, id)))
	return filepath.Join(dir, "keys", hex.EncodeToString(sum[:])+".json")
}

func (p *plugin) put(dir string, r *record) error {
	p.mu.Lock()
	if len(p.keys) >= maxKeys {
		p.keys = map[string]*record{}
	}
	p.keys[recordKey(r.Page, r.ID)] = r
	p.mu.Unlock()
	if dir == "" {
		return nil
	}
	return writeKey(dir, r)
}

func writeKey(dir string, r *record) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	f := keyFile(dir, r.Page, r.ID)
	if err := os.MkdirAll(filepath.Dir(f), 0o700); err != nil {
		return err
	}
	tmp := f + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, f)
}

// get returns the key from memory or, after a restart, from dir.
func (p *plugin) get(dir, page, id string) (*record, error) {
	p.mu.Lock()
	r := p.keys[recordKey(page, id)]
	p.mu.Unlock()
	if r != nil {
		if dir != "" {
			if _, err := os.Stat(keyFile(dir, page, id)); err != nil {
				_ = writeKey(dir, r) // the render did not know the data dir
			}
		}
		return r, nil
	}
	if dir == "" {
		return nil, errNotFound
	}
	b, err := os.ReadFile(keyFile(dir, page, id))
	if err != nil {
		return nil, errNotFound
	}
	r = &record{}
	if json.Unmarshal(b, r) != nil || r.Page != page || r.ID != id {
		return nil, errNotFound
	}
	for i := range r.Questions {
		if r.Questions[i].compile() != nil {
			return nil, errNotFound
		}
	}
	p.mu.Lock()
	p.keys[recordKey(page, id)] = r
	p.mu.Unlock()
	return r, nil
}

var errNotFound = errors.New("this quiz is not known; reload the page")

func (p *plugin) RenderBlock(ctx plugins.Context, b plugins.Block) (string, error) {
	if b.Name == "quiz-results" {
		return `<div class="zq-results"></div>`, nil
	}
	id := b.Attrs["id"]
	if id == "" {
		id = "quiz"
	}
	if !idRE.MatchString(id) {
		return "", fmt.Errorf("the quiz id %q must be 1 to 59 letters, digits, . _ -", id)
	}
	q, err := Parse(b.Body)
	if err != nil {
		return "", err
	}
	pass := 70
	if n, err := strconv.Atoi(b.Attrs["pass"]); err == nil && n >= 0 && n <= 100 {
		pass = n
	}
	if err := p.put(ctx.DataDir, &record{Page: ctx.Path, ID: id, Pass: pass, Quiz: *q}); err != nil {
		return "", fmt.Errorf("cannot keep the answer key: %v", err)
	}
	var sb strings.Builder
	sb.WriteString(`<ol class="zq-questions">`)
	for _, qu := range q.Questions {
		sb.WriteString(`<li class="zq-q zq-` + string(qu.Kind) + `"><div class="zq-prompt">` + html.EscapeString(qu.Text) + `</div>`)
		if len(qu.Options) > 0 {
			sb.WriteString(`<ul class="zq-options">`)
			for _, o := range qu.Options {
				sb.WriteString(`<li>` + html.EscapeString(o) + `</li>`)
			}
			sb.WriteString(`</ul>`)
		}
		sb.WriteString(`</li>`)
	}
	sb.WriteString(`</ol>`)
	return sb.String(), nil
}

type request struct {
	Op      string                     `json:"op"`
	Quiz    string                     `json:"quiz"`
	Page    string                     `json:"page"`
	Answers map[string]json.RawMessage `json:"answers"`
}

type questionResult struct {
	Q           int    `json:"q"`
	Correct     bool   `json:"correct"`
	Explanation string `json:"explanation,omitempty"`
}

type response struct {
	OK      bool             `json:"ok"`
	Total   int              `json:"total"`
	Score   int              `json:"score"`
	Passed  bool             `json:"passed"`
	Results []questionResult `json:"results"`
}

func (p *plugin) Action(ctx plugins.Context, payload []byte) ([]byte, error) {
	var req request
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("bad request: %v", err)
	}
	if req.Op != "check" {
		return nil, fmt.Errorf("unknown op %q", req.Op)
	}
	if !idRE.MatchString(req.Quiz) || req.Page == "" || len(req.Page) > 1024 || req.Answers == nil {
		return nil, errors.New("check needs quiz, page and answers")
	}
	rec, err := p.get(ctx.DataDir, req.Page, req.Quiz)
	if err != nil {
		return nil, err
	}
	res := response{OK: true, Total: len(rec.Questions), Results: []questionResult{}}
	right := 0
	for i := range rec.Questions {
		raw, ok := req.Answers[strconv.Itoa(i)]
		if !ok {
			continue // unanswered: not marked, nothing revealed
		}
		q := &rec.Questions[i]
		qr := questionResult{Q: i, Correct: q.Grade(decodeAnswer(raw))}
		if qr.Correct {
			right++
		}
		qr.Explanation = q.Explanation // shown once the question has been checked
		res.Results = append(res.Results, qr)
	}
	if res.Total > 0 {
		res.Score = int(math.Round(100 * float64(right) / float64(res.Total)))
	}
	res.Passed = res.Score >= rec.Pass
	return json.Marshal(res)
}

// decodeAnswer reads a number, a list of numbers or a string; anything else
// is an answer that is simply wrong.
func decodeAnswer(raw json.RawMessage) Answer {
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return Answer{Choices: []int{n}}
	}
	var l []int
	if json.Unmarshal(raw, &l) == nil {
		return Answer{Choices: l}
	}
	var s string
	if json.Unmarshal(raw, &s) == nil && len(s) <= 1000 {
		return Answer{Text: s}
	}
	return Answer{Choices: []int{-1}}
}

func (*plugin) Assets() fs.FS {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err)
	}
	return sub
}

func (*plugin) Entry() plugins.Assets {
	return plugins.Assets{CSS: []string{"quiz.css"}, JS: []string{"quiz.js"}}
}
