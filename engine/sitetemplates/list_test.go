package sitetemplates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestList(t *testing.T) {
	dir := t.TempDir()
	// Nothing there yet.
	if got, warns := List(dir); len(got) != 0 || len(warns) != 0 {
		t.Errorf("empty: %v %v", got, warns)
	}
	sites := filepath.Join(dir, "sites")
	put(t, sites, map[string]string{
		"b-folder/template.yml":  "id: beta\nname: Beta\norder: 20\n",
		"a-folder/template.yml":  "id: alpha\nname: Alpha\norder: 20\ndescription: First by name\n",
		"first/template.yml":     "id: first\nname: Zed\norder: 5\nask:\n  title: { label: Title, default: x }\n",
		"broken/template.yml":    "id: broken\nname: [\n",
		"notemplate/readme.md":   "no manifest here",
		"dup-b/template.yml":     "id: alpha\nname: Alpha again\norder: 1\n",
		".hidden/template.yml":   "id: hidden\nname: Hidden\n",
		"first.new/template.yml": "id: first\nname: Zed (update)\n",
		"loose-file.md":          "x",
	})
	got, warns := List(dir)
	var names []string
	for _, x := range got {
		names = append(names, x.ID)
	}
	// order, then name: first(5) alpha(20) beta(20), then the invalid ones last, by folder name.
	if strings.Join(names, ",") != "first,alpha,beta,broken,notemplate" {
		t.Fatalf("order = %v", names)
	}
	if got[0].Dir != filepath.Join(sites, "first") || len(got[0].Ask) != 1 || got[0].Err != "" {
		t.Errorf("first: %+v", got[0])
	}
	if got[1].Description != "First by name" {
		t.Errorf("alpha: %+v", got[1])
	}
	if got[3].Err == "" || got[3].Name != "broken" || got[4].Err == "" || !strings.Contains(got[4].Err, "template.yml") {
		t.Errorf("invalid ones: %+v %+v", got[3], got[4])
	}
	// The duplicate id: the first folder name wins ("a-folder" before "dup-b"), and a warning says so.
	if len(warns) != 1 || !strings.Contains(warns[0], "alpha") || !strings.Contains(warns[0], "dup-b") {
		t.Errorf("warnings = %v", warns)
	}
	// A template with an unreadable manifest is invalid, never used by Create.
	if _, err := Create(got[3], filepath.Join(t.TempDir(), "x"), nil); err == nil {
		t.Error("an invalid template was used")
	}
	// A folder that is a symlink to a template folder works.
	real := t.TempDir()
	put(t, real, map[string]string{"template.yml": "id: linked\nname: Linked\n", "a.md": "a"})
	if err := os.Symlink(real, filepath.Join(sites, "linked")); err != nil {
		t.Fatal(err)
	}
	again, _ := List(dir)
	found := false
	for _, x := range again {
		if x.ID == "linked" && x.Err == "" {
			found = true
		}
	}
	if !found {
		t.Error("symlinked folder not listed")
	}
}
