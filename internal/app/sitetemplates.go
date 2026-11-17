package app

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dragosh/zen/engine/session"
	"github.com/dragosh/zen/internal/cli"
	"github.com/dragosh/zen/internal/core/config"
	"github.com/dragosh/zen/internal/core/sitetemplates"
	presets "github.com/dragosh/zen/internal/sitetemplates"
)

// EventNewSite asks the main window to show the New Site dialog (File > New
// Site from Template…, Shift+Cmd+N). No payload.
const EventNewSite = "menu:new-site"

// siteBuiltins are the built-in presets; tests replace them.
var siteBuiltins fs.FS = presets.FS

// revealInFinder shows a folder in Finder; tests replace it.
var revealInFinder = func(dir string) error { return macOpen(dir) }

// SiteTemplates lists the site templates for the New Site dialog and the
// welcome screen, installing the built-in presets on first use.
func (z *Zen) SiteTemplates() []SiteTemplate { return z.s.Templates().List() }

// siteFolderPicker is the native folder panel with New Folder, for the New
// Site dialog; it opens nothing and closes no editors.
type siteFolderPicker interface{ PickSiteFolder() (string, error) }

// PickSiteFolder shows the folder panel (New Folder enabled) and returns the
// chosen folder, "" on cancel.
func (z *Zen) PickSiteFolder() (string, error) {
	if p, ok := z.dialogs.(siteFolderPicker); ok {
		return p.PickSiteFolder()
	}
	return z.dialogs.PickFolder()
}

// PickSiteFolder is the folder panel of the New Site dialog.
func (d *wailsDialogs) PickSiteFolder() (string, error) {
	return d.app.Dialog.OpenFile().
		SetTitle("Choose a Folder for the Site").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		AttachToWindow(d.win).
		PromptForSingleSelection()
}

// CreateSite makes a site from template id in dir (new or existing; nothing
// is overwritten), then opens the folder as Open Folder does: editor windows
// are closed first (each asks about unsaved changes), and the result is what
// the frontend shows.
func (z *Zen) CreateSite(id, dir string, values map[string]string) (Startup, error) {
	plan, err := z.s.Templates().Plan(id, dir)
	if err != nil {
		return Startup{}, err
	}
	// Unsaved editors are asked first, as with Open Folder: nothing is
	// written when one keeps its changes.
	if z.windows != nil && !z.windows.closeEditorsWait() {
		return Startup{}, errors.New("nothing was created: an editor window kept its unsaved changes")
	}
	if err := plan.Create(values); err != nil {
		return Startup{}, err
	}
	if err := z.s.Open(dir); err != nil {
		return Startup{}, z.s.OpenError(err)
	}
	z.page = ""
	z.setLaunchErr(nil)
	return z.Startup(), nil
}

// TemplatesDir is the templates folder (templates.dir in config.yml, or the
// default next to it), "" when there is none.
func (z *Zen) TemplatesDir() string { return z.s.Templates().Dir() }

// RevealTemplates shows the templates folder in Finder, creating it (with the
// built-in presets) when it is not there yet.
func (z *Zen) RevealTemplates() error {
	dir := z.s.Templates().Dir()
	if dir == "" {
		return errors.New("there is no templates folder")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return revealInFinder(dir)
}

// InitSite is `zen init`: it lists templates, writes a newer built-in beside
// the user's copy, or creates a site, and tells what it did on stdout.
func InitSite(i cli.Init, stdout, stderr io.Writer) error {
	globalPath, _ := config.GlobalPath()
	s := session.New(session.Options{GlobalPath: globalPath, Presets: siteBuiltins})
	defer s.Close()
	return initSite(s.Templates(), i, stdout, stderr)
}

func initSite(tpls session.TemplatesAPI, i cli.Init, stdout, stderr io.Writer) error {
	if i.ShowDir {
		dir, _ := tpls.Folder()
		if dir == "" {
			return errors.New("there is no templates folder")
		}
		fmt.Fprintln(stdout, dir)
		return nil
	}
	if i.Update {
		out, err := tpls.Update(i.Template)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Wrote the newer %s to %s. Compare it with your copy, keep what you like, then delete it.\n", i.Template, out)
		return nil
	}

	l := tpls.Listing()
	for _, w := range l.Warnings {
		fmt.Fprintln(stderr, "zen: "+w)
	}
	if l.Note != "" {
		fmt.Fprintln(stderr, "zen: "+l.Note)
	}
	if i.List {
		width := 0
		for _, t := range l.Templates {
			width = max(width, len(t.ID))
		}
		for _, t := range l.Templates {
			switch {
			case t.Err != "":
				fmt.Fprintf(stdout, "%-*s  (invalid: %s)\n", width, t.ID, t.Err)
			default:
				line := fmt.Sprintf("%-*s  %s", width, t.ID, t.Name)
				if t.Description != "" {
					line += " — " + t.Description
				}
				if t.Update != "" {
					line += fmt.Sprintf(" [update available: %s, see zen init --update %s]", t.Update, t.ID)
				}
				fmt.Fprintln(stdout, line)
			}
		}
		if len(l.Templates) == 0 {
			fmt.Fprintln(stdout, "No templates.")
		}
		return nil
	}

	var t sitetemplates.Template
	found := false
	for _, x := range l.Templates {
		if x.ID == i.Template {
			t, found = x, true
		}
	}
	if !found {
		var ids []string
		for _, x := range l.Templates {
			if x.Err == "" {
				ids = append(ids, x.ID)
			}
		}
		return cli.UsageError(fmt.Sprintf("there is no template %q (templates: %s)", i.Template, strings.Join(ids, ", ")))
	}
	if t.Err != "" {
		return fmt.Errorf("template %s is not valid: %s", t.ID, t.Err)
	}
	if l.Holds(i.Dir) {
		return errors.New("choose a folder outside the templates folder")
	}
	res, err := sitetemplates.Create(t, i.Dir, i.Values)
	if err != nil {
		return err
	}
	n := fmt.Sprintf("%d files", len(res.Files))
	if len(res.Files) == 1 {
		n = "1 file"
	}
	shown := shownFolder(i.Dir)
	fmt.Fprintf(stdout, "Created %s in %s (%s). Open it with: zen %s\n", n, shown, t.Name, shellQuote(shown))
	return nil
}

// shownFolder names dir the short way: relative to the current folder when it
// is inside it ("." for the folder itself).
func shownFolder(dir string) string {
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, dir); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return rel
		}
	}
	return dir
}

// shellQuote quotes s for a shell only when it needs it.
func shellQuote(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-/+,:@") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
