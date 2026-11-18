package builtins

import (
	"io/fs"
	"reflect"
	"testing"
)

// TestDefaultSets pins the ids of the default plugins, layouts and site
// templates: there is one list, and session and the apps use it.
func TestDefaultSets(t *testing.T) {
	var plugins []string
	for _, m := range Plugins().All() {
		plugins = append(plugins, m.ID)
	}
	if want := []string{"codeblock", "diagram", "check", "quiz", "sandbox"}; !reflect.DeepEqual(plugins, want) {
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
