package scriptplugins_test

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/dragosh/zenmd/internal/core/plugins"
	"github.com/dragosh/zenmd/internal/core/scriptplugins"
)

func TestManifestFromThePluginMap(t *testing.T) {
	s := newSite(t)
	p := s.plugin("note", `plugin := {
    version: "0.1.0",
    description: "Callout boxes",
    blocks: ["note", "warning"],
    inlines: ["badge"],
    hooks: ["source", "html", "action"],
    css: "div.note { border-left: 3px solid }",
    options: {icon: {type: "bool", default: true, description: "Show an icon"}}
}`)
	m := p.Manifest()
	if m.ID != "note" || m.Version != "0.1.0" || m.Description != "Callout boxes" || m.APIVersion != plugins.APIVersion || !m.Default {
		t.Errorf("manifest: %+v", m)
	}
	if strings.Join(m.Blocks, ",") != "note,warning" || strings.Join(m.Inlines, ",") != "badge" {
		t.Errorf("names: %+v", m)
	}
	if o := m.Config["icon"]; o.Type != "bool" || o.Default != true || o.Description != "Show an icon" {
		t.Errorf("option: %+v", o)
	}
	for hook, want := range map[string]bool{"block": true, "inline": true, "source": true, "html": true, "action": true, "stream": false} {
		if got := p.HasHook(hook); got != want {
			t.Errorf("HasHook(%s) = %v", hook, got)
		}
	}
	if p.Kind() != "script" {
		t.Errorf("kind %q", p.Kind())
	}
	if err := plugins.NewRegistry().Register(p); err != nil {
		t.Errorf("not registrable: %v", err)
	}
	// Only what is declared: a plugin with blocks only has no inline hook.
	only := s.plugin("only", `plugin := {version: "1.0.0", blocks: ["only"], default: false}`)
	if only.HasHook("inline") || only.HasHook("source") || !only.HasHook("block") || only.Manifest().Default {
		t.Errorf("only: %+v", only.Manifest())
	}
	if _, ok := plugins.InlineHookOf(only); ok {
		t.Error("InlineHookOf found a hook that is not declared")
	}
	if _, ok := plugins.BlockHookOf(only); !ok {
		t.Error("BlockHookOf lost the declared hook")
	}
}

func TestBlockHook(t *testing.T) {
	s := newSite(t)
	p := s.plugin("note", `plugin := {version: "0.1.0", blocks: ["note", "warning"], options: {icon: {type: "bool", default: true}, label: {type: "string", default: "L"}}}
if hook == "block" {
    title := attrs.title || (name == "warning" ? "Warning" : "Note")
    out = zen.html("div", {class: name}, zen.html("strong", {}, title), zen.markdown(body), " ", page.path, " ", config.icon, config.label)
}`)
	ctx := plugins.Context{
		Path:     "docs/a.md",
		Config:   map[string]any{"icon": true, "label": "Hi"},
		Markdown: func(src string) (string, error) { return "<p>" + src + "</p>", nil },
	}
	got, err := p.RenderBlock(ctx, plugins.Block{Name: "warning", Body: "mind <the> gap", Fenced: false})
	if err != nil {
		t.Fatal(err)
	}
	want := `<div class="warning"><strong>Warning</strong><p>mind <the> gap</p> docs/a.md trueHi</div>`
	if got != want {
		t.Errorf("block:\n got %s\nwant %s", got, want)
	}
	got, _ = p.RenderBlock(ctx, plugins.Block{Name: "note", Body: "x", Attrs: map[string]string{"title": "Be <careful>"}})
	if !strings.Contains(got, `<strong>Be &lt;careful&gt;</strong>`) {
		t.Errorf("attrs.title not used or not escaped: %s", got)
	}
	// Only the hook the plugin declares runs.
	if _, err := p.RenderInline(ctx, plugins.Inline{Name: "note", Text: "x"}); err == nil {
		t.Error("an undeclared inline hook ran")
	}
}

func TestPageTitleIsAvailable(t *testing.T) {
	s := newSite(t)
	s.file("docs/a.md", "---\ntitle: The A page\n---\n# Heading\n")
	s.file("docs/b.md", "# From heading\n")
	p := s.plugin("t", `plugin := {version: "1.0.0", blocks: ["x"]}
if hook == "block" { out = page.path + "|" + page.title }`)
	for path, want := range map[string]string{"docs/a.md": "docs/a.md|The A page", "docs/b.md": "docs/b.md|From heading", "docs/none.md": "docs/none.md|none"} {
		got, err := p.RenderBlock(plugins.Context{Path: path}, plugins.Block{Name: "x"})
		if err != nil || got != want {
			t.Errorf("%s: %q %v, want %q", path, got, err, want)
		}
	}
}

func TestInlineHook(t *testing.T) {
	s := newSite(t)
	p := s.plugin("badge", `plugin := {version: "1.0.0", inlines: ["badge"]}
if hook == "inline" { out = zen.html("span", {class: "badge " + name, "data-color": attrs.color || "gray"}, body) }`)
	got, err := p.RenderInline(plugins.Context{Path: "a.md"}, plugins.Inline{Name: "badge", Text: "stable <1>", Attrs: map[string]string{"color": "green"}})
	want := `<span class="badge badge" data-color="green">stable &lt;1&gt;</span>`
	if err != nil || got != want {
		t.Errorf("inline: %q %v, want %q", got, err, want)
	}
	// A hook that sets no out gives nothing.
	quiet := s.plugin("quiet", `plugin := {version: "1.0.0", inlines: ["q"], blocks: ["q"]}`)
	if got, err := quiet.RenderInline(plugins.Context{}, plugins.Inline{Name: "q", Text: "x"}); err != nil || got != "" {
		t.Errorf("no out (inline): %q %v", got, err)
	}
	if got, err := block(quiet, "q", "x", nil); err != nil || got != "" {
		t.Errorf("no out (block): %q %v", got, err)
	}
}

func TestSourceHTMLAndActionHooks(t *testing.T) {
	s := newSite(t)
	p := s.plugin("multi", `text := import("text")
plugin := {version: "1.0.0", hooks: ["source", "html", "action"]}
if hook == "source" { out = text.replace(body, "TODO", "DONE", -1) }
if hook == "html" { out = body + "<!-- seen " + page.path + " -->" }
if hook == "action" { out = {echo: payload, next: payload.n + 1, page: page.path, list: [1, "two", true, undefined]} }`)
	ctx := plugins.Context{Path: "p.md"}
	if got, err := p.Source(ctx, []byte("a TODO and TODO")); err != nil || string(got) != "a DONE and DONE" {
		t.Errorf("source: %q %v", got, err)
	}
	if got, err := p.HTML(ctx, "<p>x</p>"); err != nil || got != "<p>x</p><!-- seen p.md -->" {
		t.Errorf("html: %q %v", got, err)
	}
	got, err := p.Action(ctx, []byte(`{"n": 41, "tags": ["a"]}`))
	if err != nil {
		t.Fatal(err)
	}
	var res struct {
		Echo struct {
			N    int
			Tags []string
		}
		Next int
		Page string
		List []any
	}
	if json.Unmarshal(got, &res) != nil || res.Echo.N != 41 || res.Next != 42 || res.Page != "p.md" || len(res.Echo.Tags) != 1 || len(res.List) != 4 || res.List[3] != nil {
		t.Errorf("action: %s", got)
	}
	// Without out, source and html leave the text alone and an action answers nothing.
	idle := s.plugin("idle", `plugin := {version: "1.0.0", hooks: ["source", "html", "action"]}
if hook == "never" { out = "x" }`)
	if got, err := idle.Source(ctx, []byte("same")); err != nil || string(got) != "same" {
		t.Errorf("source without out: %q %v", got, err)
	}
	if got, err := idle.HTML(ctx, "<b>same</b>"); err != nil || got != "<b>same</b>" {
		t.Errorf("html without out: %q %v", got, err)
	}
	if got, err := idle.Action(ctx, []byte(`1`)); err != nil || (len(got) > 0 && string(got) != "null") {
		t.Errorf("action without out: %q %v", got, err)
	}
	// A payload that is not JSON is an error.
	if _, err := p.Action(ctx, []byte(`{nope`)); err == nil {
		t.Error("payload that is not JSON accepted")
	}
	// Source/html/action are refused when not declared.
	blockOnly := s.plugin("bo", header)
	if _, err := blockOnly.Source(ctx, []byte("x")); err == nil {
		t.Error("undeclared source hook ran")
	}
	if _, err := blockOnly.Action(ctx, []byte("1")); err == nil {
		t.Error("undeclared action hook ran")
	}
}

func TestActionOutputThatCannotBeJSON(t *testing.T) {
	s := newSite(t)
	p := s.plugin("bad", `plugin := {version: "1.0.0", hooks: ["action"]}
if hook == "action" {
    m := {}
    m.self = m
    out = m
}`)
	// A cyclic value must be an error, not a crash.
	if _, err := p.Action(plugins.Context{}, []byte(`1`)); err == nil {
		t.Error("a cyclic answer was accepted")
	}
}

func TestGlobalsAreFreshPerCall(t *testing.T) {
	s := newSite(t)
	p := s.plugin("fresh", `plugin := {version: "1.0.0", blocks: ["x"]}
count := 0
cache := {}
if hook == "block" {
    count = count + 1
    cache[body] = true
    out = string(count) + ":" + string(len(cache))
}`)
	for i := 0; i < 3; i++ {
		if got := mustBlock(t, p, "x", sprintf("b%d", i), nil); got != "1:1" {
			t.Fatalf("call %d: %q (state leaked between calls)", i, got)
		}
	}
}

func TestConcurrentCalls(t *testing.T) {
	s := newSite(t)
	p := s.plugin("conc", `plugin := {version: "1.0.0", blocks: ["x"], hooks: ["action"]}
if hook == "block" {
    n := 0
    for i := 0; i < 1000; i++ { n += i }
    out = body + "=" + string(n)
}
if hook == "action" { out = {n: payload.n * 2} }`)
	var wg sync.WaitGroup
	errs := make(chan string, 400)
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				body := sprintf("g%d-%d", g, i)
				got, err := p.RenderBlock(plugins.Context{Path: "a.md"}, plugins.Block{Name: "x", Body: body})
				if err != nil || got != body+"=499500" {
					errs <- sprintf("block %s: %q %v", body, got, err)
				}
				a, err := p.Action(plugins.Context{}, []byte(sprintf(`{"n": %d}`, g*100+i)))
				if err != nil || string(a) != sprintf(`{"n":%d}`, (g*100+i)*2) {
					errs <- sprintf("action: %q %v", a, err)
				}
			}
		}(g)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

func TestScriptErrorsAreShortAndPathFree(t *testing.T) {
	s := newSite(t)
	p := s.plugin("boom", withHeader("if hook == \"block\" {\n  x := 1\n  out = x.nope.deeper\n}\n"))
	_, err := block(p, "x", "", nil)
	if err == nil {
		t.Fatal("a failing script succeeded")
	}
	noPath(t, err, s.dir)
	if strings.Contains(err.Error(), "\n") {
		t.Errorf("multi-line error: %q", err)
	}
	var _ scriptplugins.Runtime
}
