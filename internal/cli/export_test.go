package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `export site` is `build`: the same Build value for the same arguments, and
// `build` keeps working (app/export behaviour 9).
func TestExportSiteIsBuild(t *testing.T) {
	h := home(t)
	ws := filepath.Join(h, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{},
		{"ws"},
		{"ws", "-o", "out", "--hosted", "--drafts", "--clean", "--no-print"},
		{"--portable", "--out=site", "ws"},
	} {
		old := run(t, h, nil, append([]string{"build"}, args...)...)
		now := run(t, h, nil, append([]string{"export", "site"}, args...)...)
		if !old.built || !now.built || old.code != 0 || now.code != 0 || old.b != now.b {
			t.Errorf("%v: build %+v, export site %+v", args, old, now)
		}
		if old.stdout != now.stdout || old.stderr != now.stderr {
			t.Errorf("%v: output differs: %q/%q vs %q/%q", args, old.stdout, old.stderr, now.stdout, now.stderr)
		}
		if old.stderr != "a.md: a warning\n" { // no notice for the alias
			t.Errorf("%v: stderr %q", args, old.stderr)
		}
	}
	// The same usage errors.
	for _, a := range [][]string{{"export", "site", "-o"}, {"export", "site", "--nope"}, {"export", "site", "a", "b"}, {"export", "site", "--portable", "--hosted"}} {
		r := run(t, h, nil, a...)
		if r.code != 2 || r.built {
			t.Errorf("%v: %+v", a, r)
		}
	}
}

func TestHelpListsExport(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}} {
		r := run(t, "/", nil, args...)
		for _, want := range []string{"zenmd export site", "zenmd export pdf", "zenmd build", "same as export site"} {
			if !strings.Contains(r.stdout, want) {
				t.Errorf("%v: help lacks %q:\n%s", args, want, r.stdout)
			}
		}
	}
	// `export --help` shows it too.
	if r := run(t, "/", nil, "export", "--help"); r.code != 0 || !strings.Contains(r.stdout, "export pdf") {
		t.Errorf("export --help: %+v", r)
	}
}

// pdfSetup makes a folder "My Docs" with a page in a fake home.
func pdfSetup(t *testing.T) (h, dir string) {
	t.Helper()
	h = home(t)
	dir = filepath.Join(h, "My Docs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return h, dir
}

// The output path rules of `export pdf` (behaviour 10).
func TestExportPDFOut(t *testing.T) {
	h, dir := pdfSetup(t)
	want := func(r result, out string) {
		t.Helper()
		if r.code != 0 || !r.pdfd || r.built || r.launched || r.p.Dir != dir || r.p.Out != out {
			t.Fatalf("got %+v (code %d, stderr %q), want Dir %q Out %q", r.p, r.code, r.stderr, dir, out)
		}
		if !strings.Contains(r.stdout, "Wrote "+out+" (2 pages)") {
			t.Errorf("stdout %q", r.stdout)
		}
	}
	// Default: <folder>/<folder name>.pdf, for a relative folder and from inside it.
	want(run(t, h, nil, "export", "pdf", "My Docs"), filepath.Join(dir, "My Docs.pdf"))
	want(run(t, dir, nil, "export", "pdf"), filepath.Join(dir, "My Docs.pdf"))
	want(run(t, h, nil, "export", "pdf", dir), filepath.Join(dir, "My Docs.pdf"))
	// A relative -o is relative to the current folder, not the exported one.
	want(run(t, h, nil, "export", "pdf", "My Docs", "-o", "out.pdf"), filepath.Join(h, "out.pdf"))
	want(run(t, h, nil, "export", "pdf", "My Docs", "--out", "out.pdf"), filepath.Join(h, "out.pdf"))
	want(run(t, h, nil, "export", "pdf", "My Docs", "--out=out.pdf"), filepath.Join(h, "out.pdf"))
	// An absolute -o as it is; a missing .pdf is added (any case is kept).
	want(run(t, h, nil, "export", "pdf", "My Docs", "-o", filepath.Join(h, "x", "..", "abs.pdf")), filepath.Join(h, "abs.pdf"))
	want(run(t, h, nil, "export", "pdf", "My Docs", "-o", "guide"), filepath.Join(h, "guide.pdf"))
	want(run(t, h, nil, "export", "pdf", "My Docs", "-o", "Guide.PDF"), filepath.Join(h, "Guide.PDF"))
	want(run(t, h, nil, "export", "pdf", "My Docs", "-o", "v1.2"), filepath.Join(h, "v1.2.pdf"))
}

func TestExportErrors(t *testing.T) {
	h, dir := pdfSetup(t)
	oneLine := func(r result, code int, contains string) {
		t.Helper()
		if r.code != code || r.pdfd || r.built || r.launched || r.stdout != "" {
			t.Fatalf("%+v", r)
		}
		if code == 1 {
			if strings.Count(r.stderr, "\n") != 1 || !strings.HasPrefix(r.stderr, "zenmd: ") || !strings.Contains(r.stderr, contains) {
				t.Errorf("stderr %q, want one line with %q", r.stderr, contains)
			}
		} else if !strings.Contains(r.stderr, "Usage:") || !strings.Contains(r.stderr, contains) {
			t.Errorf("stderr %q, want the usage and %q", r.stderr, contains)
		}
	}
	// Usage errors: exit 2 with the usage.
	oneLine(run(t, h, nil, "export"), 2, "export")
	oneLine(run(t, h, nil, "export", "epub"), 2, "epub")
	oneLine(run(t, h, nil, "export", "pdf", "-o"), 2, "-o")
	oneLine(run(t, h, nil, "export", "pdf", "--nope"), 2, "--nope")
	oneLine(run(t, h, nil, "export", "pdf", "a", "b"), 2, "more than one")
	oneLine(run(t, h, nil, "export", "pdf", "-o", ""), 2, "-o")
	// Failures: exit 1, one line.
	oneLine(run(t, h, nil, "export", "pdf", "missing"), 1, "missing")
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("# a"), 0o644); err != nil {
		t.Fatal(err)
	}
	oneLine(run(t, h, nil, "export", "pdf", filepath.Join(dir, "a.md")), 1, "takes a folder")
	oneLine(run(t, h, nil, "export", "pdf", "My Docs", "-o", filepath.Join(h, "nosuch", "x.pdf")), 1, "nosuch")
	oneLine(run(t, h, nil, "export", "pdf", "My Docs", "-o", dir), 1, "folder") // -o is a folder: no file inside it
	oneLine(run(t, h, nil, "export", "pdf", "My Docs", "-o", "My Docs"), 1, "folder")
	// The exporter's own errors come out as one line each and exit 1.
	for _, msg := range []string{"export pdf needs macOS", "export pdf needs a logged-in macOS session", "the page layout did not finish in 30 s", "no pages in " + dir} {
		pdfErr = errors.New(msg)
		r := run(t, h, nil, "export", "pdf", "My Docs")
		pdfErr = nil
		if r.code != 1 || !r.pdfd || !strings.HasSuffix(r.stderr, "zenmd: "+msg+"\n") || strings.Count(r.stderr, "\n") != 1 {
			t.Errorf("%q: %+v", msg, r)
		}
	}
	// No exporter wired: a plain error, not a crash.
	var out, errOut strings.Builder
	if code := Run([]string{"export", "pdf", "My Docs"}, h, &out, &errOut, func(Launch) error { return nil }, nil, nil, nil); code != 1 || !strings.Contains(errOut.String(), "export") {
		t.Errorf("nil exporter: %d %q", code, errOut.String())
	}
}

// The default file name comes from the folder name without path separators (security table).
func TestPDFNameSanitised(t *testing.T) {
	for in, want := range map[string]string{
		"My Docs":   "My Docs.pdf",
		"a/b":       "a-b.pdf",
		`a\b`:       "a-b.pdf",
		"a/../b":    "a-..-b.pdf",
		"..":        "export.pdf",
		".":         "export.pdf",
		"":          "export.pdf",
		"/":         "export.pdf",
		"tab\there": "tab-here.pdf",
		"nl\nx":     "nl-x.pdf",
		".hidden":   "hidden.pdf",
		"v1.2":      "v1.2.pdf",
		"Notes ✓":   "Notes ✓.pdf",
	} {
		got := PDFName(in)
		if got != want {
			t.Errorf("PDFName(%q) = %q, want %q", in, got, want)
		}
		if strings.ContainsAny(got, "/\\") || got == ".pdf" || got == "...pdf" {
			t.Errorf("PDFName(%q) = %q is unsafe", in, got)
		}
	}
	// The default output of a folder is inside it, named after it.
	if got := DefaultPDFOut(filepath.Join("/x", "My Docs")); got != filepath.Join("/x", "My Docs", "My Docs.pdf") {
		t.Errorf("DefaultPDFOut = %q", got)
	}
	if got := DefaultPDFOut("/"); got != "/export.pdf" {
		t.Errorf("DefaultPDFOut(/) = %q", got)
	}
}

// The CLI follows the build: export site needs the export feature, export pdf
// needs export and print; without them it says so and exits 2 (kit/shell-features 9).
func TestExportNeedsFeature(t *testing.T) {
	h, _ := pdfSetup(t)
	t.Cleanup(func() { buildInfo = nil })
	with := func(features ...string) {
		SetBuildInfo(func() BuildInfo { return BuildInfo{App: "demo", Features: features, Theme: "default"} })
	}
	for _, c := range []struct {
		features []string
		args     []string
		missing  string // "" = allowed
	}{
		{nil, []string{"export", "site"}, "export"},
		{nil, []string{"build"}, "export"}, // build is export site
		{nil, []string{"export", "pdf", "My Docs"}, "export"},
		{[]string{"reader", "print"}, []string{"export", "site"}, "export"},
		{[]string{"reader", "export"}, []string{"export", "pdf", "My Docs"}, "print"},
		{[]string{"reader", "print"}, []string{"export", "pdf", "My Docs"}, "export"},
		{[]string{"reader", "export"}, []string{"export", "site"}, ""},
		{[]string{"reader", "export"}, []string{"build"}, ""},
		{[]string{"reader", "print", "export"}, []string{"export", "pdf", "My Docs"}, ""},
	} {
		with(c.features...)
		r := run(t, h, nil, c.args...)
		if c.missing == "" {
			if r.code != 0 || !(r.built || r.pdfd) {
				t.Errorf("%v with %v: %+v", c.args, c.features, r)
			}
			continue
		}
		if r.code != 2 || r.built || r.pdfd || r.launched || r.stdout != "" || !strings.Contains(r.stderr, "this app is built without "+c.missing) {
			t.Errorf("%v with %v: %+v", c.args, c.features, r)
		}
	}
	// A build that does not say what it is made of allows everything, and other commands are untouched.
	buildInfo = nil
	if r := run(t, h, nil, "export", "site"); r.code != 0 || !r.built {
		t.Errorf("no build info: %+v", r)
	}
	with()
	if r := run(t, h, nil, "version"); r.code != 0 {
		t.Errorf("version in a bare build: %+v", r)
	}
	if r := run(t, h, nil, "My Docs"); r.code != 0 || !r.launched {
		t.Errorf("opening a folder in a bare build: %+v", r)
	}
}
