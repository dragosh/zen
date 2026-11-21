package app

// File › Export › PDF…: the preview window asks for the save panel and the
// PDF job here. The path comes from the panel only (never from the frontend),
// and Show in Finder reveals only the file the last export wrote.

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	window func(ctx context.Context) (application.Window, error)
	panel  func(w application.Window, name, dir string) (string, error)
	save   func(w application.Window, path, title string) (int, error)
	reveal func(path string) error
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
}

// ExportPDF shows the save panel for the window that calls it and writes the
// open folder's print document there as a PDF, without the print panel. title
// is only the job's name. It returns the file, "" when the panel was cancelled.
func (z *Zen) ExportPDF(ctx context.Context, title string) (r0 string, err error) {
	defer z.logCall("ExportPDF", &err)
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
	// The panel starts in exports/pdf of the folder, which is created for it.
	dir := filepath.Join(ws.Root, filepath.FromSlash(config.DefaultPDFDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		dir = ws.Root
	}
	path, err := pdfIO.panel(w, cli.PDFName(ws.Name), dir)
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil
	}
	if !strings.EqualFold(filepath.Ext(path), ".pdf") {
		path += ".pdf"
	}
	desktop.LogApp(slog.LevelInfo, "export started", "format", "pdf", "name", filepath.Base(path))
	start := time.Now()
	if _, err := pdfIO.save(w, path, title); err != nil {
		return "", err
	}
	desktop.LogApp(slog.LevelInfo, "export finished", "format", "pdf", "name", filepath.Base(path), "ms", time.Since(start).Milliseconds())
	z.export.set(path)
	return path, nil
}

// RevealExport shows the file the last export wrote in Finder. It takes no path.
func (z *Zen) RevealExport() (err error) {
	defer z.logCall("RevealExport", &err)
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
