package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/zenmark-org/zenmd/engine/plugins"
	"github.com/zenmark-org/zenmd/engine/session"
	"github.com/zenmark-org/zenmd/internal/cli"
	"github.com/zenmark-org/zenmd/kit/assembly"
)

// roWindowsZen is a read-only Zen with a window manager (so a window would
// open if the guard let it).
func roWindowsZen(t *testing.T, r *windowRig, files map[string]string) *Zen {
	t.Helper()
	z, err := newZen(zenOptions{
		Registry: plugins.NewRegistry(), Dir: tree(t, files), ReadOnly: true,
		GlobalPath: filepath.Join(t.TempDir(), "config.yml"),
		Events:     &sink{}, Dialogs: &fakeDialogs{}, Open: func(string) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	z.windows = r.m
	return z
}

func TestReadOnlyNoEditor(t *testing.T) {
	r := newWindowRig(t)
	z := roWindowsZen(t, r, map[string]string{"README.md": "# Home\n"})
	if err := z.OpenEditor("README.md"); !errors.Is(err, session.ErrReadOnly) {
		t.Errorf("OpenEditor: %v", err)
	}
	for _, which := range []string{"global", "workspace"} {
		if err := z.OpenConfig(which); !errors.Is(err, session.ErrReadOnly) {
			t.Errorf("OpenConfig(%s): %v", which, err)
		}
	}
	// Without a window manager too: no text editor either.
	z.windows = nil
	old := openFile
	openFile = func(string) error { t.Error("openFile called"); return nil }
	t.Cleanup(func() { openFile = old })
	if err := z.OpenConfig("global"); !errors.Is(err, session.ErrReadOnly) {
		t.Errorf("OpenConfig without windows: %v", err)
	}
	if len(r.specs) != 0 {
		t.Errorf("windows opened: %+v", r.specs)
	}
	// Writes through the bindings are refused too.
	if err := z.Save("README.md", "x"); !errors.Is(err, session.ErrReadOnly) {
		t.Errorf("Save: %v", err)
	}
	// Reading is unchanged.
	if pg, err := z.Page("README.md"); err != nil || pg.HTML == "" {
		t.Errorf("Page: %v", err)
	}
	// A build with the editor is not read-only.
	withFeatures(t, allFeatures...)
	if readOnlyBuild() {
		t.Error("features: all has the editor")
	}
}

func TestReadOnlyStartupEdit(t *testing.T) {
	r := newWindowRig(t)
	z := roWindowsZen(t, r, map[string]string{"README.md": "# Home\n", "guide.md": "# Guide\n"})
	z.page = "guide.md"
	if err := z.openStartupEditor("edit"); err != nil {
		t.Fatalf("an edit launch falls back to the reader: %v", err)
	}
	if len(r.specs) != 0 {
		t.Errorf("editor window opened: %+v", r.specs)
	}
	if st := z.Startup(); st.Mode != "read" || st.Page != "guide.md" {
		t.Errorf("startup = %+v", st)
	}
	// An editor window from an older state file is refused.
	if err := z.OpenEditor("guide.md"); !errors.Is(err, session.ErrReadOnly) {
		t.Errorf("OpenEditor: %v", err)
	}
	if _, err := os.Stat(filepath.Join(z.s.Workspace().Root, ".zenmd.yml")); err == nil {
		t.Error(".zenmd.yml created")
	}
}

func TestStartupBuildFeatures(t *testing.T) {
	z, _, _, _ := newTestZen(t, "", "", "")
	if b := z.Startup().Build; b == nil || !slices.Contains(b.Features, "editor") {
		t.Errorf("Build.Features = %+v", b)
	}
}

// withFeatures makes the build have exactly these features for the test.
func withFeatures(t *testing.T, ids ...string) {
	t.Helper()
	old := assemblyInfo.Features
	assemblyInfo.Features = slices.Clone(ids)
	t.Cleanup(func() { assemblyInfo.Features = old })
}

// without is every feature but the ones named.
func without(ids ...string) []string {
	return slices.DeleteFunc(slices.Clone(allFeatures), func(f string) bool { return slices.Contains(ids, f) })
}

// The app's list of features is the catalog's (kit/assembly), in order.
func TestAllFeaturesMatchCatalog(t *testing.T) {
	cat, err := assembly.BuiltinCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(allFeatures, cat.Features) {
		t.Errorf("allFeatures %v, catalog %v", allFeatures, cat.Features)
	}
}

func TestErrFeatureMissing(t *testing.T) {
	err := ErrFeatureMissing("find")
	if err.Error() != "this app is built without find" || !errors.Is(err, ErrFeatureMissing("find")) || errors.Is(err, ErrFeatureMissing("print")) {
		t.Errorf("ErrFeatureMissing = %v", err)
	}
	withFeatures(t, "reader")
	if !hasFeature("reader") || hasFeature("find") {
		t.Error("hasFeature does not follow the build")
	}
	if requireFeature("reader") != nil || !errors.Is(requireFeature("find"), ErrFeatureMissing("find")) {
		t.Error("requireFeature does not follow the build")
	}
}

// A binding of a feature that is not in the build refuses on the Go side, so a
// script in a page cannot call it (kit/shell-features 6, security).
func TestServiceRefusesMissing(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		feature string // the feature the methods belong to; the build has all the others
		calls   map[string]func(z *Zen) error
	}{
		{"reader", map[string]func(z *Zen) error{
			"Page":    func(z *Zen) error { _, err := z.Page("README.md"); return err },
			"Source":  func(z *Zen) error { _, err := z.Source("README.md"); return err },
			"Preview": func(z *Zen) error { _, err := z.Preview("README.md", "# x"); return err },
			"Plugins": func(z *Zen) error { _, err := z.PluginInvoke("codeblock", "README.md", "{}"); return err },
		}},
		{"nav", map[string]func(z *Zen) error{
			"RevealInFinder": func(z *Zen) error { return z.RevealInFinder("README.md") },
			"SetNavHidden":   func(z *Zen) error { return z.SetNavHidden("README.md", true) },
			"ReorderNav":     func(z *Zen) error { return z.ReorderNav("", []string{"README.md"}) },
		}},
		{"inspector", map[string]func(z *Zen) error{
			"PageInfo": func(z *Zen) error { _, err := z.PageInfo("README.md"); return err },
		}},
		{"print", map[string]func(z *Zen) error{
			"PrintPage":    func(z *Zen) error { return z.PrintPage() },
			"PrintSite":    func(z *Zen) error { return z.PrintSite() },
			"PrintWindow":  func(z *Zen) error { return z.PrintWindow(ctx, "x") },
			"ExportPDF":    func(z *Zen) error { _, err := z.ExportPDF(ctx, "x", false); return err },
			"RevealExport": func(z *Zen) error { return z.RevealExport() },
		}},
		{"export", map[string]func(z *Zen) error{
			"BuildOptions":      func(z *Zen) error { _, err := z.BuildOptions(); return err },
			"BuildWebsite":      func(z *Zen) error { _, err := z.BuildWebsite(BuildOptions{}); return err },
			"SaveBuildDefaults": func(z *Zen) error { _, err := z.SaveBuildDefaults(BuildOptions{}, false); return err },
			"PickBuildFolder":   func(z *Zen) error { _, err := z.PickBuildFolder(); return err },
			"OpenBuiltSite":     func(z *Zen) error { return z.OpenBuiltSite("out") },
			"RevealBuiltSite":   func(z *Zen) error { return z.RevealBuiltSite("out") },
			"ExportPDF":         func(z *Zen) error { _, err := z.ExportPDF(ctx, "x", false); return err },
			"RevealExport":      func(z *Zen) error { return z.RevealExport() },
			"PDFDefaults":       func(z *Zen) error { _, err := z.PDFDefaults(); return err },
			"SavePDFDefaults":   func(z *Zen) error { _, err := z.SavePDFDefaults(PDFOptions{}, false); return err },
			"PickPDFFile":       func(z *Zen) error { _, err := z.PickPDFFile(ctx); return err },
			"RunPDFExport":      func(z *Zen) error { return z.RunPDFExport(PDFRun{}) },
			"OpenExport":        func(z *Zen) error { return z.OpenExport() },
		}},
		{"newsite", map[string]func(z *Zen) error{
			"SiteTemplates":   func(z *Zen) error { _, err := z.SiteTemplates(); return err },
			"PickSiteFolder":  func(z *Zen) error { _, err := z.PickSiteFolder(); return err },
			"CreateSite":      func(z *Zen) error { _, err := z.CreateSite("learning", t.TempDir()+"/new", nil); return err },
			"TemplatesDir":    func(z *Zen) error { _, err := z.TemplatesDir(); return err },
			"RevealTemplates": func(z *Zen) error { return z.RevealTemplates() },
		}},
		{"developer", map[string]func(z *Zen) error{
			"Logs": func(z *Zen) error { _, err := z.Logs(0); return err },
		}},
	}
	for _, c := range cases {
		for name, call := range c.calls {
			t.Run(c.feature+"/"+name, func(t *testing.T) {
				withFeatures(t, without(c.feature)...)
				z, _, _, _ := newTestZen(t, tree(t, docs), "", "read")
				if err := call(z); !errors.Is(err, ErrFeatureMissing(c.feature)) {
					t.Errorf("without %s: %v", c.feature, err)
				}
			})
		}
	}
	// ExportPDF needs print as well as export.
	withFeatures(t, without("print")...)
	z, _, _, _ := newTestZen(t, tree(t, docs), "", "read")
	if _, err := z.ExportPDF(ctx, "x", false); !errors.Is(err, ErrFeatureMissing("print")) {
		t.Errorf("ExportPDF without print: %v", err)
	}
	// With the feature the guard lets the call through.
	withFeatures(t, allFeatures...)
	if _, err := z.Page("README.md"); err != nil {
		t.Errorf("Page with the reader: %v", err)
	}
	// The headless export refuses too.
	withFeatures(t, without("print")...)
	if err := ExportPDF(cli.ExportPDF{Dir: t.TempDir()}, io.Discard, io.Discard); !errors.Is(err, ErrFeatureMissing("print")) {
		t.Errorf("headless export: %v", err)
	}
	withFeatures(t, without("export")...)
	if err := ExportPDF(cli.ExportPDF{Dir: t.TempDir()}, io.Discard, io.Discard); !errors.Is(err, ErrFeatureMissing("export")) {
		t.Errorf("headless export: %v", err)
	}
}

// features: [] is a bare shell: a folder opens and the main area has something to say about it.
func TestBareShellOpens(t *testing.T) {
	withFeatures(t)
	dir := tree(t, docs)
	z, _, _, _ := newTestZen(t, dir, "", "read")
	st := z.Startup()
	if st.Workspace == nil || st.Build == nil || len(st.Build.Features) != 0 {
		t.Fatalf("startup %+v", st)
	}
	if !readOnlyBuild() {
		t.Error("a bare shell is read-only")
	}
	z2, _, dl, _ := newTestZen(t, "", "", "read")
	dl.folder = dir
	if ws, err := z2.PickFolder(); err != nil || ws == nil {
		t.Errorf("PickFolder: %v %v", ws, err)
	}
}
