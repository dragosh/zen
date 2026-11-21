package session

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/zenmark-org/zenmd/builtins/plugins/check"
	"github.com/zenmark-org/zenmd/builtins/plugins/codeblock"
	"github.com/zenmark-org/zenmd/builtins/plugins/diagram"
	"github.com/zenmark-org/zenmd/builtins/plugins/quiz"
	"github.com/zenmark-org/zenmd/builtins/plugins/rating"
	"github.com/zenmark-org/zenmd/builtins/plugins/sandbox"
	"github.com/zenmark-org/zenmd/engine/plugins"
)

// TestShowcaseFolder opens examples/showcase the way the app does and checks that it
// documents what it claims: every page renders without warnings, the sidebar
// has the shape the guide describes, and the only broken link is the
// deliberate one in reading/navigation.md.
func TestShowcaseFolder(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	demo := filepath.Join(filepath.Dir(file), "..", "..", "examples", "showcase")
	reg := plugins.NewRegistry()
	for _, p := range []plugins.Plugin{codeblock.New(), diagram.New(), check.New(), quiz.New(), sandbox.New()} {
		if err := reg.Register(p); err != nil {
			t.Fatal(err)
		}
	}
	// The demo ships external plugins (.zen/plugins): allow them through the
	// trust prompt, with a private settings folder.
	dialogs := &fakeDialogs{Allow: true}
	z, err := newZen(zenOptions{Registry: reg, ConfigDir: t.TempDir(), Dir: demo, Events: &sink{}, Dialogs: dialogs})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	// One prompt for the plugins and one for the layout of the folder.
	if len(dialogs.Asked) != 2 || !strings.Contains(dialogs.Asked[1], "memo") || !strings.Contains(dialogs.Asked[1], "changes how pages look; runs nothing") ||
		strings.Contains(dialogs.Asked[1], "page script") {
		t.Fatalf("layout prompt: %q", dialogs.Asked)
	}
	if got := layoutOf(t, z, "memo"); got.Status != "on" || got.Source != "folder" || !got.HasTemplate || len(got.Fields) != 3 {
		t.Errorf("demo layout memo: %+v", got)
	}
	if len(dialogs.Asked) != 2 || !strings.Contains(dialogs.Asked[0], "note") || !strings.Contains(dialogs.Asked[0], "./note.sh") ||
		!strings.Contains(dialogs.Asked[0], "calc") || !strings.Contains(dialogs.Asked[0], "bc -l") ||
		!strings.Contains(dialogs.Asked[0], "progress") || !strings.Contains(dialogs.Asked[0], "scripts in the page") ||
		!strings.Contains(dialogs.Asked[0], "callout") || !strings.Contains(dialogs.Asked[0], "wordcount") ||
		!strings.Contains(dialogs.Asked[0], "renders blocks inside zen; cannot read files, run programs or use the network") ||
		!strings.Contains(dialogs.Asked[0], "reads files in this folder") {
		t.Fatalf("trust prompt: %q", dialogs.Asked)
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
		"plugins/index.md", "layouts/index.md", "configuration/settings.md", "reference/shortcuts.md", "reference/markdown.md", "blog/index.md", "build/index.md",
	}
	if got := paths(tree[0].Children); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("README children = %v", got)
	}
	for _, n := range tree[0].Children {
		if n.Path == "plugins/index.md" && strings.Join(paths(n.Children), ",") != "plugins/writing-plugins.md,plugins/codeblock.md,plugins/diagram.md,plugins/quiz.md,plugins/sandbox.md,plugins/check.md,plugins/directives.md,plugins/dialects.md" {
			t.Errorf("Plugins children = %v", paths(n.Children))
		}
		if n.Path == "layouts/index.md" && strings.Join(paths(n.Children), ",") != "layouts/docs.md,layouts/post.md,layouts/landing.md,layouts/wide.md,layouts/handout.md,layouts/slides.md,layouts/course.md,layouts/writing-your-own-layout.md,layouts/memo.md" {
			t.Errorf("Layouts children = %v", paths(n.Children))
		}
		if n.Path == "blog/index.md" && strings.Join(paths(n.Children), ",") != "blog/hello-zen.md,blog/writing-with-layouts.md" {
			t.Errorf("Blog children = %v", paths(n.Children))
		}
		if n.Path == "configuration/settings.md" && strings.Join(paths(n.Children), ",") != "configuration/reference.md" {
			t.Errorf("Settings children = %v", paths(n.Children))
		}
	}

	files, _ := z.ws.Files()
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
	// Every layout of the guide shows up on its page.
	for path, want := range map[string]string{
		"layouts/index.md": "landing", "layouts/docs.md": "docs", "layouts/post.md": "post", "layouts/landing.md": "docs", "layouts/wide.md": "wide",
		"layouts/handout.md": "handout", "layouts/slides.md": "slides", "layouts/course.md": "course", "layouts/memo.md": "memo",
		"blog/index.md": "landing", "blog/hello-zen.md": "post", "blog/writing-with-layouts.md": "post", "README.md": "article",
	} {
		if pg, _ := z.Page(path); pg.Layout.Name != want {
			t.Errorf("%s: layout %+v, want %s", path, pg.Layout, want)
		}
	}
	idx, _ := z.Page("layouts/index.md")
	if strings.Count(idx.HTML, `class="zl-card"`) != 9 || !strings.Contains(idx.HTML, `class="zl-hero-cover" src="/_ws/images/zen-mark.svg"`) {
		t.Errorf("layouts index cards or cover: %s", idx.HTML)
	}
	memo, _ := z.Page("layouts/memo.md")
	if !strings.Contains(memo.HTML, "<dt>To</dt><dd>The whole team</dd>") || !strings.Contains(memo.HTML, "17 November 2026") || urlsOf(memo.Assets.CSS) != "/_layouts/memo/memo.css" {
		t.Errorf("memo: %s %+v", memo.HTML, memo.Assets)
	}
	post, _ := z.Page("blog/hello-zen.md")
	if !strings.Contains(post.HTML, `<time datetime="2026-11-10">10 November 2026</time>`) || !strings.Contains(post.HTML, `data-path="blog/writing-with-layouts.md"`) {
		t.Errorf("post: %s", post.HTML)
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
	// The docs layout's "Edit this page" link survives sanitising (the page
	// side opens the editor for it).
	if docs, err := z.Page("layouts/docs.md"); err != nil || !strings.Contains(docs.HTML, `data-zen-action="edit"`) {
		t.Errorf("docs layout edit link missing: %v", err)
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
	src, _ := z.ws.Read("plugins/writing-plugins.md")
	if !strings.Contains(string(src), "The simplest plugin: one Tengo file") ||
		strings.Index(string(src), "The simplest plugin: one Tengo file") > strings.Index(string(src), "A backend in a shell script") {
		t.Error("the Tengo section must come first, before the program section")
	}
	words := regexp.MustCompile(`data-zen-plugin="wordcount" data-zen-name="words"[^>]*><span class="words">(\d+)`).FindStringSubmatch(how.HTML)
	if words == nil || words[1] != strconv.Itoa(len(strings.Fields(string(src)))) {
		t.Errorf("wordcount inline on the how-to: %v (want %d)\n%s", words, len(strings.Fields(string(src))), how.HTML)
	}
	list := regexp.MustCompile(`(?s)data-zen-plugin="wordcount" data-zen-name="pages"[^>]*><ul class="pages">(.*?)</ul>`).FindStringSubmatch(how.HTML)
	if list == nil || strings.Count(list[1], "<li>") != 5 || !strings.Contains(list[1], "<li>Zenmark user guide (README.md): ") || !strings.Contains(list[1], " words</li>") {
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

// demoDir is the guide the app opens: examples/demo.
func demoDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "examples", "demo")
}

// TestDemoFolder opens examples/demo the way Zenmark does (only the plugins its
// build has) and checks the guide's shape: the sidebar sections in order, every
// page rendering without warnings, and the one deliberate broken link.
func TestDemoFolder(t *testing.T) {
	demo := demoDir()
	reg := plugins.NewRegistry()
	for _, p := range []plugins.Plugin{codeblock.New(), diagram.New(), check.New(), rating.New()} {
		if err := reg.Register(p); err != nil {
			t.Fatal(err)
		}
	}
	dialogs := &fakeDialogs{Allow: true}
	z, err := newZen(zenOptions{Registry: reg, ConfigDir: t.TempDir(), Dir: demo, Events: &sink{}, Dialogs: dialogs})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	if len(dialogs.Asked) != 0 {
		t.Errorf("the demo asks for trust (it has no external plugins or layouts): %q", dialogs.Asked)
	}
	st := z.Startup()
	if st.Page != "README.md" {
		t.Fatalf("home = %q", st.Page)
	}
	paths := func(ns []NavNode) string {
		var out []string
		for _, n := range ns {
			out = append(out, n.Path)
		}
		return strings.Join(out, ",")
	}
	tree := st.Workspace.Tree
	if len(tree) != 2 || tree[0].Path != "README.md" || tree[1].Title != "Other pages" {
		t.Fatalf("top level = %s", paths(tree))
	}
	// Intro is the README and its pages; then Plugins, Layouts and Configuration.
	want := "intro/opening-folders.md,intro/navigation.md,intro/find.md,intro/frontmatter.md,intro/editor.md,intro/printing.md,intro/inspector.md,intro/shortcuts.md,intro/markdown.md," +
		"plugins/index.md,layouts/index.md,configuration/settings.md"
	if got := paths(tree[0].Children); got != want {
		t.Errorf("README children = %s\nwant %s", got, want)
	}
	sub := map[string]string{
		"plugins/index.md":          "plugins/codeblock.md,plugins/diagram.md,plugins/check.md,plugins/quiz.md,plugins/sandbox.md,plugins/rating.md",
		"layouts/index.md":          "layouts/article.md,layouts/docs.md,layouts/post.md,layouts/landing.md,layouts/wide.md,layouts/handout.md,layouts/slides.md,layouts/course.md",
		"configuration/settings.md": "configuration/reference.md",
	}
	for _, n := range tree[0].Children {
		if w, ok := sub[n.Path]; ok {
			if got := paths(n.Children); got != w {
				t.Errorf("%s children = %s, want %s", n.Path, got, w)
			}
			delete(sub, n.Path)
		}
	}
	if len(sub) != 0 {
		t.Errorf("sections missing from the sidebar: %v", sub)
	}
	// Other pages groups the unlinked page by its folder.
	if len(tree[1].Children) != 1 || tree[1].Children[0].Title != "notes" || paths(tree[1].Children[0].Children) != "notes/scratch.md" {
		t.Errorf("Other pages = %+v", tree[1].Children)
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
	if strings.Contains(strings.Join(all, " "), "notes/hidden.md") {
		t.Error("hidden page shown in the sidebar")
	}

	files, _ := z.ws.Files()
	if len(files) != len(all)+1 { // every page but the hidden one is in the sidebar
		t.Errorf("%d pages, %d in the sidebar", len(files), len(all))
	}
	for _, p := range files {
		pg, err := z.Page(p)
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		if len(pg.Warnings) > 0 {
			t.Errorf("%s: warnings %v", p, pg.Warnings)
		}
		// Each layout's own page shows it; every other page is an article.
		wantLayout := map[string]string{"layouts/docs.md": "docs", "layouts/post.md": "post", "layouts/index.md": "landing", "layouts/landing.md": "docs", "layouts/wide.md": "wide", "layouts/handout.md": "handout", "layouts/slides.md": "slides", "layouts/course.md": "course"}[p]
		if wantLayout == "" {
			wantLayout = "article"
		}
		if pg.Layout.Name != wantLayout {
			t.Errorf("%s: layout %q, want %s", p, pg.Layout.Name, wantLayout)
		}
		broken := strings.Count(pg.HTML, `data-zen-link="broken"`)
		if want := map[bool]int{true: 1, false: 0}[p == "intro/navigation.md"]; broken != want {
			t.Errorf("%s: %d broken links, want %d", p, broken, want)
		}
	}
	nav, _ := z.Page("intro/navigation.md")
	if !strings.Contains(nav.HTML, `data-path="intro/opening-folders.md"`) {
		t.Error("slug link [A slug](open) not resolved")
	}
	if hl, _ := z.Page("plugins/codeblock.md"); !strings.Contains(hl.HTML, `class="chroma"`) {
		t.Error("codeblock did not colour plugins/codeblock.md")
	}
	if dg, _ := z.Page("plugins/diagram.md"); strings.Count(dg.HTML, `class="zen-diagram"`) < 2 {
		t.Error("the diagram page should draw its diagrams")
	}
	// The rating plugin has its page, and the course ends with a rating step.
	if rp, _ := z.Page("plugins/rating.md"); !strings.Contains(rp.HTML, `data-zen-name="rating"`) || !strings.Contains(rp.HTML, `data-zen-name="rating-results"`) || strings.Count(rp.HTML, `class="zr-star"`) < 5 {
		t.Errorf("plugins/rating.md should hold a live rating and a results block: %s", rp.HTML)
	}
	if cp, _ := z.Page("layouts/course.md"); !strings.Contains(cp.HTML, "Rate this course") || !strings.Contains(cp.HTML, `data-zen-name="rating"`) || !strings.Contains(cp.HTML, "Very good") || strings.Index(cp.HTML, "Rate this course") > strings.Index(cp.HTML, "Done") {
		t.Errorf("the course should have a rating step before Done: %s", cp.HTML)
	}
	if ck, _ := z.Page("plugins/check.md"); strings.Count(ck.HTML, `type="checkbox"`) < 3 {
		t.Error("the check page should show its checklists")
	}
}

// demoAssembly is what apps/zenmark/app-assembly.yml lists; a nil list means `all`.
type demoAssembly struct {
	Features yaml.Node `yaml:"features"`
	Addons   yaml.Node `yaml:"addons"`
	Plugins  yaml.Node `yaml:"plugins"`
	Layouts  yaml.Node `yaml:"layouts"`
	Dialects yaml.Node `yaml:"dialects"`
}

func assemblyIDs(t *testing.T, n yaml.Node) map[string]bool {
	t.Helper()
	if n.Kind == yaml.ScalarNode && n.Value == "all" {
		return nil
	}
	var ids []string
	if err := n.Decode(&ids); err != nil {
		t.Fatalf("assembly list: %v", err)
	}
	m := map[string]bool{}
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// TestDemoMatchesAssembly keeps the guide from getting ahead of the app: no page
// of examples/demo may use a plugin block, a layout or a dialect that
// apps/zenmark/app-assembly.yml does not list, or describe a feature the build
// lacks. When a roadmap step adds one to the assembly, its pages may use it.
func TestDemoMatchesAssembly(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "apps", "zenmark", "app-assembly.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var a demoAssembly
	if err := yaml.Unmarshal(raw, &a); err != nil {
		t.Fatal(err)
	}
	havePlugin, haveLayout, haveDialect := assemblyIDs(t, a.Plugins), assemblyIDs(t, a.Layouts), assemblyIDs(t, a.Dialects)
	allowed := func(set map[string]bool, id string) bool { return set == nil || set[id] }

	// The block syntax of each plugin that has one, and the dialects' marks.
	blockPlugin := map[string]string{"mermaid": "diagram", "diagram": "diagram", "check": "check", "quiz": "quiz", "quiz-results": "quiz", "sandbox": "sandbox", "rating": "rating", "rating-results": "rating"}
	fence := regexp.MustCompile("(?m)^\\s*(?:```+|~~~+|:::+)\\s*(\\{[a-z-]+\\}|[A-Za-z][\\w-]*)")
	// Words for features this build lacks (the showcase has them); the guide must not name them.
	lacking := []string{".zen/"}
	// Words for a feature, an add-on or a dialect: allowed once the assembly has it.
	haveFeature, haveAddon := assemblyIDs(t, a.Features), assemblyIDs(t, a.Addons)
	for _, w := range []struct {
		set   map[string]bool
		id    string
		words []string
	}{
		{haveFeature, "find", []string{"Find Next", "Find Previous", "Edit › Find"}},
		{haveFeature, "editor", []string{"Open in Editor", "Edit this page"}},
		{haveFeature, "print", []string{"Print Page"}},
		{haveFeature, "export", []string{"zenmd export"}},
		{haveFeature, "newsite", []string{"New Site", "newsite"}},
		{haveAddon, "vale", []string{"Vale"}},
		{haveDialect, "myst", []string{"MyST", "Jupyter"}},
	} {
		if !allowed(w.set, w.id) {
			lacking = append(lacking, w.words...)
		}
	}
	// A plugin's own name may appear once the assembly has it.
	for _, id := range []string{"quiz", "sandbox", "rating"} {
		if !allowed(havePlugin, id) {
			lacking = append(lacking, id)
		}
	}

	pages := 0
	err = filepath.WalkDir(demo(t), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(demo(t), p)
		if d.IsDir() {
			if rel == ".zen" || rel == ".vale" {
				t.Errorf("%s: the demo carries nothing the build lacks", rel)
				return fs.SkipDir
			}
			return nil
		}
		if rel == ".vale.ini" {
			t.Errorf("%s: the demo carries nothing the build lacks", rel)
			return nil
		}
		b, _ := os.ReadFile(p)
		text := string(b)
		var front map[string]any
		if rel == ".zenmd.yml" {
			_ = yaml.Unmarshal(b, &front)
			for _, key := range []string{"layouts", "build"} {
				if _, ok := front[key]; ok {
					t.Errorf("%s: %q does nothing in this build", rel, key)
				}
			}
			if md, ok := front["markdown"].(map[string]any); ok {
				if ds, _ := md["dialects"].([]any); len(ds) > 0 && havePlugin != nil && haveDialect != nil {
					for _, d := range ds {
						if !allowed(haveDialect, fmt.Sprint(d)) {
							t.Errorf("%s: dialect %v is not in the assembly", rel, d)
						}
					}
				}
			}
		} else if filepath.Ext(p) != ".md" {
			return nil
		} else {
			pages++
			if m := regexp.MustCompile(`(?s)\A---\n(.*?)\n---\n`).FindStringSubmatch(text); m != nil {
				_ = yaml.Unmarshal([]byte(m[1]), &front)
			}
			for _, w := range lacking {
				if strings.Contains(text, w) {
					t.Errorf("%s: names %q, which this build lacks", rel, w)
				}
			}
			for _, m := range fence.FindAllStringSubmatch(text, -1) {
				name := m[1]
				if strings.HasPrefix(name, "{") {
					if !allowed(haveDialect, "myst") {
						t.Errorf("%s: %s needs the myst dialect, which the assembly lacks", rel, name)
					}
					continue
				}
				if id, ok := blockPlugin[name]; ok && !allowed(havePlugin, id) {
					t.Errorf("%s: a %s block needs the %s plugin, which the assembly lacks", rel, name, id)
				}
			}
		}
		// layout: name, or layout: { name: ... }
		switch l := front["layout"].(type) {
		case string:
			if !allowed(haveLayout, l) {
				t.Errorf("%s: layout %q is not in the assembly", rel, l)
			}
		case map[string]any:
			if !allowed(haveLayout, fmt.Sprint(l["name"])) {
				t.Errorf("%s: layout %v is not in the assembly", rel, l["name"])
			}
		}
		if pl, ok := front["plugins"].(map[string]any); ok {
			for id := range pl {
				if !allowed(havePlugin, id) {
					t.Errorf("%s: plugin %q is not in the assembly", rel, id)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if pages < 10 {
		t.Errorf("only %d pages found", pages)
	}
	// .zenmd.yml sets only keys this build uses.
	var cfg map[string]any
	b, _ := os.ReadFile(filepath.Join(demo(t), ".zenmd.yml"))
	_ = yaml.Unmarshal(b, &cfg)
	for k := range cfg {
		if k != "version" && k != "logo" && k != "plugins" {
			t.Errorf(".zenmd.yml sets %q, which this build does not use", k)
		}
	}
}

func demo(t *testing.T) string { t.Helper(); return demoDir() }
