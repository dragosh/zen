package config

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
)

// Build is the build: section of the config files: what `zen build` does
// when the command line does not say. Zero values mean "not set".
type Build struct {
	Out      string   // output folder, relative to the folder being built
	ID       string   // storage namespace for readers; "" = the folder name
	EditURL  string   // link of the docs layout's Edit action; {path} is the page's path
	Drafts   bool     // include pages with draft: true
	Portable bool     // output that opens from file://
	Exclude  []string // globs of pages left out, on top of the app's excludes
}

// siteIDRE is what a site id may look like: it becomes part of a storage key.
var siteIDRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// mergeBuild adds a build: section; the workspace file is loaded after the
// global one, so its keys win one by one.
func (c *Config) mergeBuild(v any) {
	if v == nil {
		return
	}
	m, ok := v.(map[string]any)
	if !ok {
		c.buildErr = errors.New("build must be a mapping with keys such as out and portable")
		return
	}
	if c.build == nil {
		c.build = map[string]any{}
	}
	for k, x := range m {
		c.build[k] = x
	}
}

// Build returns the merged build: section. Unknown keys are returned as
// warnings (sorted); a value of the wrong type is an error naming the key.
func (c *Config) Build() (Build, []string, error) {
	var b Build
	if c == nil {
		return b, nil, nil
	}
	if c.buildErr != nil {
		return b, nil, c.buildErr
	}
	keys := make([]string, 0, len(c.build))
	for k := range c.build {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var warns []string
	str := func(k string, dst *string) error {
		s, ok := c.build[k].(string)
		if !ok {
			return fmt.Errorf("build.%s must be text", k)
		}
		*dst = s
		return nil
	}
	flag := func(k string, dst *bool) error {
		v, ok := c.build[k].(bool)
		if !ok {
			return fmt.Errorf("build.%s must be true or false", k)
		}
		*dst = v
		return nil
	}
	for _, k := range keys {
		var err error
		switch k {
		case "out":
			err = str(k, &b.Out)
		case "id":
			if err = str(k, &b.ID); err == nil && !siteIDRE.MatchString(b.ID) {
				err = errors.New("build.id must be 1 to 64 letters, digits, '.', '_' or '-'")
			}
		case "editURL":
			err = str(k, &b.EditURL)
		case "drafts":
			err = flag(k, &b.Drafts)
		case "portable":
			err = flag(k, &b.Portable)
		case "exclude":
			list, ok := c.build[k].([]any)
			if !ok {
				return Build{}, warns, errors.New("build.exclude must be a list of globs")
			}
			for _, e := range list {
				s, ok := e.(string)
				if !ok {
					return Build{}, warns, errors.New("build.exclude must be a list of globs (text)")
				}
				b.Exclude = append(b.Exclude, s)
			}
		default:
			warns = append(warns, fmt.Sprintf("build.%s: unknown key", k))
		}
		if err != nil {
			return Build{}, warns, err
		}
	}
	return b, warns, nil
}
