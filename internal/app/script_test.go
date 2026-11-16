package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

const shoutSrc = `plugin := {
    version: "0.2.0",
    description: "Shouts",
    blocks: ["shout"],
    inlines: ["bang"],
    hooks: ["action"],
    css: "div.shout { font-weight: bold }"
}
if hook == "block" {
    out = zen.html("div", {class: "shout"}, zen.markdown(body))
}
if hook == "inline" { out = zen.html("b", {}, body + "!") }
if hook == "action" {
    n := (zen.storage.get("n") || 0) + 1
    zen.storage.set("n", n)
    out = {n: n, echo: payload, page: page.path}
}
`

const listerSrc = `plugin := {version: "1.0.0", blocks: ["lister"], permissions: ["read"]}
if hook == "block" {
    items := []
    for p in zen.pages() { items = append(items, zen.html("li", {}, p.path + ": " + p.title)) }
    out = zen.html("ul", {}, items)
}
`

func scriptDocs() map[string]string {
	return map[string]string{
		"README.md":                        "# Home\n\n:::shout\nhello **world** :bang[hey]\n:::\n\n:::lister\n:::\n\n[Other](other.md)\n",
		"other.md":                         "# Other page\n\nplain\n",
		".zen/plugins/shout.tengo":         shoutSrc,
		".zen/plugins/lister/plugin.tengo": listerSrc,
		".zen/plugins/lister/notes.txt":    "ignored data",
	}
}

func TestScriptPluginsAreDiscoveredAskedAboutAndRendered(t *testing.T) {
	dir := pluginTree(t, scriptDocs())
	cfg := t.TempDir()
	z, ev, dl := openWith(t, dir, cfg, true)

	// Always asked, once, and truthfully.
	if len(dl.asked) != 1 {
		t.Fatalf("asked %d times: %q", len(dl.asked), dl.asked)
	}
	q := dl.asked[0]
	for _, want := range []string{
		"shout 0.2.0", "lister 1.0.0", filepath.Join(".zen", "plugins", "shout.tengo"), filepath.Join(".zen", "plugins", "lister"),
		"renders blocks inside zen; cannot read files, run programs or use the network",
		"reads files in this folder", "Script plugins",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("prompt lacks %q:\n%s", want, q)
		}
	}
	if strings.Contains(q, "runs programs and scripts on your Mac") {
		t.Errorf("a prompt about script plugins only claims they run programs:\n%s", q)
	}
	if !hasEvent(ev, EventPluginsChanged) {
		t.Error("no plugins:changed")
	}

	// Kinds: script / builtin; the rest of the info is the same as for folders.
	if got := infoOf(t, z, "shout"); got.Kind != "script" || got.Source != "folder" || got.Status != "on" || got.Version != "0.2.0" || got.Description != "Shouts" {
		t.Errorf("shout: %+v", got)
	}
	if got := infoOf(t, z, "lister"); got.Kind != "script" || len(got.Runs) != 1 || !strings.Contains(got.Runs[0], "reads files in this folder") {
		t.Errorf("lister: %+v", got)
	}
	if got := infoOf(t, z, "codeblock"); got.Kind != "builtin" {
		t.Errorf("codeblock: %+v", got)
	}

	pg, err := z.Page("README.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(pg.Warnings) != 0 {
		t.Errorf("warnings: %v", pg.Warnings)
	}
	for _, want := range []string{
		`data-zen-plugin="shout"`, `<div class="shout">`, `<strong>world</strong>`, `<b>hey!</b>`,
		`<li>README.md: Home</li>`, `<li>other.md: Other page</li>`,
	} {
		if !strings.Contains(pg.HTML, want) {
			t.Errorf("page lacks %q:\n%s", want, pg.HTML)
		}
	}
	if urlsOf(pg.Assets.CSS) != "/_plugins/shout/inline.css" || !versioned(pg.Assets.CSS, 12) {
		t.Errorf("assets: %+v", pg.Assets)
	}
	// The inline CSS is served from the app's own origin, as CSS.
	frontend := fstest.MapFS{"index.html": {Data: []byte("x")}}
	rec := httptest.NewRecorder()
	z.state.middleware(http.FileServerFS(frontend)).ServeHTTP(rec, httptest.NewRequest("GET", "/_plugins/shout/inline.css", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "font-weight: bold") || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/css") {
		t.Errorf("inline.css: %d %q %q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}

	// Remembered: no second prompt; an unrelated folder never asks.
	_, _, dl2 := openWith(t, dir, cfg, false)
	if len(dl2.asked) != 0 {
		t.Errorf("asked again: %q", dl2.asked)
	}
}

func TestScriptPluginsOnlyScriptsHeader(t *testing.T) {
	dir := pluginTree(t, scriptDocs())
	_, _, dl := openWith(t, dir, t.TempDir(), true)
	if len(dl.asked) != 1 || !strings.Contains(dl.asked[0], "Script plugins run inside zen") {
		t.Fatalf("prompt: %q", dl.asked)
	}
	// One script plugin: "Allow this plugin?".
	dir2 := pluginTree(t, map[string]string{"README.md": "# H\n", ".zen/plugins/one.tengo": `plugin := {version: "1.0.0", blocks: ["one"]}`})
	_, _, dl2 := openWith(t, dir2, t.TempDir(), true)
	if len(dl2.asked) != 1 || !strings.HasPrefix(dl2.asked[0], "Allow this plugin?") || !strings.Contains(dl2.asked[0], "cannot read files, run programs or use the network") {
		t.Fatalf("single prompt: %q", dl2.asked)
	}
	// Mixed with a program plugin the prompt keeps the program warning.
	files := pluginDocs()
	files[".zen/plugins/one.tengo"] = `plugin := {version: "1.0.0", blocks: ["one"]}`
	_, _, dl3 := openWith(t, pluginTree(t, files), t.TempDir(), true)
	if len(dl3.asked) != 1 || !strings.Contains(dl3.asked[0], "runs programs and scripts on your Mac") || !strings.Contains(dl3.asked[0], "./hello.sh") || !strings.Contains(dl3.asked[0], "cannot read files") {
		t.Fatalf("mixed prompt: %q", dl3.asked)
	}
	if got := infoOf(t, func() *Zen { z, _, _ := openWith(t, pluginTree(t, files), t.TempDir(), true); return z }(), "hello"); got.Kind != "program" {
		t.Errorf("hello: %+v", got)
	}
}

func TestDeclinedScriptPluginsRenderAsPlainMarkdown(t *testing.T) {
	dir := pluginTree(t, scriptDocs())
	z, _, dl := openWith(t, dir, t.TempDir(), false)
	if len(dl.asked) != 1 {
		t.Fatal(dl.asked)
	}
	pg, err := z.Page("README.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(pg.HTML, `data-zen-plugin="shout"`) || strings.Contains(pg.HTML, "<b>hey!</b>") {
		t.Errorf("a declined plugin ran: %s", pg.HTML)
	}
	if !hasWarning(pg, `plugin "shout" is not allowed (Settings › Plugins)`) {
		t.Errorf("warnings: %v", pg.Warnings)
	}
	if got := infoOf(t, z, "shout"); got.Status != "blocked" {
		t.Errorf("status %s", got.Status)
	}
	if _, err := z.PluginInvoke("shout", "README.md", `1`); err == nil {
		t.Error("a blocked script plugin was invoked")
	}
	// Allowing it from Settings asks again and then it runs.
	dl.confirm = true
	if err := z.PluginTrust("shout", true); err != nil {
		t.Fatal(err)
	}
	if got := infoOf(t, z, "shout"); got.Status != "on" {
		t.Errorf("after allow: %+v", got)
	}
	pg, _ = z.Page("README.md")
	if !strings.Contains(pg.HTML, "<b>hey!</b>") {
		t.Errorf("allowed plugin did not run: %s", pg.HTML)
	}
}

func TestEditingAScriptPluginAsksAgain(t *testing.T) {
	for _, form := range []string{"single", "folder"} {
		t.Run(form, func(t *testing.T) {
			dir := pluginTree(t, scriptDocs())
			z, ev, _ := openWith(t, dir, t.TempDir(), true)
			pg, _ := z.Page("README.md")
			if !strings.Contains(pg.HTML, "<b>hey!</b>") {
				t.Fatalf("setup: %s", pg.HTML)
			}
			ev.reset()
			path, id := ".zen/plugins/shout.tengo", "shout"
			evil := strings.Replace(shoutSrc, `body + "!"`, `"EVIL"`, 1)
			if form == "folder" {
				path, id = ".zen/plugins/lister/plugin.tengo", "lister"
				evil = strings.Replace(listerSrc, `p.path + ": "`, `"EVIL" + `, 1)
			}
			write(t, dir, path, evil, 0o644)
			later := time.Now().Add(3 * time.Second)
			os.Chtimes(filepath.Join(dir, path), later, later)
			if _, err := z.PluginInvoke("shout", "README.md", `1`); form == "single" && err == nil {
				t.Error("an edited script plugin was invoked")
			}
			if pg, _ := z.Page("README.md"); strings.Contains(pg.HTML, "EVIL") {
				t.Fatalf("an edited script ran: %s", pg.HTML)
			}
			waitFor(t, "status undecided", func() bool { return infoOf(t, z, id).Status == "undecided" })
			if !hasEvent(ev, EventPluginsChanged) {
				t.Error("no plugins:changed after the edit")
			}
			if pg, _ := z.Page("README.md"); strings.Contains(pg.HTML, "EVIL") || !hasWarning(pg, `plugin "`+id+`" is not allowed (Settings › Plugins)`) {
				t.Errorf("after reload: %v %s", pg.Warnings, pg.HTML)
			}
			if form == "single" {
				rec := httptest.NewRecorder()
				z.state.middleware(http.FileServerFS(fstest.MapFS{})).ServeHTTP(rec, httptest.NewRequest("GET", "/_plugins/shout/inline.css", nil))
				if rec.Code == 200 {
					t.Error("css of a changed plugin was served")
				}
			}
		})
	}
}

func TestScriptPluginErrorsAreWarningsNotFailures(t *testing.T) {
	files := scriptDocs()
	files[".zen/plugins/broken.tengo"] = "plugin := {\n  version: \"1.0.0\"\n  blocks: [\"b\"]\n}\n"
	files[".zen/plugins/bad-os.tengo"] = "os := import(\"os\")\n" + `plugin := {version: "1.0.0", blocks: ["b2"]}`
	files[".zen/plugins/slow.tengo"] = `plugin := {version: "1.0.0", blocks: ["slow"], timeout: "50ms"}` + "\nif hook == \"block\" { for { } }\n"
	files["slow.md"] = "# Slow\n\n:::slow\nx\n:::\n\nafter the block\n"
	dir := pluginTree(t, files)
	z, _, dl := openWith(t, dir, t.TempDir(), true)
	if len(dl.asked) != 1 || strings.Contains(dl.asked[0], "broken") || strings.Contains(dl.asked[0], "bad-os") {
		t.Fatalf("prompt: %q", dl.asked)
	}
	for _, id := range []string{"broken", "bad-os"} {
		got := infoOf(t, z, id)
		if got.Status != "invalid" || got.Problem == "" || got.Kind != "script" || strings.Contains(got.Problem, dir) {
			t.Errorf("%s: %+v", id, got)
		}
	}
	pg, _ := z.Page("other.md")
	joined := strings.Join(pg.Warnings, "\n")
	if !strings.Contains(joined, `plugin "broken"`) || !strings.Contains(joined, `plugin "bad-os"`) || strings.Contains(joined, dir) {
		t.Errorf("warnings: %q", pg.Warnings)
	}
	// A script that hangs costs its timeout, shows an error box and the page still renders.
	t0 := time.Now()
	pg, err := z.Page("slow.md")
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(t0) > 3*time.Second || !strings.Contains(pg.HTML, "zen-plugin-error") || !strings.Contains(pg.HTML, "timed out") || !strings.Contains(pg.HTML, "after the block") {
		t.Errorf("slow script: %v\n%s", time.Since(t0), pg.HTML)
	}
	if !hasWarning(pg, `plugin "slow": timed out after 50ms`) {
		t.Errorf("warnings: %v", pg.Warnings)
	}
}

func TestScriptPluginOutputIsSanitised(t *testing.T) {
	files := map[string]string{
		"README.md": "# H\n\n:::evil\nx\n:::\n",
		".zen/plugins/evil.tengo": `plugin := {version: "1.0.0", blocks: ["evil"]}
if hook == "block" { out = "<script>alert(1)</script><img src=x onerror=alert(1)><p style='x'>ok</p>" }`,
	}
	z, _, _ := openWith(t, pluginTree(t, files), t.TempDir(), true)
	pg, _ := z.Page("README.md")
	for _, bad := range []string{"<script", "onerror", "style="} {
		if strings.Contains(pg.HTML, bad) {
			t.Errorf("page keeps %q: %s", bad, pg.HTML)
		}
	}
	if !strings.Contains(pg.HTML, "<p>ok</p>") {
		t.Errorf("page: %s", pg.HTML)
	}
}

func TestScriptPluginActionAndStorage(t *testing.T) {
	dir := pluginTree(t, scriptDocs())
	cfg := t.TempDir()
	z, _, _ := openWith(t, dir, cfg, true)
	if _, err := z.Page("other.md"); err != nil {
		t.Fatal(err)
	}
	var res struct {
		N    int
		Echo map[string]int
		Page string
	}
	for want := 1; want <= 2; want++ {
		out, err := z.PluginInvoke("shout", "other.md", `{"x": 7}`)
		if err != nil || json.Unmarshal([]byte(out), &res) != nil || res.N != want || res.Echo["x"] != 7 || res.Page != "other.md" {
			t.Fatalf("invoke %d: %q %v", want, out, err)
		}
	}
	// zen.storage is the store of ctx.storage: the same values, the same file.
	if got, err := z.PluginStorageGet("shout", "n"); err != nil || got != "2" {
		t.Errorf("ctx.storage sees %q %v", got, err)
	}
	if err := z.PluginStorageSet("shout", "n", `41`); err != nil {
		t.Fatal(err)
	}
	if out, _ := z.PluginInvoke("shout", "other.md", `1`); !strings.Contains(out, `"n":42`) {
		t.Errorf("the script does not see the value ctx.storage set: %s", out)
	}
	if keys, _ := z.PluginStorageKeys("shout", ""); len(keys) != 1 || keys[0] != "n" {
		t.Errorf("keys %v", keys)
	}
	matches, _ := filepath.Glob(filepath.Join(cfg, "data", "*", "shout", "storage.json"))
	if len(matches) != 1 {
		t.Errorf("storage file: %v", matches)
	}
	// An action on a plugin that does not declare it is refused.
	if _, err := z.PluginInvoke("lister", "other.md", `1`); err == nil || !strings.Contains(err.Error(), "action") {
		t.Errorf("lister: %v", err)
	}
	// Markdown works in actions too (the page's pipeline).
	files := map[string]string{
		"README.md": "# H\n",
		".zen/plugins/md.tengo": `plugin := {version: "1.0.0", hooks: ["action"]}
if hook == "action" { out = {html: zen.markdown("**" + payload + "**")} }`,
	}
	z2, _, _ := openWith(t, pluginTree(t, files), t.TempDir(), true)
	out, err := z2.PluginInvoke("md", "README.md", `"hi"`)
	var md struct{ HTML string }
	if err != nil || json.Unmarshal([]byte(out), &md) != nil || !strings.Contains(md.HTML, "<strong>hi</strong>") {
		t.Errorf("markdown in an action: %q %v", out, err)
	}
}

func TestScriptPluginReadIsLimitedToTheFolder(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "secret.md")
	os.WriteFile(secret, []byte("TOP SECRET"), 0o644)
	files := map[string]string{
		"README.md": "# H\n\n:::cat\nREADME.md\n:::\n\n:::cat\nleak.md\n:::\n\n:::cat\n../secret.md\n:::\n",
		".zen/plugins/cat.tengo": `text := import("text")
plugin := {version: "1.0.0", blocks: ["cat"], permissions: ["read"]}
if hook == "block" { out = zen.html("pre", {}, zen.read(text.trim_space(body))) }`,
	}
	dir := pluginTree(t, files)
	os.Symlink(secret, filepath.Join(dir, "leak.md"))
	z, _, _ := openWith(t, dir, t.TempDir(), true)
	pg, _ := z.Page("README.md")
	if strings.Contains(pg.HTML, "SECRET") {
		t.Errorf("the page shows a file outside the folder: %s", pg.HTML)
	}
	if strings.Count(pg.HTML, "zen-plugin-error") != 2 || !strings.Contains(pg.HTML, "# H") {
		t.Errorf("page: %s", pg.HTML)
	}
}

func TestScriptPluginsInTheUserFolderApplyEverywhere(t *testing.T) {
	cfg := t.TempDir()
	os.MkdirAll(filepath.Join(cfg, "plugins"), 0o755)
	os.WriteFile(filepath.Join(cfg, "plugins", "mine.tengo"), []byte(`plugin := {version: "1.0.0", blocks: ["mine"]}
if hook == "block" { out = zen.html("div", {class: "mine"}, body) }`), 0o644)
	dirA := pluginTree(t, map[string]string{"README.md": "# A\n\n:::mine\nfrom A\n:::\n"})
	z, _, dl := openWith(t, dirA, cfg, true)
	if len(dl.asked) != 1 {
		t.Fatalf("asked %q", dl.asked)
	}
	if got := infoOf(t, z, "mine"); got.Source != "user" || got.Kind != "script" || got.Status != "on" {
		t.Errorf("mine: %+v", got)
	}
	// The folder's plugin of the same id wins over the user's.
	files := map[string]string{"README.md": "# B\n\n:::mine\nfrom B\n:::\n", ".zen/plugins/mine.tengo": `plugin := {version: "2.0.0", blocks: ["mine"]}
if hook == "block" { out = zen.html("div", {class: "folder"}, body) }`}
	z2, _, _ := openWith(t, pluginTree(t, files), cfg, true)
	pg, _ := z2.Page("README.md")
	if got := infoOf(t, z2, "mine"); got.Source != "folder" || got.Version != "2.0.0" || !strings.Contains(pg.HTML, `class="folder"`) {
		t.Errorf("shadowing: %+v %s", got, pg.HTML)
	}
	// A folder with both mine.tengo and mine/ is reported, not guessed.
	files = map[string]string{
		"README.md":                     "# C\n",
		".zen/plugins/dup.tengo":        `plugin := {version: "1.0.0", blocks: ["d1"]}`,
		".zen/plugins/dup/plugin.tengo": `plugin := {version: "1.0.0", blocks: ["d2"]}`,
	}
	z3, _, _ := openWith(t, pluginTree(t, files), t.TempDir(), true)
	if got := infoOf(t, z3, "dup"); got.Status != "invalid" || !strings.Contains(got.Problem, "both") {
		t.Errorf("dup: %+v", got)
	}
}
