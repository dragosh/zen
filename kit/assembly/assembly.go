// Package assembly says what an app is made of. One file, apps/<app>/
// app-assembly.yml, lists the app's add-ons, built-in plugins, layouts, site
// templates and theme; the build includes exactly that (specs/kit/assembly).
//
//	app: zenmark
//	features: all
//	addons: [vale]
//	plugins: all
//	layouts: [article, docs, course]
//	templates: [learning]
//	theme: default
//
// Load reads and checks the file against a Catalog of what exists (BuiltinCatalog
// for the built-ins of this repository); cmd/gen turns it into Go code.
package assembly

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Assembly is one app's make-up. Lists are in the order the app shows them.
type Assembly struct {
	App                                           string
	Features, Addons, Plugins, Layouts, Templates []string
	Theme                                         string
}

// Needs is what a layout or a template requires of the assembly.
type Needs struct{ Layouts, Plugins []string }

// Catalog is what exists: the known ids of each kind (in built-in order) and
// what the layouts and templates require.
type Catalog struct {
	Features, Addons, Plugins, Layouts, Templates, Themes []string
	LayoutNeeds, TemplateNeeds                            map[string]Needs
	// FeatureNeeds says which features a feature requires (none yet; Check reads it).
	FeatureNeeds map[string][]string
}

// DefaultTheme is the look of every app until themes exist.
const DefaultTheme = "default"

// FallbackLayout is always present, listed or not.
const FallbackLayout = "article"

var idRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// ValidID reports whether s may be an id in an assembly (and so in generated
// code and build tags).
func ValidID(s string) bool { return idRE.MatchString(s) }

const maxFile = 64 * 1024

// Load reads the assembly at path, expands `all` against cat and checks it.
// Unknown keys, unknown ids and unmet dependencies are errors that name the
// problem and, for ids, the known ones.
func Load(path string, cat Catalog) (Assembly, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Assembly{}, err
	}
	a, err := parse(data, cat)
	if err != nil {
		return Assembly{}, fmt.Errorf("%s: %w", path, err)
	}
	if err := Check(a, cat); err != nil {
		return Assembly{}, fmt.Errorf("%s: %w", path, err)
	}
	return a, nil
}

func parse(data []byte, cat Catalog) (Assembly, error) {
	var a Assembly
	if len(data) > maxFile {
		return a, errors.New("assembly is too large (more than 64 KB)")
	}
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
	var doc yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&doc); err != nil {
		return a, fmt.Errorf("assembly is not valid YAML: %s", strings.TrimPrefix(err.Error(), "yaml: "))
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return a, errors.New("assembly must be a mapping with keys app, features, addons, plugins, layouts, templates and theme")
	}
	m := doc.Content[0]
	a.Features = slices.Clone(cat.Features) // a missing features key means all, so an old file never makes an app read-only
	seen := map[string]bool{}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k, v := m.Content[i].Value, m.Content[i+1]
		if seen[k] {
			return a, fmt.Errorf("key %q is listed twice", k)
		}
		seen[k] = true
		var err error
		switch k {
		case "app":
			a.App, err = scalar(k, v)
		case "theme":
			a.Theme, err = scalar(k, v)
		case "features":
			a.Features, err = list(k, v, cat.Features)
		case "addons":
			a.Addons, err = list(k, v, cat.Addons)
		case "plugins":
			a.Plugins, err = list(k, v, cat.Plugins)
		case "layouts":
			a.Layouts, err = list(k, v, cat.Layouts)
		case "templates":
			a.Templates, err = list(k, v, cat.Templates)
		default:
			err = fmt.Errorf("unknown key %q; known keys: app, features, addons, plugins, layouts, templates, theme", k)
		}
		if err != nil {
			return Assembly{}, err
		}
	}
	if a.App == "" {
		return Assembly{}, errors.New("app is required (the app id, such as zenmark)")
	}
	if a.Theme == "" {
		a.Theme = DefaultTheme
	}
	return a, nil
}

func scalar(key string, v *yaml.Node) (string, error) {
	if v.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("%s must be a single id", key)
	}
	return strings.TrimSpace(v.Value), nil
}

// list reads a list of ids, or `all` (alone) for every id of the catalog.
func list(key string, v *yaml.Node, all []string) ([]string, error) {
	if v.Kind == yaml.ScalarNode && v.Tag == "!!null" {
		return nil, nil
	}
	if v.Kind == yaml.ScalarNode && v.Value == "all" {
		return slices.Clone(all), nil
	}
	if v.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s must be a list of ids (or all)", key)
	}
	var out []string
	for _, x := range v.Content {
		if x.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("%s must be a list of ids", key)
		}
		out = append(out, strings.TrimSpace(x.Value))
	}
	if slices.Contains(out, "all") {
		if len(out) > 1 {
			return nil, fmt.Errorf("%s: all cannot be mixed with other ids", key)
		}
		return slices.Clone(all), nil
	}
	return out, nil
}

// kinds names each list for messages: key, singular, plural.
var kinds = []struct{ key, one, many string }{
	{"features", "feature", "features"},
	{"addons", "add-on", "add-ons"},
	{"plugins", "plugin", "plugins"},
	{"layouts", "layout", "layouts"},
	{"templates", "template", "templates"},
}

// Check verifies ids (known to the catalog, none twice), the theme and the
// dependencies: a template that needs a layout or plugin the assembly lacks, a
// layout that needs a plugin, and templates without any layout. The fallback
// layout article is always present.
func Check(a Assembly, cat Catalog) error {
	if !ValidID(a.App) {
		return fmt.Errorf("app %q must be an id such as zenmark (lower case letters, digits and -)", a.App)
	}
	lists := [][]string{a.Features, a.Addons, a.Plugins, a.Layouts, a.Templates}
	known := [][]string{cat.Features, cat.Addons, cat.Plugins, cat.Layouts, cat.Templates}
	for i, k := range kinds {
		seen := map[string]bool{}
		for _, id := range lists[i] {
			if !ValidID(id) || !slices.Contains(known[i], id) {
				return fmt.Errorf("unknown %s %q; known %s: %s", k.one, id, k.many, strings.Join(known[i], ", "))
			}
			if seen[id] {
				return fmt.Errorf("%s %q is listed twice", k.one, id)
			}
			seen[id] = true
		}
	}
	if !ValidID(a.Theme) || !slices.Contains(cat.Themes, a.Theme) {
		return fmt.Errorf("unknown theme %q; known themes: %s", a.Theme, strings.Join(cat.Themes, ", "))
	}

	var problems []string
	for _, f := range a.Features {
		for _, need := range cat.FeatureNeeds[f] {
			if !slices.Contains(a.Features, need) {
				problems = append(problems, fmt.Sprintf("feature %s needs feature %s", f, need))
			}
		}
	}
	hasLayout := func(id string) bool { return id == FallbackLayout || slices.Contains(a.Layouts, id) }
	noLayouts := len(a.Layouts) == 0 && len(a.Templates) > 0
	if noLayouts {
		problems = append(problems, "templates are listed but layouts is empty; templates need the layouts they use")
	}
	for _, l := range a.Layouts {
		for _, p := range cat.LayoutNeeds[l].Plugins {
			if !slices.Contains(a.Plugins, p) {
				problems = append(problems, fmt.Sprintf("layout %s needs plugin %s", l, p))
			}
		}
	}
	for _, t := range a.Templates {
		need := cat.TemplateNeeds[t]
		if !noLayouts {
			for _, l := range need.Layouts {
				if !hasLayout(l) {
					problems = append(problems, fmt.Sprintf("template %s needs layout %s", t, l))
				}
			}
		}
		for _, p := range need.Plugins {
			if !slices.Contains(a.Plugins, p) {
				problems = append(problems, fmt.Sprintf("template %s needs plugin %s", t, p))
			}
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}
