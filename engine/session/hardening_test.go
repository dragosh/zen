package session

// Regression tests for docs/plugin-review-2026-11.md (app side).

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dragosh/zen/internal/core/plugins"
)

func hasEvent(ev *sink, name string) bool {
	for _, e := range ev.all() {
		if e.Name == name {
			return true
		}
	}
	return false
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

func write(t *testing.T, dir, rel, body string, mode os.FileMode) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

// Contract 4: plugins:changed after a folder open and after a trust decision;
// contract 2: asset URLs carry ?v=.
func TestPluginsChangedEventAndVersionedAssets(t *testing.T) {
	dir := pluginTree(t, pluginDocs())
	z, ev, _ := openWith(t, dir, t.TempDir(), true)
	if !hasEvent(ev, EventPluginsChanged) {
		t.Error("no plugins:changed after the folder open")
	}
	pg, err := z.Page("README.md")
	if err != nil || !versioned(pg.Assets.JS, 12) {
		t.Fatalf("external assets: %+v %v", pg.Assets, err)
	}
	ev.reset()
	if err := z.PluginTrust("hello", false); err != nil {
		t.Fatal(err)
	}
	if !hasEvent(ev, EventPluginsChanged) {
		t.Error("no plugins:changed after blocking")
	}
	// Built-ins: the app version.
	z2, _, _ := openWith(t, tree(t, map[string]string{"README.md": "# A\n\n```go\nx := 1\n```\n"}), t.TempDir(), true)
	pg, err = z2.Page("README.md")
	if err != nil || len(pg.Assets.JS) == 0 {
		t.Fatalf("builtin page: %+v %v", pg.Assets, err)
	}
	for _, u := range append(pg.Assets.JS, pg.Assets.CSS...) {
		if !strings.HasSuffix(u, "?v="+"dev") {
			t.Errorf("built-in asset %q lacks ?v=%s", u, "dev")
		}
	}
	// The query is ignored when serving.
	for _, u := range []string{"/_plugins/codeblock/codeblock.js?v=other", "/_plugins/codeblock/codeblock.js"} {
		rec := httptest.NewRecorder()
		z2.middleware(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest("GET", u, nil))
		if rec.Code != 200 {
			t.Errorf("%s: %d", u, rec.Code)
		}
	}
}

// Contract 5: Allow needs the user's yes in the native dialog.
func TestAllowNeedsTheDialog(t *testing.T) {
	dir := pluginTree(t, pluginDocs())
	z, ev, dl := openWith(t, dir, t.TempDir(), false)
	dl.Asked = nil
	ev.reset()
	if err := z.PluginTrust("hello", true); err != nil || len(dl.Asked) != 1 || !strings.Contains(dl.Asked[0], "hello") {
		t.Fatalf("declined: err=%v asked=%q", err, dl.Asked)
	}
	if st := infoOf(t, z, "hello").Status; st == "on" {
		t.Fatal("a declined dialog allowed the plugin")
	}
	if hasEvent(ev, EventPluginsChanged) {
		t.Error("a declined dialog changed the plugin set")
	}
	dl.Allow = true
	if err := z.PluginTrust("hello", true); err != nil || infoOf(t, z, "hello").Status != "on" {
		t.Fatalf("confirmed: %v %+v", err, infoOf(t, z, "hello"))
	}
	// Blocking needs no dialog.
	n := len(dl.Asked)
	if err := z.PluginTrust("hello", false); err != nil || len(dl.Asked) != n || infoOf(t, z, "hello").Status != "blocked" {
		t.Fatalf("block: %v asked %d", err, len(dl.Asked)-n)
	}
	// Without dialogs nothing can be allowed.
	z.prompter = nil
	if err := z.PluginTrust("hello", true); err == nil {
		t.Error("allowed without a way to ask")
	}
}

// M2: the Allow click is for the files as shown; a change in between is refused.
type editingDialogs struct {
	fakeDialogs
	edit func()
}

func (d *editingDialogs) Confirm(msg string) bool {
	if d.edit != nil {
		d.edit()
	}
	return d.fakeDialogs.Confirm(msg)
}

func TestM2ChangeWhileTheDialogIsOpen(t *testing.T) {
	dir := pluginTree(t, pluginDocs())
	dl := &editingDialogs{fakeDialogs: fakeDialogs{Allow: true}}
	dl.edit = func() { write(t, dir, ".zen/plugins/hello/hello.sh", "#!/bin/sh\necho EVIL\n", 0o755) }
	reg := plugins.NewRegistry()
	z, err := newZen(zenOptions{Registry: reg, ConfigDir: t.TempDir(), Dir: dir, Events: &sink{}, Dialogs: dl, Open: func(string) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	pg, _ := z.Page("README.md")
	if strings.Contains(pg.HTML, "EVIL") || infoOf(t, z, "hello").Status != "undecided" {
		t.Fatalf("approved files the user never saw: %+v", infoOf(t, z, "hello"))
	}
	joined := strings.Join(pg.Warnings, "\n")
	if !strings.Contains(joined, "changed") {
		t.Errorf("the refusal is not shown: %v", pg.Warnings)
	}
	// The same through PluginTrust.
	dir2 := pluginTree(t, pluginDocs())
	dl2 := &editingDialogs{}
	z2, err := newZen(zenOptions{Registry: reg, ConfigDir: t.TempDir(), Dir: dir2, Events: &sink{}, Dialogs: dl2, Open: func(string) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z2.close)
	dl2.Allow = true
	dl2.edit = func() { write(t, dir2, ".zen/plugins/hello/hello.sh", "#!/bin/sh\necho EVIL\n", 0o755) }
	if err := z2.PluginTrust("hello", true); err == nil || infoOf(t, z2, "hello").Status == "on" {
		t.Fatalf("allowed a plugin that changed during the dialog: %v", err)
	}
}

// M1: an edit after approval stops the plugin, its assets and asks again.
func TestM1EditAfterApproval(t *testing.T) {
	dir := pluginTree(t, pluginDocs())
	z, ev, _ := openWith(t, dir, t.TempDir(), true)
	pg, _ := z.Page("README.md")
	if !strings.Contains(pg.HTML, "hello from the backend") {
		t.Fatalf("setup: %s", pg.HTML)
	}
	frontend := fstest.MapFS{"index.html": {Data: []byte("x")}}
	get := func(p string) int {
		rec := httptest.NewRecorder()
		z.middleware(http.FileServerFS(frontend)).ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
		return rec.Code
	}
	if get("/_plugins/hello/hello.js") != 200 {
		t.Fatal("setup: asset not served")
	}
	ev.reset()
	os.Remove(filepath.Join(dir, "ran.txt"))
	write(t, dir, ".zen/plugins/hello/hello.sh", "#!/bin/sh\ntouch \"$ZEN_WORKSPACE/ran.txt\"\necho EVIL\n", 0o755)
	if _, err := z.PluginInvoke("hello", "README.md", `1`); err == nil {
		t.Error("an edited plugin was invoked")
	}
	if pg, _ := z.Page("README.md"); strings.Contains(pg.HTML, "EVIL") || exists(filepath.Join(dir, "ran.txt")) {
		t.Fatalf("an edited plugin ran: %s", pg.HTML)
	}
	waitFor(t, "status undecided", func() bool { return infoOf(t, z, "hello").Status == "undecided" })
	if !hasEvent(ev, EventPluginsChanged) {
		t.Error("no plugins:changed after the edit")
	}
	if get("/_plugins/hello/hello.js") == 200 {
		t.Error("asset of a changed plugin was served")
	}
	if pg, _ := z.Page("README.md"); strings.Contains(pg.HTML, "EVIL") || !hasWarning(pg, notAllowed) {
		t.Errorf("after reload: %v %s", pg.Warnings, pg.HTML)
	}
}

const optYML = `id: opt
version: 0.1.0
apiVersion: 1
default: false
options:
  label: { type: string, default: base }
backend:
  command: ["./opt.sh"]
  hooks: [action]
`

const optSH = `#!/bin/sh
cat >/dev/null
printf '{"page":"%s","label":"%s"}' "$ZEN_PAGE" "$ZEN_CONFIG_LABEL"
`

// M4: invoke and stream use the settings of the page they are given.
func TestM4CallsAreResolvedForThePage(t *testing.T) {
	files := map[string]string{
		"README.md":                   "# Home\n",
		"on.md":                       "---\nplugins:\n  opt:\n    label: fm\n---\n# On\n",
		"off.md":                      "# Off\n",
		".zen/plugins/opt/plugin.yml": optYML,
		".zen/plugins/opt/opt.sh":     optSH,
	}
	z, _, _ := openWith(t, pluginTree(t, files), t.TempDir(), true)
	if _, err := z.Page("off.md"); err != nil { // the page on screen is not the one asked about
		t.Fatal(err)
	}
	out, err := z.PluginInvoke("opt", "on.md", `1`)
	if err != nil || !strings.Contains(out, `"page":"on.md"`) || !strings.Contains(out, `"label":"fm"`) {
		t.Fatalf("on.md: %q %v", out, err)
	}
	if _, err := z.PluginInvoke("opt", "off.md", `1`); err == nil || !strings.Contains(err.Error(), "switched off") {
		t.Fatalf("off.md: %v", err)
	}
	if _, err := z.PluginInvoke("opt", "../secret.txt", `1`); err == nil {
		t.Error("a path that is not a page was accepted")
	}
}

const slowYML = `id: slow
version: 0.1.0
apiVersion: 1
blocks: [slow]
backend:
  command: ["./slow.sh"]
  hooks: [block]
  timeout: 30s
`

func slowTree(t *testing.T, secs string) string {
	return pluginTree(t, map[string]string{
		"README.md":                    "# Home\n\n:::slow\nx\n:::\n",
		".zen/plugins/slow/plugin.yml": slowYML,
		".zen/plugins/slow/slow.sh":    "#!/bin/sh\nsleep " + secs + "\necho done\n",
	})
}

// M3: a slow backend does not stall the rest of the app.
func TestM3SlowBackendDoesNotHoldTheStateLock(t *testing.T) {
	z, _, _ := openWith(t, slowTree(t, "2"), t.TempDir(), true)
	done := make(chan struct{})
	go func() { z.Page("README.md"); close(done) }()
	time.Sleep(300 * time.Millisecond)
	start := time.Now()
	z.Startup()
	z.Plugins()
	if _, err := z.Source("README.md"); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 700*time.Millisecond {
		t.Fatalf("the app was stalled for %v by a rendering backend", d)
	}
	<-done
}

// L3: a folder change ends the running backend.
func TestL3FolderChangeCancelsRenders(t *testing.T) {
	z, _, _ := openWith(t, slowTree(t, "20"), t.TempDir(), true)
	done := make(chan Page, 1)
	go func() { pg, _ := z.Page("README.md"); done <- pg }()
	time.Sleep(300 * time.Millisecond)
	start := time.Now()
	if err := z.Open(tree(t, map[string]string{"README.md": "# Other\n"})); err != nil {
		t.Fatal(err)
	}
	select {
	case pg := <-done:
		if !strings.Contains(strings.Join(pg.Warnings, " "), "cancelled") {
			t.Errorf("warnings: %v", pg.Warnings)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the render kept running after the folder changed")
	}
	if time.Since(start) > 4*time.Second {
		t.Error("slow cancel")
	}
}

// M6: blocking or switching off a plugin stops its running streams.
func TestM6BlockingStopsStreams(t *testing.T) {
	dir := tickTree(t)
	z, ev, _ := openWith(t, dir, t.TempDir(), true)
	call, err := z.PluginStream("tick", "README.md", `"hang"`)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "first output", func() bool { return strings.Contains(chunksOf(streamEvents(ev, call)), "up") })
	if err := z.PluginTrust("tick", false); err != nil {
		t.Fatal(err)
	}
	es := finish(t, ev, call)
	if es[len(es)-1].Error != "cancelled" {
		t.Fatalf("blocked plugin kept streaming: %+v", es[len(es)-1])
	}
}

func TestM6SwitchingOffStopsStreams(t *testing.T) {
	dir := pluginTree(t, pluginDocs())
	act := newActor("act")
	z, ev := openActing(t, dir, t.TempDir(), act)
	call, err := z.PluginStream("act", "README.md", `"wait"`)
	if err != nil {
		t.Fatal(err)
	}
	<-act.started
	write(t, dir, ".zen.yml", "plugins:\n  act: false\n", 0o644)
	z.reloadConfig()
	es := finish(t, ev, call)
	if es[len(es)-1].Error != "cancelled" {
		t.Fatalf("switched-off plugin kept streaming: %+v", es[len(es)-1])
	}
}

// L1: output is batched; empty lines count against the limit.
func TestL1BatcherJoinsLines(t *testing.T) {
	var mu sync.Mutex
	var got []PluginStreamEvent
	b := &batcher{call: "c", c: t.Context(), send: func(_ string, d any) {
		mu.Lock()
		got = append(got, d.(PluginStreamEvent))
		mu.Unlock()
	}}
	for i := 0; i < 5000; i++ {
		b.add([]byte("a"))
		b.add(nil)
	}
	b.finish(true)
	time.Sleep(2 * streamBatch)
	mu.Lock()
	defer mu.Unlock()
	if len(got) > 3 {
		t.Fatalf("%d events for 10000 lines", len(got))
	}
	lines := 0
	for _, e := range got {
		lines += len(strings.Split(e.Chunk, "\n"))
	}
	if lines != 10000 {
		t.Fatalf("%d lines delivered", lines)
	}
}

const nlYML = `id: nl
version: 0.1.0
apiVersion: 1
backend:
  command: ["./nl.sh"]
  hooks: [stream]
  timeout: 60s
`

func TestL1EmptyLinesAreLimitedAndBatched(t *testing.T) {
	dir := pluginTree(t, map[string]string{
		"README.md":                  "# Home\n",
		".zen/plugins/nl/plugin.yml": nlYML,
		".zen/plugins/nl/nl.sh":      "#!/bin/sh\nhead -c 5000000 /dev/zero | tr '\\0' '\\n'\n",
	})
	z, ev, _ := openWith(t, dir, t.TempDir(), true)
	call, err := z.PluginStream("nl", "README.md", `1`)
	if err != nil {
		t.Fatal(err)
	}
	es := finish(t, ev, call)
	if last := es[len(es)-1]; last.Error != "output limit" {
		t.Fatalf("end: %+v", last)
	}
	if len(es) > 400 {
		t.Fatalf("%d events for a 4 MB stream of empty lines", len(es))
	}
}

// L4: concurrent trust changes leave a consistent plugin list.
func TestL4ConcurrentTrustChanges(t *testing.T) {
	z, _, dl := openWith(t, pluginTree(t, pluginDocs()), t.TempDir(), true)
	dl.Allow = true
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 4; j++ {
				_ = z.PluginTrust("hello", (i+j)%2 == 0)
				z.Plugins()
			}
		}(i)
	}
	wg.Wait()
	if err := z.PluginTrust("hello", true); err != nil {
		t.Fatal(err)
	}
	if st := infoOf(t, z, "hello").Status; st != "on" {
		t.Fatalf("status %q", st)
	}
	if pg, _ := z.Page("README.md"); !strings.Contains(pg.HTML, "hello from the backend") {
		t.Error("the registry does not match the trust decision")
	}
}

// L5: a folder that links to an approved plugin is asked about.
func TestL5ApprovalIsPerFolder(t *testing.T) {
	shared := t.TempDir()
	for name, body := range map[string]string{"plugin.yml": helloYML, "hello.sh": helloSH, "hello.js": "1", "hello.css": "a{}"} {
		mode := os.FileMode(0o644)
		if strings.HasSuffix(name, ".sh") {
			mode = 0o755
		}
		write(t, shared, name, body, mode)
	}
	mkFolder := func() string {
		dir := tree(t, map[string]string{"README.md": "# Home\n\n```hello\nx\n```\n"})
		os.MkdirAll(filepath.Join(dir, ".zen", "plugins"), 0o755)
		if err := os.Symlink(shared, filepath.Join(dir, ".zen", "plugins", "hello")); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	cfg := t.TempDir()
	a, _, dla := openWith(t, mkFolder(), cfg, true)
	if len(dla.Asked) != 1 || infoOf(t, a, "hello").Status != "on" {
		t.Fatalf("folder A: asked %d, %+v", len(dla.Asked), infoOf(t, a, "hello"))
	}
	b, _, dlb := openWith(t, mkFolder(), cfg, false)
	if len(dlb.Asked) != 1 || infoOf(t, b, "hello").Status == "on" {
		t.Fatalf("folder B ran an approval of folder A: asked %d, %+v", len(dlb.Asked), infoOf(t, b, "hello"))
	}
}

// L8: a damaged storage file is set aside, reported, never overwritten.
func TestL8CorruptStorage(t *testing.T) {
	z, _, _ := openWith(t, pluginTree(t, pluginDocs()), t.TempDir(), true)
	if err := z.PluginStorageSet("hello", "k", `1`); err != nil {
		t.Fatal(err)
	}
	path, err := z.storagePath("hello")
	if err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("storage file: %v %v", fi, err)
	}
	os.WriteFile(path, []byte("{not json"), 0o600)
	if _, err := z.PluginStorageGet("hello", "k"); err == nil || !strings.Contains(err.Error(), "damaged") {
		t.Fatalf("first read: %v", err)
	}
	matches, _ := filepath.Glob(path + ".corrupt-*")
	if len(matches) != 1 {
		t.Fatalf("corrupt file not kept: %v", matches)
	}
	if v, err := z.PluginStorageGet("hello", "k"); err != nil || v != "null" {
		t.Fatalf("second read: %q %v", v, err)
	}
	if err := z.PluginStorageSet("hello", "k", `2`); err != nil {
		t.Fatal(err)
	}
}

// L9: files of the workspace are never served as scripts or documents.
func TestL9WorkspaceScriptsArePlainText(t *testing.T) {
	dir := tree(t, map[string]string{"README.md": "# A\n", "x.js": "alert(1)", "x.mjs": "1", "x.html": "<script>1</script>", "x.svg": "<svg xmlns='http://www.w3.org/2000/svg'/>"})
	z, _, _ := openWith(t, dir, t.TempDir(), true)
	get := func(p string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		z.middleware(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
		return rec
	}
	for _, f := range []string{"x.js", "x.mjs", "x.html"} {
		if ct := get("/_ws/" + f).Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
			t.Errorf("%s served as %q", f, ct)
		}
	}
	r := get("/_ws/x.svg")
	if !strings.HasPrefix(r.Header().Get("Content-Type"), "image/svg+xml") || r.Header().Get("Content-Security-Policy") != "sandbox" {
		t.Errorf("svg: %v", r.Header())
	}
}
