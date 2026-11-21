package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
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
	}
	t.Cleanup(func() { pdfIO = old })
}

// A cancelled save panel writes nothing and says nothing (behaviour 5).
func TestExportPDFCancelled(t *testing.T) {
	f := &fakePDF{pages: 3}
	useFakePDF(t, f)
	z, _, _, _ := newTestZen(t, tree(t, docs), "", "")
	path, err := z.ExportPDF(context.Background(), "Guide — Print")
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
	path, err := z.ExportPDF(context.Background(), evil)
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
	if _, err := z.ExportPDF(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	if filepath.Base(f.panelDir) != "pdf" || filepath.Base(filepath.Dir(f.panelDir)) != "exports" {
		t.Errorf("second panel dir %q, want <folder>/exports/pdf", f.panelDir)
	}
	// A blank title falls back to the app name for the job.
	f.chosen = filepath.Join(out, "B.pdf")
	if _, err := z.ExportPDF(context.Background(), "  "); err != nil {
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
	path, err := z.ExportPDF(context.Background(), "t")
	if err == nil || !strings.Contains(err.Error(), "disk full") || path != "" {
		t.Fatalf("got %q, %v", path, err)
	}
	if err := z.RevealExport(); err == nil {
		t.Error("a failed export is not the last export")
	}
	f.saveErr, f.panelErr = nil, errors.New("panel broke")
	if _, err := z.ExportPDF(context.Background(), "t"); err == nil {
		t.Error("a panel error is an error")
	}
	// No folder open: nothing to export.
	z2, _, _, _ := newTestZen(t, "", "", "")
	f.panelErr = nil
	if _, err := z2.ExportPDF(context.Background(), "t"); err == nil {
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
	if _, err := z.ExportPDF(context.Background(), "t"); err != nil {
		t.Fatal(err)
	}
	if err := z.RevealExport(); err != nil || len(f.revealed) != 1 || f.revealed[0] != f.chosen {
		t.Fatalf("revealed %v, %v", f.revealed, err)
	}
	f.chosen = filepath.Join(dir, "two.pdf")
	if _, err := z.ExportPDF(context.Background(), "t"); err != nil {
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
