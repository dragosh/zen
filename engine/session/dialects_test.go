package session

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/zenmark-org/zenmd/builtins"
	"github.com/zenmark-org/zenmd/engine/dialects"
	"github.com/zenmark-org/zenmd/engine/plugins"
)

// noteOnly is a plugin without hooks that owns the block "note".
type noteOnly struct{}

func (noteOnly) Manifest() plugins.Manifest {
	return plugins.Manifest{ID: "note", Version: "1.0.0", APIVersion: plugins.APIVersion, Default: true, Blocks: []string{"note"}}
}

func noteRegistry(t *testing.T) *plugins.Registry {
	t.Helper()
	r := plugins.NewRegistry()
	if err := r.Register(noteOnly{}); err != nil {
		t.Fatal(err)
	}
	return r
}

const mystPage = "```{note} Hello\nBody text.\n```\n"

// dialectSession opens dir with the MyST dialect in the build (or, with none
// true, an empty dialect registry) and a settings folder of its own.
func dialectSession(t *testing.T, dir, cfg string, readOnly, none bool) *Session {
	t.Helper()
	reg := builtins.Dialects()
	if none {
		reg, _ = dialects.NewRegistry()
	}
	s := New(Options{
		GlobalPath: filepath.Join(cfg, "config.yml"), Registry: noteRegistry(t), Dialects: reg,
		Events: &sink{}, Runtime: stubAssets(), ReadOnly: readOnly,
	})
	t.Cleanup(s.Close)
	if err := s.Open(dir); err != nil {
		t.Fatal(err)
	}
	return s
}

func pageHTML(t *testing.T, s *Session, path string) string {
	t.Helper()
	p, err := s.Page(path)
	if err != nil {
		t.Fatal(err)
	}
	return p.HTML
}

func TestExportUsesDialects(t *testing.T) {
	on := tree(t, map[string]string{"README.md": mystPage, ".zen.yml": "markdown:\n  dialects: [myst]\n"})
	r := runBuild(t, on, t.TempDir(), SiteOptions{Portable: true}, func(e *SiteEnv) {
		e.Registry, e.Dialects = noteRegistry(t), builtins.Dialects()
	})
	if r.err != nil {
		t.Fatalf("%v\n%s", r.err, r.stderr)
	}
	if html := r.read(t, "index.html"); !strings.Contains(html, `data-zen-plugin="note"`) || !strings.Contains(html, "Body text.") {
		t.Errorf("the built site did not render the MyST directive:\n%s", html)
	}
	// the folder does not list it: a code block, as before
	off := tree(t, map[string]string{"README.md": mystPage})
	r = runBuild(t, off, t.TempDir(), SiteOptions{Portable: true}, func(e *SiteEnv) {
		e.Registry, e.Dialects = noteRegistry(t), builtins.Dialects()
	})
	if html := r.read(t, "index.html"); strings.Contains(html, `data-zen-plugin="note"`) {
		t.Errorf("rendered without the folder asking:\n%s", html)
	}
	// the build does not have it: ignored with a warning
	empty, _ := dialects.NewRegistry()
	r = runBuild(t, on, t.TempDir(), SiteOptions{Portable: true}, func(e *SiteEnv) { e.Registry, e.Dialects = noteRegistry(t), empty })
	if html := r.read(t, "index.html"); r.err != nil || strings.Contains(html, `data-zen-plugin="note"`) {
		t.Errorf("a build without the dialect: %v", r.err)
	}
}

func TestSetDialect(t *testing.T) {
	dir := tree(t, map[string]string{"README.md": mystPage, ".zen.yml": "# mine\nversion: 1\n"})
	s := dialectSession(t, dir, t.TempDir(), false, false)
	if strings.Contains(pageHTML(t, s, "README.md"), `data-zen-plugin="note"`) {
		t.Fatal("on before it was turned on")
	}
	if err := s.SetDialect("myst", true); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, ".zen.yml"))
	if !strings.HasPrefix(string(b), "# mine\nversion: 1\n") || !strings.Contains(string(b), "dialects: [myst]") {
		t.Errorf(".zen.yml:\n%s", b)
	}
	if !strings.Contains(pageHTML(t, s, "README.md"), `data-zen-plugin="note"`) {
		t.Error("the page was not re-rendered with the dialect")
	}
	if infos := s.Dialects(); len(infos) != 1 || infos[0].ID != "myst" || !infos[0].On || infos[0].Name == "" || infos[0].Summary == "" {
		t.Errorf("Dialects: %+v", infos)
	}
	// again: nothing changes
	if err := s.SetDialect("myst", true); err != nil {
		t.Fatal(err)
	}
	if b2, _ := os.ReadFile(filepath.Join(dir, ".zen.yml")); string(b2) != string(b) {
		t.Errorf("a second SetDialect changed the file:\n%s", b2)
	}
	if err := s.SetDialect("myst", false); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(dir, ".zen.yml"))
	if string(b) != "# mine\nversion: 1\n" {
		t.Errorf("after off:\n%s", b)
	}
	if strings.Contains(pageHTML(t, s, "README.md"), `data-zen-plugin="note"`) {
		t.Error("still on")
	}
	if s.Dialects()[0].On {
		t.Error("Dialects still says on")
	}
	// not in this build
	if err := s.SetDialect("mkdocs", true); err == nil {
		t.Error("an unknown dialect was accepted")
	}
	// a folder without .zen.yml gets one
	dir2 := tree(t, map[string]string{"README.md": mystPage})
	s2 := dialectSession(t, dir2, t.TempDir(), false, false)
	if err := s2.SetDialect("myst", true); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir2, ".zen.yml")); string(b) != "markdown:\n  dialects: [myst]\n" {
		t.Errorf("created:\n%s", b)
	}
	// no folder
	none := New(Options{Registry: noteRegistry(t), Events: &sink{}})
	t.Cleanup(none.Close)
	if err := none.SetDialect("myst", true); !errors.Is(err, ErrNoWorkspace) {
		t.Errorf("no folder: %v", err)
	}
}

func TestSetDialectReadOnly(t *testing.T) {
	dir := tree(t, map[string]string{"README.md": mystPage, "myst.yml": "version: 1\n"})
	s := dialectSession(t, dir, t.TempDir(), true, false)
	before := snapshot(t, dir)
	if err := s.SetDialect("myst", true); !errors.Is(err, ErrReadOnly) {
		t.Errorf("SetDialect: %v", err)
	}
	if after := snapshot(t, dir); len(after) != len(before) {
		t.Errorf("the folder changed: %v", after)
	}
	if _, err := os.Stat(filepath.Join(dir, ".zen.yml")); err == nil {
		t.Error(".zen.yml was written")
	}
	// no hint in a read-only app
	if h := s.DialectHint(); h != "" {
		t.Errorf("hint %q", h)
	}
}

func TestDialectHint(t *testing.T) {
	myst := map[string]string{"README.md": mystPage, "myst.yml": "version: 1\n"}
	cfg := t.TempDir()
	dir := tree(t, myst)
	s := dialectSession(t, dir, cfg, false, false)
	if h := s.DialectHint(); h != "myst" {
		t.Fatalf("hint %q", h)
	}
	// an ordinary folder has none
	if h := dialectSession(t, tree(t, map[string]string{"README.md": "# x\n"}), t.TempDir(), false, false).DialectHint(); h != "" {
		t.Errorf("plain folder: %q", h)
	}
	// a build without the dialect has none
	if h := dialectSession(t, tree(t, myst), t.TempDir(), false, true).DialectHint(); h != "" {
		t.Errorf("build without it: %q", h)
	}
	// already listed
	listed := tree(t, map[string]string{"README.md": mystPage, "myst.yml": "", ".zen.yml": "markdown:\n  dialects: [myst]\n"})
	if h := dialectSession(t, listed, t.TempDir(), false, false).DialectHint(); h != "" {
		t.Errorf("listed: %q", h)
	}
	// turning it on ends the hint
	if err := s.SetDialect("myst", true); err != nil {
		t.Fatal(err)
	}
	if h := s.DialectHint(); h != "" {
		t.Errorf("after turning on: %q", h)
	}
	if err := s.SetDialect("myst", false); err != nil {
		t.Fatal(err)
	}
	if h := s.DialectHint(); h != "myst" {
		t.Errorf("after turning off: %q", h)
	}

	// Not now: remembered by the app, not in the folder, for this folder only
	before := snapshot(t, dir)
	if err := s.DismissDialectHint("myst"); err != nil {
		t.Fatal(err)
	}
	if h := s.DialectHint(); h != "" {
		t.Errorf("after dismissing: %q", h)
	}
	if after := snapshot(t, dir); len(after) != len(before) {
		t.Error("dismissing wrote into the folder")
	}
	again := dialectSession(t, dir, cfg, false, false)
	if h := again.DialectHint(); h != "" {
		t.Errorf("a new session forgot the dismissal: %q", h)
	}
	other := dialectSession(t, tree(t, myst), cfg, false, false)
	if h := other.DialectHint(); h != "myst" {
		t.Errorf("another folder: %q", h)
	}
	if err := s.DismissDialectHint("mkdocs"); err == nil {
		t.Error("an unknown id was accepted")
	}
	// dismissing works in a read-only app too (it writes app state only)
	ro := dialectSession(t, tree(t, myst), cfg, true, false)
	if err := ro.DismissDialectHint("myst"); err != nil {
		t.Errorf("read-only dismiss: %v", err)
	}
}

// The demo's user guide page on dialects renders with the plugins the folder
// ships: the MyST fences become the same blocks as :::.
func TestDemoDialectsPage(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	demo := filepath.Join(filepath.Dir(file), "..", "..", "examples", "demo")
	z, err := newZen(zenOptions{Registry: builtins.Plugins(), ConfigDir: t.TempDir(), Dir: demo, Events: &sink{}, Dialogs: &fakeDialogs{Allow: true}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	pg, err := z.Page("plugins/dialects.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(pg.Warnings) != 0 {
		t.Errorf("warnings: %v", pg.Warnings)
	}
	for _, want := range []string{
		`data-zen-plugin="note" data-zen-name="note"`, `<strong>Remember</strong>`,
		`data-zen-plugin="note" data-zen-name="important"`, `<strong>Back up first</strong>`,
		`data-zen-plugin="callout" data-zen-name="tip"`, `class="chroma"`,
	} {
		if !strings.Contains(pg.HTML, want) {
			t.Errorf("missing %q in\n%s", want, pg.HTML)
		}
	}
	// the examples in code fences are still code
	if !strings.Contains(pg.HTML, "```{note} Remember") && !strings.Contains(pg.HTML, "{note} Remember") {
		t.Errorf("the code samples are gone:\n%s", pg.HTML)
	}
}
