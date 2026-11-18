package layouts

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/dragosh/zenmd/engine/config"
	"github.com/dragosh/zenmd/engine/plugins"
)

// Default is the name of the layout of a page that chooses none: the built-in
// reading view.
const Default = "article"

// Choice is the layout a page uses and its options, before the layout is
// looked up.
type Choice struct {
	Name     string
	Options  map[string]any
	Warnings []string
}

// Resolve decides the layout of the page at pagePath: its frontmatter, then
// the first matching `layouts:` pattern of the config, then the config's
// `layout:`, then the built-in article. Options come from the frontmatter
// (the short form `layout: {name: post, ...}` and `layoutOptions:`, which wins).
func Resolve(pagePath string, meta map[string]any, d config.LayoutDefaults) Choice {
	c := Choice{Options: map[string]any{}}
	warn := func(format string, args ...any) { c.Warnings = append(c.Warnings, fmt.Sprintf(format, args...)) }

	name, fromPage := "", false
	switch v := meta["layout"].(type) {
	case nil: // no `layout:`, or nothing after it: the page asks for no layout of its own
	case string:
		fromPage = true
		if plugins.ValidName(v) {
			name = v
		} else {
			warn("layout must be a name such as slides")
		}
	case map[string]any:
		fromPage = true
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		n, ok := v["name"].(string)
		switch {
		case !ok || !plugins.ValidName(n):
			warn("layout needs a name, such as layout: { name: post }")
		default:
			name = n
		}
		for _, k := range keys {
			if k != "name" {
				addOption(c.Options, k, v[k], "layout", warn)
			}
		}
	default:
		fromPage = true
		warn("layout must be a name such as slides")
	}
	switch opts := meta["layoutOptions"].(type) {
	case nil:
	case map[string]any:
		keys := make([]string, 0, len(opts))
		for k := range opts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			addOption(c.Options, k, opts[k], "layoutOptions", warn)
		}
	default:
		warn("layoutOptions must be a mapping; ignored")
	}
	if name == "" && !fromPage {
		for _, r := range d.Rules {
			if Match(r.Pattern, pagePath) {
				name = r.Name
				break
			}
		}
		if name == "" {
			name = d.Default
		}
	}
	if name == "" {
		name = Default
	}
	c.Name = name
	return c
}

// addOption stores a scalar option value; anything else is ignored with a warning.
func addOption(dst map[string]any, key string, v any, where string, warn func(string, ...any)) {
	switch v.(type) {
	case string, bool, int, int64, float64:
		dst[key] = v
	default:
		warn("%s.%s must be text, a number or true/false; ignored", where, key)
	}
}

var globs sync.Map // pattern -> *regexp.Regexp (nil: invalid)

// Match reports whether the workspace-relative path matches the glob pattern:
// * matches within one folder level, ** across folders (and a leading **/ also
// matches no folder at all), ? one character. The whole path must match; the
// comparison is case-sensitive.
func Match(pattern, path string) bool {
	if pattern == "" {
		return false
	}
	re, ok := globs.Load(pattern)
	if !ok {
		re, _ = globs.LoadOrStore(pattern, compileGlob(pattern))
	}
	r, _ := re.(*regexp.Regexp)
	return r != nil && r.MatchString(path)
}

func compileGlob(p string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(p); i++ {
		switch c := p[i]; c {
		case '*':
			if i+1 < len(p) && p[i+1] == '*' {
				i++
				if i+1 < len(p) && p[i+1] == '/' { // **/ : zero or more folders
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil
	}
	return re
}
