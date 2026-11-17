package app

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

// SiteTemplate is a site template as the frontend lists it (core/init).
type SiteTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Ask         []struct {
		Key     string `json:"key"`
		Label   string `json:"label"`
		Default string `json:"default"`
	} `json:"ask"`
	Builtin bool   `json:"builtin"`
	Update  string `json:"update"`
	Error   string `json:"error"`
}

// siteEnv is what the templates need from outside: the global config file
// (its folder holds the templates folder) and the built-in presets.
type siteEnv struct {
	globalPath string
	builtins   fs.FS
}

// siteList is the result of listing the templates.
type siteList struct {
	Templates []sitetemplates.Template
	Dir       string   // the templates folder, "" when there is none
	Warnings  []string // problems with the templates and the templates: key
	Note      string   // set when the built-in presets are used straight from zen
}

// dir is the templates folder: <config folder>/templates, or the global
// config's templates: dir. "" without a settings folder.
func (e siteEnv) dir() (string, []string) {
	if e.globalPath == "" {
		return "", nil
	}
	return config.Load(e.globalPath, "").TemplatesDir(filepath.Dir(e.globalPath))
}

// list installs the presets that are not there yet, and lists the templates.
// When the folder cannot be written to, the presets that are not in it are
// listed from zen itself, with a note.
func (e siteEnv) list() siteList {
	dir, warns := e.dir()
	out := siteList{Dir: dir, Warnings: warns}
	if dir == "" {
		out.Templates, _ = sitetemplates.Builtins(e.builtins)
		out.Note = "There is no settings folder, so the built-in templates are used as they are."
		return out
	}
	err := sitetemplates.Install(dir, e.builtins)
	ts, lw := sitetemplates.ListWith(dir, e.builtins)
	out.Templates, out.Warnings = ts, append(out.Warnings, lw...)
	if err != nil {
		have := map[string]bool{}
		for _, t := range ts {
			have[t.ID] = true
		}
		bundle, _ := sitetemplates.Builtins(e.builtins)
		for _, t := range bundle {
			if !have[t.ID] && t.Err == "" {
				out.Templates = append(out.Templates, t)
			}
		}
		sort.SliceStable(out.Templates, func(i, j int) bool {
			a, b := out.Templates[i], out.Templates[j]
			if (a.Err == "") != (b.Err == "") {
				return a.Err == ""
			}
			if a.Order != b.Order {
				return a.Order < b.Order
			}
			return a.Name < b.Name
		})
		out.Note = fmt.Sprintf("The templates folder (%s) cannot be written to, so the built-in templates are used as they are.", dir)
	}
	return out
}

func (e siteEnv) find(id string) (sitetemplates.Template, siteList, bool) {
	l := e.list()
	for _, t := range l.Templates {
		if t.ID == id {
			return t, l, true
		}
	}
	return sitetemplates.Template{}, l, false
}

func (z *Zen) siteEnv() siteEnv {
	return siteEnv{globalPath: z.state.globalPath, builtins: siteBuiltins}
}

// SiteTemplates lists the site templates for the New Site dialog and the
// welcome screen, installing the built-in presets on first use.
func (z *Zen) SiteTemplates() []SiteTemplate {
	out := []SiteTemplate{}
	for _, t := range z.siteEnv().list().Templates {
		st := SiteTemplate{ID: t.ID, Name: t.Name, Description: t.Description, Version: t.Version, Builtin: t.Builtin, Update: t.Update, Error: t.Err}
		for _, a := range t.Ask {
			st.Ask = append(st.Ask, struct {
				Key     string `json:"key"`
				Label   string `json:"label"`
				Default string `json:"default"`
			}{a.Key, a.Label, a.Default})
		}
		out = append(out, st)
	}
	return out
}

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
	if !filepath.IsAbs(dir) {
		return Startup{}, errors.New("choose a folder")
	}
	env := z.siteEnv()
	t, l, ok := env.find(id)
	if !ok {
		return Startup{}, fmt.Errorf("there is no template %q", id)
	}
	if l.Dir != "" && insideDir(dir, l.Dir) {
		return Startup{}, errors.New("choose a folder outside the templates folder")
	}
	// Unsaved editors are asked first, as with Open Folder: nothing is
	// written when one keeps its changes.
	if z.windows != nil && !z.windows.closeEditorsWait() {
		return Startup{}, errors.New("nothing was created: an editor window kept its unsaved changes")
	}
	if _, err := sitetemplates.Create(t, dir, values); err != nil {
		return Startup{}, err
	}
	if err := z.state.open(dir); err != nil {
		return Startup{}, openErr(z.state, err)
	}
	z.page = ""
	z.setLaunchErr(nil)
	return z.Startup(), nil
}

// insideDir reports whether p is dir or inside it.
func insideDir(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// TemplatesDir is the templates folder (templates.dir in config.yml, or the
// default next to it), "" when there is none.
func (z *Zen) TemplatesDir() string { return z.siteEnv().list().Dir }

// RevealTemplates shows the templates folder in Finder, creating it (with the
// built-in presets) when it is not there yet.
func (z *Zen) RevealTemplates() error {
	l := z.siteEnv().list()
	if l.Dir == "" {
		return errors.New("there is no templates folder")
	}
	if err := os.MkdirAll(l.Dir, 0o755); err != nil {
		return err
	}
	return revealInFinder(l.Dir)
}

// InitSite is `zen init`: it lists templates, writes a newer built-in beside
// the user's copy, or creates a site, and tells what it did on stdout.
func InitSite(i cli.Init, stdout, stderr io.Writer) error {
	globalPath, _ := config.GlobalPath()
	return initSite(siteEnv{globalPath: globalPath, builtins: siteBuiltins}, i, stdout, stderr)
}

func initSite(env siteEnv, i cli.Init, stdout, stderr io.Writer) error {
	if i.ShowDir {
		dir, _ := env.dir()
		if dir == "" {
			return errors.New("there is no templates folder")
		}
		fmt.Fprintln(stdout, dir)
		return nil
	}
	if i.Update {
		dir, _ := env.dir()
		if dir == "" {
			return errors.New("there is no templates folder")
		}
		env.list() // install what is missing, so there is a record
		out, err := sitetemplates.Update(dir, env.builtins, i.Template)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Wrote the newer %s to %s. Compare it with your copy, keep what you like, then delete it.\n", i.Template, out)
		return nil
	}

	l := env.list()
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
	if l.Dir != "" && insideDir(i.Dir, l.Dir) {
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
