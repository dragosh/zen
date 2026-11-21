package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zenmark-org/zenmd/engine/config"
)

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// writeGlobal writes the global config.yml of the fake home.
func writeGlobal(t *testing.T, text string) {
	t.Helper()
	p, err := config.GlobalPath()
	if err != nil || p == "" {
		t.Fatalf("no global path: %v", err)
	}
	writeFile(t, p, text)
}

func TestSiteFlagsReachTheBuild(t *testing.T) {
	h, _ := pdfSetup(t)
	for _, c := range []struct {
		args []string
		want func(Build) bool
	}{
		{[]string{"--scheme", "dark"}, func(b Build) bool { return b.Scheme == "dark" }},
		{[]string{"--scheme=light"}, func(b Build) bool { return b.Scheme == "light" }},
		{[]string{"--scheme", "auto"}, func(b Build) bool { return b.Scheme == "auto" }},
		{nil, func(b Build) bool { return b.Scheme == "" && !b.NoDrafts && !b.Drafts && !b.NoPrint }},
		{[]string{"--no-drafts"}, func(b Build) bool { return b.NoDrafts && !b.Drafts }},
		{[]string{"--drafts"}, func(b Build) bool { return b.Drafts && !b.NoDrafts }},
		{[]string{"--print"}, func(b Build) bool { return !b.NoPrint }},
		{[]string{"--no-print"}, func(b Build) bool { return b.NoPrint }},
		{[]string{"--hosted"}, func(b Build) bool { return b.Hosted && !b.Portable }},
	} {
		r := run(t, h, nil, append([]string{"export", "site", "My Docs"}, c.args...)...)
		if r.code != 0 || !r.built || !c.want(r.b) {
			t.Errorf("%v: %+v", c.args, r)
		}
	}
}

// The PDF: a flag wins over the folder's export.pdf, which wins over the global config.yml, then the built-in default.
func TestExportPDFPrecedence(t *testing.T) {
	h, dir := pdfSetup(t)
	yml := filepath.Join(dir, ".zenmd.yml")
	do := func(args ...string) ExportPDF {
		t.Helper()
		r := run(t, h, nil, append([]string{"export", "pdf", "My Docs"}, args...)...)
		if r.code != 0 || !r.pdfd {
			t.Fatalf("%v: %+v", args, r)
		}
		return r.p
	}
	// Built-in defaults: no drafts, cover on, nothing opened (the app opens; the command line does not).
	if p := do(); p.Drafts || p.NoCover || p.Open || p.Section != "" {
		t.Fatalf("defaults: %+v", p)
	}
	// The global file gives defaults.
	writeGlobal(t, "export:\n  pdf:\n    drafts: true\n    cover: false\n    open: true\n")
	if p := do(); !p.Drafts || !p.NoCover || !p.Open {
		t.Fatalf("global: %+v", p)
	}
	// The folder wins over the global one, key by key.
	writeFile(t, yml, "version: 1\nexport:\n  pdf:\n    drafts: false\n    open: false\n")
	if p := do(); p.Drafts || !p.NoCover || p.Open {
		t.Fatalf("folder over global: %+v", p)
	}
	// The flags win over both, in both directions.
	if p := do("--drafts", "--cover", "--open"); !p.Drafts || p.NoCover || !p.Open {
		t.Fatalf("positive flags: %+v", p)
	}
	writeFile(t, yml, "version: 1\nexport:\n  pdf:\n    drafts: true\n    cover: true\n    open: true\n")
	if p := do("--no-drafts", "--no-cover", "--no-open"); p.Drafts || !p.NoCover || p.Open {
		t.Fatalf("negative flags: %+v", p)
	}
	// --all is the whole folder; --section a page.
	if p := do("--section", "a.md"); p.Section != "a.md" {
		t.Fatalf("--section: %+v", p)
	}
	if p := do("--all"); p.Section != "" {
		t.Fatalf("--all: %+v", p)
	}
}

// --open opens the written PDF with macOS open, after it is written and only then.
func TestExportPDFOpen(t *testing.T) {
	h, dir := pdfSetup(t)
	var opened []string
	old := openFile
	openFile = func(path string) error { opened = append(opened, path); return nil }
	t.Cleanup(func() { openFile = old })

	want := filepath.Join(dir, "exports", "pdf", "My Docs.pdf")
	if r := run(t, h, nil, "export", "pdf", "My Docs"); r.code != 0 || len(opened) != 0 {
		t.Fatalf("default: %+v opened %v", r, opened)
	}
	if r := run(t, h, nil, "export", "pdf", "My Docs", "--open"); r.code != 0 || len(opened) != 1 || opened[0] != want {
		t.Fatalf("--open: %+v opened %v", r, opened)
	}
	opened = nil
	pdfErr = os.ErrNotExist
	r := run(t, h, nil, "export", "pdf", "My Docs", "--open")
	pdfErr = nil
	if r.code != 1 || len(opened) != 0 {
		t.Fatalf("failed export: %+v opened %v", r, opened)
	}
	// A failing open is a warning: the PDF is written.
	openFile = func(string) error { return os.ErrPermission }
	if r := run(t, h, nil, "export", "pdf", "My Docs", "--open"); r.code != 0 || !strings.Contains(r.stderr, "could not open") {
		t.Fatalf("open fails: %+v", r)
	}
}

const original = "# mine\nversion: 1\nexport:\n  # site\n  out: exports/html\n  exclude:\n    - \"x/**\" # keep\nplugins:\n  check: true\n"

// --save-defaults writes the flags given into the folder's .zenmd.yml as Save as folder defaults does: comments and
// other keys stay, it prints what changed, then exports; --dry-run prints it and writes and exports nothing.
func TestSaveDefaultsSite(t *testing.T) {
	h, dir := pdfSetup(t)
	yml := filepath.Join(dir, ".zenmd.yml")
	writeFile(t, yml, original)

	// Dry run first: the change is shown, nothing is written, nothing is built.
	r := run(t, h, nil, "export", "site", "My Docs", "--drafts", "--scheme", "dark", "--hosted", "-o", "My Docs/site", "--clean", "--no-print", "--save-defaults", "--dry-run")
	if r.code != 0 || r.built || readFile(t, yml) != original {
		t.Fatalf("dry run: %+v\n%s", r, readFile(t, yml))
	}
	for _, want := range []string{"Changes .zenmd.yml:", "export.drafts: true", "export.theme.scheme: dark", "export.portable: false", "export.out: site"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("dry run output lacks %q:\n%s", want, r.stdout)
		}
	}
	for _, not := range []string{"clean", "print"} {
		if strings.Contains(r.stdout, not) {
			t.Errorf("a per-run option is saved: %q in\n%s", not, r.stdout)
		}
	}
	// For real: written, printed before the build output, then built.
	r2 := run(t, h, nil, "export", "site", "My Docs", "--drafts", "--scheme", "dark", "--hosted", "-o", "My Docs/site", "--clean", "--no-print", "--save-defaults")
	if r2.code != 0 || !r2.built || !r2.b.Drafts || r2.b.Scheme != "dark" || !r2.b.Hosted {
		t.Fatalf("save: %+v", r2)
	}
	if i, j := strings.Index(r2.stdout, "export.drafts: true"), strings.Index(r2.stdout, "Built 1 pages"); i < 0 || j < 0 || i > j {
		t.Errorf("the change is printed before the build output:\n%s", r2.stdout)
	}
	got := readFile(t, yml)
	for _, keep := range []string{"# mine", "# site", `"x/**" # keep`, "check: true", "drafts: true", "scheme: dark", "portable: false", "out: site"} {
		if !strings.Contains(got, keep) {
			t.Errorf("lacks %q:\n%s", keep, got)
		}
	}
	b, _, err := config.Load("", dir).Build()
	if err != nil || b.Scheme != "dark" || !b.Drafts || b.Portable == nil || *b.Portable || b.Out != "site" {
		t.Errorf("round trip: %+v %v", b, err)
	}
	// The same again says there is nothing to change, and still exports.
	r3 := run(t, h, nil, "export", "site", "My Docs", "--drafts", "--scheme", "dark", "--save-defaults")
	if r3.code != 0 || !r3.built || !strings.Contains(r3.stdout, "Nothing to change") || readFile(t, yml) != got {
		t.Errorf("again: %+v", r3)
	}
	// Only the flags given are saved: --no-drafts leaves the scheme alone.
	r4 := run(t, h, nil, "export", "site", "My Docs", "--no-drafts", "--save-defaults")
	after := readFile(t, yml)
	if r4.code != 0 || !strings.Contains(after, "drafts: false") || !strings.Contains(after, "scheme: dark") {
		t.Errorf("--no-drafts: %+v\n%s", r4, after)
	}
	// `build --save-defaults` is the same command.
	if r5 := run(t, h, nil, "build", "My Docs", "--scheme", "light", "--save-defaults"); r5.code != 0 || !strings.Contains(readFile(t, yml), "scheme: light") {
		t.Errorf("build: %+v", r5)
	}
	// --dry-run on its own is a usage error.
	if r := run(t, h, nil, "export", "site", "My Docs", "--dry-run"); r.code != 2 || r.built || !strings.Contains(r.stderr, "--save-defaults") {
		t.Errorf("--dry-run alone: %+v", r)
	}
}

func TestSaveDefaultsPDF(t *testing.T) {
	h, dir := pdfSetup(t)
	yml := filepath.Join(dir, ".zenmd.yml")
	writeFile(t, yml, original)

	r := run(t, h, nil, "export", "pdf", "My Docs", "--no-cover", "--open", "--drafts", "-o", filepath.Join(dir, "pdf", "guide.pdf"), "--section", "a.md", "--save-defaults", "--dry-run")
	if r.code != 0 || r.pdfd || readFile(t, yml) != original {
		t.Fatalf("dry run: %+v", r)
	}
	for _, want := range []string{"export.pdf.cover: false", "export.pdf.open: true", "export.pdf.drafts: true", "export.pdf.out: pdf/guide.pdf"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("lacks %q:\n%s", want, r.stdout)
		}
	}
	if strings.Contains(r.stdout, "section") {
		t.Errorf("--section is per run, not a default:\n%s", r.stdout)
	}
	old := openFile
	openFile = func(string) error { return nil }
	t.Cleanup(func() { openFile = old })
	r = run(t, h, nil, "export", "pdf", "My Docs", "--no-cover", "--open", "--save-defaults")
	if r.code != 0 || !r.pdfd || !r.p.NoCover || !r.p.Open {
		t.Fatalf("save: %+v", r)
	}
	got := readFile(t, yml)
	for _, keep := range []string{"# mine", "# site", "out: exports/html", `"x/**" # keep`, "pdf:", "cover: false", "open: true"} {
		if !strings.Contains(got, keep) {
			t.Errorf("lacks %q:\n%s", keep, got)
		}
	}
	if strings.Contains(got, "drafts") || strings.Contains(got, "out: pdf") {
		t.Errorf("only the flags given are saved:\n%s", got)
	}
	p, err := config.Load("", dir).PDF()
	if err != nil || p.CoverOn() || p.Open == nil || !*p.Open {
		t.Errorf("round trip: %+v %v", p, err)
	}
	// A PDF that goes outside the folder cannot be a folder default (the config refuses it when read).
	r = run(t, h, nil, "export", "pdf", "My Docs", "-o", filepath.Join(h, "far.pdf"), "--save-defaults")
	if r.code != 1 || r.pdfd || !strings.Contains(r.stderr, "inside the folder") || readFile(t, yml) != got {
		t.Errorf("outside: %+v", r)
	}
}

// The notices of the config are printed by both exports: the old file name, and a legacy build: key.
func TestExportPrintsConfigNotices(t *testing.T) {
	h := home(t)
	mk := func(name, file, text string) string {
		dir := filepath.Join(h, name)
		write(t, filepath.Join(dir, "README.md"))
		writeFile(t, filepath.Join(dir, file), text)
		return name
	}
	legacyKey := mk("legacykey", ".zenmd.yml", "version: 1\nbuild:\n  out: x\n")
	bothKeys := mk("bothkeys", ".zenmd.yml", "version: 1\nbuild:\n  out: x\nexport:\n  out: y\n")
	oldFile := mk("oldfile", ".zen.yml", "version: 1\n")
	clean := mk("clean", ".zenmd.yml", "version: 1\nexport:\n  out: x\n")
	const (
		keyNotice  = "build: in .zenmd.yml is now export:; rename the key"
		bothNotice = "build: in .zenmd.yml is ignored because export: is set"
		fileNotice = "This folder's settings are in .zen.yml; Zenmark now calls the file .zenmd.yml."
	)
	for _, cmd := range [][]string{{"export", "site"}, {"export", "pdf"}, {"build"}} {
		for dir, want := range map[string]string{legacyKey: keyNotice, bothKeys: bothNotice, oldFile: fileNotice} {
			r := run(t, h, nil, append(append([]string{}, cmd...), dir)...)
			if r.code != 0 || strings.Count(r.stderr, want+"\n") != 1 {
				t.Errorf("%v %s: want one %q in %+v", cmd, dir, want, r)
			}
		}
		if r := run(t, h, nil, append(append([]string{}, cmd...), clean)...); strings.Contains(r.stderr, "build:") || strings.Contains(r.stderr, ".zen.yml") {
			t.Errorf("%v clean: %q", cmd, r.stderr)
		}
	}
	// A legacy key of the global file too.
	writeGlobal(t, "build:\n  drafts: true\n")
	if r := run(t, h, nil, "export", "pdf", clean); !strings.Contains(r.stderr, "build: in config.yml is now export:") {
		t.Errorf("global notice: %q", r.stderr)
	}
}
