package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// ExportPDF describes `zenmd export pdf`: the whole folder as one PDF file.
type ExportPDF struct {
	Dir string // absolute folder
	Out string // absolute output file, "" = <Dir>/<name>.pdf
}

// PDFExporter writes the PDF. It prints "Wrote <path> (<n> pages)" to stdout and
// one line per block that was not ready to stderr, and returns an error when
// no file was written. Return UsageError for a wrong request (exit 2).
type PDFExporter func(ExportPDF, io.Writer, io.Writer) error

// PDFName is the default file name of the PDF of a folder called name: the name with
// path separators and control characters replaced, never empty, never a hidden file, plus ".pdf".
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

// DefaultPDFOut is where the PDF of dir goes without -o: inside it, named after it.
func DefaultPDFOut(dir string) string { return filepath.Join(dir, PDFName(filepath.Base(dir))) }

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
		return runBuild(args[1:], cwd, stdout, stderr, build)
	case "pdf":
		return runExportPDF(args[1:], cwd, stdout, stderr, pdf)
	}
	return fail(stderr, usageError("unknown export target "+args[0]+"; try: zenmd export site, zenmd export pdf"))
}

// runExportPDF parses the arguments after `zenmd export pdf` and calls the exporter.
func runExportPDF(args []string, cwd string, stdout, stderr io.Writer, pdf PDFExporter) int {
	var paths []string
	out, hasOut, flags := "", false, true
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case flags && arg == "--":
			flags = false
		case flags && (arg == "-h" || arg == "--help"):
			fmt.Fprint(stdout, usage)
			return 0
		case flags && (arg == "-o" || arg == "--out"):
			if i+1 >= len(args) {
				return fail(stderr, usageError(arg+" needs a file name"))
			}
			i++
			out, hasOut = args[i], true
		case flags && (strings.HasPrefix(arg, "-o=") || strings.HasPrefix(arg, "--out=")):
			_, out, _ = strings.Cut(arg, "=")
			hasOut = true
		case flags && strings.HasPrefix(arg, "-") && arg != "-":
			return fail(stderr, usageError("unknown option "+arg))
		default:
			paths = append(paths, arg)
		}
	}
	if hasOut && out == "" {
		return fail(stderr, usageError("-o needs a file name"))
	}
	if len(paths) > 1 {
		return fail(stderr, usageError("more than one path given"))
	}

	e := ExportPDF{Dir: cwd}
	shown := "the current folder"
	if len(paths) == 1 {
		shown = paths[0]
		e.Dir = paths[0]
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

	e.Out = DefaultPDFOut(e.Dir)
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
	if fi, err := os.Stat(filepath.Dir(e.Out)); err != nil || !fi.IsDir() {
		return fail(stderr, errors.New("the folder "+filepath.Dir(e.Out)+" does not exist"))
	}
	if pdf == nil {
		return fail(stderr, errors.New("this zenmd cannot export PDFs"))
	}
	if err := pdf(e, stdout, stderr); err != nil {
		return fail(stderr, err)
	}
	return 0
}
