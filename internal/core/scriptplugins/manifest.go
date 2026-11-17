package scriptplugins

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dragosh/zen/internal/core/extplugins"
	"github.com/dragosh/zen/internal/core/plugins"
)

// spec is what the plugin map declares beyond the manifest.
type spec struct {
	hooks   map[string]bool
	read    bool // permissions: ["read"]
	timeout time.Duration
	css     string   // inline css, served as inline.css
	js      []string // folder form: page scripts
	sheets  []string // folder form: style sheets
}

var hookNames = []string{"block", "inline", "source", "html", "action"}

var knownKeys = map[string]bool{
	"version": true, "description": true, "default": true, "blocks": true, "inlines": true, "layouts": true,
	"hooks": true, "permissions": true, "timeout": true, "css": true, "js": true, "stylesheets": true, "options": true, "help": true,
}

var (
	urlRE   = regexp.MustCompile(`(?i)url\(\s*(?:"([^"]*)"|'([^']*)'|([^)]*))\s*\)`)
	cssBad  = regexp.MustCompile(`(?i)@import|https?:|javascript:|expression\s*\(|-moz-binding|behavior\s*:`)
	semver  = plugins.ValidSemVer
	nameLbl = map[string]string{"blocks": "block", "inlines": "inline", "layouts": "layout"}
)

func stringList(m map[string]any, key string) ([]string, bool, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, false, nil
	}
	list, isList := v.([]any)
	if !isList {
		return nil, true, fmt.Errorf("%s must be a list of strings", key)
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		s, isStr := e.(string)
		if !isStr {
			return nil, true, fmt.Errorf("%s must be a list of strings", key)
		}
		out = append(out, s)
	}
	return out, true, nil
}

// checkCSS refuses inline css that loads anything: no @import, no address, no
// url() other than data: images and fragments.
func checkCSS(css string) error {
	if len(css) > maxCSS {
		return fmt.Errorf("css is larger than %d KB", maxCSS>>10)
	}
	if cssBad.MatchString(css) {
		return fmt.Errorf("css may not use @import, web addresses or scripts (a plugin has no network)")
	}
	for _, m := range urlRE.FindAllStringSubmatch(css, -1) {
		arg := strings.TrimSpace(m[1] + m[2] + m[3])
		if !strings.HasPrefix(strings.ToLower(arg), "data:image/") && !strings.HasPrefix(arg, "#") {
			return fmt.Errorf("css may only use url() for data: images, not %q", arg)
		}
	}
	return nil
}

// parse validates the plugin map object and fills the manifest and spec.
func (p *Plugin) parse(id string, res *msg) error {
	if res.Kind == kindNone {
		return fmt.Errorf(`the script must define the plugin map: plugin := {version: "1.0.0", blocks: ["name"]}`)
	}
	if res.Kind != kindMap {
		return fmt.Errorf("plugin must be a map (plugin := {version: …}), not a %s", res.Type)
	}
	v, err := decodeJSON(res.JSON)
	if err != nil {
		return fmt.Errorf("plugin: %v", err)
	}
	m, _ := normNumbers(v).(map[string]any)
	if m == nil {
		return fmt.Errorf("plugin must be a map (plugin := {version: …}), not a %s", res.Type)
	}
	for k := range m {
		if !knownKeys[k] {
			return fmt.Errorf("unknown key %q in plugin", k)
		}
	}
	version, ok := m["version"].(string)
	if !ok || version == "" {
		return fmt.Errorf(`version is required: a SemVer string such as "1.0.0"`)
	}
	if !semver(version) {
		return fmt.Errorf("version %q is not valid SemVer (such as 1.0.0)", version)
	}
	man := plugins.Manifest{ID: id, Version: version, APIVersion: plugins.APIVersion, Default: true}
	if d, present := m["description"]; present {
		s, isStr := d.(string)
		if !isStr {
			return fmt.Errorf("description must be a string")
		}
		man.Description = s
	}
	if d, present := m["default"]; present {
		b, isBool := d.(bool)
		if !isBool {
			return fmt.Errorf("default must be true or false")
		}
		man.Default = b
	}
	for _, l := range []struct {
		key string
		dst *[]string
	}{{"blocks", &man.Blocks}, {"inlines", &man.Inlines}, {"layouts", &man.Layouts}} {
		names, _, err := stringList(m, l.key)
		if err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, n := range names {
			if !plugins.ValidName(n) {
				return fmt.Errorf("invalid %s name %q (want [a-z][a-z0-9-]*)", nameLbl[l.key], n)
			}
			if seen[n] {
				return fmt.Errorf("%s %q is listed twice", nameLbl[l.key], n)
			}
			seen[n] = true
		}
		*l.dst = names
	}
	sp := spec{hooks: map[string]bool{}, timeout: defaultTimeout}
	if len(man.Blocks) > 0 {
		sp.hooks["block"] = true
	}
	if len(man.Inlines) > 0 {
		sp.hooks["inline"] = true
	}
	hooks, _, err := stringList(m, "hooks")
	if err != nil {
		return err
	}
	for _, h := range hooks {
		known := false
		for _, k := range hookNames {
			known = known || k == h
		}
		if !known {
			return fmt.Errorf("unknown hook %q in hooks (any of: %s)", h, strings.Join(hookNames, ", "))
		}
		if (h == "block" && len(man.Blocks) == 0) || (h == "inline" && len(man.Inlines) == 0) {
			return fmt.Errorf("hooks lists %s but %ss is empty", h, h)
		}
		sp.hooks[h] = true
	}
	perms, _, err := stringList(m, "permissions")
	if err != nil {
		return err
	}
	for _, perm := range perms {
		if perm != "read" {
			return fmt.Errorf(`permission %q is not known (the only one is "read")`, perm)
		}
		sp.read = true
	}
	if t, present := m["timeout"]; present {
		s, isStr := t.(string)
		if !isStr {
			return fmt.Errorf(`timeout must be a duration string such as "500ms" or "2s"`)
		}
		d, err := time.ParseDuration(s)
		if err != nil || d <= 0 {
			return fmt.Errorf("timeout %q is not a duration such as 500ms or 2s", s)
		}
		if d > maxTimeout {
			return fmt.Errorf("timeout %s is over the limit of %s", s, maxTimeout)
		}
		sp.timeout = d
	}
	if c, present := m["css"]; present {
		s, isStr := c.(string)
		if !isStr {
			return fmt.Errorf("css must be a string")
		}
		if err := checkCSS(s); err != nil {
			return err
		}
		sp.css = s
	}
	for _, f := range []struct {
		key string
		dst *[]string
	}{{"js", &sp.js}, {"stylesheets", &sp.sheets}} {
		files, present, err := stringList(m, f.key)
		if err != nil {
			return err
		}
		if !present {
			continue
		}
		if p.real == "" {
			return fmt.Errorf("%s files need the folder form (%s/plugin.tengo)", f.key, id)
		}
		for _, name := range files {
			if name == "inline.css" {
				return fmt.Errorf("%s: inline.css is reserved for the css key", f.key)
			}
			if err := extplugins.CheckInside(p.real, name, "frontend file"); err != nil {
				return err
			}
			*f.dst = append(*f.dst, strings.TrimPrefix(cleanSlash(name), "./"))
		}
	}
	if opts, present := m["options"]; present {
		om, isMap := opts.(map[string]any)
		if !isMap {
			return fmt.Errorf("options must be a map of option names")
		}
		names := make([]string, 0, len(om))
		for n := range om {
			names = append(names, n)
		}
		sort.Strings(names)
		man.Config = map[string]plugins.Option{}
		for _, n := range names {
			o, isMap := om[n].(map[string]any)
			if !isMap {
				return fmt.Errorf("option %q must be a map {type, default, description}", n)
			}
			for k := range o {
				if k != "type" && k != "default" && k != "description" && k != "scope" {
					return fmt.Errorf("option %q: unknown key %q", n, k)
				}
			}
			typ, _ := o["type"].(string)
			desc, _ := o["description"].(string)
			scope, _ := o["scope"].(string)
			opt, err := extplugins.ParseOption(typ, normaliseDefault(o["default"]), desc, scope)
			if err != nil {
				return fmt.Errorf("option %q: %v", n, err)
			}
			man.Config[n] = opt
		}
	}
	if h, present := m["help"]; present {
		if man.Help, err = plugins.ParseHelp(h); err != nil {
			return err
		}
	}
	if len(sp.hooks) == 0 && len(man.Layouts) == 0 && len(sp.js) == 0 {
		return fmt.Errorf("the plugin declares nothing: list blocks, inlines or hooks")
	}
	p.man, p.spec = man, sp
	return nil
}

// normaliseDefault turns the int64 Tengo numbers become into the int that
// option defaults use.
func normaliseDefault(v any) any {
	if n, ok := v.(int64); ok {
		return int(n)
	}
	return v
}

func cleanSlash(name string) string {
	return strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
}

// ---- assets --------------------------------------------------------------

// Entry lists the plugin's frontend files: inline.css (from the css key) and
// the folder form's stylesheets and scripts (plugins.AssetProvider).
func (p *Plugin) Entry() plugins.Assets {
	var a plugins.Assets
	if p.spec.css != "" {
		a.CSS = append(a.CSS, inlineCSS)
	}
	a.CSS = append(a.CSS, p.spec.sheets...)
	a.JS = append(a.JS, p.spec.js...)
	return a
}

const inlineCSS = "inline.css"

// Assets serves the frontend files, nothing else; nil without any.
func (p *Plugin) Assets() fs.FS {
	e := p.Entry()
	if len(e.CSS)+len(e.JS) == 0 {
		return nil
	}
	files := map[string]bool{}
	for _, f := range append(append([]string(nil), p.spec.js...), p.spec.sheets...) {
		files[f] = true
	}
	return assetFS{p: p, files: files}
}

type assetFS struct {
	p     *Plugin
	files map[string]bool // folder-form files
}

func (a assetFS) Open(name string) (fs.File, error) {
	notExist := &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	if !fs.ValidPath(name) || a.p.Changed() {
		return nil, notExist
	}
	if name == inlineCSS && a.p.spec.css != "" {
		return newMemFile(inlineCSS, a.p.spec.css), nil
	}
	if !a.files[name] || a.p.real == "" {
		return nil, notExist
	}
	full, err := extplugins.ResolveInside(a.p.real, name, "file")
	if err != nil {
		return nil, notExist
	}
	f, err := os.Open(full)
	if err != nil {
		return nil, notExist
	}
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() {
		f.Close()
		return nil, notExist
	}
	return f, nil
}

// normNumbers turns the json.Number values of a decoded plugin map into int64
// (whole numbers) or float64, as Tengo's numbers are.
func normNumbers(v any) any {
	switch v := v.(type) {
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i
		}
		f, _ := v.Float64()
		return f
	case []any:
		for i := range v {
			v[i] = normNumbers(v[i])
		}
	case map[string]any:
		for k := range v {
			v[k] = normNumbers(v[k])
		}
	}
	return v
}
