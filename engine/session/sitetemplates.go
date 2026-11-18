package session

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zenmark-org/zenmd/engine/config"
	"github.com/zenmark-org/zenmd/engine/sitetemplates"
)

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

// TemplateListing is the result of listing the templates.
type TemplateListing struct {
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
func (e siteEnv) list() TemplateListing {
	dir, warns := e.dir()
	out := TemplateListing{Dir: dir, Warnings: warns}
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

func (e siteEnv) find(id string) (sitetemplates.Template, TemplateListing, bool) {
	l := e.list()
	for _, t := range l.Templates {
		if t.ID == id {
			return t, l, true
		}
	}
	return sitetemplates.Template{}, l, false
}

// TemplatesAPI is the session's site templates: list them, plan and create a
// site from one, and write a newer built-in beside the user's copy.
type TemplatesAPI struct{ env siteEnv }

// Templates returns the site templates of this session: the templates folder
// next to its config file and the built-in presets.
func (s *Session) Templates() TemplatesAPI {
	return TemplatesAPI{siteEnv{globalPath: s.globalPath, builtins: s.presets}}
}

// List lists the site templates for the New Site dialog and the welcome
// screen, installing the built-in presets on first use.
func (t TemplatesAPI) List() []SiteTemplate {
	out := []SiteTemplate{}
	for _, tpl := range t.env.list().Templates {
		st := SiteTemplate{ID: tpl.ID, Name: tpl.Name, Description: tpl.Description, Version: tpl.Version, Builtin: tpl.Builtin, Update: tpl.Update, Error: tpl.Err}
		for _, a := range tpl.Ask {
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

// Listing is List with everything the templates package knows: warnings and
// notes included.
func (t TemplatesAPI) Listing() TemplateListing { return t.env.list() }

// Dir is the templates folder (templates.dir in config.yml, or the default
// next to it), "" when there is none. It installs the built-in presets first.
func (t TemplatesAPI) Dir() string { return t.env.list().Dir }

// Folder is Dir without installing anything, with the problems found.
func (t TemplatesAPI) Folder() (string, []string) { return t.env.dir() }

// Update writes the newer built-in template id beside the user's copy and
// returns where it wrote it.
func (t TemplatesAPI) Update(id string) (string, error) {
	dir, _ := t.env.dir()
	if dir == "" {
		return "", errors.New("there is no templates folder")
	}
	t.env.list() // install what is missing, so there is a record
	return sitetemplates.Update(dir, t.env.builtins, id)
}

// Plan is a checked request to make a site: the template exists and dir is
// not inside the templates folder.
type Plan struct {
	t   sitetemplates.Template
	dir string
}

// Plan checks that a site can be made from template id in dir (new or
// existing, absolute) and returns the plan; nothing is written yet.
func (t TemplatesAPI) Plan(id, dir string) (Plan, error) {
	if !filepath.IsAbs(dir) {
		return Plan{}, errors.New("choose a folder")
	}
	tpl, l, ok := t.env.find(id)
	if !ok {
		return Plan{}, fmt.Errorf("there is no template %q", id)
	}
	if l.Holds(dir) {
		return Plan{}, errors.New("choose a folder outside the templates folder")
	}
	return Plan{t: tpl, dir: dir}, nil
}

// Create writes the site; nothing is overwritten.
func (p Plan) Create(values map[string]string) error {
	_, err := sitetemplates.Create(p.t, p.dir, values)
	return err
}

// Holds reports whether dir is the templates folder or inside it.
func (l TemplateListing) Holds(dir string) bool { return l.Dir != "" && insideDir(dir, l.Dir) }

// insideDir reports whether p is dir or inside it.
func insideDir(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
