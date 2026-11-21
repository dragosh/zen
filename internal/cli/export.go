package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/zenmark-org/zenmd/engine/config"
)

// ExportPDF describes `zenmd export pdf`: the whole folder as one PDF file.
type ExportPDF struct {
	Dir string // absolute folder
	Out string // absolute output file; the default is DefaultPDFOut(Dir)

	Drafts  bool   // include pages with draft: true (export.pdf.drafts, --drafts)
	Section string // only this page and the pages under it ("" for all)
	NoCover bool   // leave out the cover and the contents (export.pdf.cover: false, --no-cover)
	// Open: the exporter opens the PDF with the system once it is written (export.pdf.open, --open). It is off on
	// the command line unless the config says so; the app's form starts on.
	Open bool
	// MakeDir: the output folder is made when missing (the default, or export.pdf.out); with -o it is the user's to make.
	MakeDir bool
}

// PDFExporter writes the PDF and, when e.Open, opens it. It prints "Wrote <path> (<n> pages)" to stdout and
// one line per block that was not ready to stderr, and returns an error when
// no file was written. Return UsageError for a wrong request (exit 2).
type PDFExporter func(ExportPDF, io.Writer, io.Writer) error

// PDFName is the default file name of the PDF of a folder called name (config.PDFName).
func PDFName(name string) string { return config.PDFName(name) }

// DefaultPDFOut is where the PDF of dir goes without -o: exports/pdf inside
// it, named after it. The folder is created by whoever writes the file.
func DefaultPDFOut(dir string) string {
	return filepath.Join(dir, filepath.FromSlash(config.DefaultPDFDir), PDFName(filepath.Base(dir)))
}

// needBuilt checks that the build has the features (kit/shell-features): an
// app made without export has no `zenmd export site`, and one without export or
// print has no `zenmd export pdf`. It prints the refusal and returns exit code 2.
// A build that does not say what it is made of has everything.
func needBuilt(stderr io.Writer, ids ...string) (code int, ok bool) {
	if buildInfo == nil {
		return 0, true
	}
	have := buildInfo().Features
	for _, id := range ids {
		if !slices.Contains(have, id) {
			fmt.Fprintf(stderr, "zenmd: this app is built without %s\n", id)
			return 2, false
		}
	}
	return 0, true
}

// runExport parses the arguments after `zenmd export`: the target is site or pdf.
func runExport(args []string, cwd string, stdout, stderr io.Writer, build Builder, pdf PDFExporter) int {
	if len(args) == 0 {
		return fail(stderr, usageError("zenmd export what? try: zenmd export site, zenmd export pdf"))
	}
	switch args[0] {
	case "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "site":
		if code, ok := needBuilt(stderr, "export"); !ok {
			return code
		}
		return runBuild(args[1:], cwd, stdout, stderr, build)
	case "pdf":
		if code, ok := needBuilt(stderr, "export", "print"); !ok {
			return code
		}
		return runExportPDF(args[1:], cwd, stdout, stderr, pdf)
	}
	return fail(stderr, usageError("unknown export target "+args[0]+"; try: zenmd export site, zenmd export pdf"))
}

// runExportPDF parses the arguments after `zenmd export pdf` and calls the exporter.
func runExportPDF(args []string, cwd string, stdout, stderr io.Writer, pdf PDFExporter) int {
	f, err := parseOptions(args, pdfOptions)
	if err != nil {
		return fail(stderr, err)
	}
	if f.help {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if f.on[flagDryRun] && !f.on[flagSaveDefaults] {
		return fail(stderr, usageError(flagDryRun+" goes with "+flagSaveDefaults))
	}
	hasOut, out := f.on["--out"], f.val["--out"]
	if len(f.paths) > 1 {
		return fail(stderr, usageError("more than one path given"))
	}

	e := ExportPDF{Dir: cwd}
	shown := "the current folder"
	if len(f.paths) == 1 {
		shown = f.paths[0]
		e.Dir = f.paths[0]
		if !filepath.IsAbs(e.Dir) {
			e.Dir = filepath.Join(cwd, e.Dir)
		}
	}
	e.Dir = filepath.Clean(e.Dir)
	switch fi, err := os.Stat(e.Dir); {
	case err != nil:
		return fail(stderr, errors.New(shown+": no such folder"))
	case !fi.IsDir():
		return fail(stderr, errors.New(shown+": export takes a folder, not a file"))
	}

	// A flag wins over the folder's export.pdf, which wins over config.yml, then the built-in default.
	loaded := loadConfig(e.Dir)
	pc, err := loaded.PDF()
	if err != nil {
		return fail(stderr, err)
	}
	pick := func(flag string, config bool) bool {
		switch {
		case f.on[flag]:
			return true
		case f.off[flag]:
			return false
		}
		return config
	}
	e.Drafts = pick("--drafts", pc.Drafts)
	e.NoCover = !pick("--cover", pc.CoverOn())
	e.Open = pick("--open", pc.Open != nil && *pc.Open)
	e.Section = f.val["--section"]
	e.MakeDir = !hasOut
	e.Out = DefaultPDFOut(e.Dir)
	if !hasOut && pc.Out != "" {
		o, err := config.ResolvePDFOut(e.Dir, pc.Out)
		if err != nil {
			return fail(stderr, errors.New("export.pdf.out: "+err.Error()))
		}
		e.Out = o
	}
	if hasOut {
		e.Out = out
		if !filepath.IsAbs(e.Out) {
			e.Out = filepath.Join(cwd, e.Out)
		}
		e.Out = filepath.Clean(e.Out)
		if fi, err := os.Stat(e.Out); err == nil && fi.IsDir() {
			return fail(stderr, errors.New(out+" is a folder; give the PDF a file name"))
		}
		if !strings.EqualFold(filepath.Ext(e.Out), ".pdf") {
			e.Out += ".pdf"
		}
	}
	// The default exports/pdf is made by the exporter; a chosen folder must exist.
	if fi, err := os.Stat(filepath.Dir(e.Out)); hasOut && (err != nil || !fi.IsDir()) {
		return fail(stderr, errors.New("the folder "+filepath.Dir(e.Out)+" does not exist"))
	}
	if pdf == nil {
		return fail(stderr, errors.New("this zenmd cannot export PDFs"))
	}
	printNotices(stderr, loaded)
	if f.on[flagSaveDefaults] {
		keys := []config.ExportKey{}
		if hasOut {
			v, err := config.PDFOutValue(e.Dir, e.Out)
			if err != nil {
				return fail(stderr, err)
			}
			if filepath.IsAbs(v) { // the config refuses it when it reads it back
				return fail(stderr, errors.New("export.pdf.out must be a path inside the folder, such as exports/pdf/guide.pdf; "+e.Out+" is outside it"))
			}
			keys = append(keys, config.StrKey("pdf.out", v))
		}
		for _, k := range []struct{ flag, key string }{{"--drafts", "pdf.drafts"}, {"--cover", "pdf.cover"}, {"--open", "pdf.open"}} {
			if f.given(k.flag) {
				keys = append(keys, config.BoolKey(k.key, f.on[k.flag]))
			}
		}
		if done, err := saveDefaults(stdout, e.Dir, keys, f.on[flagDryRun]); err != nil {
			return fail(stderr, err)
		} else if done {
			return 0
		}
	}
	if err := pdf(e, stdout, stderr); err != nil {
		return fail(stderr, err)
	}
	return 0
}
