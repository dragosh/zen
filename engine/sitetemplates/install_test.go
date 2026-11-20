package sitetemplates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstall(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "templates") // does not exist yet
	b := builtinsFS("1.0.0")
	if err := Install(dir, b); err != nil {
		t.Fatal(err)
	}
	if got := walk(t, filepath.Join(dir, "sites")); strings.Join(got, ",") != ".zen-templates.yml,planner/.zenmd.yml,planner/template.yml,techdocs/README.md,techdocs/template.yml" {
		t.Fatalf("installed: %v", got)
	}
	rec := read(t, dir, "sites/.zen-templates.yml")
	if !strings.Contains(rec, "techdocs: 1.0.0") || !strings.Contains(rec, "planner: 2.0.0") {
		t.Errorf("record:\n%s", rec)
	}
	list, warns := ListWith(dir, b)
	if len(warns) != 0 || len(list) != 2 || list[0].ID != "techdocs" || !list[0].Builtin || !list[1].Builtin || list[0].Update != "" {
		t.Errorf("list: %+v %v", list, warns)
	}

	// An installed template is never overwritten, not even by a newer version.
	if err := os.WriteFile(filepath.Join(dir, "sites", "techdocs", "README.md"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	b2 := builtinsFS("1.1.0")
	if err := Install(dir, b2); err != nil {
		t.Fatal(err)
	}
	if read(t, dir, "sites/techdocs/README.md") != "mine" {
		t.Error("overwritten")
	}
	// ... but it is listed as having an update.
	list, _ = ListWith(dir, b2)
	if list[0].Update != "1.1.0" || list[1].Update != "" {
		t.Errorf("update: %+v", list)
	}
	// Plain List (no bundle) knows the built-ins by the record, without update info.
	if plain, _ := List(dir); !plain[0].Builtin || plain[0].Update != "" {
		t.Errorf("plain list: %+v", plain[0])
	}

	// A preset the user deleted is not installed again.
	if err := os.RemoveAll(filepath.Join(dir, "sites", "planner")); err != nil {
		t.Fatal(err)
	}
	if err := Install(dir, b2); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sites", "planner")); err == nil {
		t.Error("deleted preset came back")
	}
	if list, _ = ListWith(dir, b2); len(list) != 1 {
		t.Errorf("deleted preset listed: %+v", list)
	}

	// A folder the user made with the same name is theirs: not replaced, not recorded.
	dir2 := t.TempDir()
	put(t, dir2, map[string]string{"sites/techdocs/template.yml": "id: techdocs\nname: Mine\n", "sites/techdocs/x.md": "x"})
	if err := Install(dir2, b); err != nil {
		t.Fatal(err)
	}
	if read(t, dir2, "sites/techdocs/x.md") != "x" || strings.Contains(read(t, dir2, "sites/.zen-templates.yml"), "techdocs") {
		t.Error("user's own template touched or recorded")
	}
	if l, _ := ListWith(dir2, b); l[1].Builtin || l[1].Update != "" {
		t.Errorf("user's template flagged: %+v", l)
	}

	// Update writes the newer built-in next to the copy, once.
	out, err := Update(dir, b2, "techdocs")
	if err != nil || out != filepath.Join(dir, "sites", "techdocs.new") {
		t.Fatalf("update: %q %v", out, err)
	}
	if !strings.Contains(read(t, out, "template.yml"), "1.1.0") || read(t, dir, "sites/techdocs/README.md") != "mine" {
		t.Error("update content or user's copy changed")
	}
	if l, _ := ListWith(dir, b2); len(l) != 1 || l[0].Update != "" {
		t.Errorf("after update: %+v", l) // the .new folder is not a template, and the record moved on
	}
	if _, err := Update(dir, b2, "techdocs"); err == nil {
		t.Error("second update must say it is up to date or that .new exists")
	}
	for _, id := range []string{"nosuch", "planner", ""} {
		if _, err := Update(dir, b2, id); err == nil {
			t.Errorf("update %q accepted", id)
		}
	}
}

func TestInstallReadOnly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes everywhere")
	}
	base := t.TempDir()
	if err := os.Chmod(base, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(base, 0o700)
	dir := filepath.Join(base, "templates")
	b := builtinsFS("1.0.0")
	if err := Install(dir, b); err == nil {
		t.Fatal("want an error for a folder that cannot be written")
	}
	// The bundle is listed straight from the app, and can create sites.
	list, warns := Builtins(b)
	if len(list) != 2 || len(warns) != 0 || list[0].ID != "techdocs" || !list[0].Builtin {
		t.Fatalf("builtins: %+v %v", list, warns)
	}
	target := filepath.Join(t.TempDir(), "My Docs")
	res, err := Create(list[0], target, map[string]string{"title": "T"})
	if err != nil || len(res.Files) != 1 || read(t, target, "README.md") != "# T\n" {
		t.Errorf("create from bundle: %+v %v", res, err)
	}
	// Dotfiles of a bundled template are copied as well.
	t2 := filepath.Join(t.TempDir(), "p")
	if _, err := Create(list[1], t2, nil); err != nil || read(t, t2, ".zenmd.yml") != "layout: wide\n" {
		t.Errorf("dotfile: %v", err)
	}
}
