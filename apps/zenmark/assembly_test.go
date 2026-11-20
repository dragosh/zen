package app

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/zenmark-org/zenmd/builtins"
	"github.com/zenmark-org/zenmd/internal/cli"
	"github.com/zenmark-org/zenmd/kit/assembly"
)

// TestAssemblyGenerated: assembly_gen.go is what the generator writes for
// app-assembly.yml today; after editing the YAML run `task build` (or
// go run ./kit/assembly/cmd/gen apps/zenmark).
func TestAssemblyGenerated(t *testing.T) {
	cat, err := assembly.BuiltinCatalog()
	if err != nil {
		t.Fatal(err)
	}
	a, err := assembly.Load("app-assembly.yml", cat)
	if err != nil {
		t.Fatal(err)
	}
	want, err := assembly.Generate(a, "app")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("assembly_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("assembly_gen.go is out of date with app-assembly.yml; run: go run ./kit/assembly/cmd/gen apps/zenmark")
	}
	if _, err := os.Stat(filepath.Join("app-assembly.yml")); err != nil {
		t.Error(err)
	}
}

func builtinIDs(t *testing.T, z *Zen) (plugins, layouts, templates []string) {
	t.Helper()
	for _, p := range z.s.Plugins() {
		if p.Source == "builtin" {
			plugins = append(plugins, p.ID)
		}
	}
	for _, l := range z.s.Layouts() {
		if l.Source == "builtin" {
			layouts = append(layouts, l.ID)
		}
	}
	tpls, _ := z.SiteTemplates()
	for _, tpl := range tpls {
		templates = append(templates, tpl.ID)
	}
	return
}

// The session gets exactly the assembled sets: what the options say, and
// without options what the generated file says.
func TestSessionUsesAssembly(t *testing.T) {
	reg, err := builtins.PluginsFor([]string{"codeblock"})
	if err != nil {
		t.Fatal(err)
	}
	lay, err := builtins.LayoutsFor("test", []string{"landing"})
	if err != nil {
		t.Fatal(err)
	}
	only, err := builtins.SiteTemplatesFor([]string{"planner"})
	if err != nil {
		t.Fatal(err)
	}
	old := siteBuiltins
	siteBuiltins = only
	t.Cleanup(func() { siteBuiltins = old })
	z, err := newZen(zenOptions{Registry: reg, Layouts: lay, Events: &sink{}, GlobalPath: filepath.Join(t.TempDir(), "config.yml")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	p, l, tp := builtinIDs(t, z)
	if !reflect.DeepEqual(p, []string{"codeblock"}) || !reflect.DeepEqual(l, []string{"article", "landing"}) || !reflect.DeepEqual(tp, []string{"planner"}) {
		t.Errorf("plugins %v, layouts %v, templates %v", p, l, tp)
	}
	siteBuiltins = old

	// By default: the generated assembly.
	z2, err := newZen(zenOptions{Events: &sink{}, GlobalPath: filepath.Join(t.TempDir(), "config.yml")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z2.close)
	p, l, tp = builtinIDs(t, z2)
	if !reflect.DeepEqual(p, assemblyInfo.Plugins) || !reflect.DeepEqual(l, assemblyInfo.Layouts) || !sameIDs(tp, assemblyInfo.Templates) {
		t.Errorf("default session: plugins %v, layouts %v, templates %v; assembly %+v", p, l, tp, assemblyInfo)
	}
}

func sameIDs(a, b []string) bool {
	return len(a) == len(b) && strings.Join(sorted(a), ",") == strings.Join(sorted(b), ",")
}

func sorted(a []string) []string {
	out := append([]string(nil), a...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// Zenmark lists everything, so the assembly it carries is every built-in.
func TestZenmarkAssemblyIsEverything(t *testing.T) {
	cat, _ := assembly.BuiltinCatalog()
	if assemblyInfo.App != "zenmark" || assemblyInfo.Theme != "default" ||
		!reflect.DeepEqual(assemblyInfo.Plugins, cat.Plugins) || !reflect.DeepEqual(assemblyInfo.Layouts, cat.Layouts) || !reflect.DeepEqual(assemblyInfo.Templates, cat.Templates) {
		t.Errorf("assemblyInfo %+v", assemblyInfo)
	}
}

// Startup carries the build: the assembly and the add-ons linked in.
func TestStartupBuild(t *testing.T) {
	z, _, _, _ := newTestZen(t, "", "", "")
	b := z.Startup().Build
	if b == nil || b.App != "zenmark" || b.Theme != "default" || len(b.Plugins) == 0 || len(b.Layouts) == 0 || len(b.Templates) == 0 {
		t.Fatalf("Startup.Build = %+v", b)
	}
	if !reflect.DeepEqual(b.Addons, z.Startup().Addons) {
		t.Errorf("Build.Addons %v differs from Addons", b.Addons)
	}
	if len(b.Addons) != len(wantAddons) {
		t.Errorf("Build.Addons %v, want %v", b.Addons, wantAddons)
	}
}

// zenmd init with a template the build does not have fails and lists the ones it has.
func TestInitMissingTemplate(t *testing.T) {
	two, err := builtins.SiteTemplatesFor([]string{"techdocs", "learning"})
	if err != nil {
		t.Fatal(err)
	}
	env := siteEnv{globalPath: filepath.Join(t.TempDir(), "config.yml"), builtins: two}
	var out, errOut bytes.Buffer
	err = initSite(env.api(), cli.Init{Dir: filepath.Join(t.TempDir(), "x"), Template: "planner", Values: map[string]string{}}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), `no template "planner"`) || !strings.Contains(err.Error(), "techdocs, learning") {
		t.Fatalf("error %v", err)
	}
	if entries, _ := fs.ReadDir(two, "."); len(entries) != 2 {
		t.Errorf("the assembled templates list %d folders", len(entries))
	}
}
