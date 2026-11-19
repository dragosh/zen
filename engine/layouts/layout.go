// Package layouts is the page layout system: a layout is the look and feel of
// a whole page (its structure, typography, chrome and behaviour), where a
// plugin is a part of a page. A layout is a folder with a layout.yml and, as it
// needs, an HTML template, style sheets and a page script. Built-in layouts
// are the same folders, embedded in zen (see builtins/layouts); folder and
// user layouts go through the same discovery, hashing and trust as external
// plugins (core/extplugins). See docs/LAYOUTS.md.
package layouts

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/zenmark-org/zenmd/engine/extplugins"
	"github.com/zenmark-org/zenmd/engine/plugins"
)

// Where a layout comes from.
const (
	SourceBuiltin = "builtin"
	SourceFolder  = "folder"
	SourceUser    = "user"
)

// ManifestFile is the name of a layout's definition.
const ManifestFile = "layout.yml"

const (
	maxManifest = 256 << 10
	maxTemplate = 256 << 10
)

// Field is a frontmatter field a template reads. Type is string, text, int,
// bool, date, image or list.
type Field struct{ Name, Type, Description string }

// Option is a layout option (frontmatter `layoutOptions:`).
type Option struct {
	Name, Type  string
	Default     any
	Description string
}

// Manifest is what layout.yml says. Fields and Options are sorted by name.
type Manifest struct {
	ID, Version, Description string
	Template                 string
	CSS, JS                  []string
	Fields                   []Field
	Options                  []Option
	Help                     plugins.Help
	// Requires lists the plugins the layout's pages rely on (`requires:` in
	// layout.yml); kit/assembly checks an app includes them.
	Requires []string
}

// Invalid describes a layout folder that was rejected.
type Invalid struct{ ID, Dir, Problem string }

// Layout is a loaded layout.
type Layout struct {
	man    Manifest
	source string
	tmpl   *template.Template // nil without a template

	// Folder and user layouts: the real folder, the hash taken at load (what
	// the user approves) and the cheap fingerprint guard.
	dir   string
	hash  string
	guard *extplugins.Guard
	// Built-in layouts: their files.
	fsys fs.FS

	files map[string]bool // css and js files that may be served
}

// Manifest returns the layout's definition.
func (l *Layout) Manifest() Manifest { return l.man }

// Source is builtin, folder or user.
func (l *Layout) Source() string { return l.source }

// HasTemplate reports whether the layout has an HTML template.
func (l *Layout) HasTemplate() bool { return l.tmpl != nil }

// HasScript reports whether the layout ships a page script.
func (l *Layout) HasScript() bool { return len(l.man.JS) > 0 }

// Dir is the real path of the layout folder ("" for a built-in layout).
func (l *Layout) Dir() string { return l.dir }

// Hash is the SHA-256 of the layout folder as it was loaded ("" for a
// built-in layout); what the user approves.
func (l *Layout) Hash() (string, error) { return l.hash, nil }

// AssetVersion is the first 12 hex digits of the hash, the ?v= of the asset
// URLs; "" for a built-in layout (the app version is used then).
func (l *Layout) AssetVersion() string {
	if len(l.hash) > 12 {
		return l.hash[:12]
	}
	return l.hash
}

// Changed reports whether the folder differs from what was loaded. Built-in
// layouts never change.
func (l *Layout) Changed() bool { return l.guard != nil && l.guard.Changed() }

// OnChange sets f to be called once when the folder is found changed.
func (l *Layout) OnChange(f func()) {
	if l.guard != nil {
		l.guard.OnChange(f)
	}
}

// Runs describes what the layout does, for the trust prompt.
func (l *Layout) Runs() []string {
	if l.HasScript() {
		return []string{"changes how pages look and runs a page script"}
	}
	return []string{"changes how pages look; runs nothing"}
}

// Entry lists the css and js files (names relative to the layout folder).
func (l *Layout) Entry() plugins.Assets {
	return plugins.Assets{CSS: append([]string(nil), l.man.CSS...), JS: append([]string(nil), l.man.JS...)}
}

type rawField struct {
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
}

type rawOption struct {
	Type        string `yaml:"type"`
	Default     any    `yaml:"default"`
	Description string `yaml:"description"`
}

type rawLayout struct {
	ID          string               `yaml:"id"`
	Version     string               `yaml:"version"`
	Description string               `yaml:"description"`
	Template    string               `yaml:"template"`
	CSS         []string             `yaml:"css"`
	JS          []string             `yaml:"js"`
	Fields      map[string]rawField  `yaml:"fields"`
	Options     map[string]rawOption `yaml:"options"`
	Help        any                  `yaml:"help"`
	Requires    []string             `yaml:"requires"`
}

var (
	fieldNameRE = regexp.MustCompile(`^[a-z][a-zA-Z0-9_-]*$`)
	yamlFieldRE = regexp.MustCompile(`line (\d+): field (\S+) not found in type \S+`)
	yamlTypeRE  = regexp.MustCompile(` into \S+`)
)

var fieldTypes = map[string]bool{"string": true, "text": true, "int": true, "bool": true, "date": true, "image": true, "list": true}

// cleanYAML turns a yaml.v3 error into a short message.
func cleanYAML(err error) string {
	msg := strings.TrimPrefix(err.Error(), "yaml: ")
	if m := yamlFieldRE.FindStringSubmatch(msg); m != nil {
		return fmt.Sprintf("unknown key %q (line %s)", m[2], m[1])
	}
	msg = strings.TrimPrefix(msg, "unmarshal errors:\n")
	msg = strings.TrimSpace(strings.Split(msg, "\n")[0])
	return yamlTypeRE.ReplaceAllString(msg, "")
}

// folderErr says "layout folder" where the shared code says "plugin folder".
func folderErr(err error) error {
	if err == nil {
		return nil
	}
	return errors.New(strings.ReplaceAll(err.Error(), "plugin folder", "layout folder"))
}

// source of the files of a layout while it is parsed.
type files interface {
	manifest() ([]byte, error)
	check(kind, name string) error               // the file is a plain file of the layout
	read(name string, max int64) ([]byte, error) // content of a checked file
}

// parse validates the manifest and builds the layout (without the folder
// fields).
func parse(data []byte, folder string, f files, source string) (*Layout, error) {
	var raw rawLayout
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("layout.yml is not valid YAML: %s", cleanYAML(err))
	}
	if !plugins.ValidName(raw.ID) {
		return nil, fmt.Errorf("invalid id %q (want [a-z][a-z0-9-]*)", raw.ID)
	}
	if raw.ID != folder {
		return nil, fmt.Errorf("id %q must equal the folder name %q", raw.ID, folder)
	}
	if !plugins.ValidSemVer(raw.Version) {
		return nil, fmt.Errorf("version %q is not valid SemVer (such as 1.0.0)", raw.Version)
	}
	if source != SourceBuiltin && raw.Template == "" && len(raw.CSS) == 0 && len(raw.JS) == 0 {
		return nil, errors.New("a layout needs a template, css or js")
	}
	m := Manifest{ID: raw.ID, Version: raw.Version, Description: raw.Description, Template: raw.Template}
	seenReq := map[string]bool{}
	for _, r := range raw.Requires {
		if !plugins.ValidName(r) {
			return nil, fmt.Errorf("requires: %q is not a plugin id", r)
		}
		if seenReq[r] {
			return nil, fmt.Errorf("requires: plugin %s is listed twice", r)
		}
		seenReq[r] = true
	}
	m.Requires = append([]string(nil), raw.Requires...)
	l := &Layout{source: source, files: map[string]bool{}}
	for _, g := range []struct {
		kind  string
		names []string
		dst   *[]string
	}{{"css file", raw.CSS, &m.CSS}, {"js file", raw.JS, &m.JS}} {
		seen := map[string]bool{}
		for _, name := range g.names {
			if err := f.check(g.kind, name); err != nil {
				return nil, err
			}
			clean := filepath.ToSlash(filepath.Clean(name))
			if seen[clean] {
				return nil, fmt.Errorf("%s %s is listed twice", g.kind, name)
			}
			seen[clean] = true
			*g.dst = append(*g.dst, clean)
			l.files[clean] = true
		}
	}
	names := make([]string, 0, len(raw.Fields))
	for n := range raw.Fields {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fd := raw.Fields[n]
		if !fieldNameRE.MatchString(n) {
			return nil, fmt.Errorf("field name %q is not valid (want a name such as date or coverAlt)", n)
		}
		if !fieldTypes[fd.Type] {
			return nil, fmt.Errorf("field %q: type %q is not string, text, int, bool, date, image or list", n, fd.Type)
		}
		m.Fields = append(m.Fields, Field{Name: n, Type: fd.Type, Description: fd.Description})
	}
	names = names[:0]
	for n := range raw.Options {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		o := raw.Options[n]
		if !fieldNameRE.MatchString(n) {
			return nil, fmt.Errorf("option name %q is not valid (want a name such as toc or coverAlt)", n)
		}
		opt, err := extplugins.ParseOption(o.Type, o.Default, o.Description, "")
		if err != nil {
			return nil, fmt.Errorf("option %q: %v", n, err)
		}
		m.Options = append(m.Options, Option{Name: n, Type: opt.Type, Default: opt.Default, Description: opt.Description})
	}
	if raw.Help != nil {
		h, err := plugins.ParseHelp(raw.Help)
		if err != nil {
			return nil, err
		}
		m.Help = h
	}
	if raw.Template != "" {
		if err := f.check("template", raw.Template); err != nil {
			return nil, err
		}
		src, err := f.read(raw.Template, maxTemplate)
		if err != nil {
			if err.Error() == "too large" {
				return nil, errors.New("the template is larger than 256 KB")
			}
			return nil, fmt.Errorf("template %s is not readable", raw.Template)
		}
		t, err := parseTemplate(m, string(src))
		if err != nil {
			return nil, err
		}
		l.tmpl = t
	}
	l.man = m
	return l, nil
}

// LoadFS loads a built-in layout whose files are in fsys (layout.yml at its
// root); id is its folder name.
func LoadFS(fsys fs.FS, id string) (*Layout, error) {
	f := fsFiles{fsys}
	data, err := f.manifest()
	if err != nil {
		return nil, err
	}
	l, err := parse(data, id, f, SourceBuiltin)
	if err != nil {
		return nil, fmt.Errorf("built-in layout %s: %v", id, err)
	}
	l.fsys = fsys
	return l, nil
}

type fsFiles struct{ fsys fs.FS }

func (f fsFiles) manifest() ([]byte, error) { return fs.ReadFile(f.fsys, ManifestFile) }
func (f fsFiles) check(kind, name string) error {
	if !fs.ValidPath(name) {
		return fmt.Errorf("%s %s is not a valid path", kind, name)
	}
	if fi, err := fs.Stat(f.fsys, name); err != nil || !fi.Mode().IsRegular() {
		return fmt.Errorf("%s %s not found", kind, name)
	}
	return nil
}
func (f fsFiles) read(name string, max int64) ([]byte, error) { return fs.ReadFile(f.fsys, name) }

// Load reads and validates the layout folder dir (its name is the layout's
// id). The folder is hashed with the hardened rules of external plugins: every
// file counts, symlinks must stay inside, special files and large folders are
// refused. The error never contains an absolute path.
func Load(dir, source string) (*Layout, error) {
	real, hash, fp, err := extplugins.ScanFolder(dir)
	if err != nil {
		return nil, folderErr(err)
	}
	f := dirFiles{real: real}
	data, err := f.manifest()
	if err != nil {
		return nil, err
	}
	l, err := parse(data, filepath.Base(dir), f, source)
	if err != nil {
		return nil, err
	}
	l.dir, l.hash = real, hash
	l.guard = extplugins.NewGuard(fp, func() (string, error) { return extplugins.FolderFingerprint(real) })
	return l, nil
}

type dirFiles struct{ real string }

func (f dirFiles) manifest() ([]byte, error) {
	full, err := extplugins.ResolveInside(f.real, ManifestFile, "file")
	if err != nil {
		return nil, errors.New("layout.yml not found")
	}
	data, err := extplugins.ReadRegular(full, maxManifest)
	switch {
	case err == nil:
		return data, nil
	case err.Error() == "too large":
		return nil, errors.New("layout.yml is too large")
	}
	return nil, errors.New("layout.yml is not a readable file")
}

func (f dirFiles) check(kind, name string) error {
	return folderErr(extplugins.CheckInside(f.real, name, kind))
}

func (f dirFiles) read(name string, max int64) ([]byte, error) {
	full, err := extplugins.ResolveInside(f.real, name, "template")
	if err != nil {
		return nil, folderErr(err)
	}
	return extplugins.ReadRegular(full, max)
}

// Scan loads every <dir>/<id>/layout.yml. Valid layouts and rejected folders
// are each sorted by id; folders without a layout.yml are ignored.
func Scan(dir, source string) ([]*Layout, []Invalid) {
	var found []*Layout
	var invalid []Invalid
	entries, err := readDir(dir)
	if err != nil {
		return nil, nil
	}
	for _, name := range entries {
		sub := filepath.Join(dir, name)
		if !isDir(sub) || !exists(filepath.Join(sub, ManifestFile)) {
			continue
		}
		l, err := Load(sub, source)
		if err != nil {
			invalid = append(invalid, Invalid{ID: name, Dir: sub, Problem: err.Error()})
			continue
		}
		found = append(found, l)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].man.ID < found[j].man.ID })
	sort.Slice(invalid, func(i, j int) bool { return invalid[i].ID < invalid[j].ID })
	return found, invalid
}
