package app

import (
	"slices"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// EventSettings asks the main window to show Settings (Zen > Settings…).
const EventSettings = "menu:settings"

// The View menu's zoom items (ui/accessibility) send these to every window; the one in front steps the
// zoom (Window.SetZoom) and saves it, so the menu, the keys and Settings share one code path.
const (
	EventZoomIn    = "menu:zoom-in"
	EventZoomOut   = "menu:zoom-out"
	EventZoomReset = "menu:zoom-reset"
	// EventZoomChanged is what a window tells Go after it zoomed: the new factor, a number. It greys
	// Zoom In at 200% and Zoom Out at 75%.
	EventZoomChanged = "app:zoom"
)

// menuActions are the callbacks app.go wires into the custom menu items, so
// this file does not depend on Zen internals.
type menuActions struct {
	NewSite, NewPage, OpenFolder, Settings func()
	CloseFolder                            func()
	BuildWebsite, PrintPage, PrintSite     func()
	Find, FindNext, FindPrevious           func()

	// emit sends the event of a custom item that only tells the windows something (View > Zoom In); buildMenu sets it.
	emit func(event string)
	// zoom greys the zoom items at the ends of the list; buildMenu sets it.
	zoom *zoomState
}

// forLabel returns the action of a custom (role-less) item, nil for others.
func (a menuActions) forLabel(label string) func() {
	switch label {
	case labelNewSite:
		return a.NewSite
	case labelExportWebsite:
		return a.BuildWebsite
	case labelPrintPage:
		return a.PrintPage
	case labelExportPDF:
		return a.PrintSite
	case labelFind:
		return a.Find
	case labelFindNext:
		return a.FindNext
	case labelFindPrevious:
		return a.FindPrevious
	case labelNewPage:
		return a.NewPage
	case labelOpenFolder:
		return a.OpenFolder
	case labelCloseFolder:
		return a.CloseFolder
	case labelSettings:
		return a.Settings
	}
	return nil
}

// menuItem is one entry of the plain menu tree that buildMenu turns into
// Wails items and that menu_test.go checks.
type menuItem struct {
	Label, Accel string // Accel in Wails syntax, "" for none
	Role         string // Wails role name, "" for a custom item
	Event        string // for a custom item that only tells the main window something: the event it sends
	Children     []menuItem
	Separator    bool
}

const (
	labelNewSite       = "New Site from Template…"
	labelPrintPage     = "Print Page…"
	labelExport        = "Export"
	labelExportWebsite = "Website…"
	labelExportPDF     = "PDF…"
	labelFind          = "Find…"
	labelFindNext      = "Find Next"
	labelFindPrevious  = "Find Previous"
	labelNewPage       = "New Page…"
	labelOpenFolder    = "Open Folder…"
	labelCloseFolder   = "Close Folder"
	labelSettings      = "Settings…"
	labelAbout         = "About " + Name
	labelActualSize    = "Actual Size"
	labelZoomIn        = "Zoom In"
	labelZoomOut       = "Zoom Out"
)

var sep = menuItem{Separator: true}

// menuSpec is the whole menu bar of a full build: Zen, File, Edit, View (zoom only), Window,
// and in dev builds Develop. Production builds have no Reload, DevTools, Services or Speech.
func menuSpec(dev bool) []menuItem { return menuSpecFor(dev, allFeatures) }

// menuSpecFor is menuSpec for a build with these features (kit/shell-features).
// An item of a feature the build leaves out is not there at all, not greyed, and
// no separator is left behind. The bare shell keeps the App menu, File with Open
// Folder and Close Folder, Edit without Find, and Window.
func menuSpecFor(dev bool, features []string) []menuItem {
	has := func(id string) bool { return slices.Contains(features, id) }
	// when keeps the items only if cond holds.
	when := func(cond bool, items ...menuItem) []menuItem {
		if cond {
			return items
		}
		return nil
	}
	cat := func(lists ...[]menuItem) []menuItem {
		var out []menuItem
		for _, l := range lists {
			out = append(out, l...)
		}
		return out
	}
	file := cat(
		when(has("newsite"), menuItem{Label: labelNewSite, Accel: "CmdOrCtrl+Shift+N"}, sep),
		when(has("editor"), menuItem{Label: labelNewPage, Accel: "CmdOrCtrl+N"}),
		[]menuItem{
			{Label: labelOpenFolder, Accel: "CmdOrCtrl+O"},
			{Label: labelCloseFolder, Accel: "CmdOrCtrl+Shift+W"},
			sep,
		},
		when(has("print"), menuItem{Label: labelPrintPage, Accel: "CmdOrCtrl+P"}),
		when(has("export"), menuItem{Label: labelExport, Children: cat(
			[]menuItem{{Label: labelExportWebsite, Accel: "CmdOrCtrl+Shift+B"}},
			when(has("print"), menuItem{Label: labelExportPDF, Accel: "CmdOrCtrl+Shift+P"}), // the PDF is the print document
		)}),
	)
	edit := cat(
		[]menuItem{
			{Label: "Undo", Accel: "CmdOrCtrl+Z", Role: "Undo"},
			{Label: "Redo", Accel: "CmdOrCtrl+Shift+Z", Role: "Redo"},
			sep,
			{Label: "Cut", Accel: "CmdOrCtrl+X", Role: "Cut"},
			{Label: "Copy", Accel: "CmdOrCtrl+C", Role: "Copy"},
			{Label: "Paste", Accel: "CmdOrCtrl+V", Role: "Paste"},
			{Label: "Select All", Accel: "CmdOrCtrl+A", Role: "SelectAll"},
			sep,
		},
		when(has("find"), menuItem{Label: "Find", Children: []menuItem{
			{Label: labelFind, Accel: "CmdOrCtrl+F"},
			{Label: labelFindNext, Accel: "CmdOrCtrl+G"},
			{Label: labelFindPrevious, Accel: "CmdOrCtrl+Shift+G"},
		}}),
	)
	// View: the zoom of the whole window, in every build (Settings › Accessibility has the same steps).
	view := []menuItem{
		{Label: labelActualSize, Accel: "CmdOrCtrl+0", Event: EventZoomReset},
		{Label: labelZoomIn, Accel: "CmdOrCtrl+Plus", Event: EventZoomIn},
		{Label: labelZoomOut, Accel: "CmdOrCtrl+-", Event: EventZoomOut},
	}
	bar := []menuItem{
		{Label: Name, Children: []menuItem{
			{Label: labelAbout, Role: "About"}, // the standard macOS panel; the build is the Inspector's Build tab
			{Label: labelSettings, Accel: "CmdOrCtrl+,"},
			sep,
			{Label: "Hide " + Name, Accel: "CmdOrCtrl+H", Role: "Hide"},
			{Label: "Hide Others", Accel: "CmdOrCtrl+OptionOrAlt+H", Role: "HideOthers"},
			{Label: "Show All", Role: "UnHide"},
			sep,
			{Label: "Quit " + Name, Accel: "CmdOrCtrl+Q", Role: "Quit"},
		}},
		{Label: "File", Children: file},
		{Label: "Edit", Children: edit},
		{Label: "View", Children: view},
	}
	if dev && has("developer") {
		bar = append(bar, menuItem{Label: "Develop", Children: []menuItem{
			{Label: "Reload", Accel: "CmdOrCtrl+R", Role: "Reload"},
			{Label: "Toggle DevTools", Accel: "CmdOrCtrl+OptionOrAlt+I", Role: "OpenDevTools"},
		}})
	}
	bar = append(bar, menuItem{Label: "Window", Children: []menuItem{
		{Label: "Minimise", Accel: "CmdOrCtrl+M", Role: "Minimise"},
		{Label: "Zoom", Role: "Zoom"},
	}})
	for i := range bar {
		bar[i].Children = tidySeparators(bar[i].Children)
	}
	return bar
}

// tidySeparators drops leading, trailing and doubled separators, so a menu whose
// items were left out has no gap.
func tidySeparators(items []menuItem) []menuItem {
	var out []menuItem
	for _, it := range items {
		if it.Separator && (len(out) == 0 || out[len(out)-1].Separator) {
			continue
		}
		out = append(out, it)
	}
	if n := len(out); n > 0 && out[n-1].Separator {
		out = out[:n-1]
	}
	return out
}

var roleByName = map[string]application.Role{
	"About": application.About, "Hide": application.Hide, "HideOthers": application.HideOthers,
	"UnHide": application.UnHide, "Quit": application.Quit,
	"Undo": application.Undo, "Redo": application.Redo, "Cut": application.Cut, "Copy": application.Copy,
	"Paste": application.Paste, "SelectAll": application.SelectAll,
	"Minimise": application.Minimise, "Zoom": application.Zoom,
	"Reload": application.Reload, "OpenDevTools": application.OpenDevTools,
}

// buildMenu turns menuSpec into the native menu. Every item is built one by
// one from its role (never AddRole(AppMenu/EditMenu/...)), so no extra items
// come along. Labels and accelerators are the spec's, whatever the role's
// default is (the Wails app name is the lowercase binary name).
func buildMenu(a *application.App, act menuActions, st *menuState, dev bool) *application.Menu {
	menu := a.Menu.New()
	act.emit = func(event string) { a.Event.Emit(event) }
	act.zoom = newZoomState()
	a.Event.On(EventZoomChanged, func(e *application.CustomEvent) {
		if z, ok := zoomFromEvent(e.Data); ok {
			act.zoom.set(z)
		}
	})
	for _, top := range menuSpecFor(dev, assemblyInfo.Features) {
		fillMenu(menu.AddSubmenu(top.Label), top.Children, act, st)
	}
	return menu
}

// fillMenu adds items to sub; an item with children becomes a submenu (Edit > Find, File > Export).
func fillMenu(sub *application.Menu, items []menuItem, act menuActions, st *menuState) {
	for _, it := range items {
		if it.Separator {
			sub.AddSeparator()
			continue
		}
		if len(it.Children) > 0 {
			inner := sub.AddSubmenu(it.Label)
			if st != nil && folderOnly(it.Label) { // the submenu entry itself (File > Export)
				if mi := sub.FindByLabel(it.Label); mi != nil {
					st.add(func(on bool) { mi.SetEnabled(on) })
				}
			}
			fillMenu(inner, it.Children, act, st)
			continue
		}
		var item *application.MenuItem
		if it.Role != "" {
			item = application.NewRole(roleByName[it.Role])
			if item == nil {
				continue // e.g. DevTools in a production build of Wails
			}
			sub.Append(application.NewMenuFromItems(item))
			item.SetLabel(it.Label)
		} else {
			item = sub.Add(it.Label)
			if f := act.forLabel(it.Label); f != nil {
				item.OnClick(func(*application.Context) { f() })
			} else if it.Event != "" && act.emit != nil {
				event := it.Event
				item.OnClick(func(*application.Context) { act.emit(event) })
			}
			if act.zoom != nil {
				switch it.Label {
				case labelZoomIn:
					act.zoom.add(func(on bool) { item.SetEnabled(on) }, nil)
				case labelZoomOut:
					act.zoom.add(nil, func(on bool) { item.SetEnabled(on) })
				}
			}
			if st != nil && folderOnly(it.Label) {
				st.add(func(on bool) { item.SetEnabled(on) })
			}
		}
		if it.Accel != "" {
			item.SetAccelerator(it.Accel)
		}
	}
}

// shower is the part of the main window the menu actions use.
type shower interface {
	Show() application.Window
	Focus()
}

// settingsAction shows the main window and tells it to open Settings.
func settingsAction(win shower, sink EventSink) func() {
	return func() {
		win.Show()
		win.Focus()
		sink.Emit(EventSettings, nil)
	}
}

// newPageAction shows the main window and tells it to ask for the new page.
func newPageAction(win shower, sink EventSink) func() { return showAndEmit(win, sink, EventNewPage) }

// emitAction sends event to every window and shows nothing: the window that
// has the focus handles it (Edit > Find).
func emitAction(sink EventSink, event string) func() {
	return func() { sink.Emit(event, nil) }
}

// showAndEmit shows and focuses the main window, then sends it event (the
// menu items whose dialog lives in the main window).
func showAndEmit(win shower, sink EventSink, event string) func() {
	return func() {
		win.Show()
		win.Focus()
		sink.Emit(event, nil)
	}
}

// zoomMin and zoomMax are the ends of the zoom list (kit/ui shell/a11y.ts ZOOM_STEPS: 75% to 200%).
const (
	zoomMin = 0.75
	zoomMax = 2.0
)

// zoomEnabled says whether Zoom In and Zoom Out do anything at zoom z: the ends do nothing, and the items are greyed there.
func zoomEnabled(z float64) (in, out bool) {
	const eps = 1e-6
	return z < zoomMax-eps, z > zoomMin+eps
}

// zoomFromEvent reads what a window sent with EventZoomChanged: a JSON number in the zoom range.
func zoomFromEvent(data any) (float64, bool) {
	z, ok := data.(float64)
	if !ok || z < zoomMin-1e-6 || z > zoomMax+1e-6 {
		return 0, false
	}
	return z, true
}

// zoomState keeps Zoom In and Zoom Out enabled or greyed to match the zoom. Like menuState it holds setters, not
// Wails items, so tests drive it without an application. A nil zoomState does nothing.
type zoomState struct {
	mu      sync.Mutex
	z       float64
	in, out []func(bool)
}

func newZoomState() *zoomState { return &zoomState{z: 1} }

// add registers the setters of Zoom In and/or Zoom Out (nil for the one it is not) and gives them the current state.
func (m *zoomState) add(in, out func(bool)) {
	m.mu.Lock()
	if in != nil {
		m.in = append(m.in, in)
	}
	if out != nil {
		m.out = append(m.out, out)
	}
	z := m.z
	m.mu.Unlock()
	canIn, canOut := zoomEnabled(z)
	if in != nil {
		in(canIn)
	}
	if out != nil {
		out(canOut)
	}
}

// set records the zoom and updates the items.
func (m *zoomState) set(z float64) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.z = z
	in, out := append([]func(bool){}, m.in...), append([]func(bool){}, m.out...)
	m.mu.Unlock()
	canIn, canOut := zoomEnabled(z)
	for _, f := range in {
		f(canIn)
	}
	for _, f := range out {
		f(canOut)
	}
}
