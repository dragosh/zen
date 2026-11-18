package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/dragosh/zenmd/internal/core/plugins"
	"github.com/dragosh/zenmd/internal/plugins/codeblock"
	"github.com/dragosh/zenmd/internal/plugins/diagram"
)

// The "hello" plugin: a shell backend that leaves a marker in the folder
// whenever it runs, an action hook that echoes its input, one script, one style.
const helloYML = `id: hello
version: 0.1.0
apiVersion: 1
description: Says hello
default: true
blocks: [hello]
backend:
  command: ["./hello.sh"]
  hooks: [block, action]
frontend:
  js: [hello.js]
  css: [hello.css]
`

const helloSH = `#!/bin/sh
if [ "$ZEN_HOOK" = action ]; then
  printf '{"echo":'; cat; printf ',"page":"%s"}' "$ZEN_PAGE"
  exit 0
fi
touch "$ZEN_WORKSPACE/ran.txt"
printf '<p class="hello">hello from the backend</p>'
`

func pluginDocs() map[string]string {
	return map[string]string{
		"README.md":                      "# Home\n\n```hello\nx\n```\n\n[Other](other.md)\n",
		"other.md":                       "# Other\n\nNo plugins here.\n",
		".zen/plugins/hello/plugin.yml":  helloYML,
		".zen/plugins/hello/hello.sh":    helloSH,
		".zen/plugins/hello/hello.js":    "zen.register('hello', {});\n",
		".zen/plugins/hello/hello.css":   ".hello{color:green}\n",
		".zen/plugins/hello/secret.txt":  "not an asset",
		".zen/plugins/hello/lib/more.js": "// not listed\n",
	}
}

// pluginTree writes files and makes every *.sh executable.
func pluginTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := tree(t, files)
	for p := range files {
		if strings.HasSuffix(p, ".sh") {
			if err := os.Chmod(filepath.Join(dir, filepath.FromSlash(p)), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	return dir
}

// openWith opens dir like the app does, with the built-in plugins, a private
// settings folder and dialogs that answer the trust prompt with confirm.
func openWith(t *testing.T, dir, cfgDir string, confirm bool) (*Session, *sink, *fakeDialogs) {
	t.Helper()
	reg := plugins.NewRegistry()
	for _, p := range []plugins.Plugin{codeblock.New(), diagram.New()} {
		if err := reg.Register(p); err != nil {
			t.Fatal(err)
		}
	}
	ev, dl := &sink{}, &fakeDialogs{Allow: confirm}
	z, err := newZen(zenOptions{Registry: reg, ConfigDir: cfgDir, Dir: dir, Events: ev, Dialogs: dl, Open: func(string) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	return z, ev, dl
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func infoOf(t *testing.T, z *Session, id string) PluginInfo {
	t.Helper()
	for _, p := range z.Plugins() {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("plugin %q not listed in %+v", id, z.Plugins())
	return PluginInfo{}
}

const notAllowed = `plugin "hello" is not allowed (Settings › Plugins)`

// urlsOf joins asset URLs without their ?v= query.
func urlsOf(urls []string) string {
	var out []string
	for _, u := range urls {
		out = append(out, strings.SplitN(u, "?", 2)[0])
	}
	return strings.Join(out, ",")
}

// versioned reports whether every URL ends in ?v=<n characters>.
func versioned(urls []string, n int) bool {
	for _, u := range urls {
		_, v, ok := strings.Cut(u, "?v=")
		if !ok || len(v) != n {
			return false
		}
	}
	return len(urls) > 0
}

func hasWarning(pg Page, w string) bool {
	for _, got := range pg.Warnings {
		if got == w {
			return true
		}
	}
	return false
}

// askingDialogs records whether the backend had already run when asked.
type askingDialogs struct {
	fakeDialogs
	marker    string
	ranBefore bool
}

func (d *askingDialogs) Confirm(msg string) bool {
	d.ranBefore = d.ranBefore || exists(d.marker)
	return d.fakeDialogs.Confirm(msg)
}

func TestTrustPrompt(t *testing.T) {
	dir := pluginTree(t, pluginDocs())
	cfg := t.TempDir()
	ran := filepath.Join(dir, "ran.txt")

	// First open: asked once, naming the plugin, where it lives, what it runs.
	z, _, dl := openWith(t, dir, cfg, false)
	if len(dl.Asked) != 1 {
		t.Fatalf("asked %d times: %q", len(dl.Asked), dl.Asked)
	}
	for _, want := range []string{"hello", filepath.Join(".zen", "plugins", "hello"), "./hello.sh", "scripts in the page"} {
		if !strings.Contains(dl.Asked[0], want) {
			t.Errorf("prompt lacks %q: %q", want, dl.Asked[0])
		}
	}
	// Declined: nothing runs, the page is plain markdown with a note.
	pg, err := z.Page("README.md")
	if err != nil {
		t.Fatal(err)
	}
	if exists(ran) {
		t.Fatal("a declined plugin ran")
	}
	if !hasWarning(pg, notAllowed) || strings.Contains(pg.HTML, `data-zen-plugin="hello"`) || !strings.Contains(pg.HTML, `<code class="language-hello">`) {
		t.Errorf("declined page: %v\n%s", pg.Warnings, pg.HTML)
	}
	// Only built-ins' assets (codeblock wraps the plain fence), none of hello's.
	for _, u := range append(append([]string{}, pg.Assets.JS...), pg.Assets.CSS...) {
		if strings.Contains(u, "/hello/") {
			t.Errorf("declined plugin's assets offered: %+v", pg.Assets)
		}
	}
	if other, _ := z.Page("other.md"); len(other.Warnings) != 0 {
		t.Errorf("page not using the plugin has warnings: %v", other.Warnings)
	}
	if got := infoOf(t, z, "hello"); got.Status != "blocked" || got.Source != "folder" || got.Version != "0.1.0" || got.Description != "Says hello" {
		t.Errorf("info: %+v", got)
	}
	st, err := os.Stat(filepath.Join(cfg, "trusted-plugins.yml"))
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("trust file: %v %v", st, err)
	}

	// Second open: the answer is remembered.
	z2, _, dl2 := openWith(t, dir, cfg, true)
	if len(dl2.Asked) != 0 {
		t.Fatalf("asked again: %q", dl2.Asked)
	}
	if pg, _ := z2.Page("README.md"); !hasWarning(pg, notAllowed) || exists(ran) {
		t.Fatal("block not remembered")
	}

	// A file of the plugin changes: asked again. Nothing runs before the answer.
	if err := os.WriteFile(filepath.Join(dir, ".zen", "plugins", "hello", "hello.css"), []byte(".hello{color:red}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ask := &askingDialogs{fakeDialogs: fakeDialogs{Allow: true}, marker: ran}
	z3, err := newZen(zenOptions{Registry: plugins.NewRegistry(), ConfigDir: cfg, Dir: dir, Events: &sink{}, Dialogs: ask})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z3.close)
	if len(ask.Asked) != 1 || ask.ranBefore {
		t.Fatalf("asked %d times, ran before the answer: %v", len(ask.Asked), ask.ranBefore)
	}
	pg, err = z3.Page("README.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(pg.Warnings) != 0 || !strings.Contains(pg.HTML, "hello from the backend") || !exists(ran) {
		t.Errorf("allowed page: %v\n%s", pg.Warnings, pg.HTML)
	}
	if !strings.Contains(pg.HTML, `data-zen-plugin="hello"`) || !strings.Contains(pg.HTML, `data-zen-name="hello"`) {
		t.Errorf("wrapper: %s", pg.HTML)
	}
	if urlsOf(pg.Assets.JS) != "/_plugins/hello/hello.js" || urlsOf(pg.Assets.CSS) != "/_plugins/hello/hello.css" || !versioned(pg.Assets.JS, 12) {
		t.Errorf("assets: %+v", pg.Assets)
	}
	if pg.Layout != (PageLayout{Name: "article", Source: "builtin"}) {
		t.Errorf("layout %+v", pg.Layout)
	}
	if got := infoOf(t, z3, "hello"); got.Status != "on" || len(got.Runs) != 2 || got.Runs[0] != "./hello.sh" || got.Problem != "" {
		t.Errorf("info: %+v", got)
	}

	// Allowed and unchanged: silent from now on.
	_, _, dl4 := openWith(t, dir, cfg, false)
	if len(dl4.Asked) != 0 {
		t.Fatalf("asked again after allow: %q", dl4.Asked)
	}
	// A folder without plugins never asks.
	_, _, dl5 := openWith(t, tree(t, docs), cfg, false)
	if len(dl5.Asked) != 0 {
		t.Fatalf("asked without plugins: %q", dl5.Asked)
	}
}

func TestOpeningAFolderRunsNothingUntrusted(t *testing.T) {
	dir := pluginTree(t, pluginDocs())
	z, _, dl := openWith(t, dir, t.TempDir(), false)
	if err := z.Open(dir); err != nil { // open it again
		t.Fatal(err)
	}
	for _, p := range []string{"README.md", "other.md"} {
		if _, err := z.Page(p); err != nil {
			t.Fatal(err)
		}
		if _, err := z.Preview(p, "```hello\n```\n"); err != nil {
			t.Fatal(err)
		}
	}
	if exists(filepath.Join(dir, "ran.txt")) {
		t.Fatal("backend ran without approval")
	}
	if len(dl.Asked) != 1 {
		t.Fatalf("asked %d times", len(dl.Asked))
	}
}

func TestOpenAsksForTrust(t *testing.T) {
	dir := pluginTree(t, pluginDocs())
	z, _, dl := openWith(t, tree(t, docs), t.TempDir(), true)
	if err := z.Open(dir); err != nil {
		t.Fatalf("open: %v", err)
	}
	if len(dl.Asked) != 1 {
		t.Fatalf("asked %d times", len(dl.Asked))
	}
	if pg, _ := z.Page("README.md"); !strings.Contains(pg.HTML, "hello from the backend") {
		t.Errorf("plugin not active after allowing: %s", pg.HTML)
	}
}

func TestPluginTrustService(t *testing.T) {
	dir := pluginTree(t, pluginDocs())
	z, ev, dl := openWith(t, dir, t.TempDir(), false)
	if _, err := z.Page("README.md"); err != nil {
		t.Fatal(err)
	}
	ev.reset()
	dl.Allow = true
	if err := z.PluginTrust("hello", true); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ev.all() {
		names = append(names, e.Name)
		if pc, ok := e.Data.(PageChanged); ok && pc.Path != "README.md" {
			t.Errorf("page:changed for %q", pc.Path)
		}
	}
	if strings.Join(names, ",") != EventPluginsChanged+","+EventWorkspaceChanged+","+EventPageChanged {
		t.Fatalf("events: %v", names)
	}
	if pg, _ := z.Page("README.md"); !strings.Contains(pg.HTML, "hello from the backend") || len(pg.Warnings) != 0 {
		t.Errorf("after allow: %v %s", pg.Warnings, pg.HTML)
	}
	if infoOf(t, z, "hello").Status != "on" {
		t.Error("status after allow")
	}
	if err := z.PluginTrust("hello", false); err != nil {
		t.Fatal(err)
	}
	if pg, _ := z.Page("README.md"); !hasWarning(pg, notAllowed) {
		t.Error("block did not take effect")
	}
	if infoOf(t, z, "hello").Status != "blocked" {
		t.Error("status after block")
	}
	for _, id := range []string{"codeblock", "nope", "", "../x"} {
		if err := z.PluginTrust(id, true); err == nil {
			t.Errorf("PluginTrust(%q) accepted", id)
		}
	}
}

func TestPluginsList(t *testing.T) {
	files := pluginDocs()
	files[".zen.yml"] = "version: 1\nplugins:\n  diagram: false\n"
	files[".zen/plugins/broken/plugin.yml"] = "id: broken\nversion: 1.0.0\napiVersion: 1\nmystery: 1\n"
	files[".zen/plugins/codeblock/plugin.yml"] = "id: codeblock\nversion: 1.0.0\napiVersion: 1\n"
	files[".zen/plugins/thief/plugin.yml"] = "id: thief\nversion: 1.0.0\napiVersion: 1\nblocks: [mermaid]\n"
	dir := pluginTree(t, files)
	cfg := t.TempDir()
	user := map[string]string{
		"plugins/mine/plugin.yml":  "id: mine\nversion: 2.0.0\napiVersion: 1\ndescription: Mine\nblocks: [mine]\n",
		"plugins/hello/plugin.yml": "id: hello\nversion: 9.9.9\napiVersion: 1\n", // the folder's hello wins
	}
	for p, c := range user {
		full := filepath.Join(cfg, filepath.FromSlash(p))
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(c), 0o644)
	}
	z, _, dl := openWith(t, dir, cfg, true)
	if len(dl.Asked) != 1 || !strings.Contains(dl.Asked[0], "mine") || strings.Contains(dl.Asked[0], "broken") || strings.Contains(dl.Asked[0], "thief") {
		t.Fatalf("prompt: %q", dl.Asked)
	}
	list := z.Plugins()
	raw, _ := json.Marshal(list)
	var shape []map[string]any
	json.Unmarshal(raw, &shape)
	for _, it := range shape {
		for _, k := range []string{"id", "version", "description", "source", "status", "runs", "problem", "kind", "help", "blocks", "inlines", "options"} {
			if _, ok := it[k]; !ok {
				t.Fatalf("JSON lacks %q: %s", k, raw)
			}
		}
		if len(it) != 12 {
			t.Fatalf("JSON shape: %s", raw)
		}
		for _, k := range []string{"blocks", "inlines", "options"} {
			if _, isList := it[k].([]any); !isList {
				t.Fatalf("%s must be a list, never null: %s", k, raw)
			}
		}
		help, _ := it["help"].(map[string]any)
		if _, isList := help["snippets"].([]any); !isList || len(help) != 2 {
			t.Fatalf("help must be {summary, snippets[]}: %s", raw)
		}
		if _, isList := it["runs"].([]any); !isList {
			t.Fatalf("runs must be a list, never null: %s", raw)
		}
	}
	if len(list) < 2 || list[0].ID != "codeblock" || list[1].ID != "diagram" {
		t.Fatalf("built-ins first: %s", raw)
	}
	want := map[string][2]string{ // id -> source, status
		"codeblock": {"builtin", "on"}, "diagram": {"builtin", "off"},
		"hello": {"folder", "on"}, "mine": {"user", "on"},
		"broken": {"folder", "invalid"}, "thief": {"folder", "invalid"},
	}
	seen := map[string]int{}
	for _, p := range list {
		seen[p.ID]++
		if w, ok := want[p.ID]; ok && p.ID != "codeblock" && (p.Source != w[0] || p.Status != w[1]) {
			t.Errorf("%s: %s/%s, want %v", p.ID, p.Source, p.Status, w)
		}
		if (p.Status == "invalid") != (p.Problem != "") {
			t.Errorf("%s: status %s with problem %q", p.ID, p.Status, p.Problem)
		}
		if strings.Contains(p.Problem, dir) || strings.Contains(p.Problem, cfg) {
			t.Errorf("%s: problem leaks a path: %q", p.ID, p.Problem)
		}
	}
	// codeblock appears as the built-in and as the rejected folder plugin.
	if seen["codeblock"] != 2 || seen["hello"] != 1 || seen["mine"] != 1 || seen["broken"] != 1 || seen["thief"] != 1 {
		t.Errorf("listed: %v", seen)
	}
	if got := infoOf(t, z, "hello"); got.Version != "0.1.0" {
		t.Errorf("the folder's plugin must win over the user's: %+v", got)
	}
	// Rejected manifests are reported on every page until fixed.
	for _, path := range []string{"README.md", "other.md"} {
		pg, err := z.Page(path)
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(pg.Warnings, "\n")
		for _, w := range []string{`plugin "broken"`, `plugin "codeblock"`, `plugin "thief"`} {
			if !strings.Contains(joined, w) {
				t.Errorf("%s: no warning for %s in %q", path, w, pg.Warnings)
			}
		}
		if len(pg.Warnings) != 3 {
			t.Errorf("%s: warnings %q", path, pg.Warnings)
		}
	}
	// The built-in codeblock still works.
	if pg, _ := z.Preview("other.md", "```go\nvar x = 1\n```\n"); !strings.Contains(pg.HTML, "chroma") {
		t.Error("built-in codeblock was replaced")
	}
}

func TestPluginStorage(t *testing.T) {
	dir := pluginTree(t, pluginDocs())
	cfg := t.TempDir()
	z, _, _ := openWith(t, dir, cfg, true)

	if got, err := z.PluginStorageGet("hello", "count"); err != nil || got != "null" {
		t.Fatalf("absent key: %q %v", got, err)
	}
	if err := z.PluginStorageSet("hello", "count", `{"n": 3, "who": ["a", "b"]}`); err != nil {
		t.Fatal(err)
	}
	if err := z.PluginStorageSet("hello", "quiz.q-1_done", `true`); err != nil {
		t.Fatal(err)
	}
	got, err := z.PluginStorageGet("hello", "count")
	var v struct {
		N   int
		Who []string
	}
	if err != nil || json.Unmarshal([]byte(got), &v) != nil || v.N != 3 || len(v.Who) != 2 {
		t.Fatalf("round trip: %q %v", got, err)
	}
	if got, _ := z.PluginStorageGet("hello", "quiz.q-1_done"); got != "true" {
		t.Fatalf("second key: %q", got)
	}

	// Stored outside the folder, per folder and per plugin, private.
	matches, _ := filepath.Glob(filepath.Join(cfg, "data", "*", "hello", "storage.json"))
	if len(matches) != 1 {
		t.Fatalf("storage file: %v", matches)
	}
	if key := filepath.Base(filepath.Dir(filepath.Dir(matches[0]))); !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(key) {
		t.Fatalf("folder key %q", key)
	}
	if st, _ := os.Stat(matches[0]); st.Mode().Perm() != 0o600 {
		t.Fatalf("storage mode %v", st.Mode().Perm())
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(matches[0]), "*")); len(left) != 1 {
		t.Fatalf("temporary files left behind: %v", left)
	}
	if exists(filepath.Join(dir, ".zen", "plugins", "hello", "storage.json")) {
		t.Fatal("storage written into the folder")
	}

	// Persisted across runs; another folder has its own values.
	z2, _, _ := openWith(t, dir, cfg, true)
	if got, _ := z2.PluginStorageGet("hello", "quiz.q-1_done"); got != "true" {
		t.Fatalf("not persisted: %q", got)
	}
	otherDir := pluginTree(t, pluginDocs())
	z3, _, _ := openWith(t, otherDir, cfg, true)
	if got, err := z3.PluginStorageGet("hello", "count"); err != nil || got != "null" {
		t.Fatalf("storage shared between folders: %q %v", got, err)
	}

	// Keys.
	for _, key := range []string{"", "a b", "a/b", "../x", "k\n", strings.Repeat("k", 65), "ключ", "a:b"} {
		if err := z.PluginStorageSet("hello", key, `1`); err == nil {
			t.Errorf("key %q accepted", key)
		}
		if _, err := z.PluginStorageGet("hello", key); err == nil {
			t.Errorf("get with key %q accepted", key)
		}
	}
	if err := z.PluginStorageSet("hello", strings.Repeat("k", 64), `1`); err != nil {
		t.Errorf("64-character key refused: %v", err)
	}
	// Values must be JSON.
	for _, bad := range []string{"", "{", "undefined", `{"a":1} trailing`} {
		if err := z.PluginStorageSet("hello", "bad", bad); err == nil {
			t.Errorf("value %q accepted", bad)
		}
	}
	// 1 MB per plugin, counted over all keys; a refused write changes nothing.
	half := `"` + strings.Repeat("x", 600<<10) + `"`
	if err := z.PluginStorageSet("hello", "big1", half); err != nil {
		t.Fatal(err)
	}
	if err := z.PluginStorageSet("hello", "big2", half); err == nil || !strings.Contains(err.Error(), "1 MB") {
		t.Fatalf("over the limit: %v", err)
	}
	if got, _ := z.PluginStorageGet("hello", "big2"); got != "null" {
		t.Error("refused value stored")
	}
	if got, _ := z.PluginStorageGet("hello", "count"); !strings.Contains(got, `"n"`) {
		t.Error("existing value lost")
	}
	if err := z.PluginStorageSet("hello", "big1", `0`); err != nil { // replacing frees the space
		t.Fatal(err)
	}
	if err := z.PluginStorageSet("hello", "big2", half); err != nil {
		t.Fatalf("after freeing space: %v", err)
	}

	// Refusals: unknown ids, path tricks (built-ins: see TestBuiltinActionAndStorage).
	for _, id := range []string{"nope", "", "../hello", "hello/..", "lint"} {
		if err := z.PluginStorageSet(id, "k", `1`); err == nil {
			t.Errorf("set for %q accepted", id)
		}
		if _, err := z.PluginStorageGet(id, "k"); err == nil {
			t.Errorf("get for %q accepted", id)
		}
	}
	// A blocked plugin has no storage.
	if err := z.PluginTrust("hello", false); err != nil {
		t.Fatal(err)
	}
	if _, err := z.PluginStorageGet("hello", "count"); err == nil {
		t.Error("blocked plugin read storage")
	}
	// Without a folder there is nothing to store for.
	z4, _, _ := openWith(t, "", cfg, true)
	if err := z4.PluginStorageSet("hello", "k", `1`); err == nil {
		t.Error("storage without a folder")
	}
}

func TestPluginInvoke(t *testing.T) {
	files := pluginDocs()
	files[".zen/plugins/quiet/plugin.yml"] = "id: quiet\nversion: 1.0.0\napiVersion: 1\nblocks: [quiet]\nbackend: { command: [\"./q.sh\"], hooks: [block] }\n"
	files[".zen/plugins/quiet/q.sh"] = "#!/bin/sh\ncat\n"
	files[".zen/plugins/chatty/plugin.yml"] = "id: chatty\nversion: 1.0.0\napiVersion: 1\nbackend: { command: [\"./c.sh\"], hooks: [action] }\n"
	files[".zen/plugins/chatty/c.sh"] = "#!/bin/sh\ncase \"$(cat)\" in\n  '\"fail\"') echo broken >&2; exit 2;;\n  '\"empty\"') ;;\n  *) echo not json;;\nesac\n"
	files[".zen/plugins/off/plugin.yml"] = "id: off\nversion: 1.0.0\napiVersion: 1\ndefault: true\nbackend: { command: [\"./o.sh\"], hooks: [action] }\n"
	files[".zen/plugins/off/o.sh"] = "#!/bin/sh\ncat\n"
	files[".zen.yml"] = "version: 1\nplugins:\n  off: false\n"
	dir := pluginTree(t, files)
	z, _, _ := openWith(t, dir, t.TempDir(), true)
	if _, err := z.Page("other.md"); err != nil {
		t.Fatal(err)
	}

	out, err := z.PluginInvoke("hello", "other.md", `{"answer": [1, 2]}`)
	if err != nil {
		t.Fatal(err)
	}
	var res struct {
		Echo struct{ Answer []int }
		Page string
	}
	if json.Unmarshal([]byte(out), &res) != nil || len(res.Echo.Answer) != 2 || res.Page != "other.md" {
		t.Fatalf("invoke result %q", out)
	}
	if exists(filepath.Join(dir, "ran.txt")) {
		t.Error("action ran the block branch")
	}

	if _, err := z.PluginInvoke("hello", "README.md", `{not json`); err == nil {
		t.Error("payload that is not JSON accepted")
	}
	if _, err := z.PluginInvoke("quiet", "README.md", `1`); err == nil || !strings.Contains(err.Error(), "action") {
		t.Errorf("plugin without the action hook: %v", err)
	}
	if _, err := z.PluginInvoke("off", "README.md", `1`); err == nil {
		t.Error("disabled plugin invoked")
	}
	if _, err := z.PluginInvoke("chatty", "README.md", `"x"`); err == nil || !strings.Contains(err.Error(), "JSON") {
		t.Errorf("output that is not JSON: %v", err)
	}
	if _, err := z.PluginInvoke("chatty", "README.md", `"fail"`); err == nil || !strings.Contains(err.Error(), "broken") || strings.Contains(err.Error(), dir) {
		t.Errorf("failing action: %v", err)
	}
	if out, err := z.PluginInvoke("chatty", "README.md", `"empty"`); err != nil || out != "null" {
		t.Errorf("empty output: %q %v", out, err)
	}
	for _, id := range []string{"codeblock", "diagram", "nope", "", "../hello"} {
		if _, err := z.PluginInvoke(id, "README.md", `1`); err == nil {
			t.Errorf("invoke of %q accepted", id)
		}
	}
	// Blocked: refused.
	if err := z.PluginTrust("hello", false); err != nil {
		t.Fatal(err)
	}
	if _, err := z.PluginInvoke("hello", "README.md", `1`); err == nil {
		t.Error("blocked plugin invoked")
	}
	// Never trusted at all: refused, nothing runs.
	dir2 := pluginTree(t, pluginDocs())
	z2, _, _ := openWith(t, dir2, t.TempDir(), false)
	if _, err := z2.PluginInvoke("hello", "README.md", `1`); err == nil {
		t.Error("untrusted plugin invoked")
	}
}

func TestPluginStorageKeys(t *testing.T) {
	dir := pluginTree(t, pluginDocs())
	z, _, _ := openWith(t, dir, t.TempDir(), true)
	if got, err := z.PluginStorageKeys("hello", ""); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty store: %#v %v", got, err)
	}
	for _, k := range []string{"quiz.b", "quiz.a", "other", "quiz.c.d", "Quiz.x"} {
		if err := z.PluginStorageSet("hello", k, `1`); err != nil {
			t.Fatal(err)
		}
	}
	got, err := z.PluginStorageKeys("hello", "quiz.")
	if err != nil || strings.Join(got, ",") != "quiz.a,quiz.b,quiz.c.d" {
		t.Fatalf("prefix: %v %v", got, err)
	}
	if got, _ := z.PluginStorageKeys("hello", ""); strings.Join(got, ",") != "Quiz.x,other,quiz.a,quiz.b,quiz.c.d" {
		t.Fatalf("all, sorted: %v", got)
	}
	if got, err := z.PluginStorageKeys("hello", "zzz"); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("no match: %#v %v", got, err)
	}
	for _, bad := range []string{"a b", "a/b", "../x", "k\n", strings.Repeat("k", 65), "a:b"} {
		if _, err := z.PluginStorageKeys("hello", bad); err == nil {
			t.Errorf("prefix %q accepted", bad)
		}
	}
	for _, id := range []string{"nope", "", "../hello", "lint"} {
		if _, err := z.PluginStorageKeys(id, ""); err == nil {
			t.Errorf("keys of %q accepted", id)
		}
	}
	if err := z.PluginTrust("hello", false); err != nil {
		t.Fatal(err)
	}
	if _, err := z.PluginStorageKeys("hello", ""); err == nil {
		t.Error("blocked plugin listed keys")
	}
}

func TestBuiltinActionAndStorage(t *testing.T) {
	files := pluginDocs()
	files[".zen.yml"] = "version: 1\nplugins:\n  off: false\n"
	dir := pluginTree(t, files)
	cfg := t.TempDir()
	z, _ := openActing(t, dir, cfg, newActor("act"), newActor("off"))
	if _, err := z.Page("other.md"); err != nil {
		t.Fatal(err)
	}
	root := z.Startup().Workspace.Root

	out, err := z.PluginInvoke("act", "other.md", `{"n": 2}`)
	if err != nil {
		t.Fatal(err)
	}
	var res struct {
		Payload   struct{ N int }
		Data      string
		Workspace string
		Page      string
		Config    map[string]any
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil || res.Payload.N != 2 {
		t.Fatalf("invoke: %q %v", out, err)
	}
	if res.Workspace != root || res.Page != "other.md" || res.Config["label"] != "hi" {
		t.Errorf("context: %+v", res)
	}
	// The data folder follows the rule of external plugins.
	if !strings.HasPrefix(res.Data, filepath.Join(cfg, "data")+string(filepath.Separator)) || filepath.Base(res.Data) != "files" || filepath.Base(filepath.Dir(res.Data)) != "act" {
		t.Errorf("data dir %q", res.Data)
	}
	if _, err := z.PluginInvoke("act", "README.md", `"fail"`); err == nil || !strings.Contains(err.Error(), "action failed") {
		t.Errorf("action error: %v", err)
	}
	// No ActionHook: refused. Unknown and switched off: refused.
	for _, id := range []string{"codeblock", "diagram", "nope", "off"} {
		if _, err := z.PluginInvoke(id, "README.md", `1`); err == nil {
			t.Errorf("invoke of %q accepted", id)
		}
	}

	// Storage works for an enabled built-in, in the same place as for others.
	if err := z.PluginStorageSet("act", "k", `{"a":1}`); err != nil {
		t.Fatal(err)
	}
	if got, err := z.PluginStorageGet("act", "k"); err != nil || got != `{"a":1}` {
		t.Fatalf("get: %q %v", got, err)
	}
	if keys, _ := z.PluginStorageKeys("act", ""); strings.Join(keys, ",") != "k" {
		t.Fatalf("keys: %v", keys)
	}
	if matches, _ := filepath.Glob(filepath.Join(cfg, "data", "*", "act", "storage.json")); len(matches) != 1 {
		t.Fatalf("storage file: %v", matches)
	}
	// ... but not for a switched-off or unknown one.
	if err := z.PluginStorageSet("off", "k", `1`); err == nil {
		t.Error("storage of a switched-off built-in")
	}
	if _, err := z.PluginStorageGet("off", "k"); err == nil {
		t.Error("read of a switched-off built-in")
	}
	if _, err := z.PluginStorageKeys("off", ""); err == nil {
		t.Error("keys of a switched-off built-in")
	}
}

func TestUserScopedOptionsFromTheGlobalConfigOnly(t *testing.T) {
	files := pluginDocs()
	files[".zen.yml"] = "version: 1\nplugins:\n  act:\n    label: folder\n    images: { evil: \"attacker:latest\" }\n"
	files["page.md"] = "---\nplugins:\n  act:\n    images: { worse: x }\n---\n# Page\n"
	dir := pluginTree(t, files)
	cfg := t.TempDir()
	if err := os.WriteFile(filepath.Join(cfg, "config.yml"), []byte("version: 1\nplugins:\n  act:\n    images: { node: \"node:20\" }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	z, _ := openActing(t, dir, cfg, newActor("act"))
	pg, err := z.Page("page.md")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, w := range pg.Warnings {
		if strings.Contains(w, `"images"`) {
			n++
		}
	}
	if n != 1 {
		t.Errorf("want one warning about images, got %v", pg.Warnings)
	}
	out, err := z.PluginInvoke("act", "README.md", `1`)
	if err != nil {
		t.Fatal(err)
	}
	var res struct{ Config map[string]any }
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatal(err)
	}
	images, _ := res.Config["images"].(map[string]any)
	if len(images) != 1 || images["node"] != "node:20" {
		t.Errorf("images %v", res.Config["images"])
	}
	if res.Config["label"] != "folder" {
		t.Errorf("a normal option should still come from the folder: %v", res.Config["label"])
	}
}
