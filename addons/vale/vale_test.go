//go:build addon_vale

package vale

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zenmark-org/zenmd/engine/session"
	"github.com/zenmark-org/zenmd/kit/addons"
)

func TestRegisters(t *testing.T) {
	var found *addons.Addon
	for _, a := range addons.List() {
		if a.ID == "vale" {
			a := a
			found = &a
		}
	}
	if found == nil || !addons.Has("vale") {
		t.Fatal("the vale add-on did not register")
	}
	if found.Name != "Vale linting" || found.Summary != "Spelling and style checks for your pages, by the Vale prose linter." {
		t.Fatalf("registered %+v", *found)
	}
}

// Importing the add-on installs the linter finder: a vale in $ZEN_VALE is found
// and run through a session's Lint.
func TestInstallsLinter(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\ncat >/dev/null\necho '{}'\n"
	if err := os.WriteFile(filepath.Join(bin, "vale"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZEN_VALE", filepath.Join(bin, "vale"))
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Home\n\nText.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := session.New(session.Options{})
	defer s.Close()
	if err := s.Open(dir); err != nil {
		t.Fatal(err)
	}
	r, err := s.Lint("README.md", "")
	if err != nil || !r.Available {
		t.Fatalf("Lint = %+v, %v; want available", r, err)
	}

}
