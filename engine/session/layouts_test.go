package session

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func layoutFiles() map[string]string {
	return map[string]string{
		"README.md":                      "---\nlayout: memo\nto: Ada\n---\n# Memo\n\nBody.\n",
		"plain.md":                       "# Plain\n",
		".zen/layouts/memo/layout.yml":   "id: memo\nversion: 0.1.0\ndescription: A memo\ntemplate: layout.html\ncss: [memo.css]\nfields:\n  to: { type: string }\n",
		".zen/layouts/memo/layout.html":  "<section class=\"memo\"><p>To: {{.Field \"to\"}}</p>{{.Content}}</section>",
		".zen/layouts/memo/memo.css":     ".memo { color: #111 }",
		".zen/layouts/fancy/layout.yml":  "id: fancy\nversion: 0.2.0\ncss: [f.css]\njs: [f.js]\n",
		".zen/layouts/fancy/f.css":       "p {}",
		".zen/layouts/fancy/f.js":        "export default { mount() {} }",
		".zen/layouts/fancy/secret.txt":  "not listed",
		".zen/layouts/broken/layout.yml": "id: broken\nversion: nope\ncss: [a.css]\n",
		".zen/layouts/broken/a.css":      "",
	}
}

func layoutOf(t *testing.T, z *Session, id string) LayoutInfo {
	t.Helper()
	for _, l := range z.Layouts() {
		if l.ID == id {
			return l
		}
	}
	t.Fatalf("layout %q not listed in %+v", id, z.Layouts())
	return LayoutInfo{}
}

func TestLayoutsListJSON(t *testing.T) {
	z, _, _ := openWith(t, pluginTree(t, layoutFiles()), t.TempDir(), true)
	raw, _ := json.Marshal(z.Layouts())
	var shape []map[string]any
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatal(err)
	}
	by := map[string]map[string]any{}
	for _, it := range shape {
		for _, k := range []string{"id", "version", "description", "source", "status", "kind", "hasTemplate", "hasScript", "fields", "options", "help", "problem"} {
			if _, ok := it[k]; !ok {
				t.Fatalf("JSON lacks %q: %s", k, raw)
			}
		}
		if len(it) != 12 || it["kind"] != "layout" {
			t.Fatalf("JSON shape: %s", raw)
		}
		for _, k := range []string{"fields", "options"} {
			if _, isList := it[k].([]any); !isList {
				t.Fatalf("%s must be a list, never null: %s", k, raw)
			}
		}
		if h, _ := it["help"].(map[string]any); len(h) != 2 {
			t.Fatalf("help shape: %s", raw)
		} else if _, isList := h["snippets"].([]any); !isList {
			t.Fatalf("snippets must be a list: %s", raw)
		}
		by[it["id"].(string)] = it
	}
	for _, id := range []string{"article", "docs", "post", "landing", "wide", "handout", "slides", "course"} {
		it := by[id]
		if it == nil || it["source"] != "builtin" || it["status"] != "on" {
			t.Errorf("built-in %s: %v", id, it)
		}
	}
	if by["docs"]["hasTemplate"] != true || by["slides"]["hasScript"] != true || by["slides"]["hasTemplate"] != false {
		t.Error("hasTemplate/hasScript")
	}
	memo := layoutOf(t, z, "memo")
	if memo.Source != "folder" || memo.Status != "on" || !memo.HasTemplate || memo.HasScript || len(memo.Fields) != 1 || memo.Fields[0].Name != "to" {
		t.Errorf("memo: %+v", memo)
	}
	if b := layoutOf(t, z, "broken"); b.Status != "invalid" || b.Problem == "" || b.Source != "folder" {
		t.Errorf("broken: %+v", b)
	}
	post := by["post"]
	if opts, _ := post["options"].([]any); len(opts) != 1 {
		t.Errorf("post options: %v", post["options"])
	}
}

func TestLayoutTrustPromptAndUse(t *testing.T) {
	cfg := t.TempDir()
	dir := pluginTree(t, layoutFiles())
	z, ev, dl := openWith(t, dir, cfg, true)
	var prompt string
	for _, m := range dl.Asked {
		if strings.Contains(m, "layout") {
			prompt = m
		}
	}
	if !strings.Contains(prompt, "memo 0.1.0") || !strings.Contains(prompt, "changes how pages look; runs nothing") ||
		!strings.Contains(prompt, "fancy 0.2.0") || !strings.Contains(prompt, "changes how pages look and runs a page script") ||
		strings.Contains(prompt, "broken") {
		t.Fatalf("prompt: %q", dl.Asked)
	}
	pg, err := z.Page("README.md")
	if err != nil {
		t.Fatal(err)
	}
	if pg.Layout != (PageLayout{Name: "memo", Source: "folder"}) || len(pg.Warnings) != 0 {
		t.Fatalf("layout %+v warnings %v", pg.Layout, pg.Warnings)
	}
	if !strings.Contains(pg.HTML, `data-zen-layout="memo" data-zen-layout-source="folder"`) || !strings.Contains(pg.HTML, `<section class="memo"><p>To: Ada</p>`) {
		t.Errorf("html: %s", pg.HTML)
	}
	if urlsOf(pg.Assets.CSS) != "/_layouts/memo/memo.css" || !versioned(pg.Assets.CSS, 12) {
		t.Errorf("assets %+v", pg.Assets)
	}
	if n := countEvents(ev, EventLayoutsChanged); n == 0 {
		t.Error("no layouts:changed after open")
	}
	raw, _ := json.Marshal(pg)
	if !strings.Contains(string(raw), `"layout":{"name":"memo","source":"folder"}`) {
		t.Errorf("JSON: %s", raw)
	}
	plain, _ := z.Page("plain.md")
	raw, _ = json.Marshal(plain)
	if !strings.Contains(string(raw), `"layout":{"name":"article","source":"builtin"}`) {
		t.Errorf("JSON of a plain page: %s", raw)
	}

	// Assets: listed files only.
	h := z.middleware(http.NotFoundHandler())
	get := func(p string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
		return rec
	}
	if rec := get("/_layouts/memo/memo.css"); rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/css") || !strings.Contains(rec.Body.String(), ".memo") {
		t.Errorf("css: %d %s", rec.Code, rec.Header())
	}
	if rec := get("/_layouts/fancy/f.js"); rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/javascript") {
		t.Errorf("js: %d", rec.Code)
	}
	for _, p := range []string{"/_layouts/memo/layout.yml", "/_layouts/memo/layout.html", "/_layouts/fancy/secret.txt", "/_layouts/memo/../fancy/f.js", "/_layouts/memo", "/_layouts/nope/x.css", "/_layouts/broken/a.css"} {
		if rec := get(p); rec.Code != 404 {
			t.Errorf("%s served: %d", p, rec.Code)
		}
	}
	// Built-in assets.
	if rec := get("/_layouts/slides/slides.js"); rec.Code != 200 {
		t.Errorf("builtin js: %d", rec.Code)
	}
	if rec := get("/_layouts/docs/layout.html"); rec.Code != 404 {
		t.Errorf("a built-in template is served: %d", rec.Code)
	}
}

func countEvents(ev *sink, name string) int {
	n := 0
	for _, e := range ev.all() {
		if e.Name == name {
			n++
		}
	}
	return n
}

func TestLayoutBlockedAndUndecided(t *testing.T) {
	cfg := t.TempDir()
	dir := pluginTree(t, layoutFiles())
	// Declined at open: undecided is not what happens, blocked is.
	z, ev, _ := openWith(t, dir, cfg, false)
	if memo := layoutOf(t, z, "memo"); memo.Status != "blocked" {
		t.Fatalf("memo: %+v", memo)
	}
	pg, _ := z.Page("README.md")
	if pg.Layout.Name != "article" || strings.Contains(pg.HTML, "data-zen-layout") || !hasWarning(pg, `layout "memo" is not allowed (Settings › Layouts)`) {
		t.Errorf("blocked layout used: %+v %v", pg.Layout, pg.Warnings)
	}
	if got := urlsOf(pg.Assets.CSS); got != "" {
		t.Errorf("blocked layout assets: %s", got)
	}
	// A blocked layout's files are not served.
	rec := httptest.NewRecorder()
	z.middleware(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest("GET", "/_layouts/memo/memo.css", nil))
	if rec.Code != 404 {
		t.Errorf("blocked layout css served: %d", rec.Code)
	}
	// Allow: the dialog is shown, and only its yes records the decision.
	z2, _, dl := openWith(t, dir, t.TempDir(), false)
	ev.reset()
	if err := z2.LayoutTrust("memo", true); err != nil {
		t.Fatal(err)
	}
	if layoutOf(t, z2, "memo").Status == "on" {
		t.Error("allowed although the dialog said no")
	}
	dl.Allow = true
	ev2 := z2.events.(*sink)
	ev2.reset()
	if err := z2.LayoutTrust("memo", true); err != nil {
		t.Fatal(err)
	}
	if layoutOf(t, z2, "memo").Status != "on" || countEvents(ev2, EventLayoutsChanged) == 0 {
		t.Errorf("not allowed: %+v events %v", layoutOf(t, z2, "memo"), ev2.all())
	}
	pg, _ = z2.Page("README.md")
	if pg.Layout.Name != "memo" {
		t.Errorf("layout %+v %v", pg.Layout, pg.Warnings)
	}
	if err := z2.LayoutTrust("memo", false); err != nil {
		t.Fatal(err)
	}
	if layoutOf(t, z2, "memo").Status != "blocked" {
		t.Error("not blocked")
	}
	for _, bad := range []string{"nope", "Bad Name", "docs", "broken"} {
		if err := z2.LayoutTrust(bad, true); err == nil {
			t.Errorf("LayoutTrust(%q) accepted", bad)
		}
	}
}

func TestLayoutEditedAsksAgain(t *testing.T) {
	cfg := t.TempDir()
	dir := pluginTree(t, layoutFiles())
	z, ev, dl := openWith(t, dir, cfg, true)
	if pg, _ := z.Page("README.md"); pg.Layout.Name != "memo" {
		t.Fatalf("layout %+v", pg.Layout)
	}
	asked := len(dl.Asked)
	time.Sleep(20 * time.Millisecond)
	ev.reset()
	if err := os.WriteFile(filepath.Join(dir, ".zen/layouts/memo/layout.html"), []byte("<section class=\"memo\">EDITED {{.Content}}</section>"), 0o644); err != nil {
		t.Fatal(err)
	}
	dl.Allow = false
	// The page notices on the next render, never uses the changed layout, and
	// the layouts reload.
	pg, _ := z.Page("README.md")
	if pg.Layout.Name != "article" || strings.Contains(pg.HTML, "EDITED") || !hasWarning(pg, `layout "memo" changed; allow it again (Settings › Layouts)`) {
		t.Errorf("changed layout used: %+v %v", pg.Layout, pg.Warnings)
	}
	deadline := time.Now().Add(3 * time.Second)
	for countEvents(ev, EventLayoutsChanged) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if countEvents(ev, EventLayoutsChanged) == 0 {
		t.Fatal("no layouts:changed after the edit")
	}
	if st := layoutOf(t, z, "memo").Status; st != "undecided" {
		t.Errorf("status after edit: %q", st)
	}
	_ = asked
	// Allowing again shows the prompt and works with the new content.
	dl.Allow = true
	if err := z.LayoutTrust("memo", true); err != nil {
		t.Fatal(err)
	}
	pg, _ = z.Page("README.md")
	if !strings.Contains(pg.HTML, "EDITED") {
		t.Errorf("edited layout not used after allowing: %s", pg.HTML)
	}
}

func TestLayoutPatternInFolderConfig(t *testing.T) {
	files := layoutFiles()
	files[".zenmd.yml"] = "version: 1\nlayouts:\n  \"notes/**\": memo\n  \"blog/**\": post\nlayout: docs\n"
	files["notes/a.md"] = "# A\n"
	files["blog/p.md"] = "---\ndate: 2026-11-17\n---\n# P\n"
	z, _, _ := openWith(t, pluginTree(t, files), t.TempDir(), true)
	for path, want := range map[string]string{"notes/a.md": "memo", "blog/p.md": "post", "plain.md": "docs", "README.md": "memo"} {
		pg, err := z.Page(path)
		if err != nil || pg.Layout.Name != want {
			t.Errorf("%s: %+v %v (want %s)", path, pg.Layout, err, want)
		}
	}
	// Patterns only apply approved layouts.
	z2, _, _ := openWith(t, pluginTree(t, files), t.TempDir(), false)
	pg, _ := z2.Page("notes/a.md")
	if pg.Layout.Name != "article" || !hasWarning(pg, `layout "memo" is not allowed (Settings › Layouts)`) {
		t.Errorf("unapproved layout applied by a pattern: %+v %v", pg.Layout, pg.Warnings)
	}
}

func TestLayoutNavigationData(t *testing.T) {
	files := map[string]string{
		"README.md":  "---\nlayout: landing\ntagline: Hi\n---\n# Home\n\nWelcome.\n\n[Guide](guide/a.md)\n[B](guide/b.md)\n[C](guide/c.md)\n",
		"guide/a.md": "---\nlayout: docs\nnav: { order: 1 }\n---\n# Alpha\n\nThe *first* page of the [guide](b.md). It has `code` too.\n\n## Part\n",
		"guide/b.md": "---\nlayout: docs\nnav: { order: 2 }\n---\n# Beta\n\n- a list first\n\nThe second page text.\n",
		"guide/c.md": "---\nlayout: docs\nnav: { order: 3 }\n---\n# Gamma\n",
	}
	z, _, _ := openWith(t, tree(t, files), t.TempDir(), true)
	b, err := z.Page("guide/b.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`data-path="guide/a.md"`, ">Alpha<", `data-path="guide/c.md"`, ">Gamma<", `data-zen-action="edit"`} {
		if !strings.Contains(b.HTML, want) {
			t.Errorf("guide/b.md lacks %q: %s", want, b.HTML)
		}
	}
	home, _ := z.Page("README.md")
	for _, want := range []string{"<strong>Alpha</strong>", "The first page of the guide. It has code too.", "<strong>Beta</strong>", "The second page text.", `<p class="zl-tagline">Hi</p>`} {
		if !strings.Contains(home.HTML, want) {
			t.Errorf("home lacks %q: %s", want, home.HTML)
		}
	}
	if len(home.Warnings) != 0 || len(b.Warnings) != 0 {
		t.Errorf("warnings %v %v", home.Warnings, b.Warnings)
	}
}

func TestFirstParagraph(t *testing.T) {
	for in, want := range map[string]string{
		"# T\n\nHello **world** and [a link](x.md).\n\nSecond.":     "Hello world and a link.",
		"---\ntitle: x\n---\n# T\n\n> quote\n\n- list\n\nReal text": "Real text",
		"```\ncode\n```\n\nAfter the code":                          "After the code",
		":::note\ninside\n:::\n\nOutside":                           "Outside",
		"# Only a heading":                                          "",
		"line one\nline two\n\nnext":                                "line one line two",
		"| a | b |\n|---|---|\n| 1 | 2 |\n\nAfter the table":        "After the table",
		strings.Repeat("word ", 100):                                strings.TrimSpace(strings.Repeat("word ", 36)) + "…",
	} {
		if got := firstParagraph([]byte(in)); got != want && !(strings.HasSuffix(want, "…") && strings.HasSuffix(got, "…") && strings.HasPrefix(got, "word")) {
			t.Errorf("firstParagraph(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLayoutStorage(t *testing.T) {
	cfg := t.TempDir()
	dir := pluginTree(t, layoutFiles())
	z, _, _ := openWith(t, dir, cfg, true)
	if v, err := z.LayoutStorageGet("memo", "k"); err != nil || v != "null" {
		t.Fatalf("get empty: %q %v", v, err)
	}
	if err := z.LayoutStorageSet("memo", "k", `{"a":[1,2]}`); err != nil {
		t.Fatal(err)
	}
	if err := z.LayoutStorageSet("memo", "other", `1`); err != nil {
		t.Fatal(err)
	}
	if v, _ := z.LayoutStorageGet("memo", "k"); v != `{"a":[1,2]}` {
		t.Errorf("get: %q", v)
	}
	if keys, _ := z.LayoutStorageKeys("memo", "o"); strings.Join(keys, ",") != "other" {
		t.Errorf("keys: %v", keys)
	}
	if keys, _ := z.LayoutStorageKeys("memo", ""); strings.Join(keys, ",") != "k,other" {
		t.Errorf("keys: %v", keys)
	}
	// Per layout: another layout and the plugin of the same id do not share it.
	if err := z.LayoutStorageSet("fancy", "k", `"fancy"`); err != nil {
		t.Fatal(err)
	}
	if v, _ := z.LayoutStorageGet("memo", "k"); v != `{"a":[1,2]}` {
		t.Errorf("storage is shared between layouts: %q", v)
	}
	for _, bad := range []struct{ id, key, val string }{
		{"nope", "k", "1"}, {"memo", "bad key", "1"}, {"memo", "", "1"}, {"memo", "k", "{"}, {"Bad", "k", "1"}, {"broken", "k", "1"},
	} {
		if err := z.LayoutStorageSet(bad.id, bad.key, bad.val); err == nil {
			t.Errorf("LayoutStorageSet(%+v) accepted", bad)
		}
	}
	if _, err := z.LayoutStorageGet("nope", "k"); err == nil {
		t.Error("get of an unknown layout")
	}
	// A layout that is not allowed has no storage.
	zb, _, _ := openWith(t, dir, t.TempDir(), false)
	if err := zb.LayoutStorageSet("memo", "k", "1"); err == nil {
		t.Error("storage of a blocked layout")
	}
	if err := zb.LayoutStorageSet("course", "k", "1"); err != nil {
		t.Errorf("storage of a built-in layout: %v", err)
	}
}

// The course layout reads and writes the file the course plugin used, so the
// progress people saved before is not lost.
func TestCourseStorageKeepsThePluginsFile(t *testing.T) {
	cfg := t.TempDir()
	dir := tree(t, map[string]string{"README.md": "# Home\n"})
	z, _, _ := openWith(t, dir, cfg, true)
	legacy := filepath.Join(z.dataDir(z.ws.Root(), "course"), "storage.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte(`{"progress.intro":{"done":[0,1]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if v, err := z.LayoutStorageGet("course", "progress.intro"); err != nil || v != `{"done":[0,1]}` {
		t.Fatalf("saved progress lost: %q %v", v, err)
	}
	if keys, _ := z.LayoutStorageKeys("course", "progress."); strings.Join(keys, ",") != "progress.intro" {
		t.Errorf("keys %v", keys)
	}
	if err := z.LayoutStorageSet("course", "progress.two", `{"done":[0]}`); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(legacy)
	if !strings.Contains(string(b), "progress.two") || !strings.Contains(string(b), "progress.intro") {
		t.Errorf("written elsewhere: %s", b)
	}
}

func TestPluginsCannotDeclareLayouts(t *testing.T) {
	files := map[string]string{
		"README.md":                   "# Home\n",
		".zen/plugins/old/plugin.yml": "id: old\nversion: 1.0.0\napiVersion: 1\nlayouts: [old]\nfrontend: { js: [a.js] }\n",
		".zen/plugins/old/a.js":       "export default {}",
		".zen/plugins/tg.tengo":       "plugin := {version: \"1.0.0\", blocks: [\"x\"], layouts: [\"deck\"]}\n",
	}
	z, _, dl := openWith(t, pluginTree(t, files), t.TempDir(), true)
	for _, id := range []string{"old", "tg"} {
		p := infoOf(t, z, id)
		if p.Status != "invalid" || !strings.Contains(p.Problem, ".zen/layouts/<id>/layout.yml") {
			t.Errorf("%s: %+v", id, p)
		}
	}
	if len(dl.Asked) != 0 {
		t.Errorf("asked about invalid plugins: %q", dl.Asked)
	}
}

// Every built-in layout's help snippets render through the real pipeline
// without warnings, using that layout.
func TestBuiltinLayoutHelpRenders(t *testing.T) {
	z, _, _ := openWith(t, tree(t, map[string]string{"README.md": "# Home\n", "images/cover.png": "x"}), t.TempDir(), true)
	for _, l := range z.Layouts() {
		if l.Source != "builtin" {
			continue
		}
		if l.Help.Summary == "" || len(l.Help.Snippets) == 0 {
			t.Errorf("%s: no help", l.ID)
		}
		for _, sn := range l.Help.Snippets {
			pg, err := z.Preview("README.md", sn.Markdown)
			if err != nil || len(pg.Warnings) != 0 {
				t.Errorf("%s / %s: %v %v", l.ID, sn.Title, err, pg.Warnings)
			}
		}
	}
}
