package builtins

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"
)

// TestDefaultSets pins the ids of the default plugins, layouts and site
// templates: there is one list, and session and the apps use it.
func TestDefaultSets(t *testing.T) {
	var plugins []string
	for _, m := range Plugins().All() {
		plugins = append(plugins, m.ID)
	}
	if want := []string{"codeblock", "diagram", "check", "quiz", "sandbox", "rating"}; !reflect.DeepEqual(plugins, want) {
		t.Errorf("plugins = %v, want %v", plugins, want)
	}

	var layouts []string
	for _, l := range Layouts("test").All() {
		layouts = append(layouts, l.Manifest().ID)
	}
	if want := []string{"article", "docs", "post", "landing", "wide", "handout", "slides", "course"}; !reflect.DeepEqual(layouts, want) {
		t.Errorf("layouts = %v, want %v", layouts, want)
	}

	entries, err := fs.ReadDir(SiteTemplates(), ".")
	if err != nil {
		t.Fatal(err)
	}
	var templates []string
	for _, e := range entries {
		if e.IsDir() {
			templates = append(templates, e.Name())
		}
	}
	if want := []string{"learning", "planner", "techdocs"}; !reflect.DeepEqual(templates, want) {
		t.Errorf("site templates = %v, want %v", templates, want)
	}
}

// Each call returns a registry of its own, so one session's changes never
// reach another.
func TestConstructorsAreIndependent(t *testing.T) {
	a, b := Plugins(), Plugins()
	if a == b {
		t.Fatal("Plugins returned the same registry twice")
	}
	if Layouts("1") == Layouts("1") {
		t.Fatal("Layouts returned the same registry twice")
	}
}

// TestFor: the selection helpers keep the order asked for and reject unknown
// ids with the known ones listed (kit/assembly).
func TestFor(t *testing.T) {
	r, err := PluginsFor([]string{"sandbox", "codeblock"})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range r.All() {
		got = append(got, m.ID)
	}
	if !reflect.DeepEqual(got, []string{"sandbox", "codeblock"}) {
		t.Errorf("PluginsFor order = %v", got)
	}
	if r, err := PluginsFor(nil); err != nil || len(r.All()) != 0 {
		t.Errorf("PluginsFor(nil) = %v, %v; want an empty registry", r, err)
	}
	if _, err := PluginsFor([]string{"codeblock", "nope"}); err == nil || !strings.Contains(err.Error(), "nope") || !strings.Contains(err.Error(), "codeblock, diagram, check, quiz, sandbox, rating") {
		t.Errorf("unknown plugin: %v", err)
	}
	if _, err := PluginsFor([]string{"quiz", "quiz"}); err == nil {
		t.Error("a duplicate plugin should fail")
	}

	l, err := LayoutsFor("1.2.3", []string{"course", "docs"})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, x := range l.All() {
		ids = append(ids, x.Manifest().ID)
	}
	if !reflect.DeepEqual(ids, []string{"article", "course", "docs"}) { // article is always there, first when not listed
		t.Errorf("LayoutsFor = %v", ids)
	}
	if l, err = LayoutsFor("1", nil); err != nil || len(l.All()) != 1 {
		t.Errorf("LayoutsFor(nil) = %v, %v; want only article", l, err)
	}
	if l, _ = LayoutsFor("1", []string{"docs", "article"}); l.All()[0].Manifest().ID != "docs" {
		t.Error("a listed article keeps its place")
	}
	if _, err := LayoutsFor("1", []string{"nope"}); err == nil || !strings.Contains(err.Error(), "article, docs, post, landing, wide, handout, slides, course") {
		t.Errorf("unknown layout: %v", err)
	}

	fsys, err := SiteTemplatesFor([]string{"planner", "techdocs"})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if !reflect.DeepEqual(names, []string{"planner", "techdocs"}) {
		t.Errorf("SiteTemplatesFor lists %v", names)
	}
	if _, err := fs.Stat(fsys, "planner/template.yml"); err != nil {
		t.Error(err)
	}
	if _, err := fs.Stat(fsys, "learning/template.yml"); err == nil {
		t.Error("learning was not selected")
	}
	if _, err := SiteTemplatesFor([]string{"nope"}); err == nil || !strings.Contains(err.Error(), "techdocs, learning, planner") {
		t.Errorf("unknown template: %v", err)
	}
	if fsys, err = SiteTemplatesFor(nil); err != nil {
		t.Fatal(err)
	} else if entries, _ := fs.ReadDir(fsys, "."); len(entries) != 0 {
		t.Errorf("SiteTemplatesFor(nil) lists %d", len(entries))
	}
}

func TestDialects(t *testing.T) {
	all := Dialects()
	if l := all.List(); len(l) != 1 || l[0].ID() != "myst" || l[0].Name() == "" || l[0].Summary() == "" {
		t.Errorf("Dialects: %+v", l)
	}
	if r, err := DialectsFor(nil); err != nil || len(r.List()) != 0 {
		t.Errorf("none: %v %v", r, err)
	}
	if _, err := DialectsFor([]string{"mkdocs"}); err == nil || !strings.Contains(err.Error(), "myst") {
		t.Errorf("unknown id: %v", err)
	}
	if _, err := DialectsFor([]string{"myst", "myst"}); err == nil {
		t.Error("repeated id accepted")
	}
}
