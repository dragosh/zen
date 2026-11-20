package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TemplatesDir is where site templates live (core/init): the global config's
//
//	templates:
//	  dir: ~/Documents/zen templates
//
// "~" is the home folder and a relative path is relative to configDir, the
// folder of config.yml. Without the key it is <configDir>/templates. A folder's
// .zenmd.yml cannot set it (that is a warning when the file loads). The strings
// returned are problems with the key; the default is used then.
func (c *Config) TemplatesDir(configDir string) (string, []string) {
	def := filepath.Join(configDir, "templates")
	if c == nil || c.templates == nil {
		return def, nil
	}
	m, ok := c.templates.(map[string]any)
	if !ok {
		return def, []string{"templates must be a mapping such as { dir: ~/Documents/zen templates }"}
	}
	var warns []string
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	dir := ""
	for _, k := range keys {
		if k != "dir" {
			warns = append(warns, fmt.Sprintf("templates: unknown key %q", k))
			continue
		}
		s, ok := m[k].(string)
		if !ok {
			warns = append(warns, "templates: dir must be a folder path")
			continue
		}
		dir = strings.TrimSpace(s)
	}
	if len(warns) > 0 {
		return def, warns[:1]
	}
	if dir == "" {
		return def, nil
	}
	switch {
	case dir == "~" || strings.HasPrefix(dir, "~/"):
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
		}
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(configDir, dir)
	}
	return filepath.Clean(dir), nil
}
