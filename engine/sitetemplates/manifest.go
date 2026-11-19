// Package sitetemplates starts a folder from a ready-made site template: a
// plain folder with a template.yml and the files to copy. Templates are
// listed from a templates folder the user can edit, built-in presets are
// installed there once, and creating a site never overwrites anything.
//
// Placeholders are plain text replacement; nothing in a template is executed.
package sitetemplates

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Ask is a value asked when a site is created. Default may hold {{folder}}.
type Ask struct{ Key, Label, Default string }

// Needs says which layouts and plugins (built-in ids) a template's files use:
// `requires: { layouts: [...], plugins: [...] }` in template.yml. kit/assembly
// checks an app includes them.
type Needs struct{ Layouts, Plugins []string }

// Template is one site template. Err is set for an invalid one, which is
// listed but cannot be used.
type Template struct {
	ID, Name, Description, Version, Dir string
	Order                               int
	Requires                            Needs // layouts and plugins the template's files use
	Ask                                 []Ask
	Builtin                             bool   // installed from zen's presets
	Update                              string // newer built-in version, "" if none
	Err                                 string // invalid: why

	ignore []string // names or paths not copied
	fsys   fs.FS    // where the files are; set by List and Builtins
}

const (
	maxManifest  = 64 * 1024
	defaultOrder = 100
)

var (
	idRE      = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	askKeyRE  = regexp.MustCompile(`^[a-z][A-Za-z0-9_-]{0,31}$`)
	versionRE = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)
)

// reserved names are filled by zen and cannot be asked.
var reserved = map[string]bool{"date": true, "folder": true}

// ParseManifest reads a template.yml. Unknown keys are an error, so a typo
// never goes unnoticed. The result has no Dir or files; List adds them.
func ParseManifest(data []byte) (Template, error) {
	var t Template
	if len(data) > maxManifest {
		return t, errors.New("template.yml is too large (more than 64 KB)")
	}
	var doc yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF")))).Decode(&doc); err != nil {
		return t, fmt.Errorf("template.yml: %v", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return t, errors.New("template.yml must be a mapping with keys such as id and name")
	}
	m := doc.Content[0]
	seen := map[string]bool{}
	t.Order, t.Version = defaultOrder, "0.0.0"
	for i := 0; i+1 < len(m.Content); i += 2 {
		k, v := m.Content[i].Value, m.Content[i+1]
		seen[k] = true
		str := func(dst *string) error {
			if v.Kind != yaml.ScalarNode {
				return fmt.Errorf("%s must be text", k)
			}
			*dst = strings.TrimSpace(v.Value)
			return nil
		}
		var err error
		switch k {
		case "id":
			err = str(&t.ID)
		case "name":
			err = str(&t.Name)
		case "description":
			err = str(&t.Description)
		case "version":
			var s string
			if err = str(&s); err == nil && s != "" {
				t.Version = s
			}
		case "order":
			if err = v.Decode(&t.Order); err != nil {
				err = errors.New("order must be a whole number")
			}
		case "ignore":
			if v.Kind != yaml.SequenceNode {
				err = errors.New("ignore must be a list of names")
				break
			}
			for _, x := range v.Content {
				if x.Kind != yaml.ScalarNode || x.Value == "" {
					err = errors.New("ignore must be a list of names")
					break
				}
				t.ignore = append(t.ignore, x.Value)
			}
		case "requires":
			t.Requires, err = parseNeeds(v)
		case "ask":
			t.Ask, err = parseAsk(v)
		default:
			err = fmt.Errorf("unknown key %q", k)
		}
		if err != nil {
			return Template{}, fmt.Errorf("template.yml: %v", err)
		}
	}
	switch {
	case !seen["id"] || t.ID == "":
		return Template{}, errors.New("template.yml: id is required (lower case letters, digits and -)")
	case !idRE.MatchString(t.ID):
		return Template{}, fmt.Errorf("template.yml: id %q must use lower case letters, digits and -, and start with a letter", t.ID)
	case t.Name == "":
		return Template{}, errors.New("template.yml: name is required")
	case !versionRE.MatchString(t.Version):
		return Template{}, fmt.Errorf("template.yml: version %q is not a version such as 1.0.0", t.Version)
	}
	return t, nil
}

// parseAsk reads the ask: mapping; title always comes first.
func parseAsk(v *yaml.Node) ([]Ask, error) {
	if v.Kind == yaml.ScalarNode && v.Tag == "!!null" {
		return nil, nil
	}
	if v.Kind != yaml.MappingNode {
		return nil, errors.New("ask must be a mapping of keys to { label, default }")
	}
	var out []Ask
	for i := 0; i+1 < len(v.Content); i += 2 {
		key, spec := v.Content[i].Value, v.Content[i+1]
		switch {
		case !askKeyRE.MatchString(key):
			return nil, fmt.Errorf("ask key %q must start with a lower case letter and use letters, digits, _ and -", key)
		case reserved[key]:
			return nil, fmt.Errorf("ask key %q is filled in by zen and cannot be asked", key)
		}
		a := Ask{Key: key, Label: key}
		if spec.Kind == yaml.ScalarNode && spec.Tag == "!!null" {
			out = append(out, a)
			continue
		}
		if spec.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("ask %s must be a mapping with label and default", key)
		}
		for j := 0; j+1 < len(spec.Content); j += 2 {
			f, x := spec.Content[j].Value, spec.Content[j+1]
			if x.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("ask %s: %s must be text", key, f)
			}
			switch f {
			case "label":
				if x.Value != "" {
					a.Label = x.Value
				}
			case "default":
				a.Default = x.Value
			default:
				return nil, fmt.Errorf("ask %s: unknown key %q", key, f)
			}
		}
		out = append(out, a)
	}
	for i, a := range out {
		if a.Key == "title" && i > 0 {
			copy(out[1:i+1], out[:i])
			out[0] = a
		}
	}
	return out, nil
}

// parseNeeds reads the requires: mapping.
func parseNeeds(v *yaml.Node) (Needs, error) {
	var n Needs
	if v.Kind == yaml.ScalarNode && v.Tag == "!!null" {
		return n, nil
	}
	if v.Kind != yaml.MappingNode {
		return n, errors.New("requires must be a mapping with layouts and plugins")
	}
	for i := 0; i+1 < len(v.Content); i += 2 {
		key, list := v.Content[i].Value, v.Content[i+1]
		var dst *[]string
		switch key {
		case "layouts":
			dst = &n.Layouts
		case "plugins":
			dst = &n.Plugins
		default:
			return Needs{}, fmt.Errorf("requires: unknown key %q (want layouts or plugins)", key)
		}
		if list.Kind != yaml.SequenceNode {
			return Needs{}, fmt.Errorf("requires: %s must be a list of ids", key)
		}
		for _, x := range list.Content {
			if x.Kind != yaml.ScalarNode || !idRE.MatchString(x.Value) {
				return Needs{}, fmt.Errorf("requires: %s must be ids such as docs (lower case letters, digits and -)", key)
			}
			*dst = append(*dst, x.Value)
		}
	}
	return n, nil
}
