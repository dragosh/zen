package session

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dragosh/zenmd/builtins/plugins/check"
	"github.com/dragosh/zenmd/builtins/plugins/codeblock"
	"github.com/dragosh/zenmd/builtins/plugins/diagram"
	"github.com/dragosh/zenmd/builtins/plugins/quiz"
	"github.com/dragosh/zenmd/builtins/plugins/sandbox"
	"github.com/dragosh/zenmd/engine/config"
	"github.com/dragosh/zenmd/engine/plugins"
	"github.com/dragosh/zenmd/engine/render"
)

func builtins() []plugins.Plugin {
	return []plugins.Plugin{codeblock.New(), diagram.New(), check.New(), quiz.New(), sandbox.New()}
}

// Every built-in plugin documents itself: a summary and snippets that
// render without warnings through the real pipeline with the plugin on.
func TestBuiltinHelp(t *testing.T) {
	for _, p := range builtins() {
		m := p.Manifest()
		t.Run(m.ID, func(t *testing.T) {
			if strings.TrimSpace(m.Help.Summary) == "" {
				t.Fatal("no help summary")
			}
			if len(m.Help.Snippets) == 0 {
				t.Fatal("no help snippets")
			}
			if err := plugins.CheckHelp(m.Help); err != nil {
				t.Fatal(err)
			}
			reg := plugins.NewRegistry()
			for _, q := range builtins() {
				if err := reg.Register(q); err != nil {
					t.Fatal(err)
				}
			}
			r := render.New(reg, config.Default(), nil)
			for _, s := range m.Help.Snippets {
				// A layout snippet is a whole page (frontmatter); the others are fragments.
				page, err := r.Render("help.md", []byte(s.Markdown))
				if err != nil {
					t.Fatalf("%s: %v", s.Title, err)
				}
				if len(page.Warnings) > 0 {
					t.Errorf("%s: warnings %q", s.Title, page.Warnings)
				}
				if strings.TrimSpace(page.HTML) == "" {
					t.Errorf("%s: renders nothing", s.Title)
				}
			}
		})
	}
	sb := sandbox.New().Manifest().Help
	all := sb.Summary
	for _, s := range sb.Snippets {
		all += s.Markdown
	}
	if !strings.Contains(all, "config.yml") {
		t.Error("sandbox help must say that images come from the global config.yml")
	}
}

// Plugins() carries the help and what each plugin adds.
func TestPluginsHelpAndAdds(t *testing.T) {
	demo := filepath.Join("..", "..", "examples", "demo")
	reg := plugins.NewRegistry()
	for _, p := range builtins() {
		if err := reg.Register(p); err != nil {
			t.Fatal(err)
		}
	}
	z, err := newZen(zenOptions{Registry: reg, ConfigDir: t.TempDir(), Dir: demo, Events: &sink{}, Dialogs: &fakeDialogs{Allow: true}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)

	raw, _ := json.Marshal(z.Plugins())
	var shape []struct {
		ID   string
		Help struct {
			Summary  string
			Snippets []struct{ Title, Markdown string }
		}
		Blocks  []string
		Inlines []string
		Options []struct {
			Name, Type, Description string
			Default                 any
		}
	}
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatal(err)
	}
	by := map[string]int{}
	for i, p := range shape {
		by[p.ID] = i
		if p.Help.Summary == "" || len(p.Help.Snippets) == 0 {
			t.Errorf("%s: no help in Plugins()", p.ID)
		}
	}
	d := shape[by["diagram"]]
	if strings.Join(d.Blocks, ",") != "mermaid,diagram" || len(d.Options) != 1 || d.Options[0].Name != "theme" ||
		d.Options[0].Type != "string" || d.Options[0].Default != "neutral" || d.Options[0].Description == "" {
		t.Errorf("diagram: %+v", d)
	}
	if c := shape[by["check"]]; strings.Join(c.Blocks, ",") != "check" {
		t.Errorf("course: %+v", c)
	}
	cb := shape[by["codeblock"]]
	if len(cb.Options) != 3 || cb.Options[0].Name != "lineNumbers" || cb.Options[1].Name != "style" || cb.Options[2].Name != "wrap" {
		t.Errorf("codeblock options are sorted by name: %+v", cb.Options)
	}
	// The demo folder's plugins document themselves too.
	for _, id := range []string{"note", "calc", "progress", "callout", "wordcount"} {
		p := shape[by[id]]
		if p.Help.Summary == "" || len(p.Help.Snippets) == 0 {
			t.Errorf("demo plugin %s has no help", id)
		}
		for _, sn := range p.Help.Snippets {
			pg, err := z.Preview("README.md", sn.Markdown)
			if err != nil || len(pg.Warnings) > 0 || strings.Contains(pg.HTML, "zen-plugin-error") {
				t.Errorf("demo plugin %s, snippet %q: %v %q", id, sn.Title, err, pg.Warnings)
			}
		}
	}
}
