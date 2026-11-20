package main

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func gen(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

// appDir makes apps/<app> with an app-assembly.yml and a Go file of the app.
func appDir(t *testing.T, yml string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"app-assembly.yml": yml, "app.go": "package demoapp\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const full = `app: demo
addons: [vale]
plugins: [codeblock, quiz, check]
layouts: [docs, landing, course]
templates: [learning]
theme: default
`

func TestTagsFromAssembly(t *testing.T) {
	dir := appDir(t, full)
	code, out, errOut := gen("-tags-only", dir)
	if code != 0 || strings.TrimSpace(out) != "addon_vale" || errOut != "" {
		t.Errorf("code %d, out %q, err %q", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "assembly_gen.go")); err == nil {
		t.Error("-tags-only writes nothing")
	}
	code, out, _ = gen("-ids-only", dir)
	if code != 0 || strings.TrimSpace(out) != "vale" {
		t.Errorf("-ids-only: code %d, out %q", code, out)
	}
	dir = appDir(t, "app: demo\n")
	if code, out, errOut = gen("-tags-only", dir); code != 0 || strings.TrimSpace(out) != "" || errOut != "" {
		t.Errorf("no add-ons: code %d, out %q, err %q", code, out, errOut)
	}
}

func TestAddonsOverride(t *testing.T) {
	dir := appDir(t, full)
	// An explicit, even empty, list wins over the file.
	if code, out, errOut := gen("-tags-only", "-addons", "", dir); code != 0 || strings.TrimSpace(out) != "" || errOut != "" {
		t.Errorf("empty override: code %d, out %q, err %q", code, out, errOut)
	}
	if code, out, _ := gen("-ids-only", "-addons", "", dir); code != 0 || strings.TrimSpace(out) != "" {
		t.Errorf("empty override ids: code %d, out %q", code, out)
	}
	none := appDir(t, "app: demo\n")
	if code, out, _ := gen("-tags-only", "-addons", " vale ", none); code != 0 || strings.TrimSpace(out) != "addon_vale" {
		t.Errorf("override: code %d, out %q", code, out)
	}
	// A typo fails and lists the known add-ons.
	code, out, errOut := gen("-tags-only", "-addons", "valee", dir)
	if code == 0 || out != "" || !strings.Contains(errOut, "known add-ons: vale") {
		t.Errorf("unknown: code %d, out %q, err %q", code, out, errOut)
	}
}

func TestOnlyListedPluginsImported(t *testing.T) {
	dir := appDir(t, full)
	if code, _, errOut := gen(dir); code != 0 {
		t.Fatalf("gen: %s", errOut)
	}
	src, err := os.ReadFile(filepath.Join(dir, "assembly_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	if !strings.HasPrefix(text, "// Code generated") || !strings.Contains(text, "DO NOT EDIT.") || !strings.Contains(text, "package demoapp") {
		t.Errorf("header or package wrong:\n%s", text)
	}
	f, err := parser.ParseFile(token.NewFileSet(), "assembly_gen.go", src, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("not Go: %v\n%s", err, text)
	}
	var plugins []string
	for _, im := range f.Imports {
		p := strings.Trim(im.Path.Value, `"`)
		if rest, ok := strings.CutPrefix(p, "github.com/zenmark-org/zenmd/builtins/plugins/"); ok {
			plugins = append(plugins, rest)
		}
	}
	if strings.Join(plugins, ",") != "check,codeblock,quiz" {
		t.Errorf("imported plugin packages %v; want the three listed", plugins)
	}
	for _, w := range []string{"func assemblyPlugins()", "func assemblyLayouts(", "func assemblyTemplates()", "assemblyInfo", `"learning"`, `"course"`, `"default"`} {
		if !strings.Contains(text, w) {
			t.Errorf("generated file lacks %s", w)
		}
	}
	for _, w := range []string{"diagram", "sandbox", `"planner"`, `"slides"`} {
		if strings.Contains(text, w) {
			t.Errorf("generated file mentions %s, which is not listed", w)
		}
	}
	// Idempotent.
	before, _ := os.ReadFile(filepath.Join(dir, "assembly_gen.go"))
	gen(dir)
	after, _ := os.ReadFile(filepath.Join(dir, "assembly_gen.go"))
	if !bytes.Equal(before, after) {
		t.Error("a second run changed the file")
	}
}

func TestNoInjection(t *testing.T) {
	for _, bad := range []string{
		`plugins: ['codeblock"}; os.Exit(1); _ = []string{"']`,
		`plugins: ["codeblock\n\tfoo"]`,
		`layouts: ['article"); panic("x']`,
		`templates: ['learning` + "`" + `']`,
		`theme: 'default"; panic("x")'`,
		`app: 'demo"; panic("x")'`,
		`features: ['editor"}; panic("x")']`,
	} {
		dir := appDir(t, "app: demo\n"+bad+"\n")
		code, _, errOut := gen(dir)
		if code == 0 || errOut == "" {
			t.Errorf("%q: code %d, err %q; want failure", bad, code, errOut)
		}
		if _, err := os.Stat(filepath.Join(dir, "assembly_gen.go")); err == nil {
			t.Errorf("%q: a file was written", bad)
		}
	}
}

func TestErrors(t *testing.T) {
	if code, _, errOut := gen(); code == 0 || errOut == "" {
		t.Error("no folder should fail")
	}
	if code, _, errOut := gen(filepath.Join(t.TempDir(), "nope")); code == 0 || errOut == "" {
		t.Error("a missing folder should fail")
	}
	dir := appDir(t, "app: demo\nlayouts: [course]\n")
	if code, _, errOut := gen(dir); code == 0 || !strings.Contains(errOut, "layout course needs plugin check") {
		t.Errorf("dependency: code %d, err %q", code, errOut)
	}
}

func TestFeaturesGenerated(t *testing.T) {
	read := func(yml string) string {
		dir := appDir(t, yml)
		if code, _, errOut := gen(dir); code != 0 {
			t.Fatalf("gen: %s", errOut)
		}
		src, _ := os.ReadFile(filepath.Join(dir, "assembly_gen.go"))
		return string(src)
	}
	if got := read("app: demo\nfeatures: all\n"); !strings.Contains(got, `Features:  []string{"editor"}`) {
		t.Errorf("features all:\n%s", got)
	}
	if got := read("app: demo\n"); !strings.Contains(got, `Features:  []string{"editor"}`) {
		t.Errorf("missing features means all:\n%s", got)
	}
	if got := read("app: demo\nfeatures: []\n"); !strings.Contains(got, `Features:  []string{}`) {
		t.Errorf("features none:\n%s", got)
	}
}
