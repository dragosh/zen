package assembly

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

var testFeatures = []string{"reader", "nav", "find", "editor", "inspector", "print", "export", "newsite", "developer"}

var testFeatureNeeds = map[string][]string{
	"nav": {"reader"}, "find": {"reader"}, "editor": {"reader"}, "inspector": {"reader"}, "print": {"reader"}, "export": {"reader"},
}

func testCatalog() Catalog {
	return Catalog{
		Features:     testFeatures,
		FeatureNeeds: testFeatureNeeds,
		AddonNeeds:   map[string][]string{"vale": {"reader"}},
		Addons:       []string{"vale"},
		Plugins:      []string{"codeblock", "diagram", "check", "quiz", "sandbox"},
		Layouts:      []string{"article", "docs", "landing", "course"},
		Templates:    []string{"techdocs", "learning"},
		Themes:       []string{"default"},
		Dialects:     []string{"myst"},
		LayoutNeeds: map[string]Needs{
			"course": {Plugins: []string{"quiz"}},
		},
		TemplateNeeds: map[string]Needs{
			"techdocs": {Layouts: []string{"docs", "landing"}, Plugins: []string{"codeblock"}},
			"learning": {Layouts: []string{"course", "landing"}, Plugins: []string{"quiz", "check"}},
		},
	}
}

func write(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "app-assembly.yml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func load(t *testing.T, content string) (Assembly, error) {
	t.Helper()
	return Load(write(t, content), testCatalog())
}

func TestLoad(t *testing.T) {
	a, err := load(t, `app: demo
addons: [vale]
plugins: [codeblock, quiz, check]
layouts: [article, docs, landing, course]
templates: [learning]
theme: default
`)
	if err != nil {
		t.Fatal(err)
	}
	want := Assembly{App: "demo", Features: testFeatures, Addons: []string{"vale"}, Plugins: []string{"codeblock", "quiz", "check"},
		Layouts: []string{"article", "docs", "landing", "course"}, Templates: []string{"learning"}, Theme: "default"}
	if !reflect.DeepEqual(a, want) {
		t.Errorf("got %+v\nwant %+v", a, want)
	}

	// Missing lists mean none; Windows line endings and a BOM are fine.
	a, err = load(t, "\xEF\xBB\xBFapp: tiny\r\nplugins: []\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if a.App != "tiny" || len(a.Addons)+len(a.Plugins)+len(a.Layouts)+len(a.Templates) != 0 || a.Theme != "default" {
		t.Errorf("tiny: %+v", a)
	}

	// all expands to the catalog.
	a, err = load(t, "app: x\naddons: all\nplugins: all\nlayouts: all\ntemplates: all\n")
	if err != nil {
		t.Fatal(err)
	}
	c := testCatalog()
	if !reflect.DeepEqual(a.Plugins, c.Plugins) || !reflect.DeepEqual(a.Layouts, c.Layouts) ||
		!reflect.DeepEqual(a.Templates, c.Templates) || !reflect.DeepEqual(a.Addons, c.Addons) {
		t.Errorf("all: %+v", a)
	}
	a, err = load(t, "app: x\nplugins: [all]\n")
	if err != nil || !reflect.DeepEqual(a.Plugins, c.Plugins) {
		t.Errorf("[all]: %+v, %v", a, err)
	}

	if _, err := Load(filepath.Join(t.TempDir(), "nope.yml"), c); err == nil {
		t.Error("a missing file should fail")
	}
}

func TestUnknown(t *testing.T) {
	cases := []struct{ yml, want string }{
		{"app: x\ncolour: red\n", "unknown key"},
		{"app: x\nplugins: [nope]\n", "known plugins: codeblock, diagram, check, quiz, sandbox"},
		{"app: x\nlayouts: [nope]\n", "known layouts: article, docs, landing, course"},
		{"app: x\ntemplates: [nope]\n", "known templates: techdocs, learning"},
		{"app: x\naddons: [nope]\n", "known add-ons: vale"},
		{"app: x\nplugins: [\"../x\"]\n", "known plugins"},
		{"app: x\nplugins: [\"Quiz\"]\n", "known plugins"},
		{"app: x\nplugins: quiz\n", "list"},
		{"plugins: [quiz]\n", "app"},
		{"app: \"Bad Name\"\n", "app"},
		{"app: x\nplugins: [quiz, quiz]\n", "twice"},
		{"app: x\ntemplates: [learning, learning]\n", "twice"},
		{"app: x\nplugins: [all, quiz]\n", "all"},
		{"app: x\nplugins: [quiz, all]\n", "all"},
		{"- a\n- b\n", "mapping"},
		{"app: [", "assembly"},
	}
	for _, c := range cases {
		_, err := load(t, c.yml)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: error %v, want mention of %q", c.yml, err, c.want)
		}
	}
}

func TestOrder(t *testing.T) {
	a, err := load(t, "app: x\nplugins: [sandbox, codeblock, quiz, check]\nlayouts: [course, article, landing, docs]\ntemplates: [learning, techdocs]\n")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.Plugins, []string{"sandbox", "codeblock", "quiz", "check"}) ||
		!reflect.DeepEqual(a.Layouts, []string{"course", "article", "landing", "docs"}) ||
		!reflect.DeepEqual(a.Templates, []string{"learning", "techdocs"}) {
		t.Errorf("order not kept: %+v", a)
	}
}

func TestDependencies(t *testing.T) {
	cases := []struct{ name, yml, want string }{
		{"template needs layout", "app: x\nplugins: [quiz, check]\nlayouts: [landing]\ntemplates: [learning]\n", "template learning needs layout course"},
		{"template needs plugin", "app: x\nplugins: [codeblock]\nlayouts: [docs, landing, course]\ntemplates: [learning]\n", "template learning needs plugin quiz"},
		{"layout needs plugin", "app: x\nlayouts: [course]\n", "layout course needs plugin quiz"},
		{"templates without layouts", "app: x\nplugins: [codeblock]\ntemplates: [techdocs]\n", "layouts"},
	}
	for _, c := range cases {
		_, err := load(t, c.yml)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, want %q", c.name, err, c.want)
		}
	}
	// Both sides are named in one error, one sentence each.
	_, err := load(t, "app: x\nlayouts: [course]\ntemplates: [learning]\n")
	if err == nil || !strings.Contains(err.Error(), "layout course needs plugin quiz") || !strings.Contains(err.Error(), "template learning needs plugin check") {
		t.Errorf("all problems should be reported: %v", err)
	}

	// article is implied: a template that needs it is satisfied without it,
	// and listing it is allowed.
	cat := testCatalog()
	cat.TemplateNeeds["techdocs"] = Needs{Layouts: []string{"article", "docs", "landing"}, Plugins: []string{"codeblock"}}
	a := Assembly{App: "x", Features: []string{"reader"}, Plugins: []string{"codeblock"}, Layouts: []string{"docs", "landing"}, Templates: []string{"techdocs"}, Theme: "default"}
	if err := Check(a, cat); err != nil {
		t.Errorf("article is implied: %v", err)
	}
	if _, err := load(t, "app: x\nlayouts: [docs]\n"); err != nil {
		t.Errorf("leaving article out is not an error: %v", err)
	}
	if _, err := load(t, "app: x\nlayouts: [article]\n"); err != nil {
		t.Errorf("listing article is allowed: %v", err)
	}
	// A satisfied assembly passes.
	if _, err := load(t, "app: x\nplugins: [codeblock, quiz, check]\nlayouts: [docs, landing, course]\ntemplates: [learning, techdocs]\n"); err != nil {
		t.Error(err)
	}
	// Check also guards an Assembly built in code.
	if err := Check(Assembly{App: "x", Plugins: []string{"nope"}, Theme: "default"}, cat); err == nil || !strings.Contains(err.Error(), "known plugins") {
		t.Errorf("Check ids: %v", err)
	}
}

func TestTheme(t *testing.T) {
	if a, err := load(t, "app: x\ntheme: default\n"); err != nil || a.Theme != "default" {
		t.Errorf("default theme: %+v, %v", a, err)
	}
	if a, err := load(t, "app: x\n"); err != nil || a.Theme != "default" {
		t.Errorf("no theme means default: %+v, %v", a, err)
	}
	_, err := load(t, "app: x\ntheme: neon\n")
	if err == nil || !strings.Contains(err.Error(), "known themes: default") {
		t.Errorf("unknown theme: %v", err)
	}
}

func TestBuiltinCatalog(t *testing.T) {
	c, err := BuiltinCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.Plugins, []string{"codeblock", "diagram", "check", "quiz", "sandbox", "rating"}) {
		t.Errorf("plugins %v", c.Plugins)
	}
	if !reflect.DeepEqual(c.Layouts, []string{"article", "docs", "post", "landing", "wide", "handout", "slides", "course"}) {
		t.Errorf("layouts %v", c.Layouts)
	}
	if !reflect.DeepEqual(c.Templates, []string{"techdocs", "learning", "planner"}) {
		t.Errorf("templates %v", c.Templates)
	}
	if !reflect.DeepEqual(c.Dialects, []string{"myst"}) {
		t.Errorf("dialects %v", c.Dialects)
	}
	if !reflect.DeepEqual(c.Themes, []string{"default"}) || !reflect.DeepEqual(c.Addons, []string{"vale"}) {
		t.Errorf("themes %v addons %v", c.Themes, c.Addons)
	}
	if got := c.LayoutNeeds["course"].Plugins; !reflect.DeepEqual(got, []string{"check"}) {
		t.Errorf("course needs %v", got)
	}
	if n := c.TemplateNeeds["learning"]; !reflect.DeepEqual(n.Layouts, []string{"landing", "course"}) || !reflect.DeepEqual(n.Plugins, []string{"quiz", "check"}) {
		// order is not significant; compare as sets below
		if !sameSet(n.Layouts, []string{"landing", "course"}) || !sameSet(n.Plugins, []string{"quiz", "check"}) {
			t.Errorf("learning needs %+v", n)
		}
	}
	// Zenmark's own file passes against it: everything it lists exists.
	if _, err := Load(filepath.Join("..", "..", "apps", "zenmark", "app-assembly.yml"), c); err != nil {
		t.Errorf("Zenmark's assembly: %v", err)
	}
}

// The add-on ids of the catalog are the folders under addons/ and the
// Taskfile's ADDONS_KNOWN, so a new add-on is a one-place change plus its
// folder.
func TestKnownAddonsMatch(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("..", "..", "addons"))
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	c, _ := BuiltinCatalog()
	if !sameSet(c.Addons, dirs) {
		t.Errorf("catalog add-ons %v, folders under addons/ %v", c.Addons, dirs)
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "ADDONS_KNOWN:"); ok {
			if got := strings.Split(strings.TrimSpace(v), ","); !sameSet(got, c.Addons) {
				t.Errorf("Taskfile ADDONS_KNOWN %v, catalog %v", got, c.Addons)
			}
			return
		}
	}
	t.Error("ADDONS_KNOWN not found in the Taskfile")
}

func sameSet(a, b []string) bool {
	m := map[string]int{}
	for _, s := range a {
		m[s]++
	}
	for _, s := range b {
		m[s]--
	}
	for _, n := range m {
		if n != 0 {
			return false
		}
	}
	return true
}

func TestFeatures(t *testing.T) {
	// A missing key means all, unlike the other lists; newsite joins only when
	// there is a template to start from.
	noNewsite := slices.DeleteFunc(slices.Clone(testFeatures), func(f string) bool { return f == "newsite" })
	a, err := load(t, "app: x\n")
	if err != nil || !reflect.DeepEqual(a.Features, noNewsite) {
		t.Errorf("missing: %+v, %v", a.Features, err)
	}
	a, err = load(t, "app: x\nlayouts: [docs, landing]\ntemplates: [techdocs]\nplugins: [codeblock]\n")
	if err != nil || !reflect.DeepEqual(a.Features, testFeatures) {
		t.Errorf("missing, with templates: %+v, %v", a.Features, err)
	}
	// [] and null mean none: the bare shell.
	for _, y := range []string{"app: x\nfeatures: []\n", "app: x\nfeatures:\n"} {
		a, err = load(t, y)
		if err != nil || len(a.Features) != 0 {
			t.Errorf("%q: %+v, %v", y, a.Features, err)
		}
	}
	for _, y := range []string{"app: x\nfeatures: all\n", "app: x\nfeatures: [all]\n"} {
		a, err = load(t, y)
		if err != nil || !reflect.DeepEqual(a.Features, noNewsite) {
			t.Errorf("%q: %+v, %v", y, a.Features, err)
		}
	}
	if a, err = load(t, "app: x\nfeatures: [reader, editor]\n"); err != nil || !reflect.DeepEqual(a.Features, []string{"reader", "editor"}) {
		t.Errorf("list: %+v, %v", a.Features, err)
	}
	for _, c := range []struct{ yml, want string }{
		{"app: x\nfeatures: [nope]\n", "known features: reader, nav, find, editor, inspector, print, export, newsite, developer"},
		{"app: x\nfeatures: [reader, reader]\n", "twice"},
		{"app: x\nfeatures: [all, editor]\n", "all"},
		{"app: x\nfeatures: editor\n", "list"},
		{"app: x\nfeatures: [\"../x\"]\n", "known features"},
	} {
		if _, err := load(t, c.yml); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: %v, want %q", c.yml, err, c.want)
		}
	}
	// The built-in catalog lists the features in order, with their needs.
	c, _ := BuiltinCatalog()
	if !reflect.DeepEqual(c.Features, testFeatures) {
		t.Errorf("builtin features %v", c.Features)
	}
	if !reflect.DeepEqual(c.FeatureNeeds, testFeatureNeeds) {
		t.Errorf("builtin feature needs %v", c.FeatureNeeds)
	}
}

func TestFeatureNeeds(t *testing.T) {
	// Each feature that needs another fails alone, naming both.
	for _, f := range []string{"nav", "find", "editor", "inspector", "print", "export"} {
		err := Check(Assembly{App: "x", Features: []string{f}, Theme: "default"}, testCatalog())
		if want := "feature " + f + " needs feature reader"; err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s alone: %v, want %q", f, err, want)
		}
		if err := Check(Assembly{App: "x", Features: []string{"reader", f}, Theme: "default"}, testCatalog()); err != nil {
			t.Errorf("reader and %s: %v", f, err)
		}
	}
	// reader and developer need nothing.
	for _, f := range []string{"reader", "developer"} {
		if err := Check(Assembly{App: "x", Features: []string{f}, Theme: "default"}, testCatalog()); err != nil {
			t.Errorf("%s alone: %v", f, err)
		}
	}
	// newsite needs a template.
	err := Check(Assembly{App: "x", Features: []string{"newsite"}, Theme: "default"}, testCatalog())
	if err == nil || !strings.Contains(err.Error(), "feature newsite needs at least one template") {
		t.Errorf("newsite without templates: %v", err)
	}
	ok := Assembly{App: "x", Features: []string{"newsite"}, Layouts: []string{"docs", "landing"}, Plugins: []string{"codeblock"}, Templates: []string{"techdocs"}, Theme: "default"}
	// (templates also need reader for their plugins and layouts)
	if err := Check(ok, testCatalog()); err == nil || !strings.Contains(err.Error(), "need feature reader") {
		t.Errorf("newsite with templates but no reader: %v", err)
	}
	ok.Features = []string{"reader", "newsite"}
	if err := Check(ok, testCatalog()); err != nil {
		t.Errorf("newsite with a template: %v", err)
	}
	// Plugins, layouts and dialects are only useful with the reader.
	for _, c := range []struct {
		a    Assembly
		want string
	}{
		{Assembly{Plugins: []string{"codeblock"}}, "plugins need feature reader"},
		{Assembly{Layouts: []string{"article"}}, "layouts need feature reader"},
		{Assembly{Dialects: []string{"myst"}}, "dialects need feature reader"},
	} {
		c.a.App, c.a.Features, c.a.Theme = "x", []string{"developer"}, "default"
		if err := Check(c.a, testCatalog()); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: %v, want %q", c.a, err, c.want)
		}
		c.a.Features = []string{"reader"}
		if err := Check(c.a, testCatalog()); err != nil {
			t.Errorf("%+v with reader: %v", c.a, err)
		}
	}
	// Through a file, and all problems are reported together.
	_, err = load(t, "app: x\nfeatures: [find, print]\nplugins: [codeblock]\n")
	for _, want := range []string{"feature find needs feature reader", "feature print needs feature reader", "plugins need feature reader"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("file: %v, want %q", err, want)
		}
	}
}

func TestAddonNeeds(t *testing.T) {
	err := Check(Assembly{App: "x", Features: []string{"developer"}, Addons: []string{"vale"}, Theme: "default"}, testCatalog())
	if err == nil || !strings.Contains(err.Error(), "add-on vale needs feature reader") {
		t.Errorf("vale without reader: %v", err)
	}
	if err := Check(Assembly{App: "x", Features: []string{"reader"}, Addons: []string{"vale"}, Theme: "default"}, testCatalog()); err != nil {
		t.Errorf("vale with reader (no inspector needed): %v", err)
	}
	c, _ := BuiltinCatalog()
	if !reflect.DeepEqual(c.AddonNeeds, map[string][]string{"vale": {"reader"}}) {
		t.Errorf("builtin add-on needs %v", c.AddonNeeds)
	}
}

func TestDialects(t *testing.T) {
	// A missing key, null and [] mean none, as for plugins.
	for _, y := range []string{"app: x\n", "app: x\ndialects: []\n", "app: x\ndialects:\n"} {
		a, err := load(t, y)
		if err != nil || len(a.Dialects) != 0 {
			t.Errorf("%q: %+v, %v", y, a.Dialects, err)
		}
	}
	for _, y := range []string{"app: x\ndialects: all\n", "app: x\ndialects: [all]\n", "app: x\ndialects: [myst]\n"} {
		a, err := load(t, y)
		if err != nil || !reflect.DeepEqual(a.Dialects, []string{"myst"}) {
			t.Errorf("%q: %+v, %v", y, a.Dialects, err)
		}
	}
	for _, c := range []struct{ yml, want string }{
		{"app: x\ndialects: [mkdocs]\n", "unknown dialect \"mkdocs\"; known dialects: myst"},
		{"app: x\ndialects: [myst, myst]\n", "twice"},
		{"app: x\ndialects: [all, myst]\n", "all"},
		{"app: x\ndialects: myst\n", "list"},
		{"app: x\ndialects: [\"../x\"]\n", "known dialects"},
		{"app: x\nfoo: 1\n", "dialects"}, // the known keys name it
	} {
		if _, err := load(t, c.yml); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: %v, want %q", c.yml, err, c.want)
		}
	}
}
