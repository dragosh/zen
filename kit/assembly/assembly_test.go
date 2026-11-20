package assembly

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func testCatalog() Catalog {
	return Catalog{
		Features:  []string{"editor"},
		Addons:    []string{"vale"},
		Plugins:   []string{"codeblock", "diagram", "check", "quiz", "sandbox"},
		Layouts:   []string{"article", "docs", "landing", "course"},
		Templates: []string{"techdocs", "learning"},
		Themes:    []string{"default"},
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
	want := Assembly{App: "demo", Features: []string{"editor"}, Addons: []string{"vale"}, Plugins: []string{"codeblock", "quiz", "check"},
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
	a := Assembly{App: "x", Plugins: []string{"codeblock"}, Layouts: []string{"docs", "landing"}, Templates: []string{"techdocs"}, Theme: "default"}
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
	if !reflect.DeepEqual(c.Plugins, []string{"codeblock", "diagram", "check", "quiz", "sandbox"}) {
		t.Errorf("plugins %v", c.Plugins)
	}
	if !reflect.DeepEqual(c.Layouts, []string{"article", "docs", "post", "landing", "wide", "handout", "slides", "course"}) {
		t.Errorf("layouts %v", c.Layouts)
	}
	if !reflect.DeepEqual(c.Templates, []string{"techdocs", "learning", "planner"}) {
		t.Errorf("templates %v", c.Templates)
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
	// A missing key means all, unlike the other lists.
	a, err := load(t, "app: x\n")
	if err != nil || !reflect.DeepEqual(a.Features, []string{"editor"}) {
		t.Errorf("missing: %+v, %v", a.Features, err)
	}
	// [] and null mean none.
	for _, y := range []string{"app: x\nfeatures: []\n", "app: x\nfeatures:\n"} {
		a, err = load(t, y)
		if err != nil || len(a.Features) != 0 {
			t.Errorf("%q: %+v, %v", y, a.Features, err)
		}
	}
	for _, y := range []string{"app: x\nfeatures: all\n", "app: x\nfeatures: [all]\n", "app: x\nfeatures: [editor]\n"} {
		a, err = load(t, y)
		if err != nil || !reflect.DeepEqual(a.Features, []string{"editor"}) {
			t.Errorf("%q: %+v, %v", y, a.Features, err)
		}
	}
	for _, c := range []struct{ yml, want string }{
		{"app: x\nfeatures: [nope]\n", "known features: editor"},
		{"app: x\nfeatures: [editor, editor]\n", "twice"},
		{"app: x\nfeatures: [all, editor]\n", "all"},
		{"app: x\nfeatures: editor\n", "list"},
		{"app: x\nfeatures: [\"../x\"]\n", "known features"},
	} {
		if _, err := load(t, c.yml); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: %v, want %q", c.yml, err, c.want)
		}
	}
	// FeatureNeeds are read by Check.
	cat := testCatalog()
	cat.Features = []string{"editor", "newsite"}
	cat.FeatureNeeds = map[string][]string{"newsite": {"editor"}}
	if err := Check(Assembly{App: "x", Features: []string{"newsite"}, Theme: "default"}, cat); err == nil || !strings.Contains(err.Error(), "feature newsite needs feature editor") {
		t.Errorf("needs: %v", err)
	}
	if err := Check(Assembly{App: "x", Features: []string{"editor", "newsite"}, Theme: "default"}, cat); err != nil {
		t.Error(err)
	}
	// The built-in catalog knows the editor, and Zenmark's file passes.
	c, _ := BuiltinCatalog()
	if !reflect.DeepEqual(c.Features, []string{"editor"}) {
		t.Errorf("builtin features %v", c.Features)
	}
}
