package engine_test

import (
	"os/exec"
	"sort"
	"strings"
	"testing"
)

const module = "github.com/dragosh/zenmd/"

// goList runs go list and returns its output lines.
func goList(t *testing.T, args ...string) []string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"list"}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		var msg string
		if ee, ok := err.(*exec.ExitError); ok {
			msg = string(ee.Stderr)
		}
		t.Fatalf("go list %v: %v\n%s", args, err, msg)
	}
	return strings.Fields(string(out))
}

// TestPackageLayout lists the packages of the module that hold code: the
// public ones under engine/ and builtins/, and the internal ones.
func TestPackageLayout(t *testing.T) {
	public := []string{
		"engine",
		"engine/config", "engine/extplugins", "engine/index", "engine/layouts", "engine/lint",
		"engine/plugins", "engine/render", "engine/scriptplugins", "engine/session",
		"engine/session/sessiontest", "engine/site", "engine/sitetemplates", "engine/watch", "engine/workspace",
		"builtins", "builtins/layouts", "builtins/sitetemplates",
		"builtins/plugins/check", "builtins/plugins/codeblock", "builtins/plugins/diagram",
		"builtins/plugins/quiz", "builtins/plugins/sandbox",
	}
	internal := []string{"internal/app", "internal/atomicfile", "internal/cli"}
	got := func(patterns ...string) []string {
		var out []string
		for _, p := range goList(t, patterns...) {
			out = append(out, strings.TrimPrefix(p, module))
		}
		sort.Strings(out)
		return out
	}
	for _, c := range []struct {
		name string
		want []string
		got  []string
	}{
		{"public", public, got("./../engine/...", "./../builtins/...")},
		{"internal", internal, got("./../internal/...")},
	} {
		want := append([]string(nil), c.want...)
		sort.Strings(want)
		if strings.Join(want, " ") != strings.Join(c.got, " ") {
			t.Errorf("%s packages:\n got  %v\n want %v", c.name, c.got, want)
		}
	}
	// The old homes are gone.
	for _, old := range []string{"internal/core", "internal/plugins", "internal/layouts", "internal/sitetemplates"} {
		for _, p := range goList(t, "-e", "./../internal/...") {
			if strings.HasPrefix(strings.TrimPrefix(p, module), old) {
				t.Errorf("%s still exists", p)
			}
		}
	}
}

// TestBoundaries enforces the import rules: nothing under engine/ or
// builtins/ depends (even indirectly) on internal/app, internal/cli or
// Wails; and engine/ imports builtins/ only where it must: engine/session,
// the front door, uses the default set when it is not given one (tests and
// examples may import builtins freely).
func TestBoundaries(t *testing.T) {
	forbidden := []string{module + "internal/app", module + "internal/cli", "github.com/wailsapp/"}
	pkgs := goList(t, "./../engine/...", "./../builtins/...")
	for _, pkg := range pkgs {
		for _, dep := range goList(t, "-deps", pkg) {
			for _, f := range forbidden {
				if dep == f || strings.HasPrefix(dep, f+"/") || (strings.HasSuffix(f, "/") && strings.HasPrefix(dep, f)) {
					t.Errorf("%s depends on %s", pkg, dep)
				}
			}
		}
	}
	// go list reports the non-test imports only.
	for _, pkg := range pkgs {
		if !strings.HasPrefix(pkg, module+"engine") || pkg == module+"engine/session" {
			continue
		}
		for _, imp := range goList(t, "-f", `{{join .Imports " "}}`, pkg) {
			if strings.HasPrefix(imp, module+"builtins") {
				t.Errorf("%s imports %s: engine packages import builtins only in tests and examples", pkg, imp)
			}
		}
	}
}

// TestRuntimeStdlibOnly: the Go package that embeds the browser runtime
// (runtime/, imported as .../zenmd/runtime) imports only the standard
// library, so any engine consumer can take it.
func TestRuntimeStdlibOnly(t *testing.T) {
	pkg := module + "runtime"
	for _, imp := range goList(t, "-f", `{{join .Imports " "}}`, pkg) {
		if strings.Contains(strings.Split(imp, "/")[0], ".") {
			t.Errorf("%s imports %s: only the standard library is allowed", pkg, imp)
		}
	}
}
