package app

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/plugins/codeblock"
	"github.com/dragosh/zen/internal/plugins/course"
	"github.com/dragosh/zen/internal/plugins/diagram"
	"github.com/dragosh/zen/internal/plugins/quiz"
	"github.com/dragosh/zen/internal/plugins/sandbox"
	"github.com/dragosh/zen/internal/plugins/slides"
)

// TestDemoFolder opens examples/demo the way the app does and checks that it
// documents what it claims: every page renders without warnings, the sidebar
// has the shape the guide describes, and the only broken link is the
// deliberate one in reading/navigation.md.
func TestDemoFolder(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	demo := filepath.Join(filepath.Dir(file), "..", "..", "examples", "demo")
	reg := plugins.NewRegistry()
	for _, p := range []plugins.Plugin{codeblock.New(), diagram.New(), slides.New(), course.New(), quiz.New(), sandbox.New()} {
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
		!strings.Contains(dialogs.asked[0], "progress") || !strings.Contains(dialogs.asked[0], "scripts in the page") ||
		!strings.Contains(dialogs.asked[0], "callout") || !strings.Contains(dialogs.asked[0], "wordcount") ||
		!strings.Contains(dialogs.asked[0], "renders blocks inside zen; cannot read files, run programs or use the network") ||
		!strings.Contains(dialogs.asked[0], "reads files in this folder") {
		t.Fatalf("trust prompt: %q", dialogs.asked)
	}
	for id, kind := range map[string]string{"note": "program", "calc": "program", "progress": "page", "callout": "script", "wordcount": "script"} {
		if got := infoOf(t, z, id); got.Status != "on" || got.Source != "folder" || got.Kind != kind {
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
		if n.Path == "plugins/index.md" && strings.Join(paths(n.Children), ",") != "plugins/writing-plugins.md,plugins/codeblock.md,plugins/diagram.md,plugins/quiz.md,plugins/sandbox.md,layouts/slides.md,layouts/course.md,plugins/directives.md" {
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
	if hl, _ := z.Page("plugins/codeblock.md"); !strings.Contains(hl.HTML, `class="chroma"`) {
		t.Error("codeblock did not colour plugins/codeblock.md")
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
	// The Tengo samples: callout (one file, inline CSS, zen.markdown) and
	// wordcount (folder form, read permission, zen.pages and zen.read).
	if !strings.Contains(how.HTML, `data-zen-plugin="callout" data-zen-name="tip"`) || !strings.Contains(how.HTML, `<div class="callout tip">`) {
		t.Errorf("callout tip did not render on the how-to: %s", how.HTML)
	}
	if !strings.Contains(strings.Join(how.Assets.CSS, ","), "/_plugins/callout/inline.css") {
		t.Errorf("callout css not offered: %+v", how.Assets)
	}
	// The title has its own block wrapper (kept by the sanitiser); bold text in
	// the body stays inline in its paragraph, and no style targets every
	// <strong> in a callout (that pushed bold body text onto its own line).
	if !strings.Contains(how.HTML, `<div class="callout-title"><strong>One file</strong></div>`) {
		t.Errorf("callout title wrapper missing: %s", how.HTML)
	}
	if !strings.Contains(how.HTML, `<p>A <strong>Tengo</strong> script`) {
		t.Errorf("bold text in a callout body must stay inline in its paragraph: %s", how.HTML)
	}
	if css, _ := os.ReadFile(filepath.Join(demo, ".zen", "plugins", "callout.tengo")); strings.Contains(string(css), "callout strong") {
		t.Error("callout.tengo styles every <strong> in a callout")
	}
	src, _ := z.state.ws.Read("plugins/writing-plugins.md")
	if !strings.Contains(string(src), "The simplest plugin: one Tengo file") ||
		strings.Index(string(src), "The simplest plugin: one Tengo file") > strings.Index(string(src), "A backend in a shell script") {
		t.Error("the Tengo section must come first, before the program section")
	}
	words := regexp.MustCompile(`data-zen-plugin="wordcount" data-zen-name="words"[^>]*><span class="words">(\d+)`).FindStringSubmatch(how.HTML)
	if words == nil || words[1] != strconv.Itoa(len(strings.Fields(string(src)))) {
		t.Errorf("wordcount inline on the how-to: %v (want %d)\n%s", words, len(strings.Fields(string(src))), how.HTML)
	}
	list := regexp.MustCompile(`(?s)data-zen-plugin="wordcount" data-zen-name="pages"[^>]*><ul class="pages">(.*?)</ul>`).FindStringSubmatch(how.HTML)
	if list == nil || strings.Count(list[1], "<li>") != 5 || !strings.Contains(list[1], "<li>zen user guide (README.md): ") || !strings.Contains(list[1], " words</li>") {
		t.Errorf("wordcount pages block did not list five pages: %v", list)
	}
	everyPage, err := z.Preview("plugins/directives.md", ":::pages\n:::\n")
	if err != nil || strings.Count(everyPage.HTML, "<li>") != len(files) || !strings.Contains(everyPage.HTML, "reading/navigation.md") {
		t.Errorf("wordcount pages without a limit: %v %d items for %d pages", err, strings.Count(everyPage.HTML, "<li>"), len(files))
	}
	caution, err := z.Preview("plugins/directives.md", ":::caution{title=\"Careful\"}\nMind the *gap* <i onclick=\"x()\">x</i>.\n:::\n")
	if err != nil || !strings.Contains(caution.HTML, `<div class="callout caution">`) || !strings.Contains(caution.HTML, "Careful") ||
		!strings.Contains(caution.HTML, "<em>gap</em>") || strings.Contains(caution.HTML, "onclick") {
		t.Errorf("caution block: %v %s", err, caution.HTML)
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
