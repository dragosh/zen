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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/zenmark-org/zenmd/engine/config"
	"github.com/zenmark-org/zenmd/internal/cli"
	"github.com/zenmark-org/zenmd/kit/desktop"
)

// EventPDFExport tells the main window how an Export PDF of the form ended; the payload is a pdfExported.
const EventPDFExport = "pdf:export"

// pdfExported is the payload of pdf:export: the file written, or why not.
type pdfExported struct {
	Path  string `json:"path,omitempty"`
	Error string `json:"error,omitempty"`
}

// pdfJob is an Export PDF of the form that waits for its preview window to lay the document out.
type pdfJob struct {
	out  string // the resolved file
	open bool   // open it when done
}

// exportState is what a service remembers of its PDF exports (one field of Zen).
type exportState struct {
	mu   sync.Mutex
	last string // the file the last successful export wrote
	job  *pdfJob
	n    int // the window of the last auto export
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

// next numbers the windows of the exports that run.
func (e *exportState) next() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.n++
	return e.n
}

func (e *exportState) setJob(j *pdfJob) {
	e.mu.Lock()
	e.job = j
	e.mu.Unlock()
}

// takeJob returns the waiting job and clears it, so a job is written once.
func (e *exportState) takeJob() *pdfJob {
	e.mu.Lock()
	defer e.mu.Unlock()
	j := e.job
	e.job = nil
	return j
}

// pdfHooks are the parts of an export that need the GUI; tests replace pdfIO.
type pdfHooks struct {
	window func(ctx context.Context) (application.Window, error)
	panel  func(w application.Window, name, dir string) (string, error)
	save   func(w application.Window, path, title string) (int, error)
	reveal func(path string) error
	open   func(path string) error
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
	open:   func(path string) error { return desktop.OpenFile(path) },
}

// panelStart is where the save panel starts: the folder's export.pdf.out (the
// file name and its folder, which is created for it), else exports/pdf and the folder's name.
func (z *Zen) panelStart(ws *Workspace) (name, dir string) {
	name = cli.PDFName(ws.Name)
	dir = filepath.Join(ws.Root, filepath.FromSlash(config.DefaultPDFDir))
	if o, err := z.s.PDFDefaults(); err == nil && o.Out != "" {
		name, dir = filepath.Base(o.Out), filepath.Dir(o.Out)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		dir = ws.Root
	}
	return name, dir
}

// pdfFile adds .pdf to a chosen name that lacks it.
func pdfFile(path string) string {
	if !strings.EqualFold(filepath.Ext(path), ".pdf") {
		path += ".pdf"
	}
	return path
}

// ExportPDF writes the open folder's print document, as laid out in the window
// that calls it, as a PDF, without the print panel. title is only the job's
// name. Without auto the save panel asks for the file (it starts at the folder's
// export.pdf.out) and returns "" when cancelled; with auto the file is the one
// RunPDFExport resolved, and pdf:export tells the main window how it went.
func (z *Zen) ExportPDF(ctx context.Context, title string, auto bool) (r0 string, err error) {
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
	var job *pdfJob
	if auto {
		if job = z.export.takeJob(); job == nil {
			return "", errors.New("no export is waiting")
		}
	}
	path, err := z.writePDF(ctx, ws, title, job)
	if auto {
		ev := pdfExported{Path: path}
		if err != nil {
			ev.Error = err.Error()
		}
		z.emitExport(ev)
	}
	return path, err
}

// writePDF asks for the file (a job knows it) and saves the PDF there.
func (z *Zen) writePDF(ctx context.Context, ws *Workspace, title string, job *pdfJob) (string, error) {
	w, err := pdfIO.window(ctx)
	if err != nil {
		return "", err
	}
	var path string
	if job != nil {
		path = job.out
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", errors.New("could not make the folder " + filepath.Dir(path))
		}
	} else {
		name, dir := z.panelStart(ws)
		if path, err = pdfIO.panel(w, name, dir); err != nil {
			return "", err
		}
		if path == "" {
			return "", nil
		}
		path = pdfFile(path)
	}
	desktop.LogApp(slog.LevelInfo, "export started", "format", "pdf", "name", filepath.Base(path))
	start := time.Now()
	if _, err := pdfIO.save(w, path, title); err != nil {
		return "", err
	}
	desktop.LogApp(slog.LevelInfo, "export finished", "format", "pdf", "name", filepath.Base(path), "ms", time.Since(start).Milliseconds())
	z.export.set(path)
	if job != nil && job.open {
		if err := pdfIO.open(path); err != nil {
			desktop.LogApp(slog.LevelWarn, "export: could not open the file", "name", filepath.Base(path))
		}
	}
	return path, nil
}

func (z *Zen) emitExport(e pdfExported) {
	if z.events != nil {
		z.events.Emit(EventPDFExport, e)
	}
}

// RunPDFExport is Export PDF of Settings › Export › PDF: it resolves the file
// (a relative path is inside the folder and never leaves it), then opens a
// preview window that lays the document out with the form's options and asks
// ExportPDF(…, true) for the file. pdf:export says how it went.
func (z *Zen) RunPDFExport(o PDFRun) (err error) {
	defer z.logCall("RunPDFExport", &err)
	if err := requireExportPDF(); err != nil {
		return err
	}
	ws := z.s.Workspace()
	if ws == nil {
		return errNoWorkspace
	}
	out, err := config.ResolvePDFOut(ws.Root, o.Out)
	if err != nil {
		return err
	}
	z.export.setJob(&pdfJob{out: out, open: o.Open})
	q := z.previewQuery(o)
	q.Set("auto", "1")
	n := z.export.next()
	if err := printJobs.Open(new(int), z.previewSpec(ws.Name, q, "-export-"+strconv.Itoa(n))); err != nil {
		z.export.setJob(nil)
		return err
	}
	return nil
}

// PDFExportFailed is the preview of an Export PDF telling that it could not
// prepare the document; the form shows message.
func (z *Zen) PDFExportFailed(message string) (err error) {
	defer z.logCall("PDFExportFailed", &err)
	if err := requireExportPDF(); err != nil {
		return err
	}
	z.export.setJob(nil)
	if message = strings.TrimSpace(message); message == "" {
		message = "the document could not be prepared"
	}
	z.emitExport(pdfExported{Error: message})
	return nil
}

// PickPDFFile is Choose… of the form: the save panel for the window that calls
// it, starting at the folder's output file. It writes nothing and returns the
// file, "" when the panel was cancelled.
func (z *Zen) PickPDFFile(ctx context.Context) (r0 string, err error) {
	defer z.logCall("PickPDFFile", &err)
	if err := requireExportPDF(); err != nil {
		return "", err
	}
	ws := z.s.Workspace()
	if ws == nil {
		return "", errNoWorkspace
	}
	w, err := pdfIO.window(ctx)
	if err != nil {
		return "", err
	}
	name, dir := z.panelStart(ws)
	path, err := pdfIO.panel(w, name, dir)
	if err != nil || path == "" {
		return "", err
	}
	return pdfFile(path), nil
}

// printJobs holds the preview windows of the exports that run (one each).
var printJobs = &desktop.PrintWindows{}

// OpenExport opens the file the last export wrote in its default viewer. It takes no path.
func (z *Zen) OpenExport() (err error) {
	defer z.logCall("OpenExport", &err)
	if err := requireExportPDF(); err != nil {
		return err
	}
	p := z.export.get()
	if p == "" {
		return errors.New("nothing has been exported yet")
	}
	return pdfIO.open(p)
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
