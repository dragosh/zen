// Package extplugins loads external plugins: folders with a plugin.yml whose
// backend is a program zen runs (stdin, stdout, environment) and whose
// frontend is scripts and styles served to the page. See docs/PLUGINS.md,
// "Writing your own plugin". Nothing here runs a plugin before the app has
// asked the user (see Trust).
package extplugins

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dragosh/zen/internal/core/plugins"
)

// Limits of a plugin folder.
const (
	maxFiles       = 500
	maxBytes       = 20 << 20
	maxManifest    = 256 << 10
	defaultTimeout = 5 * time.Second
	maxTimeout     = 60 * time.Second
	// A plugin with the stream hook may ask for longer; it is the stream's limit.
	maxStreamTimeout     = 10 * time.Minute
	defaultStreamTimeout = 60 * time.Second
)

var validHooks = map[string]bool{"source": true, "block": true, "inline": true, "html": true, "action": true, "stream": true}

type rawOption struct {
	Type        string `yaml:"type"`
	Default     any    `yaml:"default"`
	Description string `yaml:"description"`
	Scope       string `yaml:"scope"`
}

type rawAllow struct {
	Element string   `yaml:"element"`
	Attrs   []string `yaml:"attrs"`
}

type rawBackend struct {
	Command []string `yaml:"command"`
	Hooks   []string `yaml:"hooks"`
	Output  string   `yaml:"output"`
	Timeout string   `yaml:"timeout"`
}

type rawFrontend struct {
	JS  []string `yaml:"js"`
	CSS []string `yaml:"css"`
}

type rawManifest struct {
	ID          string               `yaml:"id"`
	Version     string               `yaml:"version"`
	APIVersion  *int                 `yaml:"apiVersion"`
	Description string               `yaml:"description"`
	Default     *bool                `yaml:"default"` // on unless "default: false"
	Blocks      []string             `yaml:"blocks"`
	Inlines     []string             `yaml:"inlines"`
	Layouts     []string             `yaml:"layouts"`
	Options     map[string]rawOption `yaml:"options"`
	Allow       []rawAllow           `yaml:"allow"`
	Backend     *rawBackend          `yaml:"backend"`
	Frontend    *rawFrontend         `yaml:"frontend"`
}

type backend struct {
	command []string
	hooks   map[string]bool
	text    bool // output: text
	timeout time.Duration
	// timeoutSet: the manifest gave a timeout (streams default to 60s otherwise).
	timeoutSet bool
}

// callTimeout is the limit of a hook call other than a stream: the manifest's
// timeout, never over 60s.
func (p *Plugin) callTimeout() time.Duration {
	if p.back.timeout > maxTimeout {
		return maxTimeout
	}
	return p.back.timeout
}

// streamTimeout is the limit of a stream: the manifest's timeout (up to 10m),
// 60s when it gave none.
func (p *Plugin) streamTimeout() time.Duration {
	if p.back.timeoutSet {
		return p.back.timeout
	}
	return defaultStreamTimeout
}

// Runtime is what a plugin learns about the folder it runs for. Bind it
// before any hook runs.
type Runtime struct {
	Workspace string // folder root (ZEN_WORKSPACE)
	DataDir   string // writable data folder of this plugin (ZEN_DATA_DIR), created on demand
	// Ctx ends every call of the plugin when cancelled (the folder changed, the
	// app quits); nil means never.
	Ctx context.Context
}

// Plugin is a loaded external plugin. It implements plugins.Plugin, the
// hook interfaces (HasHook says which are declared) and plugins.AssetProvider.
type Plugin struct {
	dir      string // real path of the plugin folder
	man      plugins.Manifest
	back     *backend
	frontend plugins.Assets
	rt       atomic.Pointer[Runtime]

	hash  string // SHA-256 of the folder when it was loaded (what the user approves)
	guard *Guard // cheap fingerprint of the folder when it was loaded
}

// Invalid describes a plugin folder that was rejected.
type Invalid struct {
	ID      string // folder name
	Dir     string
	Problem string
}

// Manifest implements plugins.Plugin.
func (p *Plugin) Manifest() plugins.Manifest { return p.man }

// External marks plugins that run programs.
func (p *Plugin) External() bool { return true }

// Kind is "program" for a plugin with a backend and "page" for one that only
// ships frontend files.
func (p *Plugin) Kind() string {
	if p.back != nil {
		return "program"
	}
	return "page"
}

// Dir is the real path of the plugin folder.
func (p *Plugin) Dir() string { return p.dir }

// Bind sets the folder the plugin runs for.
func (p *Plugin) Bind(rt Runtime) { p.rt.Store(&rt) }

// HasHook reports whether the manifest declares the hook (source, block,
// inline, html, action or stream).
func (p *Plugin) HasHook(name string) bool { return p.back != nil && p.back.hooks[name] }

// Entry lists the frontend files (plugins.AssetProvider).
func (p *Plugin) Entry() plugins.Assets { return p.frontend }

// Runs describes what the plugin runs, one line each, for the trust prompt.
func (p *Plugin) Runs() []string {
	var out []string
	if p.back != nil {
		parts := make([]string, len(p.back.command))
		for i, a := range p.back.command {
			parts[i] = a
			if strings.ContainsAny(a, " \t\"'") {
				parts[i] = fmt.Sprintf("%q", a)
			}
		}
		out = append(out, strings.Join(parts, " "))
	}
	if len(p.frontend.JS) > 0 {
		out = append(out, "scripts in the page")
	}
	return out
}

var yamlFieldRE = regexp.MustCompile(`line (\d+): field (\S+) not found in type \S+`)

// cleanYAMLError turns a yaml.v3 error into a short message naming the
// offending key.
func cleanYAMLError(err error) string {
	msg := strings.TrimPrefix(err.Error(), "yaml: ")
	if m := yamlFieldRE.FindStringSubmatch(msg); m != nil {
		return fmt.Sprintf("unknown key %q (line %s)", m[2], m[1])
	}
	msg = strings.TrimPrefix(msg, "unmarshal errors:\n")
	msg = strings.TrimSpace(strings.Split(msg, "\n")[0])
	msg = regexp.MustCompile(` into \S+`).ReplaceAllString(msg, "")
	return msg
}

// Load reads and validates the plugin in dir. The error never contains an
// absolute path.
func Load(dir string) (*Plugin, error) {
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, errors.New("plugin folder not readable")
	}
	hash, fp, err := scanFolder(real, true)
	if err != nil {
		return nil, err
	}
	data, err := readManifest(real)
	if err != nil {
		return nil, err
	}
	var raw rawManifest
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("plugin.yml is not valid YAML: %s", cleanYAMLError(err))
	}
	folder := filepath.Base(dir)
	if !plugins.ValidName(raw.ID) {
		return nil, fmt.Errorf("invalid id %q (want [a-z][a-z0-9-]*)", raw.ID)
	}
	if raw.ID != folder {
		return nil, fmt.Errorf("id %q must equal the folder name %q", raw.ID, folder)
	}
	if raw.APIVersion == nil || *raw.APIVersion != plugins.APIVersion {
		return nil, fmt.Errorf("apiVersion must be %d", plugins.APIVersion)
	}
	if !plugins.ValidSemVer(raw.Version) {
		return nil, fmt.Errorf("version %q is not valid SemVer (such as 1.0.0)", raw.Version)
	}
	p := &Plugin{dir: real, hash: hash, guard: NewGuard(fp, func() (string, error) { return FolderFingerprint(real) })}
	m := plugins.Manifest{
		ID: raw.ID, Version: raw.Version, APIVersion: plugins.APIVersion,
		Description: raw.Description, Default: raw.Default == nil || *raw.Default,
	}
	for _, list := range []struct {
		label string
		names []string
		dst   *[]string
	}{{"block", raw.Blocks, &m.Blocks}, {"inline", raw.Inlines, &m.Inlines}, {"layout", raw.Layouts, &m.Layouts}} {
		seen := map[string]bool{}
		for _, n := range list.names {
			if !plugins.ValidName(n) {
				return nil, fmt.Errorf("invalid %s name %q (want [a-z][a-z0-9-]*)", list.label, n)
			}
			if seen[n] {
				return nil, fmt.Errorf("%s %q is listed twice", list.label, n)
			}
			seen[n] = true
		}
		*list.dst = append([]string(nil), list.names...)
	}
	if len(raw.Options) > 0 {
		m.Config = map[string]plugins.Option{}
	}
	for name, o := range raw.Options {
		def, err := optionDefault(o)
		if err != nil {
			return nil, fmt.Errorf("option %q: %v", name, err)
		}
		switch o.Scope {
		case "", plugins.ScopeUser:
		default:
			return nil, fmt.Errorf("option %q: scope %q is not user", name, o.Scope)
		}
		m.Config[name] = plugins.Option{Type: o.Type, Default: def, Description: o.Description, Scope: o.Scope}
	}
	for _, a := range raw.Allow {
		if strings.TrimSpace(a.Element) == "" {
			return nil, errors.New("allow: element is empty")
		}
		m.Allow = append(m.Allow, plugins.AllowRule{Element: a.Element, Attrs: a.Attrs})
	}
	if raw.Backend != nil {
		if p.back, err = loadBackend(real, raw.Backend); err != nil {
			return nil, err
		}
		if err := checkArgs(real, p.back.command); err != nil {
			return nil, err
		}
	}
	if raw.Frontend != nil {
		for _, f := range []struct {
			kind  string
			files []string
			dst   *[]string
		}{{"js", raw.Frontend.JS, &p.frontend.JS}, {"css", raw.Frontend.CSS, &p.frontend.CSS}} {
			for _, name := range f.files {
				if err := checkInside(real, name, "frontend file"); err != nil {
					return nil, err
				}
				*f.dst = append(*f.dst, filepath.ToSlash(filepath.Clean(name)))
			}
		}
	}
	p.man = m
	return p, nil
}

func optionDefault(o rawOption) (any, error) {
	switch o.Type {
	case "bool":
		if o.Default == nil {
			return false, nil
		}
		if b, ok := o.Default.(bool); ok {
			return b, nil
		}
	case "int":
		if o.Default == nil {
			return 0, nil
		}
		if n, ok := o.Default.(int); ok {
			return n, nil
		}
	case "string":
		if o.Default == nil {
			return "", nil
		}
		if s, ok := o.Default.(string); ok {
			return s, nil
		}
	case "map", "list":
		if o.Default == nil {
			v, _ := plugins.CoerceOption(o.Type, map[string]any{})
			if o.Type == "list" {
				v, _ = plugins.CoerceOption(o.Type, []any{})
			}
			return v, nil
		}
		if v, ok := plugins.CoerceOption(o.Type, o.Default); ok {
			return v, nil
		}
		what := "a mapping of strings"
		if o.Type == "list" {
			what = "a list of strings"
		}
		return nil, fmt.Errorf("default %v is not %s", o.Default, what)
	default:
		return nil, fmt.Errorf("type %q is not string, bool, int, map or list", o.Type)
	}
	return nil, fmt.Errorf("default %v is not of type %s", o.Default, o.Type)
}

func loadBackend(real string, rb *rawBackend) (*backend, error) {
	b := &backend{hooks: map[string]bool{}, timeout: defaultTimeout}
	if len(rb.Command) == 0 || strings.TrimSpace(rb.Command[0]) == "" {
		return nil, errors.New("backend.command is empty")
	}
	b.command = append([]string(nil), rb.Command...)
	prog := b.command[0]
	switch {
	case filepath.IsAbs(prog):
		return nil, errors.New("backend.command must be relative to the plugin folder (./run.sh) or a program name on PATH")
	case strings.Contains(prog, "/"):
		path, err := resolveInside(real, prog, "backend.command file")
		if err != nil {
			return nil, err
		}
		if fi, err := os.Stat(path); err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0o111 == 0 {
			return nil, fmt.Errorf("backend.command file %s is not executable", prog)
		}
	}
	if len(rb.Hooks) == 0 {
		return nil, errors.New("backend.hooks is empty (any of: source, block, inline, html, action, stream)")
	}
	for _, h := range rb.Hooks {
		if !validHooks[h] {
			return nil, fmt.Errorf("unknown hook %q in backend.hooks", h)
		}
		b.hooks[h] = true
	}
	switch rb.Output {
	case "", "html":
	case "text":
		b.text = true
	default:
		return nil, fmt.Errorf("backend.output %q is not html or text", rb.Output)
	}
	if rb.Timeout != "" {
		d, err := time.ParseDuration(rb.Timeout)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("backend.timeout %q is not a duration such as 5s", rb.Timeout)
		}
		limit := maxTimeout
		if b.hooks["stream"] {
			limit = maxStreamTimeout
		}
		if d > limit {
			return nil, fmt.Errorf("backend.timeout %s is over the limit of %s", rb.Timeout, limit)
		}
		b.timeout, b.timeoutSet = d, true
	}
	return b, nil
}

// readManifest reads plugin.yml of the folder: a regular file inside it (a
// symlink is followed only when it stays inside), at most maxManifest bytes.
func readManifest(real string) ([]byte, error) {
	path := filepath.Join(real, "plugin.yml")
	if _, err := os.Lstat(path); err != nil {
		return nil, errors.New("plugin.yml not found")
	}
	full, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, errors.New("plugin.yml not found")
	}
	if !insideRel(real, full) {
		return nil, errors.New("plugin.yml points outside the plugin folder")
	}
	fi, err := os.Lstat(full)
	if err != nil || !fi.Mode().IsRegular() {
		return nil, errors.New("plugin.yml is not a regular file")
	}
	data, err := readRegular(full, maxManifest)
	switch {
	case err == nil:
		return data, nil
	case err.Error() == "too large":
		return nil, errors.New("plugin.yml is too large")
	}
	return nil, errors.New("plugin.yml is not readable")
}

// resolveInside resolves a path relative to the plugin folder (symlinks
// included) and requires the result to be inside it. rel is only used in
// messages.
func resolveInside(real, rel, what string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s %s is outside the plugin folder", what, rel)
	}
	full, err := filepath.EvalSymlinks(filepath.Join(real, clean))
	if err != nil {
		return "", fmt.Errorf("%s %s not found", what, rel)
	}
	if !insideRel(real, full) {
		return "", fmt.Errorf("%s %s is outside the plugin folder", what, rel)
	}
	return full, nil
}

// checkInside requires name to be a plain relative file path that exists
// inside the plugin folder.
func checkInside(real, name, what string) error {
	if name == "" || strings.ContainsAny(name, "\\:?#") {
		return fmt.Errorf("%s %q is not a valid path", what, name)
	}
	if err := refuseFinderFile(real, name, what); err != nil {
		return err
	}
	full, err := resolveInside(real, name, what)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(full); err != nil || !fi.Mode().IsRegular() {
		return fmt.Errorf("%s %s is not a file", what, name)
	}
	if !fs.ValidPath(filepath.ToSlash(filepath.Clean(name))) {
		return fmt.Errorf("%s %s is outside the plugin folder", what, name)
	}
	return nil
}

// Scan loads every <dir>/<id>/plugin.yml in the given plugin folders, in
// order; the first folder wins for an id. Valid plugins and rejected folders
// are each sorted by id. Folders without a plugin.yml are ignored.
func Scan(dirs ...string) ([]*Plugin, []Invalid) {
	var found []*Plugin
	var invalid []Invalid
	seen := map[string]bool{}
	for _, base := range dirs {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			dir := filepath.Join(base, name)
			if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
				continue
			}
			if _, err := os.Lstat(filepath.Join(dir, "plugin.yml")); err != nil {
				continue
			}
			if seen[name] {
				continue
			}
			seen[name] = true
			p, err := Load(dir)
			if err != nil {
				invalid = append(invalid, Invalid{ID: name, Dir: dir, Problem: err.Error()})
				continue
			}
			found = append(found, p)
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].man.ID < found[j].man.ID })
	sort.Slice(invalid, func(i, j int) bool { return invalid[i].ID < invalid[j].ID })
	return found, invalid
}

// Discover is Scan with the rejected folders turned into warnings.
func Discover(dirs ...string) (found []*Plugin, warnings []string) {
	found, invalid := Scan(dirs...)
	for _, in := range invalid {
		warnings = append(warnings, fmt.Sprintf("plugin %q: %s", in.ID, in.Problem))
	}
	return found, warnings
}
