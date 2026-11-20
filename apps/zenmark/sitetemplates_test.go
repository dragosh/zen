package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/zenmark-org/zenmd/engine/lint"
	"github.com/zenmark-org/zenmd/engine/plugins"
	"github.com/zenmark-org/zenmd/engine/session"
	"github.com/zenmark-org/zenmd/engine/sitetemplates"
	"github.com/zenmark-org/zenmd/internal/cli"
	"github.com/zenmark-org/zenmd/kit/desktop"
)

// siteRig is a settings folder with a global config, and an env that uses
// zen's real presets.
func siteRig(t *testing.T, config string) (siteEnv, string) {
	t.Helper()
	cfg := t.TempDir()
	global := filepath.Join(cfg, "config.yml")
	if config != "" {
		if err := os.WriteFile(global, []byte(config), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return siteEnv{globalPath: global, builtins: siteBuiltins}, cfg
}

func idsOf(l []sitetemplates.Template) string {
	var ids []string
	for _, t := range l {
		ids = append(ids, t.ID)
	}
	return strings.Join(ids, ",")
}

func TestSiteTemplatesService(t *testing.T) {
	cfg := t.TempDir()
	z, err := newZen(zenOptions{Registry: plugins.NewRegistry(), GlobalPath: filepath.Join(cfg, "config.yml"), Events: &sink{}, Dialogs: &fakeDialogs{}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	got, _ := z.SiteTemplates()
	if len(got) != 3 || got[0].ID != "techdocs" || got[1].ID != "learning" || got[2].ID != "planner" {
		t.Fatalf("templates: %+v", got)
	}
	if got[0].Name != "Technical documentation" || len(got[0].Ask) != 2 || got[0].Ask[0].Key != "title" || got[0].Ask[0].Default != "{{folder}}" || !got[0].Builtin || got[0].Error != "" {
		t.Errorf("techdocs: %+v", got[0])
	}
	// They were installed for editing, next to config.yml.
	if _, err := os.Stat(filepath.Join(cfg, "templates", "sites", "techdocs", "template.yml")); err != nil {
		t.Errorf("not installed: %v", err)
	}
	// The JSON the frontend reads.
	b, _ := json.Marshal(got[0])
	for _, want := range []string{`"id":"techdocs"`, `"ask":[{"key":"title","label":"Site title","default":"{{folder}}"}`, `"builtin":true`, `"update":""`, `"error":""`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("json lacks %s: %s", want, b)
		}
	}
	// Edits are picked up; an invalid template is listed with its error.
	if err := os.WriteFile(filepath.Join(cfg, "templates", "sites", "planner", "template.yml"), []byte("id: planner\nname: My planner\norder: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cfg, "templates", "sites", "broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, _ = z.SiteTemplates()
	if got[0].Name != "My planner" || got[len(got)-1].ID != "broken" || got[len(got)-1].Error == "" {
		t.Errorf("edited: %+v", got)
	}
	// No settings folder at all: the presets come straight from zen.
	z2, _, _, _ := newTestZen(t, "", "", "")
	if l, _ := z2.SiteTemplates(); len(l) != 3 {
		t.Errorf("without settings folder: %+v", l)
	}
}

func TestTemplatesDirFromConfig(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "my templates")
	env, cfg := siteRig(t, "templates:\n  dir: "+custom+"\n")
	l := env.list()
	if l.Dir != custom || len(l.Templates) != 3 || l.Note != "" {
		t.Fatalf("custom dir: %+v", l)
	}
	if _, err := os.Stat(filepath.Join(custom, "sites", "learning", "template.yml")); err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(filepath.Join(cfg, "templates")); err == nil {
		t.Error("the default folder was used as well")
	}
}

func TestUnwritableTemplatesDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes everywhere")
	}
	base := t.TempDir()
	if err := os.Chmod(base, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(base, 0o700)
	env := siteEnv{globalPath: filepath.Join(base, "config.yml"), builtins: siteBuiltins}
	l := env.list()
	if idsOf(l.Templates) != "techdocs,learning,planner" || !strings.Contains(l.Note, "cannot be written to") {
		t.Errorf("fallback: %s %q", idsOf(l.Templates), l.Note)
	}
	// And they work.
	var out, errOut bytes.Buffer
	target := filepath.Join(t.TempDir(), "docs")
	if err := initSite(env.api(), cli.Init{Dir: target, Template: "planner", Values: map[string]string{}}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "cannot be written to") || !strings.Contains(out.String(), "Created 8 files") {
		t.Errorf("out %q err %q", out.String(), errOut.String())
	}
}

func TestCreateSite(t *testing.T) {
	z, ev, dl, _ := newTestZen(t, "", "", "")
	_ = ev
	// Relative folders, unknown templates and the templates folder are refused.
	if _, err := z.CreateSite("techdocs", "docs", nil); err == nil {
		t.Error("relative folder accepted")
	}
	if _, err := z.CreateSite("nosuch", t.TempDir(), nil); err == nil || !strings.Contains(err.Error(), "nosuch") {
		t.Errorf("unknown: %v", err)
	}
	cfg := t.TempDir()
	zc, err := newZen(zenOptions{Registry: plugins.NewRegistry(), GlobalPath: filepath.Join(cfg, "config.yml"), Events: &sink{}, Dialogs: &fakeDialogs{}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(zc.close)
	_, _ = zc.SiteTemplates()
	if _, err := zc.CreateSite("techdocs", filepath.Join(cfg, "templates"), nil); err == nil || !strings.Contains(err.Error(), "templates folder") {
		t.Errorf("templates folder: %v", err)
	}

	// A folder with spaces and unicode; the result is the new workspace, starting at its README.
	target := filepath.Join(t.TempDir(), "Dokümente ✨")
	st, err := z.CreateSite("techdocs", target, map[string]string{"title": "Käse", "author": "Ana", "extra": "x"})
	if err != nil {
		t.Fatal(err)
	}
	if st.Workspace == nil || sameDir(st.Workspace.Root, target) == false || st.Page != "README.md" || st.Mode != "read" {
		t.Fatalf("startup: %+v", st)
	}
	if src, err := z.Source("README.md"); err != nil || !strings.Contains(src, "# Käse") || !strings.Contains(src, "by Ana") && !strings.Contains(src, "Documentation by Ana") {
		t.Errorf("README: %v %q", err, src)
	}
	// The same folder again: every file clashes, nothing changes, the workspace stays.
	if _, err := z.CreateSite("techdocs", target, nil); err == nil || !strings.Contains(err.Error(), "already exist") {
		t.Errorf("second create: %v", err)
	}
	// Into the folder that is open right now, with other files: allowed when nothing clashes.
	busy := t.TempDir()
	if err := os.WriteFile(filepath.Join(busy, "mine.md"), []byte("# Mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := z.s.Open(busy); err != nil {
		t.Fatal(err)
	}
	if st, err := z.CreateSite("planner", busy, nil); err != nil || !sameDir(st.Workspace.Root, busy) {
		t.Errorf("into the open folder: %+v %v", st, err)
	} else if b, _ := os.ReadFile(filepath.Join(busy, "mine.md")); string(b) != "# Mine\n" {
		t.Error("existing file changed")
	}

	// The folder panel: the dialogs' PickFolder when there is no special panel.
	dl.folder = "/somewhere"
	if p, err := z.PickSiteFolder(); err != nil || p != "/somewhere" {
		t.Errorf("pick: %q %v", p, err)
	}
}

func TestCreateSiteClosesEditors(t *testing.T) {
	r := newWindowRig(t)
	z := windowsZen(t, r, map[string]string{"README.md": "# Old\n", "a.md": "# A\n", "b.md": "# B\n"})
	r.open(t, "a.md", "b.md")
	target := filepath.Join(t.TempDir(), "new site")

	// An editor keeps its unsaved changes: nothing is created or opened.
	type result struct {
		st  Startup
		err error
	}
	done := make(chan result, 1)
	go func() { st, err := z.CreateSite("learning", target, nil); done <- result{st, err} }()
	waitAsked(t, r, 1)
	r.m.Reply("editor:a.md", desktop.ReplyPending)
	r.m.Reply("editor:a.md", desktop.ReplyCancel)
	res := <-done
	if res.err == nil || !strings.Contains(res.err.Error(), "nothing was created") || !strings.Contains(res.err.Error(), "unsaved") {
		t.Fatalf("cancelled: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(target, "README.md")); err == nil {
		t.Error("the site was written although an editor kept its changes")
	}
	if z.Startup().Workspace.Name == filepath.Base(target) {
		t.Error("the folder was opened although an editor said no")
	}

	// Every editor closes: the new folder replaces the old one.
	target2 := filepath.Join(t.TempDir(), "second")
	go func() { st, err := z.CreateSite("learning", target2, nil); done <- result{st, err} }()
	waitAsked(t, r, 2)
	r.m.Reply("editor:a.md", desktop.ReplyClose)
	waitAsked(t, r, 3)
	r.m.Reply("editor:b.md", desktop.ReplyClose)
	res = <-done
	if res.err != nil || res.st.Workspace == nil || !sameDir(res.st.Workspace.Root, target2) || res.st.Page != "README.md" || r.m.EditorCount() != 0 {
		t.Fatalf("closed: %+v editors %d", res, r.m.EditorCount())
	}
}

func TestRevealTemplates(t *testing.T) {
	var shown []string
	old := revealInFinder
	revealInFinder = func(d string) error { shown = append(shown, d); return nil }
	t.Cleanup(func() { revealInFinder = old })
	cfg := t.TempDir()
	z, err := newZen(zenOptions{Registry: plugins.NewRegistry(), GlobalPath: filepath.Join(cfg, "config.yml"), Events: &sink{}, Dialogs: &fakeDialogs{}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	if err := z.RevealTemplates(); err != nil || len(shown) != 1 || shown[0] != filepath.Join(cfg, "templates") {
		t.Errorf("reveal: %v %v", shown, err)
	}
	if _, err := os.Stat(filepath.Join(cfg, "templates", "sites", "techdocs")); err != nil {
		t.Error("the folder has no templates in it")
	}
	z2, _, _, _ := newTestZen(t, "", "", "")
	if err := z2.RevealTemplates(); err == nil {
		t.Error("no settings folder: want an error")
	}
}

func TestInitSite(t *testing.T) {
	env, cfg := siteRig(t, "")
	run := func(i cli.Init) (string, string, error) {
		var out, errOut bytes.Buffer
		if i.Values == nil {
			i.Values = map[string]string{}
		}
		err := initSite(env.api(), i, &out, &errOut)
		return out.String(), errOut.String(), err
	}
	// A private working folder: the summary names the target relative to it,
	// and nothing is created in the source tree (where go list may be looking).
	cwd := t.TempDir()
	t.Chdir(cwd)

	out, _, err := run(cli.Init{List: true})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if err != nil || len(lines) != 3 || !strings.HasPrefix(lines[0], "techdocs  Technical documentation — ") || !strings.HasPrefix(lines[1], "learning  Learning path — ") {
		t.Fatalf("list: %q %v", out, err)
	}
	if out, _, err := run(cli.Init{ShowDir: true}); err != nil || out != filepath.Join(cfg, "templates")+"\n" {
		t.Errorf("dir: %q %v", out, err)
	}

	// Create: the summary names the folder as the user would type it.
	target, err := os.MkdirTemp(cwd, "zen-init-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(target) })
	rel := filepath.Base(target)
	out, _, err = run(cli.Init{Dir: target, Template: "techdocs", Values: map[string]string{"title": "T"}})
	if err != nil || out != "Created 11 files in "+rel+" (Technical documentation). Open it with: zenmd "+rel+"\n" {
		t.Fatalf("create: %q %v", out, err)
	}
	// A clash is an error that lists the files; nothing is overwritten.
	if _, _, err := run(cli.Init{Dir: target, Template: "techdocs"}); err == nil || !errors.Is(err, sitetemplates.ErrClash) || !strings.Contains(err.Error(), "README.md") {
		t.Errorf("clash: %v", err)
	}
	// A folder with a space is quoted in the hint, and one outside the current folder is shown in full.
	spaced := filepath.Join(t.TempDir(), "my docs")
	out, _, err = run(cli.Init{Dir: spaced, Template: "planner"})
	if err != nil || !strings.Contains(out, "Created 8 files in "+spaced+" (Planner). Open it with: zenmd '"+spaced+"'") {
		t.Errorf("spaced: %q %v", out, err)
	}
	// An unknown template is a usage error naming the choices; the templates folder is not a target.
	_, _, err = run(cli.Init{Dir: t.TempDir(), Template: "nosuch"})
	if err == nil || !strings.Contains(err.Error(), `"nosuch"`) || !strings.Contains(err.Error(), "techdocs, learning, planner") {
		t.Errorf("unknown: %v", err)
	}
	var o, e bytes.Buffer
	code := cli.Run([]string{"init", "site", "--nosuch", t.TempDir()}, cwd, &o, &e, func(cli.Launch) error { return nil }, nil,
		func(i cli.Init, so, se io.Writer) error { return initSite(env.api(), i, so, se) }, nil)
	if code != 2 || !strings.Contains(e.String(), "Usage:") {
		t.Errorf("unknown template through the CLI: %d %q", code, e.String())
	}
	if _, _, err := run(cli.Init{Dir: filepath.Join(cfg, "templates", "x"), Template: "techdocs"}); err == nil || !strings.Contains(err.Error(), "templates folder") {
		t.Errorf("into templates: %v", err)
	}
	// An invalid template cannot be used, and is marked in the list.
	if err := os.MkdirAll(filepath.Join(cfg, "templates", "sites", "bad"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := run(cli.Init{Dir: t.TempDir(), Template: "bad"}); err == nil || !strings.Contains(err.Error(), "not valid") {
		t.Errorf("invalid: %v", err)
	}
	if out, _, _ := run(cli.Init{List: true}); !strings.Contains(out, "bad       (invalid: template.yml is missing") {
		t.Errorf("list with invalid: %q", out)
	}

	// Updates: a newer built-in is listed, and written next to the copy on request.
	newer := fstest.MapFS{}
	for _, id := range []string{"techdocs", "learning", "planner"} {
		data, _ := fs.ReadFile(siteBuiltins, id+"/template.yml")
		if id == "techdocs" {
			data = []byte(strings.Replace(string(data), "version: 1.0.0", "version: 1.1.0", 1))
		}
		newer[id+"/template.yml"] = &fstest.MapFile{Data: data}
	}
	env.builtins = newer
	if out, _, _ := run(cli.Init{List: true}); !strings.Contains(out, "[update available: 1.1.0, see zenmd init --update techdocs]") {
		t.Errorf("update marker: %q", out)
	}
	out, _, err = run(cli.Init{Update: true, Template: "techdocs"})
	if err != nil || !strings.Contains(out, filepath.Join(cfg, "templates", "sites", "techdocs.new")) {
		t.Errorf("update: %q %v", out, err)
	}
	if _, _, err := run(cli.Init{Update: true, Template: "techdocs"}); err == nil {
		t.Error("second update accepted")
	}
}

// presetFiles is what each preset creates for the title "Acme Docs".
func presetFiles(id, date string) []string {
	files := map[string][]string{
		"techdocs": {".zenmd.yml", "CONTRIBUTING.md", "README.md", "explanation/how-it-works.md", "explanation/index.md", "how-to/index.md", "how-to/solve-a-common-problem.md",
			"reference/index.md", "reference/settings.md", "tutorials/getting-started.md", "tutorials/index.md"},
		"learning": {".zenmd.yml", "README.md", "authoring.md", "glossary.md", "paths/first-path/index.md"},
		"planner": {".zenmd.yml", "README.md", "decisions/0001-record-decisions.md", "decisions/template.md", "ideas.md", "meetings/template.md", "roadmap.md",
			"weeks/" + date + ".md"},
	}[id]
	sort.Strings(files)
	return files
}

func TestPresets(t *testing.T) {
	date := time.Now().Format("2006-01-02")
	layoutsWant := map[string]map[string]string{
		"techdocs": {"README.md": "landing", "tutorials/index.md": "docs", "how-to/solve-a-common-problem.md": "docs", "CONTRIBUTING.md": "docs", "explanation/how-it-works.md": "docs"},
		"learning": {"README.md": "landing", "paths/first-path/index.md": "course", "glossary.md": "article", "authoring.md": "article"},
		"planner":  {"README.md": "landing", "roadmap.md": "wide", "weeks/" + date + ".md": "article", "decisions/0001-record-decisions.md": "article"},
	}
	cards := map[string]int{"techdocs": 4, "learning": 3, "planner": 5}
	for _, id := range []string{"techdocs", "learning", "planner"} {
		t.Run(id, func(t *testing.T) {
			env, _ := siteRig(t, "")
			tmpl, _, ok := env.find(id)
			if !ok || tmpl.Err != "" {
				t.Fatalf("not listed: %+v", tmpl)
			}
			target := filepath.Join(t.TempDir(), "Acme Docs")
			res, err := sitetemplates.Create(tmpl, target, map[string]string{"title": "Acme Docs", "author": "Ana"})
			if err != nil {
				t.Fatal(err)
			}
			got := append([]string(nil), res.Files...)
			sort.Strings(got)
			if strings.Join(got, "\n") != strings.Join(presetFiles(id, date), "\n") {
				t.Fatalf("files:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(presetFiles(id, date), "\n"))
			}
			// A template brings no code: no plugins or layouts of its own, and no placeholder is left over.
			_ = filepath.Walk(target, func(p string, fi os.FileInfo, err error) error {
				if err != nil || fi.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(target, p)
				if strings.HasPrefix(rel, ".zen"+string(filepath.Separator)) {
					t.Errorf("preset writes into .zen/: %s", rel)
				}
				b, _ := os.ReadFile(p)
				if m := regexp.MustCompile(`\{\{[a-z]+\}\}`).Find(b); m != nil {
					t.Errorf("%s: placeholder %s left", rel, m)
				}
				return nil
			})
			if b, _ := os.ReadFile(filepath.Join(target, "README.md")); !strings.Contains(string(b), "# Acme Docs") {
				t.Errorf("README does not use the title: %s", b)
			}

			// It opens, and every page renders with no warning and no broken link.
			z, err := newZen(zenOptions{Registry: session.DefaultPlugins(), ConfigDir: t.TempDir(), Dir: target, Events: &sink{}, Dialogs: &fakeDialogs{}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(z.close)
			var files []string
			_ = filepath.WalkDir(target, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() && strings.HasPrefix(d.Name(), ".") && p != target {
					return filepath.SkipDir
				}
				if rel, err := filepath.Rel(target, p); err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") {
					files = append(files, filepath.ToSlash(rel))
				}
				return nil
			})
			if len(files) != len(presetFiles(id, date))-1 { // the .zenmd.yml is not a page
				t.Errorf("pages: %v", files)
			}
			for _, p := range files {
				pg, err := z.Page(p)
				if err != nil {
					t.Errorf("%s: %v", p, err)
					continue
				}
				if len(pg.Warnings) > 0 {
					t.Errorf("%s: warnings %v", p, pg.Warnings)
				}
				if n := strings.Count(pg.HTML, `data-zen-link="broken"`); n != 0 {
					t.Errorf("%s: %d broken links", p, n)
				}
				if want, ok := layoutsWant[id][p]; ok && pg.Layout.Name != want {
					t.Errorf("%s: layout %q, want %q", p, pg.Layout.Name, want)
				}
			}
			home, _ := z.Page("README.md")
			if n := strings.Count(home.HTML, `class="zl-card"`); n != cards[id] {
				t.Errorf("home has %d cards, want %d", n, cards[id])
			}
			if tr := z.s.Workspace().Tree; tr[len(tr)-1].Title == "Other pages" {
				other := tr[len(tr)-1]
				t.Errorf("pages nothing links to: %+v", other.Children)
			}
			switch id {
			case "learning":
				pg, _ := z.Page("paths/first-path/index.md")
				if strings.Count(pg.HTML, `data-zen-name="quiz"`) != 1 || strings.Count(pg.HTML, `data-zen-name="check"`) != 1 {
					t.Errorf("course page lacks its quiz or checklist: %s", pg.HTML)
				}
				if strings.Count(pg.HTML, "<h2") != 4 {
					t.Errorf("a course of four steps has %d headings", strings.Count(pg.HTML, "<h2"))
				}
			case "planner":
				pg, _ := z.Page("roadmap.md")
				if strings.Count(pg.HTML, `class="zen-diagram"`) != 1 || !strings.Contains(pg.HTML, "<table>") {
					t.Errorf("roadmap lacks its chart or table")
				}
			}

			// It builds with no warnings.
			cfg2 := t.TempDir()
			var out, errOut bytes.Buffer
			err = session.WriteSite(session.SiteEnv{GlobalPath: filepath.Join(cfg2, "config.yml"), Runtime: stubAssets()}, session.SiteOptions{Dir: target}, &out, &errOut)
			if err != nil || errOut.String() != "" || !strings.HasPrefix(out.String(), "Built ") || strings.Contains(out.String(), "warning") {
				t.Errorf("build: %v\nstdout %q\nstderr %q", err, out.String(), errOut.String())
			}
		})
	}
}

// TestPresetsLint runs Vale with the demo's vocabulary over each preset. It
// needs the vale helper that `task` fetches into build/darwin/helpers.
func TestPresetsLint(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	vale := filepath.Join(root, "build", "darwin", "helpers", runtime.GOARCH, "vale")
	if fi, err := os.Stat(vale); err != nil || fi.IsDir() {
		t.Skip("no vale helper in build/darwin/helpers")
	}
	demo := filepath.Join(root, "examples", "demo")
	linter := lint.New(vale)
	for _, id := range []string{"techdocs", "learning", "planner"} {
		env, _ := siteRig(t, "")
		tmpl, _, _ := env.find(id)
		target := filepath.Join(t.TempDir(), "docs")
		res, err := sitetemplates.Create(tmpl, target, map[string]string{"title": "Acme Docs", "author": "Ana"})
		if err != nil {
			t.Fatal(err)
		}
		// The demo's Vale settings and vocabulary.
		copyDemo := func(rel string) {
			b, err := os.ReadFile(filepath.Join(demo, rel))
			if err != nil {
				t.Fatal(err)
			}
			dst := filepath.Join(target, rel)
			_ = os.MkdirAll(filepath.Dir(dst), 0o755)
			_ = os.WriteFile(dst, b, 0o644)
		}
		copyDemo(".vale.ini")
		copyDemo(".vale/styles/config/vocabularies/Zen/accept.txt")
		for _, f := range res.Files {
			if !strings.HasSuffix(f, ".md") {
				continue
			}
			content, _ := os.ReadFile(filepath.Join(target, f))
			alerts, err := linter.Lint(context.Background(), target, f, content)
			if err != nil {
				t.Fatalf("%s/%s: %v", id, f, err)
			}
			for _, a := range alerts {
				if a.Severity == "error" || a.Severity == "warning" {
					t.Errorf("%s/%s:%d: %s (%s)", id, f, a.Line, a.Message, a.Check)
				}
			}
		}
	}
}

// sameDir compares two folders after resolving links (macOS temp folders are links).
func sameDir(a, b string) bool {
	ra, _ := filepath.EvalSymlinks(a)
	rb, _ := filepath.EvalSymlinks(b)
	return ra != "" && ra == rb
}
