package assembly

import (
	"fmt"
	"io/fs"

	"github.com/zenmark-org/zenmd/builtins"
	"github.com/zenmark-org/zenmd/builtins/layouts"
	"github.com/zenmark-org/zenmd/builtins/sitetemplates"
	enginetemplates "github.com/zenmark-org/zenmd/engine/sitetemplates"
)

// knownFeatures are the optional pieces of the app itself (specs/kit/features).
var knownFeatures = []string{"editor"}

// knownAddons are the add-ons that exist: the folders under addons/ (a test
// compares them, and the Taskfile's ADDONS_KNOWN, with this list).
var knownAddons = []string{"vale"}

// BuiltinCatalog is what this repository ships: the add-ons, the built-in
// plugins and layouts, the site templates and the themes, with what the layouts
// and templates declare they need.
func BuiltinCatalog() (Catalog, error) {
	c := Catalog{
		Features:      append([]string(nil), knownFeatures...),
		FeatureNeeds:  map[string][]string{},
		Addons:        append([]string(nil), knownAddons...),
		Plugins:       append([]string(nil), builtins.PluginIDs...),
		Layouts:       append([]string(nil), layouts.IDs...),
		Templates:     append([]string(nil), sitetemplates.IDs...),
		Themes:        []string{DefaultTheme},
		LayoutNeeds:   map[string]Needs{},
		TemplateNeeds: map[string]Needs{},
	}
	for _, l := range layouts.All() {
		m := l.Manifest()
		if len(m.Requires) > 0 {
			c.LayoutNeeds[m.ID] = Needs{Plugins: append([]string(nil), m.Requires...)}
		}
	}
	for _, id := range c.Templates {
		data, err := fs.ReadFile(sitetemplates.FS, id+"/template.yml")
		if err != nil {
			return Catalog{}, fmt.Errorf("site template %s: %w", id, err)
		}
		t, err := enginetemplates.ParseManifest(data)
		if err != nil {
			return Catalog{}, fmt.Errorf("site template %s: %w", id, err)
		}
		c.TemplateNeeds[id] = Needs{Layouts: t.Requires.Layouts, Plugins: t.Requires.Plugins}
	}
	return c, nil
}
