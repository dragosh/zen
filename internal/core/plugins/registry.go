package plugins

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"net/url"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
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
	denied  []Manifest        // plugins the user did not allow

	user     atomic.Pointer[userOptions]
	assetVer atomic.Pointer[string]
}

// SetAssetVersion sets the ?v= value of the asset URLs of plugins that are
// not AssetVersioners (the app version, for the built-in plugins).
func (r *Registry) SetAssetVersion(v string) { r.assetVer.Store(&v) }

// AssetVersion returns what SetAssetVersion set, "" when nothing was set.
func (r *Registry) AssetVersion() string {
	if v := r.assetVer.Load(); v != nil {
		return *v
	}
	return ""
}

type userOptions struct {
	get func(id string) map[string]any
}

// SetUserOptions sets where options with scope "user" take their values
// from: the options the user's global config sets for plugin id (nil for
// none). Without it those options keep their defaults.
func (r *Registry) SetUserOptions(get func(id string) map[string]any) {
	r.user.Store(&userOptions{get: get})
}

func (r *Registry) userOptionsOf(id string) map[string]any {
	if u := r.user.Load(); u != nil && u.get != nil {
		return u.get(id)
	}
	return nil
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		byID:    map[string]int{},
		blocks:  map[string]string{},
		inlines: map[string]string{},
	}
}

// Plugins returns the registered plugins in registration order.
func (r *Registry) Plugins() []Plugin {
	out := make([]Plugin, len(r.items))
	for i, it := range r.items {
		out[i] = it.plugin
	}
	return out
}

// Deny records a plugin that exists but may not be used (not allowed by the
// user). It owns no names; pages that use its names render plain and get a
// warning. Names owned by a registered plugin are never reported as denied.
func (r *Registry) Deny(m Manifest) { r.denied = append(r.denied, m) }

// Register validates p's manifest and adds the plugin. Nothing is stored
// when an error is returned.
// Reserved ids are switches under `plugins:` handled by the app rather than
// the render pipeline (e.g. `lint` for the Vale linter). They never produce
// an "unknown plugin" warning and cannot be registered.
var Reserved = map[string]bool{"lint": true}

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
	if Reserved[m.ID] {
		return fail("id is reserved")
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
		case "string", "bool", "int", "map", "list":
		default:
			return fail("option %q has invalid type %q (want string, bool, int, map or list)", name, opt.Type)
		}
		switch opt.Scope {
		case "", ScopeUser:
		default:
			return fail("option %q has invalid scope %q (want user or nothing)", name, opt.Scope)
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
		version: r.AssetVersion(),
		config:  map[string]map[string]any{},
		blocks:  map[string]Plugin{},
		inlines: map[string]Plugin{},
		denied:  map[string]map[string]string{"block": {}, "inline": {}},
	}
	var warnings []string
	deniedIDs := map[string]bool{}
	for _, m := range r.denied {
		deniedIDs[m.ID] = true
	}

	seen := map[string]bool{}
	for _, id := range mentionedIDs {
		if Reserved[id] {
			continue // a built-in switch read elsewhere, not a render plugin
		}
		if _, ok := r.byID[id]; !ok && !seen[id] && !deniedIDs[id] {
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
		hasUser := false
		for name, opt := range m.Config {
			cfg[name] = cloneOption(opt.Default)
			hasUser = hasUser || opt.Scope == ScopeUser
		}
		var user map[string]any
		if hasUser {
			user = r.userOptionsOf(m.ID)
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
			if opt.Scope == ScopeUser {
				// Only the global config counts; the merged value is a conflict
				// unless it is that same value.
				if uv, set := user[name]; !set || !reflect.DeepEqual(uv, opts[name]) {
					warnings = append(warnings, fmt.Sprintf("plugin %q: option %q can only be set in the global config; ignored", m.ID, name))
				}
				continue
			}
			v, ok := coerce(opt.Type, opts[name])
			if !ok {
				warnings = append(warnings, fmt.Sprintf("plugin %q: option %q must be of type %s", m.ID, name, opt.Type))
				continue
			}
			cfg[name] = v
		}
		unames := make([]string, 0, len(user))
		for name := range user {
			unames = append(unames, name)
		}
		sort.Strings(unames)
		for _, name := range unames {
			opt, known := m.Config[name]
			if !known || opt.Scope != ScopeUser {
				continue // other options of the global config come through the normal merge
			}
			v, ok := coerce(opt.Type, user[name])
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

	for _, m := range r.denied {
		on := m.Default
		if lookup != nil {
			if e, _, mentioned := lookup(m.ID); mentioned {
				on = e
			}
		}
		if !on {
			continue
		}
		for _, k := range []struct {
			kind  string
			names []string
			owned map[string]string
		}{{"block", m.Blocks, r.blocks}, {"inline", m.Inlines, r.inlines}} {
			for _, n := range k.names {
				if _, taken := k.owned[n]; !taken {
					if _, dup := s.denied[k.kind][n]; !dup {
						s.denied[k.kind][n] = m.ID
					}
				}
			}
		}
	}
	return s, warnings
}

// cloneOption copies a map or list default so no two pages share one.
func cloneOption(v any) any {
	switch t := v.(type) {
	case map[string]string:
		out := make(map[string]string, len(t))
		for k, x := range t {
			out[k] = x
		}
		return out
	case []string:
		return append([]string{}, t...)
	}
	return v
}

// CoerceOption checks v against an option type and returns it in its
// canonical Go shape (string, bool, int, map[string]string or []string).
func CoerceOption(typ string, v any) (any, bool) { return coerce(typ, v) }

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
	case "map":
		switch t := v.(type) {
		case map[string]string:
			return cloneOption(t), true
		case map[string]any:
			out := make(map[string]string, len(t))
			for k, x := range t {
				str, ok := x.(string)
				if !ok {
					return nil, false
				}
				out[k] = str
			}
			return out, true
		}
		return nil, false
	case "list":
		switch t := v.(type) {
		case []string:
			return cloneOption(t), true
		case []any:
			out := make([]string, 0, len(t))
			for _, x := range t {
				str, ok := x.(string)
				if !ok {
					return nil, false
				}
				out = append(out, str)
			}
			return out, true
		}
		return nil, false
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
	denied    map[string]map[string]string // kind -> name -> id of the denied plugin
	version   string                       // registry's asset version
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
		cfg[k] = cloneOption(v)
	}
	return Context{Path: pagePath, Config: cfg}
}

// Block returns the enabled plugin owning block name, or nil.
func (s *Set) Block(name string) Plugin { return s.blocks[name] }

// Inline returns the enabled plugin owning inline name, or nil.
func (s *Set) Inline(name string) Plugin { return s.inlines[name] }

// Denied returns the id of the not-allowed plugin that owns name of the given
// kind ("block" or "inline"), or "".
func (s *Set) Denied(kind, name string) string { return s.denied[kind][name] }

// Allow returns the extra sanitiser rules of all enabled plugins.
func (s *Set) Allow() []AllowRule {
	var out []AllowRule
	for _, m := range s.manifests {
		out = append(out, m.Allow...)
	}
	return out
}

// AllowByPlugin returns the sanitiser rules of the enabled plugins by plugin
// id; a plugin's rules apply only inside that plugin's own wrapper.
func (s *Set) AllowByPlugin() map[string][]AllowRule {
	out := map[string][]AllowRule{}
	for _, m := range s.manifests {
		if len(m.Allow) > 0 {
			out[m.ID] = append([]AllowRule(nil), m.Allow...)
		}
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
		ver := s.version
		if v, ok := p.(AssetVersioner); ok {
			ver = v.AssetVersion()
		}
		out.CSS = appendURLs(out.CSS, id, ver, e.CSS)
		out.JS = appendURLs(out.JS, id, ver, e.JS)
	}
	return out
}

func appendURLs(dst []string, id, ver string, files []string) []string {
	for _, f := range files {
		if f == "" || strings.ContainsAny(f, `\:?#`) {
			continue
		}
		clean := path.Clean(f)
		if clean == "." || !fs.ValidPath(clean) {
			continue
		}
		u := "/_plugins/" + id + "/" + clean
		if ver != "" {
			u += "?v=" + url.QueryEscape(ver)
		}
		dst = append(dst, u)
	}
	return dst
}
