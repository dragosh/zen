package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func templatesOf(t *testing.T, global, folder string) (string, []string, []string) {
	t.Helper()
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "cfg")
	g, root := "", ""
	if global != "" {
		g = write(t, filepath.Join(cfgDir, "config.yml"), global)
	}
	if folder != "" {
		root = filepath.Join(dir, "ws")
		write(t, filepath.Join(root, WorkspaceFile), folder)
	}
	c := Load(g, root)
	d, w := c.TemplatesDir(cfgDir)
	return filepath.ToSlash(strings.TrimPrefix(d, dir)), w, c.Warnings()
}

func TestTemplatesDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Default: the folder next to config.yml.
	if d, w, lw := templatesOf(t, "", ""); d != "/cfg/templates" || len(w) != 0 || len(lw) != 0 {
		t.Errorf("default: %q %v %v", d, w, lw)
	}
	if d, _, _ := templatesOf(t, "theme: dark\n", ""); d != "/cfg/templates" {
		t.Errorf("no key: %q", d)
	}
	// ~ expands to the home folder; an absolute path is kept as it is.
	c := Load("", "")
	_ = c
	dir := t.TempDir()
	g := write(t, filepath.Join(dir, "config.yml"), "templates:\n  dir: ~/Documents/zen templates\n")
	got, warns := Load(g, "").TemplatesDir(dir)
	if got != filepath.Join(home, "Documents", "zen templates") || len(warns) != 0 {
		t.Errorf("tilde: %q %v", got, warns)
	}
	g = write(t, filepath.Join(dir, "config.yml"), "templates:\n  dir: /srv/zen/t\n")
	if got, _ = Load(g, "").TemplatesDir(dir); got != "/srv/zen/t" {
		t.Errorf("absolute: %q", got)
	}
	// A relative path is relative to the config folder, and is cleaned.
	g = write(t, filepath.Join(dir, "config.yml"), "templates:\n  dir: ../shared/./site templates\n")
	if got, _ = Load(g, "").TemplatesDir(dir); got != filepath.Join(filepath.Dir(dir), "shared", "site templates") {
		t.Errorf("relative: %q", got)
	}
	// "~user" is not expanded (a relative name), "~" alone is home.
	g = write(t, filepath.Join(dir, "config.yml"), "templates:\n  dir: \"~\"\n")
	if got, _ = Load(g, "").TemplatesDir(dir); got != home {
		t.Errorf("home alone: %q", got)
	}

	// Wrong shapes: the default is used, and TemplatesDir says why.
	for _, src := range []string{"templates: yes\n", "templates:\n  dir: 3\n", "templates:\n  dir: [a]\n", "templates:\n  folder: x\n"} {
		d, w, lw := templatesOf(t, src, "")
		if d != "/cfg/templates" || len(w) != 1 || !strings.Contains(w[0], "templates") || len(lw) != 0 {
			t.Errorf("%q: %q %v %v", src, d, w, lw)
		}
	}
	// An empty dir means the default.
	if d, w, _ := templatesOf(t, "templates:\n  dir: \"\"\n", ""); d != "/cfg/templates" || len(w) != 0 {
		t.Errorf("empty: %q %v", d, w)
	}

	// A folder's .zen.yml cannot set it: the key is a warning, the global value stays.
	d, w, lw := templatesOf(t, "templates:\n  dir: /srv/g\n", "templates:\n  dir: /tmp/evil\n")
	if d != "/srv/g" || len(w) != 0 || len(lw) != 1 || !strings.Contains(lw[0], "templates can only be set in the global config") {
		t.Errorf("workspace key: %q %v %v", d, w, lw)
	}
	if d, _, _ := templatesOf(t, "", "templates:\n  dir: /tmp/evil\n"); d != "/cfg/templates" {
		t.Errorf("workspace only: %q", d)
	}
	// A nil config answers with the default.
	var nilCfg *Config
	if got, _ := nilCfg.TemplatesDir("/x"); got != "/x/templates" {
		t.Errorf("nil: %q", got)
	}
}
