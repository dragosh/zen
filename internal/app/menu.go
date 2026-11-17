package app

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// EventSettings asks the main window to show Settings (Zen > Settings…).
const EventSettings = "menu:settings"

// menuActions are the callbacks app.go wires into the custom menu items, so
// this file does not depend on Zen internals.
type menuActions struct {
	NewPage, OpenFolder, Settings func()
}

// forLabel returns the action of a custom (role-less) item, nil for others.
func (a menuActions) forLabel(label string) func() {
	switch label {
	case labelNewPage:
		return a.NewPage
	case labelOpenFolder:
		return a.OpenFolder
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
	Children     []menuItem
	Separator    bool
}

const (
	labelNewPage    = "New Page…"
	labelOpenFolder = "Open Folder…"
	labelSettings   = "Settings…"
)

var sep = menuItem{Separator: true}

// menuSpec is the whole menu bar: Zen, File, Edit, Window, and in dev builds
// Develop. Production builds have no Reload, DevTools, View, Services or Speech.
func menuSpec(dev bool) []menuItem {
	bar := []menuItem{
		{Label: "Zen", Children: []menuItem{
			{Label: "About Zen", Role: "About"},
			{Label: labelSettings, Accel: "CmdOrCtrl+,"},
			sep,
			{Label: "Hide Zen", Accel: "CmdOrCtrl+H", Role: "Hide"},
			{Label: "Hide Others", Accel: "CmdOrCtrl+OptionOrAlt+H", Role: "HideOthers"},
			{Label: "Show All", Role: "UnHide"},
			sep,
			{Label: "Quit Zen", Accel: "CmdOrCtrl+Q", Role: "Quit"},
		}},
		{Label: "File", Children: []menuItem{
			{Label: labelNewPage, Accel: "CmdOrCtrl+N"},
			{Label: labelOpenFolder, Accel: "CmdOrCtrl+O"},
		}},
		{Label: "Edit", Children: []menuItem{
			{Label: "Undo", Accel: "CmdOrCtrl+Z", Role: "Undo"},
			{Label: "Redo", Accel: "CmdOrCtrl+Shift+Z", Role: "Redo"},
			sep,
			{Label: "Cut", Accel: "CmdOrCtrl+X", Role: "Cut"},
			{Label: "Copy", Accel: "CmdOrCtrl+C", Role: "Copy"},
			{Label: "Paste", Accel: "CmdOrCtrl+V", Role: "Paste"},
			{Label: "Select All", Accel: "CmdOrCtrl+A", Role: "SelectAll"},
		}},
	}
	if dev {
		bar = append(bar, menuItem{Label: "Develop", Children: []menuItem{
			{Label: "Reload", Accel: "CmdOrCtrl+R", Role: "Reload"},
			{Label: "Toggle DevTools", Accel: "CmdOrCtrl+OptionOrAlt+I", Role: "OpenDevTools"},
		}})
	}
	return append(bar, menuItem{Label: "Window", Children: []menuItem{
		{Label: "Minimise", Accel: "CmdOrCtrl+M", Role: "Minimise"},
		{Label: "Zoom", Role: "Zoom"},
	}})
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
func buildMenu(a *application.App, act menuActions, dev bool) *application.Menu {
	menu := a.Menu.New()
	for _, top := range menuSpec(dev) {
		sub := menu.AddSubmenu(top.Label)
		for _, it := range top.Children {
			if it.Separator {
				sub.AddSeparator()
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
				}
			}
			if it.Accel != "" {
				item.SetAccelerator(it.Accel)
			}
		}
	}
	return menu
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
func newPageAction(win shower, sink EventSink) func() {
	return func() {
		win.Show()
		win.Focus()
		sink.Emit(EventNewPage, nil)
	}
}
