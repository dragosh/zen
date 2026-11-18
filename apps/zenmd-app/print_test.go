package app

import (
	"net/url"
	"strings"
	"testing"

	"github.com/zenmark-org/zenmd/kit/desktop"
)

// fakePrintWindow is a preview window that is just counted.
type fakePrintWindow struct {
	focused int
	onClose func()
}

func (w *fakePrintWindow) Focus()               { w.focused++ }
func (w *fakePrintWindow) Close()               { w.onClose() }
func (w *fakePrintWindow) Position() (int, int) { return 0, 0 }
func (w *fakePrintWindow) Size() (int, int)     { return 0, 0 }

func TestPrintWindowAndMenuActions(t *testing.T) {
	var specs []desktop.WindowSpec
	var wins []*fakePrintWindow
	old := desktop.NewPrintWindow
	desktop.NewPrintWindow = func(s desktop.WindowSpec, onClose func(), _ func()) desktop.Window {
		w := &fakePrintWindow{onClose: onClose}
		specs, wins = append(specs, s), append(wins, w)
		return w
	}
	t.Cleanup(func() { desktop.NewPrintWindow = old })

	files := map[string]string{"README.md": "# Home\n", "docs/a b.md": "# A\n"}
	z, ev, _, _ := newTestZen(t, tree(t, files), "", "")
	if _, err := z.Page("docs/a b.md"); err != nil { // the page the main window shows
		t.Fatal(err)
	}

	if err := z.PrintSite(); err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || specs[0].Name != "print" || !strings.HasSuffix(specs[0].Title, " — Print") ||
		specs[0].URL != "/?window=print&page="+url.QueryEscape("docs/a b.md") {
		t.Fatalf("spec %+v", specs)
	}
	if !strings.HasPrefix(specs[0].Title, z.Startup().Workspace.Name) {
		t.Errorf("title %q does not start with the folder name %q", specs[0].Title, z.Startup().Workspace.Name)
	}
	// Opened twice: the first is focused, no second window.
	if err := z.PrintSite(); err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || wins[0].focused != 1 {
		t.Errorf("windows %d, focused %d", len(specs), wins[0].focused)
	}
	// Closed: the next one opens again.
	wins[0].onClose()
	if err := z.PrintSite(); err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 {
		t.Errorf("no new window after closing: %d", len(specs))
	}

	// Print Page asks the main window.
	ev.reset()
	if err := z.PrintPage(); err != nil {
		t.Fatal(err)
	}
	if !hasEvent(ev, EventPrintPage) || EventPrintPage != "menu:print-page" {
		t.Errorf("events %v", ev.all())
	}
}
