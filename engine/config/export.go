package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// The website export's folder settings are under the export: key of
// .zenmd.yml and the global config.yml. Their old key was build:; it is still
// read, and these notices (Problems, the log) say how to move on.
const (
	legacyKeyNotice = "build: in %s is now export:; rename the key"
	bothKeysNotice  = "build: in %s is ignored because export: is set"
)

// legacyBuild handles a build: key found in the file at path: with no export:
// key it is read as export: and noted; with both, it is ignored and noted.
func (c *Config) legacyBuild(path string, doc map[string]any, v any) {
	name := filepath.Base(path)
	if _, both := doc["export"]; both {
		c.notices = append(c.notices, fmt.Sprintf(bothKeysNotice, name))
		return
	}
	c.notices = append(c.notices, fmt.Sprintf(legacyKeyNotice, name))
	c.mergeBuild(v)
}

// Where an export goes when nothing says otherwise, relative to the folder.
// Both are kept out of the pages and the watcher by their .zen-build marker or
// by never holding markdown (see workspace.Files).
const (
	// ExportsDir is the folder at the top of a workspace that holds the
	// exports; it is never read as pages or watched.
	ExportsDir       = "exports"
	DefaultExportOut = "exports/html" // the website
	DefaultPDFDir    = "exports/pdf"  // the PDF
)

// PDF is the export.pdf mapping: what the PDF export does when nothing says
// otherwise (the Settings form, File › Export › PDF…, the preview and `zenmd
// export pdf`). Zero values mean "not set".
type PDF struct {
	Out    string // output file; relative is inside the folder
	Drafts bool   // include pages with draft: true
	Cover  *bool  // cover sheet and contents (nil: on)
	Open   *bool  // open the file when done (nil: on)
}

// CoverOn reports whether the cover and contents are written (the default).
func (p PDF) CoverOn() bool { return p.Cover == nil || *p.Cover }

// OpenOn reports whether the file opens when done (the default).
func (p PDF) OpenOn() bool { return p.Open == nil || *p.Open }

// PDF returns the merged export.pdf mapping; a value of the wrong type is an
// error naming the key, and unknown keys are ignored.
func (c *Config) PDF() (PDF, error) {
	var p PDF
	if c == nil {
		return p, nil
	}
	if c.buildErr != nil {
		return p, c.buildErr
	}
	v, ok := c.build["pdf"]
	if !ok || v == nil {
		return p, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return p, errors.New("export.pdf must be a mapping")
	}
	flag := func(k string) (*bool, error) {
		x, ok := m[k]
		if !ok {
			return nil, nil
		}
		b, ok := x.(bool)
		if !ok {
			return nil, fmt.Errorf("export.pdf.%s must be true or false", k)
		}
		return &b, nil
	}
	if x, ok := m["out"]; ok {
		s, ok := x.(string)
		if !ok {
			return p, errors.New("export.pdf.out must be text")
		}
		// A settings file (which may come with a folder someone else wrote) never aims the PDF outside the
		// folder: no absolute path, no "..". A place the user picks (the form's Choose…, -o) is not read here.
		if c := filepath.Clean(filepath.FromSlash(s)); s != "" && (filepath.IsAbs(c) || strings.HasPrefix(s, "~") || c == ".." || strings.HasPrefix(c, ".."+string(filepath.Separator))) {
			return p, errors.New("export.pdf.out must be a path inside the folder, such as exports/pdf/guide.pdf")
		}
		p.Out = s
	}
	d, err := flag("drafts")
	if err != nil {
		return p, err
	}
	p.Drafts = d != nil && *d
	if p.Cover, err = flag("cover"); err != nil {
		return p, err
	}
	if p.Open, err = flag("open"); err != nil {
		return p, err
	}
	return p, nil
}

// PDFName is the default file name of the PDF of a folder called name: the name
// with path separators and control characters replaced, never empty, never a
// hidden file, plus ".pdf".
func PDFName(name string) string {
	name = strings.Trim(name, `/\`)
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || unicode.IsControl(r) {
			return '-'
		}
		return r
	}, name)
	name = strings.Trim(strings.TrimSpace(name), ". ")
	if name == "" {
		name = "export"
	}
	return name + ".pdf"
}

// ResolvePDFOut is the file the PDF of the folder at root goes to: "" is
// exports/pdf/<folder name>.pdf, a relative path is inside the folder (and
// never leaves it), an absolute one is the user's own choice. ".pdf" is added
// when the name lacks it. The result is absolute and clean.
func ResolvePDFOut(root, out string) (string, error) {
	out = strings.TrimSpace(out)
	if out == "" {
		return filepath.Join(root, filepath.FromSlash(DefaultPDFDir), PDFName(filepath.Base(root))), nil
	}
	path := filepath.FromSlash(out)
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
		if rel, err := filepath.Rel(root, path); err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			if rel == "." {
				return "", errors.New(out + " is the folder; give the PDF a file name")
			}
			return "", errors.New(out + " is outside the folder")
		}
	}
	path = filepath.Clean(path)
	if !strings.EqualFold(filepath.Ext(path), ".pdf") {
		path += ".pdf"
	}
	return path, nil
}
