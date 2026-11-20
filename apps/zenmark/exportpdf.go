package app

// File › Export › PDF…: the preview window asks for the save panel and the
// PDF job here. The path comes from the panel only (never from the frontend),
// and Show in Finder reveals only the file the last export wrote.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/zenmark-org/zenmd/engine/config"
	"github.com/zenmark-org/zenmd/internal/cli"
	"github.com/zenmark-org/zenmd/kit/desktop"
)

// exportState is what a service remembers of its PDF exports (one field of Zen).
type exportState struct {
	mu   sync.Mutex
	last string // the file the last successful export wrote
}

func (e *exportState) set(p string) {
	e.mu.Lock()
	e.last = p
	e.mu.Unlock()
}

func (e *exportState) get() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.last
}

// pdfHooks are the parts of an export that need the GUI; tests replace pdfIO.
type pdfHooks struct {
	window   func(ctx context.Context) (application.Window, error)
	panel    func(w application.Window, name, dir string) (string, error)
	save     func(w application.Window, path, title string) (int, error)
	reveal   func(path string) error
	settings func() string // the folder of the settings files
}

var pdfIO = pdfHooks{
	window: func(ctx context.Context) (application.Window, error) {
		w, ok := ctx.Value(application.WindowKey).(application.Window)
		if !ok || w == nil {
			return nil, errors.New("no window to export")
		}
		return w, nil
	},
	panel: func(w application.Window, name, dir string) (string, error) {
		return desktop.Dialogs(application.Get()).PickPDFPath(w, name, dir)
	},
	save:   desktop.SavePDF,
	reveal: func(path string) error { return desktop.OpenFile("-R", path) },
	settings: func() string {
		if p, err := config.GlobalPath(); err == nil && p != "" {
			return filepath.Dir(p)
		}
		return ""
	},
}

const lastPDFFolderFile = "export-pdf-folder"

// ExportPDF shows the save panel for the window that calls it and writes the
// open folder's print document there as a PDF, without the print panel. title
// is only the job's name. It returns the file, "" when the panel was cancelled.
func (z *Zen) ExportPDF(ctx context.Context, title string) (string, error) {
	if err := requireFeature("export"); err != nil {
		return "", err
	}
	if err := requireFeature("print"); err != nil {
		return "", err
	}
	ws := z.s.Workspace()
	if ws == nil {
		return "", errNoWorkspace
	}
	if title = strings.TrimSpace(title); title == "" {
		title = Name
	}
	w, err := pdfIO.window(ctx)
	if err != nil {
		return "", err
	}
	path, err := pdfIO.panel(w, cli.PDFName(ws.Name), lastPDFFolder())
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil
	}
	if !strings.EqualFold(filepath.Ext(path), ".pdf") {
		path += ".pdf"
	}
	if _, err := pdfIO.save(w, path, title); err != nil {
		return "", err
	}
	z.export.set(path)
	rememberPDFFolder(filepath.Dir(path))
	return path, nil
}

// RevealExport shows the file the last export wrote in Finder. It takes no path.
func (z *Zen) RevealExport() error {
	if err := requireFeature("export"); err != nil {
		return err
	}
	if err := requireFeature("print"); err != nil {
		return err
	}
	p := z.export.get()
	if p == "" {
		return errors.New("nothing has been exported yet")
	}
	return pdfIO.reveal(p)
}

// lastPDFFolder is the folder of the last PDF export, else Documents.
func lastPDFFolder() string {
	if dir := pdfIO.settings(); dir != "" {
		if b, err := os.ReadFile(filepath.Join(dir, lastPDFFolderFile)); err == nil {
			if p := strings.TrimSpace(string(b)); filepath.IsAbs(p) {
				if fi, err := os.Stat(p); err == nil && fi.IsDir() {
					return p
				}
			}
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents")
}

func rememberPDFFolder(dir string) {
	if s := pdfIO.settings(); s != "" {
		_ = os.MkdirAll(s, 0o755)
		_ = os.WriteFile(filepath.Join(s, lastPDFFolderFile), []byte(dir+"\n"), 0o644)
	}
}
