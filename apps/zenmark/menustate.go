package app

import "sync"

// folderOnly reports whether a custom menu item needs an open folder: it is
// greyed out on the home screen (app/menu).
func folderOnly(label string) bool {
	switch label {
	case labelNewPage, labelCloseFolder, labelPrintPage, labelExport, labelExportWebsite, labelExportPDF, labelFind, labelFindNext, labelFindPrevious:
		return true
	}
	return false
}

// menuState keeps the enabled state of the folder-only menu items in step
// with whether a folder is open. It holds setters, not Wails items, so tests
// drive it without an application.
type menuState struct {
	mu    sync.Mutex
	open  bool
	items []func(enabled bool)
}

func newMenuState(folderOpen bool) *menuState { return &menuState{open: folderOpen} }

// add registers a folder-only item and gives it the current state.
func (m *menuState) add(set func(enabled bool)) {
	m.mu.Lock()
	m.items = append(m.items, set)
	open := m.open
	m.mu.Unlock()
	set(open)
}

// setFolderOpen enables the folder-only items when a folder is open and
// disables them otherwise. A nil menuState (tests, headless) does nothing.
func (m *menuState) setFolderOpen(open bool) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.open = open
	items := append([]func(bool){}, m.items...)
	m.mu.Unlock()
	for _, set := range items {
		set(open)
	}
}

// folderOpened tells the menu whether a folder is open now.
func (z *Zen) folderOpened(open bool) { z.menu.setFolderOpen(open) }
