package sitetemplates

import (
	"reflect"
	"strings"
	"testing"
)

func TestManifest(t *testing.T) {
	m, err := ParseManifest([]byte(`
id: techdocs
name: Technical documentation
description: Tutorials, how-to guides
version: 1.2.3
order: 10
ask:
  author: { label: Author, default: "" }
  title: { label: Site title, default: "{{folder}}" }
ignore: [".DS_Store", "*.bak"]
`))
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "techdocs" || m.Name != "Technical documentation" || m.Version != "1.2.3" || m.Order != 10 || m.Description != "Tutorials, how-to guides" {
		t.Errorf("%+v", m)
	}
	// title is always asked first, the rest in file order
	want := []Ask{{"title", "Site title", "{{folder}}"}, {"author", "Author", ""}}
	if !reflect.DeepEqual(m.Ask, want) {
		t.Errorf("ask = %+v", m.Ask)
	}

	// Defaults: only id and name are needed.
	m, err = ParseManifest([]byte("id: a-b\nname: Mine\n"))
	if err != nil || m.Order != 100 || m.Version != "0.0.0" || len(m.Ask) != 0 || m.Description != "" {
		t.Errorf("defaults: %+v %v", m, err)
	}
	// An empty ask: is fine; a label defaults to the key.
	if m, err = ParseManifest([]byte("id: a\nname: A\nask: {}\n")); err != nil || len(m.Ask) != 0 {
		t.Errorf("empty ask: %+v %v", m, err)
	}
	if m, err = ParseManifest([]byte("id: a\nname: A\nask:\n  tagline: {}\n")); err != nil || m.Ask[0].Label != "tagline" {
		t.Errorf("label default: %+v %v", m, err)
	}

	bad := map[string]string{
		"id: \"\"\nname: A\n":                           "id",
		"name: A\n":                                     "id",
		"id: Has Space\nname: A\n":                      "id",
		"id: a\n":                                       "name",
		"id: a\nname: A\nversion: one\n":                "version",
		"id: a\nname: A\norder: first\n":                "order",
		"id: a\nname: A\nflavour: x\n":                  "flavour",
		"id: a\nname: A\nask: [title]\n":                "ask",
		"id: a\nname: A\nask:\n  Bad Key: {}\n":         "ask",
		"id: a\nname: A\nask:\n  date: {}\n":            "date",
		"id: a\nname: A\nask:\n  folder: {}\n":          "folder",
		"id: a\nname: A\nask:\n  t: { colour: red }\n":  "colour",
		"id: a\nname: A\nignore: x\n":                   "ignore",
		"- a\n- b\n":                                    "mapping",
		"id: a\nname: [unclosed\n":                      "",
		"id: a\nname: A\n" + strings.Repeat("#", 70000): "too large",
	}
	for src, mention := range bad {
		if _, err := ParseManifest([]byte(src)); err == nil || !strings.Contains(err.Error(), mention) {
			t.Errorf("%.40q: err = %v, want mention of %q", src, err, mention)
		}
	}
}
