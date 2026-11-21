package app

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/zenmark-org/zenmd/kit/desktop"
)

// fakePDF replaces the save panel, SavePDF and Finder for one test.
type fakePDF struct {
	panelName, panelDir string // what the panel was asked for (last call)
	panelCalls          int
	chosen              string // what the panel returns
	panelErr            error
	saved               []string // paths SavePDF was given
	titles              []string
	pages               int
	saveErr             error
	revealed            []string
	opened              []string
}

func useFakePDF(t *testing.T, f *fakePDF) {
	t.Helper()
	old := pdfIO
	pdfIO = pdfHooks{
		window: func(context.Context) (application.Window, error) { return nil, nil },
		panel: func(_ application.Window, name, dir string) (string, error) {
			f.panelCalls++
			f.panelName, f.panelDir = name, dir
			return f.chosen, f.panelErr
		},
		save: func(_ application.Window, path, title string) (int, error) {
			f.saved = append(f.saved, path)
			f.titles = append(f.titles, title)
			return f.pages, f.saveErr
		},
		reveal: func(path string) error { f.revealed = append(f.revealed, path); return nil },
		open:   func(path string) error { f.opened = append(f.opened, path); return nil },
	}
	t.Cleanup(func() { pdfIO = old })
}

// A cancelled save panel writes nothing and says nothing (behaviour 5).
func TestExportPDFCancelled(t *testing.T) {
	f := &fakePDF{pages: 3}
	useFakePDF(t, f)
	z, _, _, _ := newTestZen(t, tree(t, docs), "", "")
	path, err := z.ExportPDF(context.Background(), "Guide — Print", false)
	if err != nil || path != "" {
		t.Fatalf("cancelled: %q, %v", path, err)
	}
	if f.panelCalls != 1 || len(f.saved) != 0 {
		t.Errorf("panel %d, saved %v", f.panelCalls, f.saved)
	}
	if err := z.RevealExport(); err == nil {
		t.Error("nothing was exported: RevealExport must say so")
	}
	// The panel starts in exports/pdf of the folder, with the folder's name; the
	// folder is created for it (behaviour 6).
	st := z.Startup().Workspace
	if f.panelName != st.Name+".pdf" {
		t.Errorf("name %q, want %q", f.panelName, st.Name+".pdf")
	}
	if want := filepath.Join(st.Root, "exports", "pdf"); f.panelDir != want {
		t.Errorf("dir %q, want %q", f.panelDir, want)
	}
	if fi, err := os.Stat(f.panelDir); err != nil || !fi.IsDir() {
		t.Errorf("exports/pdf was not created: %v", err)
	}
}

// The file path comes from the save panel only; the frontend sends a title, never a path (security table).
func TestExportPathFromPanelOnly(t *testing.T) {
	out := t.TempDir()
	f := &fakePDF{pages: 2, chosen: filepath.Join(out, "Chosen.pdf")}
	useFakePDF(t, f)
	z, _, _, _ := newTestZen(t, tree(t, docs), "", "")
	evil := "../../../etc/passwd"
	path, err := z.ExportPDF(context.Background(), evil, false)
	if err != nil || path != f.chosen {
		t.Fatalf("got %q, %v", path, err)
	}
	if len(f.saved) != 1 || f.saved[0] != f.chosen || f.titles[0] != evil {
		t.Errorf("SavePDF got %v / %v: the path must be the panel's, the title only a title", f.saved, f.titles)
	}
	// The default name never uses the title.
	if strings.Contains(f.panelName, "..") || strings.ContainsAny(f.panelName, "/\\") {
		t.Errorf("panel name %q", f.panelName)
	}
	// The next panel starts in exports/pdf again, wherever the last file went.
	f.chosen = ""
	if _, err := z.ExportPDF(context.Background(), "x", false); err != nil {
		t.Fatal(err)
	}
	if filepath.Base(f.panelDir) != "pdf" || filepath.Base(filepath.Dir(f.panelDir)) != "exports" {
		t.Errorf("second panel dir %q, want <folder>/exports/pdf", f.panelDir)
	}
	// A blank title falls back to the app name for the job.
	f.chosen = filepath.Join(out, "B.pdf")
	if _, err := z.ExportPDF(context.Background(), "  ", false); err != nil {
		t.Fatal(err)
	}
	if f.titles[len(f.titles)-1] == "" {
		t.Error("the job needs a title")
	}
}

func TestExportPDFErrors(t *testing.T) {
	f := &fakePDF{chosen: filepath.Join(t.TempDir(), "A.pdf"), saveErr: errors.New("disk full")}
	useFakePDF(t, f)
	z, _, _, _ := newTestZen(t, tree(t, docs), "", "")
	path, err := z.ExportPDF(context.Background(), "t", false)
	if err == nil || !strings.Contains(err.Error(), "disk full") || path != "" {
		t.Fatalf("got %q, %v", path, err)
	}
	if err := z.RevealExport(); err == nil {
		t.Error("a failed export is not the last export")
	}
	f.saveErr, f.panelErr = nil, errors.New("panel broke")
	if _, err := z.ExportPDF(context.Background(), "t", false); err == nil {
		t.Error("a panel error is an error")
	}
	// No folder open: nothing to export.
	z2, _, _, _ := newTestZen(t, "", "", "")
	f.panelErr = nil
	if _, err := z2.ExportPDF(context.Background(), "t", false); err == nil {
		t.Error("no folder open is an error")
	}
}

// RevealExport takes no path and shows only the file the last export wrote (security table).
func TestRevealExportOnlyLast(t *testing.T) {
	dir := t.TempDir()
	f := &fakePDF{pages: 1}
	useFakePDF(t, f)
	z, _, _, _ := newTestZen(t, tree(t, docs), "", "")
	if err := z.RevealExport(); err == nil || len(f.revealed) != 0 {
		t.Fatalf("before any export: %v %v", err, f.revealed)
	}
	f.chosen = filepath.Join(dir, "one.pdf")
	if _, err := z.ExportPDF(context.Background(), "t", false); err != nil {
		t.Fatal(err)
	}
	if err := z.RevealExport(); err != nil || len(f.revealed) != 1 || f.revealed[0] != f.chosen {
		t.Fatalf("revealed %v, %v", f.revealed, err)
	}
	f.chosen = filepath.Join(dir, "two.pdf")
	if _, err := z.ExportPDF(context.Background(), "t", false); err != nil {
		t.Fatal(err)
	}
	if err := z.RevealExport(); err != nil || f.revealed[len(f.revealed)-1] != f.chosen {
		t.Fatalf("revealed %v, %v", f.revealed, err)
	}
	// Another service (another run) has its own last export.
	z2, _, _, _ := newTestZen(t, tree(t, docs), "", "")
	if err := z2.RevealExport(); err == nil {
		t.Error("a fresh service has exported nothing")
	}
}

// ---- Export PDF of the Settings form: no panel, the options and the file are the form's (ui/export-pdf-settings)

func exportEvents(ev *sink) []pdfExported {
	var out []pdfExported
	for _, e := range ev.all() {
		if e.Name == EventPDFExport {
			out = append(out, e.Data.(pdfExported))
		}
	}
	return out
}

func fakeJobWindow(t *testing.T) *[]desktop.WindowSpec {
	t.Helper()
	var specs []desktop.WindowSpec
	old := desktop.NewPrintWindow
	desktop.NewPrintWindow = func(s desktop.WindowSpec, onClose func(), _ func()) desktop.Window {
		specs = append(specs, s)
		return &fakePrintWindow{onClose: onClose}
	}
	t.Cleanup(func() { desktop.NewPrintWindow = old })
	return &specs
}

func TestRunPDFExportAutoWritesWithoutPanel(t *testing.T) {
	f := &fakePDF{pages: 4}
	useFakePDF(t, f)
	specs := fakeJobWindow(t)
	z, ev, _, _ := newTestZen(t, tree(t, docs), "docs/guide.md", "")
	root := z.Startup().Workspace.Root
	if _, err := z.Page("docs/guide.md"); err != nil {
		t.Fatal(err)
	}
	err := z.RunPDFExport(PDFRun{PDFOptions: PDFOptions{Out: "out/Guide", Drafts: true, Cover: false, Open: true}, Section: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(*specs) != 1 {
		t.Fatalf("windows %+v", *specs)
	}
	u, _ := url.Parse((*specs)[0].URL)
	q := u.Query()
	if q.Get("window") != "print" || q.Get("auto") != "1" || q.Get("drafts") != "1" || q.Get("cover") != "0" || q.Get("pages") != "section" || q.Get("page") != "docs/guide.md" {
		t.Errorf("preview URL %q", (*specs)[0].URL)
	}
	// The preview, laid out, asks for the file: no panel, the form's file.
	path, err := z.ExportPDF(context.Background(), "Guide — Print", true)
	want := filepath.Join(root, "out", "Guide.pdf")
	if err != nil || path != want {
		t.Fatalf("got %q, %v want %q", path, err, want)
	}
	if f.panelCalls != 0 || len(f.saved) != 1 || f.saved[0] != want {
		t.Errorf("panel %d, saved %v", f.panelCalls, f.saved)
	}
	if fi, err := os.Stat(filepath.Join(root, "out")); err != nil || !fi.IsDir() {
		t.Errorf("the output folder is made: %v", err)
	}
	if len(f.opened) != 1 || f.opened[0] != want {
		t.Errorf("open when done: %v", f.opened)
	}
	if got := exportEvents(ev); len(got) != 1 || got[0].Path != want || got[0].Error != "" {
		t.Errorf("events %+v", got)
	}
	if err := z.RevealExport(); err != nil || f.revealed[0] != want {
		t.Errorf("reveal %v %v", f.revealed, err)
	}
	// The job is done: another auto call has nothing to write.
	if _, err := z.ExportPDF(context.Background(), "x", true); err == nil {
		t.Error("no job pending")
	}
}

func TestRunPDFExportOptionsAndErrors(t *testing.T) {
	f := &fakePDF{pages: 1, saveErr: errors.New("disk full")}
	useFakePDF(t, f)
	specs := fakeJobWindow(t)
	z, ev, _, _ := newTestZen(t, tree(t, docs), "", "")
	// Open when done off: not opened. Section without a page: all pages.
	if err := z.RunPDFExport(PDFRun{PDFOptions: PDFOptions{Out: "a.pdf", Cover: true}, Section: true}); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse((*specs)[0].URL)
	if u.Query().Get("pages") == "section" {
		t.Errorf("no page is open: %q", (*specs)[0].URL)
	}
	if _, err := z.ExportPDF(context.Background(), "t", true); err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("save error: %v", err)
	}
	if got := exportEvents(ev); len(got) != 1 || !strings.Contains(got[0].Error, "disk full") {
		t.Errorf("events %+v", got)
	}
	// The preview could not prepare the document: the form is told.
	if err := z.RunPDFExport(PDFRun{PDFOptions: PDFOptions{Out: "a.pdf", Cover: true}}); err != nil {
		t.Fatal(err)
	}
	z.PDFExportFailed("too many pages")
	if got := exportEvents(ev); len(got) != 2 || got[1].Error != "too many pages" {
		t.Errorf("events %+v", got)
	}
	// A path that leaves the folder is refused before any window opens.
	n := len(*specs)
	if err := z.RunPDFExport(PDFRun{PDFOptions: PDFOptions{Out: "../evil.pdf"}}); err == nil || len(*specs) != n {
		t.Errorf("escape: %v, windows %d", err, len(*specs))
	}
	if len(f.saved) != 1 {
		t.Errorf("saved %v", f.saved)
	}
}

// Choose… is the save panel for the form: it starts in the output file's place and writes nothing.
func TestPickPDFFile(t *testing.T) {
	f := &fakePDF{chosen: "/somewhere/Mine"}
	useFakePDF(t, f)
	z, _, _, _ := newTestZen(t, tree(t, docs), "", "")
	p, err := z.PickPDFFile(context.Background())
	if err != nil || p != "/somewhere/Mine.pdf" || len(f.saved) != 0 {
		t.Fatalf("%q %v %v", p, err, f.saved)
	}
	f.chosen = ""
	if p, err := z.PickPDFFile(context.Background()); err != nil || p != "" {
		t.Errorf("cancelled: %q %v", p, err)
	}
	if err := z.OpenExport(); err == nil {
		t.Error("nothing exported yet")
	}
}

// The panel of the preview starts where the folder's export.pdf.out says.
func TestExportPDFPanelStartsAtFolderDefault(t *testing.T) {
	f := &fakePDF{}
	useFakePDF(t, f)
	z, _, _, _ := newTestZen(t, tree(t, docs), "", "")
	root := z.Startup().Workspace.Root
	if err := os.WriteFile(filepath.Join(root, ".zenmd.yml"), []byte("export:\n  pdf:\n    out: books/Guide.pdf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := z.ExportPDF(context.Background(), "t", false); err != nil {
		t.Fatal(err)
	}
	if f.panelName != "Guide.pdf" || f.panelDir != filepath.Join(root, "books") {
		t.Errorf("panel %q in %q", f.panelName, f.panelDir)
	}
}
