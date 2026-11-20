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
// the one that is open.
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
	current := z.s.Current()
	u := "/?window=" + printWindowName
	if current != "" {
		u += "&page=" + url.QueryEscape(current)
	}
	name := ws.Name
	if name == "" {
		name = Name
	}
	return printWins.Open(z, desktop.WindowSpec{
		Name: printWindowName, Title: name + " — Print", URL: u,
		Width: printWindowWidth, Height: printWindowHeight, X: 60, Y: 60,
	})
}

// printWins tracks the open preview window of each running service.
var printWins = &desktop.PrintWindows{}
