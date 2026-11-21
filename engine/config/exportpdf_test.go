package config

import (
	"reflect"
	"testing"
)

func yes(b bool) *bool { return &b }

// export.pdf holds out, drafts, cover and open; Build() does not warn about the key.
func TestExportPDFKey(t *testing.T) {
	c := loadKeys(t, "", "export:\n  out: _site\n  pdf:\n    out: exports/pdf/guide.pdf\n    drafts: true\n    cover: false\n    open: false\n")
	p, err := c.PDF()
	want := PDF{Out: "exports/pdf/guide.pdf", Drafts: true, Cover: yes(false), Open: yes(false)}
	if err != nil || !reflect.DeepEqual(p, want) {
		t.Fatalf("%+v %v", p, err)
	}
	b, warns, err := c.Build()
	if err != nil || len(warns) != 0 || b.Out != "_site" {
		t.Errorf("Build: %+v %v %v", b, warns, err)
	}
}

// Defaults: cover and open are on when the file does not say.
func TestExportPDFDefaults(t *testing.T) {
	p, err := loadKeys(t, "", "version: 1\n").PDF()
	if err != nil || p.Out != "" || p.Drafts || !p.CoverOn() || !p.OpenOn() {
		t.Fatalf("%+v %v", p, err)
	}
}

// The folder's file wins key by key over the global one.
func TestExportPDFKeysMerge(t *testing.T) {
	c := loadKeys(t, "export:\n  pdf:\n    drafts: true\n    out: g.pdf\n", "export:\n  pdf:\n    out: f.pdf\n")
	p, err := c.PDF()
	if err != nil || p.Out != "f.pdf" || !p.Drafts {
		t.Fatalf("%+v %v", p, err)
	}
}

// A wrong type names the key; an unknown key is a warning of Build-style, not an error.
func TestExportPDFKeyErrors(t *testing.T) {
	for in, want := range map[string]string{
		"export:\n  pdf: yes\n":               "export.pdf must be a mapping",
		"export:\n  pdf:\n    out: 3\n":       "export.pdf.out must be text",
		"export:\n  pdf:\n    drafts: x\n":    "export.pdf.drafts must be true or false",
		"export:\n  pdf:\n    cover: 1\n":     "export.pdf.cover must be true or false",
		"export:\n  pdf:\n    open: \"no\"\n": "export.pdf.open must be true or false",
	} {
		_, err := loadKeys(t, "", in).PDF()
		if err == nil || err.Error() != want {
			t.Errorf("%q: %v, want %q", in, err, want)
		}
	}
}

func TestResolvePDFOut(t *testing.T) {
	root := "/w/My Docs"
	for in, want := range map[string]string{
		"":                       "/w/My Docs/exports/pdf/My Docs.pdf",
		"x":                      "/w/My Docs/x.pdf",
		"./out/a.pdf":            "/w/My Docs/out/a.pdf",
		"/elsewhere/A.PDF":       "/elsewhere/A.PDF",
		"/elsewhere/a":           "/elsewhere/a.pdf",
		"exports/pdf/guide.pdf ": "/w/My Docs/exports/pdf/guide.pdf",
	} {
		got, err := ResolvePDFOut(root, in)
		if err != nil || got != want {
			t.Errorf("%q: %q, %v, want %q", in, got, err, want)
		}
	}
	if PDFName("a/b") != "a-b.pdf" {
		t.Error("PDFName moved here: the same name rules")
	}
	for _, in := range []string{"../x.pdf", "a/../../x.pdf", "."} {
		if got, err := ResolvePDFOut(root, in); err == nil {
			t.Errorf("%q should not leave the folder or be a folder: %q", in, got)
		}
	}
}
