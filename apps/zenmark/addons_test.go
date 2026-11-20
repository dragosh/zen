package app

import (
	"github.com/zenmark-org/zenmd/internal/cli"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/zenmark-org/zenmd/kit/addons"
)

// wantAddons is what this build includes: nothing by default, the add-ons of
// the tags otherwise (addons_default_test.go, addons_vale_test.go).

// The app links an add-on only when it is built with its tag.
func TestNoAddonsByDefault(t *testing.T) {
	var got []string
	for _, a := range addons.List() {
		got = append(got, a.ID)
	}
	if !reflect.DeepEqual(got, wantAddons) {
		t.Fatalf("add-ons of this build = %v, want %v", got, wantAddons)
	}
}

// Startup tells the frontend which add-ons the build has, in a fixed order.
func TestStartupAddons(t *testing.T) {
	z, _, _, _ := newTestZen(t, tree(t, docs), "", "read")
	s := z.Startup()
	var got []string
	for _, a := range s.Addons {
		if a.Name == "" || a.Summary == "" {
			t.Errorf("add-on without a name or summary: %+v", a)
		}
		got = append(got, a.ID)
	}
	if !reflect.DeepEqual(got, wantAddons) {
		t.Fatalf("Startup.Addons = %v, want %v", got, wantAddons)
	}
	// Also when the folder given at launch did not open.
	z2, _, _, _ := newTestZen(t, "", "", "")
	if n := len(z2.Startup().Addons); n != len(wantAddons) {
		t.Fatalf("Startup without a folder lists %d add-ons, want %d", n, len(wantAddons))
	}
}

// With a vale on PATH and in $ZEN_VALE, Lint answers only when the build has the vale add-on.
func TestLintFollowsAddon(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\ncat >/dev/null\necho '{}'\n"
	if err := os.WriteFile(filepath.Join(bin, "vale"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZEN_VALE", filepath.Join(bin, "vale"))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	z, _, _, _ := newTestZen(t, tree(t, docs), "", "read")
	r, err := z.Lint("README.md", "")
	if err != nil {
		t.Fatal(err)
	}
	if want := valeIncluded; r.Available != want {
		t.Fatalf("Lint available = %v, want %v", r.Available, want)
	}
}

// Zenmark › About shows the version the build was stamped with ("dev" when it was not).
func TestBuildInfoVersion(t *testing.T) {
	if got := buildInfo().Version; got != cli.Version || got == "" {
		t.Fatalf("version %q, want %q", got, cli.Version)
	}
	old := cli.Version
	cli.Version = "1.2.3"
	t.Cleanup(func() { cli.Version = old })
	if got := buildInfo().Version; got != "1.2.3" {
		t.Fatalf("version %q", got)
	}
}
