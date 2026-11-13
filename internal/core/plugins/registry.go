package plugins

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"path"
	"regexp"
	"sort"
	"strings"
)

var (
	nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	// semverRE is the regular expression suggested by semver.org.
	semverRE = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
		`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
		`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
)

type registered struct {
	plugin   Plugin
	manifest Manifest
}

// Registry holds the plugins known to the app, in registration order. It is
// filled at start-up and is not safe for concurrent registration.
type Registry struct {
	items   []registered
	byID    map[string]int
	blocks  map[string]string // block name -> owning plugin id
	inlines map[string]string // inline name -> owning plugin id
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		byID:    map[string]int{},
		blocks:  map[string]string{},
		inlines: map[string]string{},
	}
}

// Register validates p's manifest and adds the plugin. Nothing is stored
// when an error is returned.
func (r *Registry) Register(p Plugin) error {
	if p == nil {
		return errors.New("plugins: cannot register a nil plugin")
	}
	m := p.Manifest()
	fail := func(format string, args ...any) error {
		return fmt.Errorf("plugin %q: %s", m.ID, fmt.Sprintf(format, args...))
	}
	if !nameRE.MatchString(m.ID) {
		return fail("invalid id (want [a-z][a-z0-9-]*)")
	}
	if m.APIVersion != APIVersion {
		return fail("API version %d is not supported (want %d)", m.APIVersion, APIVersion)
	}
	if !semverRE.MatchString(m.Version) {
		return fail("version %q is not valid SemVer", m.Version)
	}
	if _, dup := r.byID[m.ID]; dup {
		return fail("already registered")
	}
	for _, kind := range []struct {
		label string
		names []string
		owned map[string]string
	}{{"block", m.Blocks, r.blocks}, {"inline", m.Inlines, r.inlines}} {
		for _, n := range kind.names {
			if !nameRE.MatchString(n) {
				return fail("invalid %s name %q (want [a-z][a-z0-9-]*)", kind.label, n)
			}
			if owner, taken := kind.owned[n]; taken {
				return fail("%s %q is already owned by plugin %q", kind.label, n, owner)
			}
		}
	}
	for name, opt := range m.Config {
		switch opt.Type {
		case "string", "bool", "int":
		default:
			return fail("option %q has invalid type %q (want string, bool or int)", name, opt.Type)
		}
	}

	for _, n := range m.Blocks {
		r.blocks[n] = m.ID
	}
	for _, n := range m.Inlines {
		r.inlines[n] = m.ID
	}
	r.byID[m.ID] = len(r.items)
	r.items = append(r.items, registered{plugin: p, manifest: m})
	return nil
}

// All returns the manifests of every registered plugin in registration order.
func (r *Registry) All() []Manifest {
	out := make([]Manifest, len(r.items))
	for i, it := range r.items {
		out[i] = it.manifest
	}
	return out
}

// AssetFS returns the file system of plugin id's assets. It reports false
// when the plugin is unknown or ships no assets. Enablement is not checked.
func (r *Registry) AssetFS(id string) (fs.FS, bool) {
	i, ok := r.byID[id]
	if !ok {
		return nil, false
	}
	ap, ok := r.items[i].plugin.(AssetProvider)
	if !ok {
		return nil, false
	}
	fsys := ap.Assets()
	return fsys, fsys != nil
}

// Enabled returns the plugins enabled for a page, in registration order, and
// warnings for the page.
//
// lookup(id) reports (enabled, options, mentioned) for a registered plugin as
// resolved from config and frontmatter; when not mentioned (or lookup is
// nil), Manifest.Default decides and option defaults apply. Options are
// merged over the manifest's option defaults. mentionedIDs are all plugin ids
// named by config; those not registered produce a warning, as do unknown
// options and options of the wrong type (which keep their default).
func (r *Registry) Enabled(lookup func(id string) (bool, map[string]any, bool), mentionedIDs []string) (*Set, []string) {
	s := &Set{
		config:  map[string]map[string]any{},
		blocks:  map[string]Plugin{},
		inlines: map[string]Plugin{},
	}
	var warnings []string

	seen := map[string]bool{}
	for _, id := range mentionedIDs {
		if _, ok := r.byID[id]; !ok && !seen[id] {
			seen[id] = true
			warnings = append(warnings, fmt.Sprintf("unknown plugin %q", id))
		}
	}

	for _, it := range r.items {
		m := it.manifest
		enabled, opts := m.Default, map[string]any(nil)
		if lookup != nil {
			if on, o, mentioned := lookup(m.ID); mentioned {
				enabled, opts = on, o
			}
		}
		if !enabled {
			continue
		}
		cfg := make(map[string]any, len(m.Config))
		for name, opt := range m.Config {
			cfg[name] = opt.Default
		}
		names := make([]string, 0, len(opts))
		for name := range opts {
			names = append(names, name)
		}
		sort.Strings(names) // stable warning order
		for _, name := range names {
			opt, known := m.Config[name]
			if !known {
				warnings = append(warnings, fmt.Sprintf("plugin %q: unknown option %q", m.ID, name))
				continue
			}
			v, ok := coerce(opt.Type, opts[name])
			if !ok {
				warnings = append(warnings, fmt.Sprintf("plugin %q: option %q must be of type %s", m.ID, name, opt.Type))
				continue
			}
			cfg[name] = v
		}
		s.plugins = append(s.plugins, it.plugin)
		s.manifests = append(s.manifests, m)
		s.config[m.ID] = cfg
		for _, n := range m.Blocks {
			s.blocks[n] = it.plugin
		}
		for _, n := range m.Inlines {
			s.inlines[n] = it.plugin
		}
	}
	return s, warnings
}

// coerce checks v against an option type. Integers may arrive in any integer
// shape, or as an integral float, depending on the config decoder.
func coerce(typ string, v any) (any, bool) {
	switch typ {
	case "string":
		s, ok := v.(string)
		return s, ok
	case "bool":
		b, ok := v.(bool)
		return b, ok
	case "int":
		switch n := v.(type) {
		case int:
			return n, true
		case int8:
			return int(n), true
		case int16:
			return int(n), true
		case int32:
			return int(n), true
		case int64:
			return int(n), int64(int(n)) == n
		case uint:
			return int(n), n <= math.MaxInt
		case uint8:
			return int(n), true
		case uint16:
			return int(n), true
		case uint32:
			return int(n), uint64(n) <= math.MaxInt
		case uint64:
			return int(n), n <= math.MaxInt
		case float32:
			return coerce(typ, float64(n))
		case float64:
			if n == math.Trunc(n) && math.Abs(n) < 1<<53 {
				return int(n), true
			}
		}
	}
	return nil, false
}

// Set is the plugins enabled for one page, with their effective options.
type Set struct {
	plugins   []Plugin
	manifests []Manifest
	config    map[string]map[string]any
	blocks    map[string]Plugin
	inlines   map[string]Plugin
}

// Plugins returns the enabled plugins in registration order.
func (s *Set) Plugins() []Plugin {
	return append([]Plugin(nil), s.plugins...)
}

// Context returns the hook context of plugin id for the page at pagePath.
// Config is a fresh copy of the plugin's effective options; it is empty when
// the plugin is not enabled.
func (s *Set) Context(id, pagePath string) Context {
	cfg := make(map[string]any, len(s.config[id]))
	for k, v := range s.config[id] {
		cfg[k] = v
	}
	return Context{Path: pagePath, Config: cfg}
}

// Block returns the enabled plugin owning block name, or nil.
func (s *Set) Block(name string) Plugin { return s.blocks[name] }

// Inline returns the enabled plugin owning inline name, or nil.
func (s *Set) Inline(name string) Plugin { return s.inlines[name] }

// Allow returns the extra sanitiser rules of all enabled plugins.
func (s *Set) Allow() []AllowRule {
	var out []AllowRule
	for _, m := range s.manifests {
		out = append(out, m.Allow...)
	}
	return out
}

// Assets returns the CSS/JS URLs (/_plugins/<id>/<file>) of the enabled
// plugins that implement AssetProvider, in registration order. Entries that
// are not plain relative paths inside the plugin's file system are dropped.
func (s *Set) Assets() Assets {
	var out Assets
	for i, p := range s.plugins {
		ap, ok := p.(AssetProvider)
		if !ok {
			continue
		}
		id := s.manifests[i].ID
		e := ap.Entry()
		out.CSS = appendURLs(out.CSS, id, e.CSS)
		out.JS = appendURLs(out.JS, id, e.JS)
	}
	return out
}

func appendURLs(dst []string, id string, files []string) []string {
	for _, f := range files {
		if f == "" || strings.ContainsAny(f, `\:?#`) {
			continue
		}
		clean := path.Clean(f)
		if clean == "." || !fs.ValidPath(clean) {
			continue
		}
		dst = append(dst, "/_plugins/"+id+"/"+clean)
	}
	return dst
}
