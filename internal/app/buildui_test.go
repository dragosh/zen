package app

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/dragosh/zen/internal/cli"
	"github.com/dragosh/zen/internal/core/plugins"
)

// hookSink delivers events to a callback as well as recording them.
type hookSink struct {
	mu   sync.Mutex
	evs  []event
	hook func(name string, data any)
}

func (s *hookSink) Emit(name string, data any) {
	s.mu.Lock()
	s.evs = append(s.evs, event{name, data})
	h := s.hook
	s.mu.Unlock()
	if h != nil {
		h(name, data)
	}
}

func (s *hookSink) progress() []BuildProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []BuildProgress
	for _, e := range s.evs {
		if e.Name == EventBuildProgress {
			out = append(out, e.Data.(BuildProgress))
		}
	}
	return out
}

type buildZen struct {
	z      *Zen
	dir    string
	cfg    string
	sink   *hookSink
	opened []string
}

func newBuildZen(t *testing.T, files map[string]string) *buildZen {
	t.Helper()
	prev := StaticAssets
	StaticAssets = stubAssets()
	t.Cleanup(func() { StaticAssets = prev })
	return newBuildZenIn(t, tree(t, files))
}

func newBuildZenIn(t *testing.T, dir string) *buildZen {
	t.Helper()
	if dir != "" {
		dir, _ = filepath.EvalSymlinks(dir) // the workspace root is symlink-resolved
	}
	b := &buildZen{dir: dir, cfg: t.TempDir(), sink: &hookSink{}}
	z, err := newZen(zenOptions{
		Registry: plugins.NewRegistry(), GlobalPath: filepath.Join(b.cfg, "config.yml"),
		Dir: dir, Mode: "read", Events: b.sink, Dialogs: &fakeDialogs{},
		Open: func(u string) error { b.opened = append(b.opened, u); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	b.z = z
	return b
}

var fivePages = map[string]string{
	"README.md": "# Home\n\n[B](b.md)\n", "b.md": "# B\n", "c.md": "# C\n", "d.md": "# D\n", "e.md": "# E\n",
}

func htmlCount(t *testing.T, out string) int {
	t.Helper()
	n := 0
	for _, e := range mustReadDir(t, out) {
		if strings.HasSuffix(e, ".html") && e != "index.html" {
			n++
		}
	}
	return n
}

func mustReadDir(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

func TestBuildOptions(t *testing.T) {
	b := newBuildZen(t, map[string]string{"README.md": "# Home\n"})
	o, err := b.z.BuildOptions()
	if err != nil {
		t.Fatal(err)
	}
	if want := (BuildOptions{Out: filepath.Join(b.dir, "_site"), Portable: true, Print: true}); o != want {
		t.Fatalf("defaults: %+v, want %+v", o, want)
	}

	// The folder's build: section pre-fills the form.
	b2 := newBuildZen(t, map[string]string{
		"README.md": "# Home\n",
		".zen.yml":  "version: 1\nbuild:\n  out: public\n  portable: false\n  drafts: true\n",
	})
	o, err = b2.z.BuildOptions()
	if err != nil {
		t.Fatal(err)
	}
	if want := (BuildOptions{Out: filepath.Join(b2.dir, "public"), Drafts: true, Print: true}); o != want {
		t.Fatalf("from config: %+v, want %+v", o, want)
	}

	// What was used last time wins, and is kept per folder, atomically.
	used := BuildOptions{Out: filepath.Join(b2.dir, "other"), Portable: true, Drafts: false, Clean: true, Print: false}
	if _, err := b2.z.BuildWebsite(used); err != nil {
		t.Fatal(err)
	}
	if o, err = b2.z.BuildOptions(); err != nil || o != used {
		t.Fatalf("remembered: %+v, %v; want %+v", o, err, used)
	}
	data := filepath.Join(b2.cfg, "data", folderKey(b2.dir))
	if _, err := os.Stat(filepath.Join(data, "build.json")); err != nil {
		t.Fatalf("build.json: %v", err)
	}
	for _, n := range mustReadDir(t, data) {
		if strings.Contains(n, "tmp") || strings.HasSuffix(n, "~") {
			t.Errorf("leftover %s", n)
		}
	}
	// Another folder does not see it.
	if o, _ := b.z.BuildOptions(); o.Clean {
		t.Error("options leaked to another folder")
	}

	// A wrong build: type is an error with the CLI's text.
	b3 := newBuildZen(t, map[string]string{"README.md": "# H\n", ".zen.yml": "build: 3\n"})
	if _, err := b3.z.BuildOptions(); err == nil || !strings.Contains(err.Error(), "build must be a mapping") {
		t.Errorf("wrong type: %v", err)
	}

	// No folder: an error, not a panic.
	z0 := newBuildZenIn(t, "")
	if _, err := z0.z.BuildOptions(); err == nil {
		t.Error("BuildOptions with no folder")
	}
	if _, err := z0.z.BuildWebsite(BuildOptions{}); err == nil {
		t.Error("BuildWebsite with no folder")
	}
}

func TestBuildProgressAndCancel(t *testing.T) {
	if EventBuildProgress != "build:progress" || EventBuildWebsite != "menu:build-website" {
		t.Fatal("event names")
	}
	b := newBuildZen(t, fivePages)
	out := filepath.Join(b.dir, "_site")
	r, err := b.z.BuildWebsite(BuildOptions{Out: out, Portable: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Cancelled || r.Pages != 5 || r.Out != out || r.Millis < 0 {
		t.Fatalf("result: %+v", r)
	}
	got := b.sink.progress()
	if len(got) != 5 {
		t.Fatalf("progress: %+v", got)
	}
	for i, p := range got {
		if p.Done != i+1 || p.Total != 5 {
			t.Errorf("progress[%d] = %+v", i, p)
		}
	}

	// Cancel after the second page: the build stops, keeps what it wrote,
	// and says so; the marker is there so the folder can be built again.
	b2 := newBuildZen(t, fivePages)
	b2.sink.hook = func(name string, data any) {
		if p, ok := data.(BuildProgress); ok && name == EventBuildProgress && p.Done == 2 {
			b2.z.CancelBuild()
		}
	}
	out2 := filepath.Join(b2.dir, "_site")
	r, err = b2.z.BuildWebsite(BuildOptions{Out: out2, Portable: true})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Cancelled || r.Pages != 2 {
		t.Fatalf("cancelled result: %+v", r)
	}
	for _, w := range r.Warnings {
		if strings.Contains(w.Message, "cancel") {
			t.Errorf("a cancel warning leaked: %+v", w)
		}
	}
	if _, err := os.Stat(filepath.Join(out2, ".zen-build")); err != nil {
		t.Errorf("no marker after cancel: %v", err)
	}
	if n := htmlCount(t, out2); n != 2 { // README.html and b.html
		t.Errorf("cancelled build wrote %d pages, want 2", n)
	}
	// And the next build is a normal one.
	b2.sink.hook = nil
	r, err = b2.z.BuildWebsite(BuildOptions{Out: out2, Portable: true, Clean: true})
	if err != nil || r.Cancelled || r.Pages != 5 {
		t.Fatalf("after cancel: %+v, %v", r, err)
	}
	b2.z.CancelBuild() // idle: harmless
}

func TestOneBuildAtATime(t *testing.T) {
	b := newBuildZen(t, fivePages)
	in, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	b.sink.hook = func(name string, data any) {
		if name == EventBuildProgress {
			once.Do(func() { close(in); <-release })
		}
	}
	done := make(chan error, 1)
	go func() {
		_, err := b.z.BuildWebsite(BuildOptions{Out: filepath.Join(b.dir, "_site"), Portable: true})
		done <- err
	}()
	<-in
	if _, err := b.z.BuildWebsite(BuildOptions{Out: filepath.Join(b.dir, "_site2"), Portable: true}); err == nil || !strings.Contains(err.Error(), "already") {
		t.Errorf("second build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(b.dir, "_site2")); err == nil {
		t.Error("the refused build wrote output")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := b.z.BuildWebsite(BuildOptions{Out: filepath.Join(b.dir, "_site2"), Portable: true}); err != nil {
		t.Errorf("after the first finished: %v", err)
	}
}

func TestBuildCancelledByFolderSwitch(t *testing.T) {
	b := newBuildZen(t, fivePages)
	other := tree(t, map[string]string{"README.md": "# Other\n"})
	b.sink.hook = func(name string, data any) {
		if p, ok := data.(BuildProgress); ok && p.Done == 1 {
			if err := b.z.state.open(other); err != nil {
				t.Error(err)
			}
		}
	}
	r, err := b.z.BuildWebsite(BuildOptions{Out: filepath.Join(b.dir, "_site"), Portable: true})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Cancelled {
		t.Errorf("a folder switch did not cancel: %+v", r)
	}
}

func TestBuildErrors(t *testing.T) {
	b := newBuildZen(t, fivePages)
	// The folder itself: refused with the CLI's message.
	if _, err := b.z.BuildWebsite(BuildOptions{Out: b.dir, Portable: true}); err == nil || !strings.Contains(err.Error(), "is the folder being built") {
		t.Errorf("out = folder: %v", err)
	}
	if _, err := b.z.BuildWebsite(BuildOptions{Out: filepath.Dir(b.dir), Portable: true}); err == nil || !strings.Contains(err.Error(), "contains the folder being built") {
		t.Errorf("out = parent: %v", err)
	}
	if _, err := b.z.BuildWebsite(BuildOptions{Out: filepath.Join(b.dir, ".zen", "x"), Portable: true}); err == nil || !strings.Contains(err.Error(), "inside .zen") {
		t.Errorf("out in .zen: %v", err)
	}
	// Clean refuses a folder with no marker, with the CLI's text.
	foreign := t.TempDir()
	if err := os.WriteFile(filepath.Join(foreign, "mine.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := b.z.BuildWebsite(BuildOptions{Out: foreign, Portable: true, Clean: true}); err == nil || !strings.Contains(err.Error(), "will not clean") {
		t.Errorf("clean foreign: %v", err)
	}
	if _, err := os.Stat(filepath.Join(foreign, "mine.txt")); err != nil {
		t.Error("clean removed a foreign file")
	}
	// An error ends the build: the next one can start.
	if _, err := b.z.BuildWebsite(BuildOptions{Out: filepath.Join(b.dir, "_site"), Portable: true}); err != nil {
		t.Errorf("after errors: %v", err)
	}

	// A wrong build: type, with the config's text.
	b2 := newBuildZen(t, map[string]string{"README.md": "# H\n", ".zen.yml": "build:\n  drafts: 7\n"})
	if _, err := b2.z.BuildWebsite(BuildOptions{Portable: true}); err == nil || !strings.Contains(err.Error(), "build.drafts must be true or false") {
		t.Errorf("wrong type: %v", err)
	}
	// A missing runtime.
	b3 := newBuildZen(t, fivePages)
	StaticAssets = nil
	if _, err := b3.z.BuildWebsite(BuildOptions{Portable: true}); err == nil || !strings.Contains(err.Error(), "static runtime") {
		t.Errorf("no runtime: %v", err)
	}
}

func TestSameAsCLI(t *testing.T) {
	files := pluginDocs()
	for p, c := range layoutFiles() {
		if p != "README.md" && !strings.Contains(p, "/broken/") {
			files[p] = c
		}
	}
	files["README.md"] = "# Home\n\n```hello\nx\n```\n\n[memo](memo.md)\n"
	files["memo.md"] = "---\nlayout: memo\nto: Ada\n---\n# Memo\n"
	files["wip.md"] = "---\ndraft: true\n---\n# WIP\n"
	dir := pluginTree(t, files)
	prev := StaticAssets
	StaticAssets = stubAssets()
	t.Cleanup(func() { StaticAssets = prev })
	b := newBuildZenIn(t, dir)
	trustFile := filepath.Join(b.cfg, "trusted-plugins.yml")
	trustBefore, _ := os.ReadFile(trustFile)

	for _, c := range []struct {
		name string
		cli  cli.Build
		opt  BuildOptions
	}{
		{"portable", cli.Build{}, BuildOptions{Portable: true, Print: true}},
		{"hosted drafts", cli.Build{Hosted: true, Drafts: true}, BuildOptions{Drafts: true, Print: true}},
		{"no print", cli.Build{NoPrint: true}, BuildOptions{Portable: true}},
	} {
		cliOut := filepath.Join(t.TempDir(), "cli")
		c.cli.Out = cliOut
		ref := runBuild(t, dir, b.cfg, c.cli)
		if ref.err != nil {
			t.Fatal(ref.err)
		}
		uiOut := filepath.Join(t.TempDir(), "ui")
		c.opt.Out = uiOut
		r, err := b.z.BuildWebsite(c.opt)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		ui := buildRun{out: uiOut}
		a, bf := ref.files(t), ui.files(t)
		if strings.Join(a, ",") != strings.Join(bf, ",") {
			t.Fatalf("%s: files differ\ncli: %v\nui:  %v", c.name, a, bf)
		}
		for _, f := range a {
			if strings.HasSuffix(f, ".html") && ref.read(t, f) != ui.read(t, f) {
				t.Errorf("%s: %s differs", c.name, f)
			}
		}
		if r.Pages == 0 || r.Files == 0 {
			t.Errorf("%s: counts %+v", c.name, r)
		}
		// Warnings are the CLI's, by page.
		var cliWarn []string
		for _, l := range strings.Split(strings.TrimSpace(ref.stderr), "\n") {
			if l != "" {
				cliWarn = append(cliWarn, l)
			}
		}
		var uiWarn []string
		for _, w := range r.Warnings {
			if w.Path == "" {
				uiWarn = append(uiWarn, "site: "+w.Message)
			} else {
				uiWarn = append(uiWarn, w.Path+": "+w.Message)
			}
		}
		if strings.Join(cliWarn, "\n") != strings.Join(uiWarn, "\n") || len(uiWarn) == 0 {
			t.Errorf("%s: warnings\ncli: %q\nui:  %q", c.name, cliWarn, uiWarn)
		}
		if !strings.Contains(strings.Join(uiWarn, "\n"), `plugin "hello"`) {
			t.Errorf("untrusted plugin not reported: %q", uiWarn)
		}
	}
	// Read-only trust: the build never wrote the trust file.
	if after, _ := os.ReadFile(trustFile); string(after) != string(trustBefore) {
		t.Error("the build changed the trust file")
	}
}

func TestSaveBuildDefaults(t *testing.T) {
	b := newBuildZen(t, map[string]string{"README.md": "# Home\n"})
	yml := filepath.Join(b.dir, ".zen.yml")
	opt := BuildOptions{Out: filepath.Join(b.dir, "docs-site"), Portable: false, Drafts: true, Clean: true}

	// No .zen.yml: preview first, then create it with version and build:.
	prev, err := b.z.SaveBuildDefaults(opt, false)
	if err != nil || !strings.Contains(prev, "Adds to .zen.yml") || !strings.Contains(prev, "docs-site") {
		t.Fatalf("preview: %q, %v", prev, err)
	}
	if _, err := os.Stat(yml); err == nil {
		t.Fatal("preview wrote the file")
	}
	if _, err := b.z.SaveBuildDefaults(opt, true); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(yml)
	for _, want := range []string{"version: 1", "build:", "out: docs-site", "portable: false", "drafts: true"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("created file lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(string(got), "clean") {
		t.Error("clean is a per-run option, not a default")
	}
	if o, err := b.z.BuildOptions(); err != nil || o.Out != opt.Out || o.Portable || !o.Drafts {
		t.Errorf("config round trip: %+v, %v", o, err)
	}
	for _, n := range mustReadDir(t, b.dir) {
		if strings.Contains(n, "tmp") {
			t.Errorf("leftover %s", n)
		}
	}

	// An existing file keeps its comments, other keys and other build keys.
	orig := "# my site\nversion: 1\ntheme: dark # keep\n\nbuild:\n  # where it goes\n  out: _site\n  exclude:\n    - \"drafts/**\"\n  portable: true\nplugins:\n  check: true\n"
	if err := os.WriteFile(yml, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	opt2 := BuildOptions{Out: filepath.Join(b.dir, "_site"), Portable: true, Drafts: true}
	prev, err = b.z.SaveBuildDefaults(opt2, false)
	if err != nil || !strings.Contains(prev, "drafts") || strings.Contains(prev, "portable") || strings.Contains(prev, "out:") {
		t.Fatalf("only the changed keys are previewed: %q, %v", prev, err)
	}
	if after, _ := os.ReadFile(yml); string(after) != orig {
		t.Fatal("preview changed the file")
	}
	if _, err := b.z.SaveBuildDefaults(opt2, true); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(yml)
	for _, keep := range []string{"# my site", "theme: dark # keep", "# where it goes", "out: _site", `"drafts/**"`, "portable: true", "check: true", "drafts: true"} {
		if !strings.Contains(string(after), keep) {
			t.Errorf("lost %q:\n%s", keep, after)
		}
	}

	// Nothing to change says so.
	prev, err = b.z.SaveBuildDefaults(opt2, false)
	if err != nil || !strings.Contains(prev, "already") {
		t.Errorf("no-op preview: %q, %v", prev, err)
	}

	// An output outside the folder is stored as an absolute path.
	far := filepath.Join(t.TempDir(), "site")
	if _, err := b.z.SaveBuildDefaults(BuildOptions{Out: far, Portable: true, Drafts: true}, true); err != nil {
		t.Fatal(err)
	}
	if o, _ := b.z.BuildOptions(); o.Out != far {
		t.Errorf("outside out: %q", o.Out)
	}

	// A file that is not a YAML mapping is refused and left alone.
	bad := "- a\n- b\n"
	if err := os.WriteFile(yml, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := b.z.SaveBuildDefaults(opt, true); err == nil {
		t.Error("saved into a list")
	}
	if now, _ := os.ReadFile(yml); string(now) != bad {
		t.Error("a refused save changed the file")
	}
	// A build: that is not a mapping likewise.
	if err := os.WriteFile(yml, []byte("build: 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := b.z.SaveBuildDefaults(opt, true); err == nil {
		t.Error("saved into build: 3")
	}
}

func TestOpenBuiltSite(t *testing.T) {
	b := newBuildZen(t, fivePages)
	var revealed []string
	prev := revealPath
	revealPath = func(p string) error { revealed = append(revealed, p); return nil }
	t.Cleanup(func() { revealPath = prev })

	out := filepath.Join(b.dir, "_site")
	if err := b.z.OpenBuiltSite(out); err == nil {
		t.Error("opened a site that was never built")
	}
	if _, err := b.z.BuildWebsite(BuildOptions{Out: out, Portable: true}); err != nil {
		t.Fatal(err)
	}
	if err := b.z.OpenBuiltSite(out); err != nil {
		t.Fatal(err)
	}
	if len(b.opened) != 1 || !strings.HasPrefix(b.opened[0], "file://") || !strings.HasSuffix(b.opened[0], "/_site/index.html") {
		t.Fatalf("opened %q", b.opened)
	}
	if err := b.z.RevealBuiltSite(out); err != nil || len(revealed) != 1 || revealed[0] != out {
		t.Errorf("reveal: %v %v", revealed, err)
	}

	// Only the last build's folder.
	for _, p := range []string{b.dir, filepath.Join(b.dir, "b.md"), filepath.Join(b.dir, "_site", ".."), "/", "/etc"} {
		if err := b.z.OpenBuiltSite(p); err == nil {
			t.Errorf("opened %s", p)
		}
		if err := b.z.RevealBuiltSite(p); err == nil {
			t.Errorf("revealed %s", p)
		}
	}
	// A marker that is gone, or an index that is a symlink: refused.
	if err := os.Remove(filepath.Join(out, ".zen-build")); err != nil {
		t.Fatal(err)
	}
	if err := b.z.OpenBuiltSite(out); err == nil {
		t.Error("opened without the marker")
	}
	if len(b.opened) != 1 {
		t.Errorf("opened %q", b.opened)
	}
}

func TestPickBuildFolder(t *testing.T) {
	b := newBuildZen(t, fivePages)
	b.z.dialogs.(*fakeDialogs).folder = "/some/where"
	if got, err := b.z.PickBuildFolder(); err != nil || got != "/some/where" {
		t.Errorf("pick: %q, %v", got, err)
	}
	// Choosing does not open it as the workspace.
	if w := b.z.Startup().Workspace; w == nil || w.Root != b.dir {
		t.Errorf("workspace changed: %+v", w)
	}
}

// The dialog can switch drafts off although .zen.yml has build.drafts: true.
func TestDialogDraftsOverrideConfig(t *testing.T) {
	dir := tree(t, map[string]string{
		"README.md": "# Home\n",
		"wip.md":    "---\ndraft: true\n---\n# WIP\n",
		".zen.yml":  "version: 1\nbuild:\n  drafts: true\n",
	})
	out := filepath.Join(t.TempDir(), "site")
	r := runBuild(t, dir, t.TempDir(), cli.Build{Out: out, NoDrafts: true})
	if r.err != nil || r.has("wip.html") {
		t.Fatalf("drafts off: %v %v", r.err, r.files(t))
	}
	r = runBuild(t, dir, t.TempDir(), cli.Build{Out: filepath.Join(t.TempDir(), "s2")})
	if r.err != nil || !r.has("wip.html") {
		t.Fatalf("config drafts on: %v %v", r.err, r.files(t))
	}
}
