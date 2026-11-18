package desktop

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// DialogSet are the native dialogs, attached to the main window. It
// implements session.Prompter (ConfirmTrust) and the folder pickers.
type DialogSet struct {
	app *application.App
	win *application.WebviewWindow
	// beforeOpen runs once a folder was chosen; false cancels opening it
	// (an editor window kept its unsaved changes).
	beforeOpen func() bool
}

// Dialogs returns the dialogs of a; a may be nil when the application does
// not exist yet (the session needs its Prompter first): Run binds them.
func Dialogs(a *application.App) *DialogSet { return &DialogSet{app: a} }

func (d *DialogSet) bind(a *application.App, win *application.WebviewWindow, beforeOpen func() bool) {
	d.app, d.win, d.beforeOpen = a, win, beforeOpen
}

// PickFolder asks for a folder to open; "" on cancel. Choosing one first
// lets editor windows keep unsaved changes (cancelling it).
func (d *DialogSet) PickFolder() (string, error) {
	dir, err := d.pick("Open Folder")
	if err != nil || dir == "" {
		return dir, err
	}
	if d.beforeOpen != nil && !d.beforeOpen() {
		return "", nil
	}
	return dir, nil
}

// PickFolderTitled shows the folder panel (New Folder enabled) with title and
// returns the chosen folder, "" on cancel. It opens nothing and closes no editors.
func (d *DialogSet) PickFolderTitled(title string) (string, error) { return d.pick(title) }

func (d *DialogSet) pick(title string) (string, error) {
	return d.app.Dialog.OpenFile().
		SetTitle(title).
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		AttachToWindow(d.win).
		PromptForSingleSelection()
}

// Ready reports whether the dialogs can be shown (the window exists).
func (d *DialogSet) Ready() bool { return d != nil && d.app != nil && d.win != nil }

// ConfirmTrust is the prompt before plugins of a folder are used.
func (d *DialogSet) ConfirmTrust(message string) bool {
	answer := make(chan bool, 2)
	dlg := d.app.Dialog.Question().SetTitle("Allow?").SetMessage(message)
	allow := dlg.AddButton("Allow").OnClick(func() { answer <- true })
	block := dlg.AddButton("Block").OnClick(func() { answer <- false })
	dlg.SetDefaultButton(block).SetCancelButton(block).AttachToWindow(d.win)
	_ = allow
	dlg.Show()
	return <-answer
}

// Confirm asks "Open Link"-style yes/no questions.
func (d *DialogSet) Confirm(message string) bool {
	answer := make(chan bool, 2)
	dlg := d.app.Dialog.Question().SetTitle("Open Link").SetMessage(message)
	open := dlg.AddButton("Open").OnClick(func() { answer <- true })
	cancel := dlg.AddButton("Cancel").OnClick(func() { answer <- false })
	dlg.SetDefaultButton(open).SetCancelButton(cancel).AttachToWindow(d.win)
	dlg.Show()
	return <-answer
}
