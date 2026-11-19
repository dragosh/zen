package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ErrPDFUnsupported is what SavePDF returns where macOS printing is not available.
var ErrPDFUnsupported = errors.New("PDF export needs macOS")

// SavePDF writes w's web view to path as a PDF through macOS printing as a
// save job: no print panel and no progress panel, portrait, the system's
// default paper, half-inch margins, fitted to the page width (the setup of
// NativePrint). The file is written next to path and moved into place, so an
// existing file is replaced only by a complete one. It returns the page count.
func SavePDF(w application.Window, path, title string) (pages int, err error) {
	if err := checkPDFTarget(path); err != nil {
		return 0, err
	}
	return nativeSavePDF(w, path, title)
}

// checkPDFTarget refuses a path SavePDF cannot write: empty, a folder, or in a folder that does not exist.
func checkPDFTarget(path string) error {
	if path == "" {
		return errors.New("no file to write")
	}
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		return errors.New(path + " is a folder, not a file")
	}
	dir := filepath.Dir(path)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return errors.New("the folder " + dir + " does not exist")
	}
	return nil
}

// HasWindowServer reports whether there is a logged-in macOS session that can show windows
// (false over SSH without one, and off macOS). SavePDF and the headless export need it.
func HasWindowServer() bool { return hasWindowServer() }

var pdfPageRE = regexp.MustCompile(`/Type\s*/Page(?:[^A-Za-z]|$)`)

// countPDFPages counts the page objects of a PDF that is not compressed into object streams.
func countPDFPages(data []byte) int { return len(pdfPageRE.FindAll(data, -1)) }

// PickPDFPath shows the save panel (a sheet on w when it is given) with the file name name in
// directory dir, and returns the chosen path, "" on cancel.
func (d *DialogSet) PickPDFPath(w application.Window, name, dir string) (string, error) {
	dlg := d.app.Dialog.SaveFile().
		SetFilename(name).
		SetDirectory(dir).
		AddFilter("PDF", "*.pdf").
		CanCreateDirectories(true)
	if w != nil {
		dlg.AttachToWindow(w)
	}
	return dlg.PromptForSingleSelection()
}
