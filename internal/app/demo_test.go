package app

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/plugins/diagram"
	"github.com/dragosh/zen/internal/plugins/highlight"
)

// TestDemoFolder opens examples/demo the way the app does and checks that it
// documents what it claims: every page renders without warnings, the sidebar
// has the shape the guide describes, and the only broken link is the
// deliberate one in reading/navigation.md.
func TestDemoFolder(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	demo := filepath.Join(filepath.Dir(file), "..", "..", "examples", "demo")
	reg := plugins.NewRegistry()
	for _, p := range []plugins.Plugin{highlight.New(), diagram.New()} {
		if err := reg.Register(p); err != nil {
			t.Fatal(err)
		}
	}
	// The demo ships external plugins (.zen/plugins): allow them through the
	// trust prompt, with a private settings folder.
	dialogs := &fakeDialogs{confirm: true}
	z, err := newZen(zenOptions{Registry: reg, ConfigDir: t.TempDir(), Dir: demo, Events: &sink{}, Dialogs: dialogs})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	if len(dialogs.asked) != 1 || !strings.Contains(dialogs.asked[0], "note") || !strings.Contains(dialogs.asked[0], "./note.sh") ||
		!strings.Contains(dialogs.asked[0], "calc") || !strings.Contains(dialogs.asked[0], "bc -l") ||
		!strings.Contains(dialogs.asked[0], "progress") || !strings.Contains(dialogs.asked[0], "scripts in the page") {
		t.Fatalf("trust prompt: %q", dialogs.asked)
	}
	for _, id := range []string{"note", "calc", "progress"} {
		if got := infoOf(t, z, id); got.Status != "on" || got.Source != "folder" {
			t.Errorf("demo plugin %s: %+v", id, got)
		}
	}

	st := z.Startup()
	if st.Page != "README.md" {
		t.Fatalf("home = %q", st.Page)
	}
	tree := st.Workspace.Tree
	var top []string
	for _, n := range tree {
		top = append(top, n.Title)
	}
	if len(tree) < 2 || tree[0].Path != "README.md" || tree[len(tree)-1].Title != "Other pages" {
		t.Fatalf("top level = %v", top)
	}
	var all []string
	var walk func([]NavNode)
	walk = func(ns []NavNode) {
		for _, n := range ns {
			if n.Kind == "page" {
				all = append(all, n.Path)
			}
			walk(n.Children)
		}
	}
	walk(tree)
	joined := strings.Join(all, " ")
	if strings.Contains(joined, "notes/hidden.md") {
		t.Error("hidden page shown in the sidebar")
	}
	if !strings.Contains(joined, "notes/scratch.md") {
		t.Error("unlinked page missing from Other pages")
	}
	// README links the start of each section, in reading order; sections nest.
	paths := func(ns []NavNode) []string {
		var out []string
		for _, n := range ns {
			out = append(out, n.Path)
		}
		return out
	}
	want := []string{
		"getting-started/install.md", "getting-started/opening-folders.md", "reading/navigation.md",
		"reading/frontmatter.md", "editing/editor.md", "editing/linting.md",
		"plugins/index.md", "configuration/settings.md", "reference/shortcuts.md", "reference/markdown.md",
	}
	if got := paths(tree[0].Children); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("README children = %v", got)
	}
	for _, n := range tree[0].Children {
		if n.Path == "plugins/index.md" && strings.Join(paths(n.Children), ",") != "plugins/writing-plugins.md,plugins/highlight.md,plugins/diagram.md,plugins/directives.md" {
			t.Errorf("Plugins children = %v", paths(n.Children))
		}
		if n.Path == "configuration/settings.md" && strings.Join(paths(n.Children), ",") != "configuration/reference.md" {
			t.Errorf("Settings children = %v", paths(n.Children))
		}
	}

	files, _ := z.state.ws.Files()
	for _, p := range files {
		pg, err := z.Page(p)
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		if len(pg.Warnings) > 0 {
			t.Errorf("%s: warnings %v", p, pg.Warnings)
		}
		broken := strings.Count(pg.HTML, `data-zen-link="broken"`)
		if want := map[bool]int{true: 1, false: 0}[p == "reading/navigation.md"]; broken != want {
			t.Errorf("%s: %d broken links, want %d", p, broken, want)
		}
	}
	// The slug link on the navigation page resolves to the install page.
	nav, _ := z.Page("reading/navigation.md")
	if !strings.Contains(nav.HTML, `data-path="getting-started/install.md"`) {
		t.Error("slug link [A slug](install) not resolved")
	}
	// Plugins actually ran on their pages.
	if hl, _ := z.Page("plugins/highlight.md"); !strings.Contains(hl.HTML, `class="chroma"`) {
		t.Error("highlight did not colour plugins/highlight.md")
	}
	if dg, _ := z.Page("plugins/diagram.md"); strings.Count(dg.HTML, `class="zen-diagram"`) != 3 {
		t.Error("diagram page should have three diagrams")
	}
	// The external demo plugins: :::note on the directives page goes through
	// note.sh; calc pipes a fence through bc.
	dir, _ := z.Page("plugins/directives.md")
	if !strings.Contains(dir.HTML, `data-zen-plugin="note" data-zen-name="note"`) || !strings.Contains(dir.HTML, `<div class="note"><strong>Remember</strong>`) {
		t.Errorf("note plugin did not render the container: %s", dir.HTML)
	}
	if !strings.Contains(strings.Join(dir.Assets.CSS, ","), "/_plugins/note/note.css") {
		t.Errorf("note assets: %+v", dir.Assets)
	}
	// The how-to shows the real plugins at work: bc's answer, a ticked-able
	// checklist rendered by the frontend-only progress plugin.
	how, err := z.Page("plugins/writing-plugins.md")
	if err != nil || !strings.Contains(how.HTML, `<pre class="zen-plugin-output">1024`) || !strings.Contains(how.HTML, "3.1428") {
		t.Errorf("calc block on the how-to: %v %s", err, how.HTML)
	}
	if !strings.Contains(how.HTML, `data-zen-plugin="progress" data-zen-name="progress" data-zen-attrs="{&#34;id&#34;:&#34;first-plugin&#34;}"`) ||
		strings.Count(how.HTML, `type="checkbox"`) < 3 {
		t.Errorf("progress container lost its task list: %s", how.HTML)
	}
	if !strings.Contains(strings.Join(how.Assets.JS, ","), "/_plugins/progress/progress.js") {
		t.Errorf("progress script not offered: %+v", how.Assets)
	}
	warn, err := z.Preview("plugins/directives.md", ":::warning\nMind the <gap> & co.\n:::\n")
	if err != nil || !strings.Contains(warn.HTML, `<div class="warning"><strong>Warning</strong>`) || !strings.Contains(warn.HTML, "Mind the &lt;gap&gt; &amp; co.") {
		t.Errorf("warning block: %v %s", err, warn.HTML)
	}
	calc, err := z.Preview("plugins/directives.md", "```calc\n2 + 3 * 4\n```\n")
	if err != nil || !strings.Contains(calc.HTML, `<pre class="zen-plugin-output">14`) || len(calc.Warnings) != 0 {
		t.Errorf("calc block: %v %v %s", err, calc.Warnings, calc.HTML)
	}
}
