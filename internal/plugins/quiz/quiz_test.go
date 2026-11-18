package quiz

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dragosh/zenmd/internal/core/plugins"
	"github.com/dragosh/zenmd/internal/core/render"
)

const page = `# Test

:::quiz{id=http-basics title="HTTP basics" pass=60 attempts=3}
1. Which method is idempotent?
   - ( ) POST
   - (x) PUT
   - ( ) PATCH
   > SECRETEXPLAIN-one
2. Select the 2xx codes.
   - [x] 200
   - [x] 204
   - [ ] 301
   > SECRETEXPLAIN-two
3. What does TLS stand for? = Transport Layer Security
   > SECRETEXPLAIN-three
:::

:::quiz-results
:::
`

func newRenderer(t *testing.T, p plugins.Plugin) *render.Renderer {
	reg := plugins.NewRegistry()
	if err := reg.Register(p); err != nil {
		t.Fatal(err)
	}
	return render.New(reg, nil, nil)
}

func TestManifest(t *testing.T) {
	p := New()
	m := p.Manifest()
	if m.ID != "quiz" || !m.Default || m.APIVersion != plugins.APIVersion || m.Version == "" {
		t.Fatalf("manifest %+v", m)
	}
	if strings.Join(m.Blocks, ",") != "quiz,quiz-results" {
		t.Errorf("blocks %v", m.Blocks)
	}
	if _, ok := p.(plugins.BlockHook); !ok {
		t.Error("not a BlockHook")
	}
	if _, ok := p.(plugins.ActionHook); !ok {
		t.Error("not an ActionHook")
	}
	ap, ok := p.(plugins.AssetProvider)
	if !ok {
		t.Fatal("not an AssetProvider")
	}
	e := ap.Entry()
	if strings.Join(e.JS, ",") != "quiz.js" || strings.Join(e.CSS, ",") != "quiz.css" {
		t.Errorf("entry %+v", e)
	}
	for _, f := range []string{"quiz.js", "quiz.css"} {
		if _, err := fs.Stat(ap.Assets(), f); err != nil {
			t.Error(err)
		}
	}
}

func TestRenderDoesNotLeakTheKey(t *testing.T) {
	r := newRenderer(t, New())
	p, err := r.Render("quizzes/http.md", []byte(page))
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"(x)", "[x]", "(X)", "[X]", "SECRETEXPLAIN", "Transport Layer Security", "transport layer security", "checked", "pass=", "= "} {
		if strings.Contains(p.HTML, leak) {
			t.Errorf("page leaks %q:\n%s", leak, p.HTML)
		}
	}
	for _, want := range []string{
		`data-zen-plugin="quiz"`, `data-zen-name="quiz"`, `class="zq-q zq-single"`, `class="zq-q zq-multi"`, `class="zq-q zq-text"`,
		"Which method is idempotent?", "PATCH", "204", "What does TLS stand for?", `class="zq-options"`,
		`data-zen-name="quiz-results"`, `class="zq-results"`,
	} {
		if !strings.Contains(p.HTML, want) {
			t.Errorf("missing %q in\n%s", want, p.HTML)
		}
	}
	if strings.Join(p.Assets.JS, ",") != "/_plugins/quiz/quiz.js" {
		t.Errorf("assets %+v", p.Assets)
	}
}

func TestRenderErrorBox(t *testing.T) {
	r := newRenderer(t, New())
	p, err := r.Render("a.md", []byte(":::quiz{id=x}\n1. no answer here\n:::\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.HTML, "zen-plugin-error") || !strings.Contains(p.HTML, "no options") {
		t.Errorf("%s", p.HTML)
	}
	p, _ = r.Render("a.md", []byte(":::quiz{id=\"bad id!\"}\n1. Q = a\n:::\n"))
	if !strings.Contains(p.HTML, "zen-plugin-error") {
		t.Errorf("bad id accepted: %s", p.HTML)
	}
}

type result struct {
	OK      bool `json:"ok"`
	Total   int  `json:"total"`
	Score   int  `json:"score"`
	Passed  bool `json:"passed"`
	Results []struct {
		Q           int    `json:"q"`
		Correct     bool   `json:"correct"`
		Explanation string `json:"explanation"`
	} `json:"results"`
	Error string `json:"error"`
}

func act(t *testing.T, p plugins.Plugin, ctx plugins.Context, payload string) (result, string) {
	t.Helper()
	out, err := p.(plugins.ActionHook).Action(ctx, []byte(payload))
	if err != nil {
		t.Fatalf("action %s: %v", payload, err)
	}
	var r result
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("answer %s: %v", out, err)
	}
	return r, string(out)
}

func renderPage(t *testing.T, p plugins.Plugin, ctx plugins.Context) {
	t.Helper()
	if _, err := newRenderer(t, p).Render("quizzes/http.md", []byte(page)); err != nil {
		t.Fatal(err)
	}
}

func TestCheckAction(t *testing.T) {
	p := New()
	renderPage(t, p, plugins.Context{})
	ctx := plugins.Context{Path: "quizzes/http.md"}
	const base = `{"op":"check","quiz":"http-basics","page":"quizzes/http.md","answers":`

	r, _ := act(t, p, ctx, base+`{"0":1,"1":[0,1],"2":"transport layer  SECURITY"}}`)
	if !r.OK || r.Score != 100 || !r.Passed || r.Total != 3 || len(r.Results) != 3 {
		t.Fatalf("all right: %+v", r)
	}
	if r.Results[0].Explanation != "SECRETEXPLAIN-one" || !r.Results[2].Correct {
		t.Errorf("%+v", r.Results)
	}

	r, _ = act(t, p, ctx, base+`{"0":0,"1":[0,1],"2":"nope"}}`)
	if r.Score != 33 || r.Passed || r.Results[0].Correct || !r.Results[1].Correct || r.Results[2].Correct {
		t.Fatalf("one right: %+v", r)
	}

	// pass=60: two of three is 67%, enough
	r, _ = act(t, p, ctx, base+`{"0":1,"1":[0,1],"2":"x"}}`)
	if r.Score != 67 || !r.Passed {
		t.Fatalf("two right: %+v", r)
	}

	// unanswered questions: not marked, no explanation, the key stays hidden
	r, raw := act(t, p, ctx, base+`{"0":1}}`)
	if len(r.Results) != 1 || r.Results[0].Q != 0 || r.Score != 33 || r.Passed {
		t.Fatalf("partial: %+v", r)
	}
	for _, leak := range []string{"SECRETEXPLAIN-two", "SECRETEXPLAIN-three", "Transport", "transport"} {
		if strings.Contains(raw, leak) {
			t.Errorf("answer leaks %q: %s", leak, raw)
		}
	}
	// wrong answers never reveal the key either
	_, raw = act(t, p, ctx, base+`{"0":0,"2":"zzz"}}`)
	if strings.Contains(raw, "Transport") || strings.Contains(raw, "correctAnswer") {
		t.Errorf("wrong answer leaks: %s", raw)
	}

	// malformed answers are wrong, not errors
	r, _ = act(t, p, ctx, base+`{"0":"x","1":"y","2":5,"9":1}}`)
	if r.Score != 0 || len(r.Results) != 3 {
		t.Errorf("malformed: %+v", r)
	}
}

func TestActionErrors(t *testing.T) {
	p := New().(plugins.ActionHook)
	ctx := plugins.Context{}
	for _, payload := range []string{`{"op":"nope"}`, `{"op":"check","quiz":"missing","page":"a.md","answers":{}}`, `not json`, `{"op":"check","quiz":"x"}`} {
		if _, err := p.Action(ctx, []byte(payload)); err == nil {
			t.Errorf("%s: no error", payload)
		}
	}
}

func TestKeySurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	a := New()
	if _, err := newRenderer(t, a).Render("quizzes/http.md", []byte(page)); err != nil {
		t.Fatal(err)
	}
	// the render context may not know the data dir: the first action writes the key
	ctx := plugins.Context{Path: "quizzes/http.md", DataDir: dir}
	r, _ := act(t, a, ctx, `{"op":"check","quiz":"http-basics","page":"quizzes/http.md","answers":{"0":1}}`)
	if r.Score != 33 {
		t.Fatalf("%+v", r)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "keys", "*.json"))
	if len(files) != 1 {
		t.Fatalf("key files %v", files)
	}
	// a fresh plugin (a restart) with an empty memory reads the file
	b := New()
	r, _ = act(t, b, ctx, `{"op":"check","quiz":"http-basics","page":"quizzes/http.md","answers":{"0":1,"1":[0,1],"2":"transport layer security"}}`)
	if r.Score != 100 {
		t.Fatalf("after restart: %+v", r)
	}
	// rendering with a data dir writes it directly
	dir2 := t.TempDir()
	c := New()
	if _, err := c.(plugins.BlockHook).RenderBlock(plugins.Context{Path: "q.md", DataDir: dir2}, plugins.Block{Name: "quiz", Attrs: map[string]string{"id": "x"}, Body: "1. Q = a\n"}); err != nil {
		t.Fatal(err)
	}
	if files, _ := filepath.Glob(filepath.Join(dir2, "keys", "*.json")); len(files) != 1 {
		t.Errorf("render did not persist: %v", files)
	}
	if fi, err := os.Stat(files[0]); err != nil || fi.Mode().Perm()&0o077 != 0 {
		t.Errorf("key file mode %v %v", fi, err)
	}
}

func TestKeyIsPerPageAndId(t *testing.T) {
	p := New()
	h := p.(plugins.BlockHook)
	for path, ans := range map[string]string{"a.md": "one", "b.md": "two"} {
		if _, err := h.RenderBlock(plugins.Context{Path: path}, plugins.Block{Name: "quiz", Attrs: map[string]string{"id": "q"}, Body: "1. Q = " + ans + "\n"}); err != nil {
			t.Fatal(err)
		}
	}
	r, _ := act(t, p, plugins.Context{}, `{"op":"check","quiz":"q","page":"b.md","answers":{"0":"two"}}`)
	if r.Score != 100 {
		t.Errorf("%+v", r)
	}
	r, _ = act(t, p, plugins.Context{}, `{"op":"check","quiz":"q","page":"a.md","answers":{"0":"two"}}`)
	if r.Score != 0 {
		t.Errorf("%+v", r)
	}
	// a path traversal in the page never reaches the file system
	dir := t.TempDir()
	if _, err := p.(plugins.ActionHook).Action(plugins.Context{DataDir: dir}, []byte(`{"op":"check","quiz":"../../etc","page":"../x","answers":{}}`)); err == nil {
		t.Error("expected an error")
	}
}
