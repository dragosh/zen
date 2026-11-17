// Package config answers "which settings apply to this page" by merging
// defaults, the global config file, the workspace `.zen.yml` and the page's
// frontmatter. Configuration is data only: nothing in it is executed,
// included or resolved as a path.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

const (
	// WorkspaceFile is the name of the workspace config file in the root.
	WorkspaceFile = ".zen.yml"

	schemaVersion  = 1
	defaultTheme   = "system"
	maxFrontmatter = 64 * 1024   // bytes of YAML between the delimiters
	maxConfigFile  = 1024 * 1024 // bytes read from a config file
)

// plugin is what one or more merged layers say about a plugin.
type plugin struct {
	enabled bool
	options map[string]any
}

// Config is the merged global + workspace configuration.
type Config struct {
	// Extra holds unknown top-level keys from the config files (workspace
	// wins over global). Each one also produces a warning.
	Extra map[string]any

	theme    string
	plugins  map[string]plugin
	warnings []string
	limits   Limits
	layouts  LayoutDefaults

	templates any            // the global templates: key, read by TemplatesDir
	build     map[string]any // merged build: sections, checked by Build
	buildErr  error          // a build: key that is not a mapping
}

// LayoutRule maps a path pattern (glob: * within a folder, ** across folders)
// to the name of a layout.
type LayoutRule struct{ Pattern, Name string }

// LayoutDefaults are the layouts the config files choose for pages that do not
// name one: the layout of every page, and the rules by path. Rules are tried
// in order and the first match wins; the workspace's rules come before the
// global ones.
type LayoutDefaults struct {
	Default string
	Rules   []LayoutRule
}

// Limits caps the folder scan when a workspace opens (see core/workspace).
// Zero means "use the built-in default". Only the global config sets them,
// so a folder cannot raise its own limits.
type Limits struct {
	Files   int // markdown files
	Entries int // files and folders visited
}

// Hard ceilings for configured limits.
const (
	MaxLimitFiles   = 100_000
	MaxLimitEntries = 5_000_000
)

// Limits returns the configured folder limits (zero fields: defaults).
func (c *Config) Limits() Limits {
	if c == nil {
		return Limits{}
	}
	return c.limits
}

// Settings are the effective settings for one page.
type Settings struct {
	Theme    string
	Warnings []string

	plugins map[string]plugin
	layouts LayoutDefaults
}

// Layouts returns the default layout and the layout rules of the config files.
func (s Settings) Layouts() LayoutDefaults {
	return LayoutDefaults{Default: s.layouts.Default, Rules: append([]LayoutRule(nil), s.layouts.Rules...)}
}

// GlobalPath returns <os.UserConfigDir()>/zen/config.yml.
func GlobalPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "zen", "config.yml"), nil
}

// Default returns the built-in configuration.
func Default() *Config {
	return &Config{
		Extra:   map[string]any{},
		theme:   defaultTheme,
		plugins: map[string]plugin{},
	}
}

// Load reads the global file and <workspaceRoot>/.zen.yml on top of the
// defaults. A missing file is not an error; an unreadable or invalid file
// contributes nothing but a warning. An empty path skips that layer.
func Load(globalPath, workspaceRoot string) *Config {
	c := Default()
	if globalPath != "" {
		c.loadFile(globalPath, true)
	}
	if workspaceRoot != "" {
		c.loadFile(filepath.Join(workspaceRoot, WorkspaceFile), false)
	}
	return c
}

// Warnings returns the problems found while loading the config files.
func (c *Config) Warnings() []string {
	return append([]string(nil), c.warnings...)
}

// ForPage returns the effective settings for a page with the given
// frontmatter: defaults < global < workspace < frontmatter.
func (c *Config) ForPage(frontmatter map[string]any) Settings {
	if c == nil {
		c = Default()
	}
	s := Settings{
		Theme:    c.theme,
		Warnings: append([]string(nil), c.warnings...),
		plugins:  make(map[string]plugin, len(c.plugins)),
		layouts:  LayoutDefaults{Default: c.layouts.Default, Rules: append([]LayoutRule(nil), c.layouts.Rules...)},
	}
	for id, p := range c.plugins {
		s.plugins[id] = plugin{enabled: p.enabled, options: cloneMap(p.options)}
	}
	warn := func(format string, args ...any) {
		s.Warnings = append(s.Warnings, "frontmatter: "+fmt.Sprintf(format, args...))
	}
	if v, ok := frontmatter["theme"]; ok {
		if t, ok := v.(string); ok && validTheme(t) {
			s.Theme = t
		} else {
			warn("theme must be system, light or dark")
		}
	}
	if v, ok := frontmatter["plugins"]; ok && v != nil {
		mergePlugins(s.plugins, v, warn)
	}
	return s
}

// Plugin reports whether the plugin is enabled and with which options.
// mentioned is false when no layer names the plugin; the caller then uses
// the plugin's own default. The options map is a copy and never nil.
func (s Settings) Plugin(id string) (enabled bool, options map[string]any, mentioned bool) {
	p, ok := s.plugins[id]
	if !ok {
		return false, map[string]any{}, false
	}
	options = cloneMap(p.options)
	if options == nil {
		options = map[string]any{}
	}
	return p.enabled, options, true
}

// SplitFrontmatter separates a leading YAML block (between a first line
// `---` and the next `---` or `...` line) from the body. Without
// frontmatter it returns an empty map and src unchanged. Invalid or
// non-mapping YAML returns an empty map, the body without the block, and a
// warning. A block larger than 64 KB is treated as body.
func SplitFrontmatter(src []byte) (meta map[string]any, body []byte, warning string) {
	meta = map[string]any{}
	rest := bytes.TrimPrefix(src, []byte("\xEF\xBB\xBF"))

	line, next := cutLine(rest)
	if !isDelimiter(line, false) {
		return meta, src, ""
	}
	start := next
	for pos := start; pos < len(rest); {
		if pos-start > maxFrontmatter {
			return meta, src, ""
		}
		line, n := cutLine(rest[pos:])
		if isDelimiter(line, true) {
			block := rest[start:pos]
			body = rest[pos+n:]
			m, err := decodeMap(block)
			if err != nil {
				return meta, body, "frontmatter ignored: " + err.Error()
			}
			if m != nil {
				meta = m
			}
			return meta, body, ""
		}
		pos += n
	}
	return meta, src, "" // never closed: not frontmatter
}

// cutLine returns the first line of b without its line ending, and the
// number of bytes the line occupies including the ending.
func cutLine(b []byte) (line []byte, n int) {
	i := bytes.IndexByte(b, '\n')
	if i < 0 {
		return b, len(b)
	}
	return bytes.TrimSuffix(b[:i], []byte("\r")), i + 1
}

func isDelimiter(line []byte, closing bool) bool {
	line = bytes.TrimRight(line, " \t\r")
	return string(line) == "---" || (closing && string(line) == "...")
}

// decodeMap decodes one YAML document into plain maps. An empty document
// yields a nil map; anything but a mapping is an error. yaml.v3 itself
// rejects documents with excessive alias expansion.
func decodeMap(data []byte) (map[string]any, error) {
	var v any
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&v); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if v == nil {
		return nil, nil
	}
	m, ok := normalize(v).(map[string]any)
	if !ok {
		return nil, errors.New("expected a YAML mapping")
	}
	return m, nil
}

// normalize turns map[any]any (non-string keys) into map[string]any so the
// result is JSON-friendly plain data.
func normalize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, e := range t {
			t[k] = normalize(e)
		}
		return t
	case map[any]any:
		m := make(map[string]any, len(t))
		for k, e := range t {
			m[fmt.Sprint(k)] = normalize(e)
		}
		return m
	case []any:
		for i, e := range t {
			t[i] = normalize(e)
		}
		return t
	}
	return v
}

func (c *Config) loadFile(path string, global bool) {
	warn := func(format string, args ...any) {
		c.warnings = append(c.warnings, path+": "+fmt.Sprintf(format, args...))
	}
	data, err := readCapped(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			warn("ignored: %v", err)
		}
		return
	}
	doc, err := decodeMap(bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF")))
	if err != nil {
		warn("ignored: %v", err)
		return
	}
	if v, ok := doc["version"]; ok {
		n, isInt := v.(int)
		switch {
		case !isInt || n < 1:
			warn("ignored: version must be %d", schemaVersion)
			return
		case n > schemaVersion:
			warn("ignored: version %d is newer than this zen understands (%d)", n, schemaVersion)
			return
		}
	}

	keys := make([]string, 0, len(doc))
	for k := range doc {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := doc[k]
		switch k {
		case "version":
		case "theme":
			if t, ok := v.(string); ok && validTheme(t) {
				c.theme = t
			} else {
				warn("theme must be system, light or dark")
			}
		case "plugins":
			if v != nil {
				mergePlugins(c.plugins, v, warn)
			}
		case "layout":
			if n, ok := v.(string); ok && validLayoutName(n) {
				c.layouts.Default = n
			} else if v != nil {
				warn("layout must be a layout name such as docs")
			}
		case "layouts":
			if v == nil {
				continue
			}
			rules, ok := orderedRules(data, warn)
			if !ok {
				warn("layouts must be a mapping of path patterns to layout names")
				continue
			}
			// The file's own rules come before those of the layers below it.
			c.layouts.Rules = append(rules, c.layouts.Rules...)
		case "build":
			c.mergeBuild(v)
		case "templates":
			if !global {
				warn("templates can only be set in the global config")
				continue
			}
			c.templates = v
		case "limits":
			if !global {
				warn("limits can only be set in the global config")
				continue
			}
			c.limits = parseLimits(v, warn)
		default:
			c.Extra[k] = v
			warn("unknown key %q", k)
		}
	}
}

// validLayoutName reports whether n looks like a layout id: [a-z][a-z0-9-]*.
func validLayoutName(n string) bool {
	if n == "" || n[0] < 'a' || n[0] > 'z' {
		return false
	}
	for i := 0; i < len(n); i++ {
		c := n[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

// orderedRules reads the `layouts` mapping of a config file in file order (a
// Go map would lose it). Entries whose pattern or name is not valid are left
// out with a warning. ok is false when the key is not a mapping.
func orderedRules(data []byte, warn func(string, ...any)) (rules []LayoutRule, ok bool) {
	var doc yaml.Node
	if yaml.Unmarshal(bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF")), &doc) != nil || len(doc.Content) == 0 {
		return nil, false
	}
	top := doc.Content[0]
	for i := 0; i+1 < len(top.Content); i += 2 {
		if top.Content[i].Value != "layouts" {
			continue
		}
		m := top.Content[i+1]
		if m.Kind != yaml.MappingNode {
			return nil, false
		}
		for j := 0; j+1 < len(m.Content); j += 2 {
			pattern, val := m.Content[j].Value, m.Content[j+1]
			if val.Kind != yaml.ScalarNode || val.Tag != "!!str" || !validLayoutName(val.Value) {
				warn("layouts: %q must name a layout such as docs", pattern)
				continue
			}
			if pattern == "" {
				warn("layouts: a pattern is empty")
				continue
			}
			rules = append(rules, LayoutRule{Pattern: pattern, Name: val.Value})
		}
		return rules, true
	}
	return nil, false
}

func parseLimits(v any, warn func(string, ...any)) Limits {
	var l Limits
	m, ok := v.(map[string]any)
	if !ok {
		if v != nil {
			warn("limits must be a mapping")
		}
		return l
	}
	get := func(key string, max int) int {
		raw, ok := m[key]
		if !ok {
			return 0
		}
		n, ok := raw.(int)
		switch {
		case !ok || n < 1:
			warn("limits.%s must be a whole number above 0", key)
			return 0
		case n > max:
			warn("limits.%s is capped at %d", key, max)
			return max
		}
		return n
	}
	l.Files = get("files", MaxLimitFiles)
	l.Entries = get("entries", MaxLimitEntries)
	for k := range m {
		if k != "files" && k != "entries" {
			warn("unknown key limits.%s", k)
		}
	}
	return l
}

func readCapped(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxConfigFile+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxConfigFile {
		return nil, fmt.Errorf("larger than %d bytes", maxConfigFile)
	}
	return data, nil
}

func validTheme(t string) bool {
	return t == "system" || t == "light" || t == "dark"
}

// mergePlugins applies one layer's `plugins` value on top of dst. A bool
// switches the plugin and keeps lower-layer options; a mapping switches it
// on and merges options key by key.
func mergePlugins(dst map[string]plugin, layer any, warn func(string, ...any)) {
	m, ok := layer.(map[string]any)
	if !ok {
		warn("plugins must be a mapping of plugin id to true, false or options")
		return
	}
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		p := dst[id]
		switch v := m[id].(type) {
		case bool:
			p.enabled = v
		case map[string]any:
			p.enabled = true
			if p.options == nil {
				p.options = make(map[string]any, len(v))
			}
			for k, o := range v {
				p.options[k] = cloneValue(o)
			}
		default:
			warn("plugin %q must be true, false or a mapping of options", id)
			continue
		}
		dst[id] = p
	}
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = cloneValue(v)
	}
	return out
}

func cloneValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return cloneMap(t)
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = cloneValue(e)
		}
		return out
	}
	return v
}
