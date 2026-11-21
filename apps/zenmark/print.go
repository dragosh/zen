package app

import (
	"context"
	"log/slog"
	"net/url"

	"github.com/zenmark-org/zenmd/kit/desktop"
)

// EventPrintPage asks the main window to print the page it shows (File ›
// Print Page…, ⌘P). It is delivered to every window; each one prints only
// when it has the focus.
const EventPrintPage = "menu:print-page"

const (
	printWindowName   = "print"
	printWindowWidth  = 980
	printWindowHeight = 800
)

// PrintPage asks the main window to print the page it shows, through the
// macOS print panel.
func (z *Zen) PrintPage() (err error) {
	defer z.logCall("PrintPage", &err)
	if err := requireFeature("print"); err != nil {
		return err
	}
	if z.s.Workspace() == nil {
		return errNoWorkspace
	}
	desktop.LogApp(slog.LevelInfo, "print", "what", "page", "page", z.s.Current())
	if z.events != nil {
		z.events.Emit(EventPrintPage, nil)
	}
	return nil
}

// PrintWindow opens the print panel for the window that calls it (Print
// Page, Print Site), once the page has prepared itself for paper.
// window.print() does nothing in the macOS web view, so the frontend calls
// this instead. title becomes the job name (and the suggested PDF file name).
func (z *Zen) PrintWindow(ctx context.Context, title string) (err error) {
	defer z.logCall("PrintWindow", &err)
	if err := requireFeature("print"); err != nil {
		return err
	}
	desktop.LogApp(slog.LevelInfo, "print panel opened")
	return desktop.PrintFromContext(ctx, title, Name)
}

// PrintSite opens the Print Preview window for the whole folder, or focuses
// the one that is open. It starts from the folder's PDF defaults (the window asks).
func (z *Zen) PrintSite() (err error) {
	defer z.logCall("PrintSite", &err)
	if err := requireFeature("print"); err != nil {
		return err
	}
	ws := z.s.Workspace()
	if ws == nil {
		return errNoWorkspace
	}
	desktop.LogApp(slog.LevelInfo, "print", "what", "site", "pages", len(z.s.Workspace().Tree))
	return printWins.Open(z, z.previewSpec(ws.Name, nil, ""))
}

// previewSpec is the preview window of the folder; opts is nil for the folder's
// defaults, else the options go in the URL. extra is the window name's suffix.
func (z *Zen) previewSpec(folder string, opts url.Values, suffix string) desktop.WindowSpec {
	current := z.s.Current()
	u := "/?window=" + printWindowName
	if opts != nil {
		u += "&" + opts.Encode()
	}
	if current != "" {
		u += "&page=" + url.QueryEscape(current)
	}
	name := folder
	if name == "" {
		name = Name
	}
	return desktop.WindowSpec{
		Name: printWindowName + suffix, Title: name + " — Print", URL: u,
		Width: printWindowWidth, Height: printWindowHeight, X: 60, Y: 60,
	}
}

// previewQuery is how the options of a run reach the preview window: drafts,
// cover and pages (a section only when a page is open).
func (z *Zen) previewQuery(o PDFRun) url.Values {
	b := func(v bool) string {
		if v {
			return "1"
		}
		return "0"
	}
	pages := "all"
	if o.Section && z.s.Current() != "" {
		pages = "section"
	}
	return url.Values{"drafts": {b(o.Drafts)}, "cover": {b(o.Cover)}, "pages": {pages}}
}

// PreviewPDF opens the Print Preview window with the form's options set
// (Settings › Export › PDF › Preview…), or focuses the one that is open.
func (z *Zen) PreviewPDF(o PDFRun) (err error) {
	defer z.logCall("PreviewPDF", &err)
	if err := requireExportPDF(); err != nil {
		return err
	}
	ws := z.s.Workspace()
	if ws == nil {
		return errNoWorkspace
	}
	return printWins.Open(z, z.previewSpec(ws.Name, z.previewQuery(o), ""))
}

// printWins tracks the open preview window of each running service.
var printWins = &desktop.PrintWindows{}
