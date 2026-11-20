package sitetemplates_test

import (
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/zenmark-org/zenmd/builtins/plugins/check"
	"github.com/zenmark-org/zenmd/builtins/plugins/codeblock"
	"github.com/zenmark-org/zenmd/builtins/plugins/diagram"
	"github.com/zenmark-org/zenmd/builtins/plugins/quiz"
	"github.com/zenmark-org/zenmd/builtins/plugins/sandbox"
	"github.com/zenmark-org/zenmd/builtins/sitetemplates"
	"github.com/zenmark-org/zenmd/engine/plugins"
	enginetemplates "github.com/zenmark-org/zenmd/engine/sitetemplates"
)

var (
	fenceRE  = regexp.MustCompile("(?m)^[ \t]*```+[ \t]*([A-Za-z0-9_-]+)")
	colonRE  = regexp.MustCompile(`(?m)^[ \t]*:::([a-z][a-z0-9-]*)`)
	frontRE  = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---`)
	layoutRE = regexp.MustCompile(`(?m)^layout:\s*"?([a-z][a-z0-9-]*)`)
)

// blockOwner maps a block name (:::quiz, ```mermaid) to the built-in plugin
// that owns it.
func blockOwner(t *testing.T) map[string]string {
	owner := map[string]string{}
	for _, p := range []plugins.Plugin{codeblock.New(), diagram.New(), check.New(), quiz.New(), sandbox.New()} {
		m := p.Manifest()
		for _, b := range m.Blocks {
			owner[b] = m.ID
		}
	}
	return owner
}

func TestTemplatesDeclareNeeds(t *testing.T) {
	owner := blockOwner(t)
	entries, err := fs.ReadDir(sitetemplates.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		data, err := fs.ReadFile(sitetemplates.FS, id+"/template.yml")
		if err != nil {
			t.Fatal(err)
		}
		tpl, err := enginetemplates.ParseManifest(data)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		declLayouts, declPlugins := set(tpl.Requires.Layouts), set(tpl.Requires.Plugins)
		if len(declLayouts) == 0 {
			t.Errorf("%s: requires.layouts is empty", id)
		}
		usedLayouts, usedPlugins := map[string]bool{}, map[string]bool{}
		err = fs.WalkDir(sitetemplates.FS, id, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			raw, err := fs.ReadFile(sitetemplates.FS, p)
			if err != nil {
				return err
			}
			text := string(raw)
			switch {
			case path.Base(p) == ".zenmd.yml":
				var cfg struct {
					Layout  string
					Layouts map[string]string
					Plugins map[string]any
				}
				if err := yaml.Unmarshal(raw, &cfg); err != nil {
					t.Fatalf("%s: %v", p, err)
				}
				if cfg.Layout != "" {
					usedLayouts[cfg.Layout] = true
				}
				for _, l := range cfg.Layouts {
					usedLayouts[l] = true
				}
				for name := range cfg.Plugins {
					usedPlugins[name] = true
				}
			case strings.HasSuffix(p, ".md"):
				if m := frontRE.FindStringSubmatch(text); m != nil {
					if l := layoutRE.FindStringSubmatch(m[1]); l != nil {
						usedLayouts[l[1]] = true
					}
				}
				for _, m := range colonRE.FindAllStringSubmatch(text, -1) {
					if o := owner[m[1]]; o != "" {
						usedPlugins[o] = true
					}
				}
				for _, m := range fenceRE.FindAllStringSubmatch(text, -1) {
					if o := owner[m[1]]; o != "" {
						usedPlugins[o] = true
					} else {
						usedPlugins["codeblock"] = true // any other language is highlighted
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		for l := range usedLayouts {
			if !declLayouts[l] {
				t.Errorf("template %s uses layout %s but does not declare it (declared: %v)", id, l, keys(declLayouts))
			}
		}
		for p := range usedPlugins {
			if !declPlugins[p] {
				t.Errorf("template %s uses plugin %s but does not declare it (declared: %v)", id, p, keys(declPlugins))
			}
		}
	}
}

func TestRequiresValid(t *testing.T) {
	if _, err := enginetemplates.ParseManifest([]byte("id: x\nname: X\nrequires: { layouts: [docs], plugins: [codeblock] }\n")); err != nil {
		t.Errorf("valid requires: %v", err)
	}
	for _, bad := range []string{"requires: docs", "requires: { colors: [x] }", "requires: { layouts: docs }", "requires: { layouts: [Docs] }"} {
		if _, err := enginetemplates.ParseManifest([]byte("id: x\nname: X\n" + bad + "\n")); err == nil {
			t.Errorf("%q should fail", bad)
		}
	}
}

func set(l []string) map[string]bool {
	m := map[string]bool{}
	for _, s := range l {
		m[s] = true
	}
	return m
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestIDsMatchFolders(t *testing.T) {
	entries, _ := fs.ReadDir(sitetemplates.FS, ".")
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if strings.Join(set2(dirs), ",") != strings.Join(set2(sitetemplates.IDs), ",") {
		t.Errorf("IDs %v, folders %v", sitetemplates.IDs, dirs)
	}
}

func set2(l []string) []string { m := keys(set(l)); return m }
