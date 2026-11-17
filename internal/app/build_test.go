package app

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dragosh/zen/internal/cli"
	"github.com/dragosh/zen/internal/core/plugins"
)

// stubAssets stands in for frontend/dist (which a test run may not have built).
func stubAssets() fstest.MapFS {
	return fstest.MapFS{
		"static/zen.js":  {Data: []byte("/*static runtime*/")},
		"static/zen.css": {Data: []byte("/*static css*/")},
	}
}

type buildRun struct {
	out, stdout, stderr string
	err                 error
}

// runBuild builds dir with a private settings folder cfg.
func runBuild(t *testing.T, dir, cfg string, b cli.Build, edit ...func(*buildEnv)) buildRun {
	t.Helper()
	env := buildEnv{GlobalPath: filepath.Join(cfg, "config.yml"), Assets: stubAssets()}
	for _, e := range edit {
		e(&env)
	}
	b.Dir = dir
	var stdout, stderr bytes.Buffer
	err := buildSite(env, b, &stdout, &stderr)
	out := b.Out
	if out == "" {
		out = filepath.Join(dir, "_site")
	}
	return buildRun{out: out, stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func (r buildRun) read(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(r.out, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func (r buildRun) has(rel string) bool {
	_, err := os.Lstat(filepath.Join(r.out, filepath.FromSlash(rel)))
	return err == nil
}

func (r buildRun) files(t *testing.T) []string {
	t.Helper()
	var out []string
	_ = filepath.Walk(r.out, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			rel, _ := filepath.Rel(r.out, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func TestBuildSiteReport(t *testing.T) {
	dir := tree(t, map[string]string{
		"README.md": "# Home\n\n[A](a.md)\n",
		"a.md":      "---\nlayout: nosuch\n---\n# A\n\n![ok](img/a.png)\n",
		"img/a.png": "\x89PNG\r\n\x1a\n",
	})
	cfg := t.TempDir()
	r := runBuild(t, dir, cfg, cli.Build{})
	if r.err != nil {
		t.Fatal(r.err)
	}
	if !regexp.MustCompile(`^Built 2 pages and 3 files into _site \(1 warning\)\nPrint version: _site/print.html\n$`).MatchString(r.stdout) {
		t.Errorf("summary %q", r.stdout)
	}
	if !strings.Contains(r.stderr, "a.md: ") || !strings.Contains(r.stderr, "nosuch") || strings.Count(r.stderr, "\n") != 1 {
		t.Errorf("warnings %q", r.stderr)
	}
	for _, f := range []string{"README.html", "index.html", "a.html", "img/a.png", "_zen/zen.js", "_zen/zen.css", ".zen-build"} {
		if !r.has(f) {
			t.Errorf("missing %s: %v", f, r.files(t))
		}
	}
	// Nothing was written into the folder but the output.
	if exists(filepath.Join(dir, ".zen-build")) {
		t.Error("marker in the folder")
	}
	// Clean builds report no warnings in the summary.
	clean := tree(t, map[string]string{"README.md": "# Home\n"})
	r = runBuild(t, clean, cfg, cli.Build{})
	if r.err != nil || r.stdout != "Built 1 pages and 2 files into _site\nPrint version: _site/print.html\n" || r.stderr != "" {
		t.Errorf("clean build: %+v", r)
	}
	// An output outside the folder is named in full.
	other := filepath.Join(t.TempDir(), "site")
	r = runBuild(t, clean, cfg, cli.Build{Out: other})
	if r.err != nil || !strings.Contains(r.stdout, "into "+other) {
		t.Errorf("outside output: %+v", r)
	}

	// Errors: the output may not be the folder, and the runtime must exist.
	if r := runBuild(t, clean, cfg, cli.Build{Out: clean}); r.err == nil {
		t.Error("output = folder accepted")
	}
	if r := runBuild(t, clean, cfg, cli.Build{}, func(e *buildEnv) { e.Assets = fstest.MapFS{} }); r.err == nil || !strings.Contains(r.err.Error(), "runtime") {
		t.Errorf("missing runtime: %v", r.err)
	}
	if r := runBuild(t, filepath.Join(clean, "nope"), cfg, cli.Build{}); r.err == nil {
		t.Error("missing folder accepted")
	}
	// The limits of the global config apply, like when the app opens the folder.
	if err := os.WriteFile(filepath.Join(cfg, "config.yml"), []byte("version: 1\nlimits:\n  files: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if r := runBuild(t, dir, cfg, cli.Build{}); r.err == nil || !strings.Contains(r.err.Error(), "markdown") {
		t.Errorf("file limit: %v", r.err)
	}
}

func TestBuildMatchesApp(t *testing.T) {
	files := map[string]string{
		"README.md": "# Home\n\nSome *text* and `code`.\n\n```go\nfmt.Println(1)\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n```diagram\ngraph TD\n A-->B\n```\n",
		"evil.md":   "# Evil\n\n<script>alert(1)</script>\n\n<a href=\"javascript:alert(2)\" onclick=\"x()\">j</a> <img src=x onerror=alert(3)>\n\n[x](javascript:alert(4))\n",
	}
	dir := tree(t, files)
	cfg := t.TempDir()
	z, _, _ := openWith(t, dir, cfg, true)
	r := runBuild(t, dir, cfg, cli.Build{}, func(e *buildEnv) { e.Registry = z.state.base })
	if r.err != nil {
		t.Fatal(r.err)
	}
	for _, p := range []string{"README.md", "evil.md"} {
		pg, err := z.Page(p)
		if err != nil {
			t.Fatal(err)
		}
		built := r.read(t, strings.TrimSuffix(p, ".md")+".html")
		// Page.html of the app, verbatim, inside the shell's article.
		if !strings.Contains(built, "<article") || !strings.Contains(built, pg.HTML) {
			t.Errorf("%s: page HTML not in the shell.\napp:   %s\nbuilt: %s", p, pg.HTML, built)
		}
		for _, bad := range []string{"<script>alert", "onclick", "onerror", "javascript:"} {
			if strings.Contains(built, bad) {
				t.Errorf("%s: built page has %q", p, bad)
			}
		}
	}
	if !strings.Contains(r.read(t, "README.html"), `class="chroma"`) {
		t.Error("codeblock plugin did not run in the build")
	}
}

// hasLine reports whether one line of s has both a and b.
func hasLine(s, a, b string) bool {
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, a) && strings.Contains(l, b) {
			return true
		}
	}
	return false
}

func TestBuildRunsRenderPlugins(t *testing.T) {
	files := scriptDocs()
	for p, c := range pluginDocs() {
		if p != "README.md" && p != "other.md" {
			files[p] = c
		}
	}
	files["README.md"] = "# Home\n\n:::shout\nhello **world** :bang[hey]\n:::\n\n```hello\nx\n```\n"
	dir := pluginTree(t, files)
	cfg := t.TempDir()
	// The user allows both kinds in the app first.
	openWith(t, dir, cfg, true)
	if err := os.Remove(filepath.Join(dir, "ran.txt")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	r := runBuild(t, dir, cfg, cli.Build{Hosted: true})
	if r.err != nil {
		t.Fatal(r.err)
	}
	home := r.read(t, "README.html")
	for _, want := range []string{`<div class="shout">`, "<b>hey!</b>", "hello from the backend"} {
		if !strings.Contains(home, want) {
			t.Errorf("home lacks %q:\n%s", want, home)
		}
	}
	if !exists(filepath.Join(dir, "ran.txt")) {
		t.Error("the backend did not run during the build")
	}
	// Only the listed files of an allowed plugin are copied, and only for the page that uses it.
	for _, f := range []string{"_zen/plugins/hello/hello.js", "_zen/plugins/hello/hello.css"} {
		if !r.has(f) {
			t.Errorf("missing %s: %v", f, r.files(t))
		}
	}
	for _, f := range []string{"_zen/plugins/hello/secret.txt", "_zen/plugins/hello/lib/more.js", "_zen/plugins/hello/hello.sh", "_zen/plugins/hello/plugin.yml"} {
		if r.has(f) {
			t.Errorf("%s was copied", f)
		}
	}
	if strings.Contains(r.read(t, "other.html"), "hello.css") {
		t.Error("other.md links the plugin css")
	}
	if r.stderr != "" {
		t.Errorf("warnings: %q", r.stderr)
	}
}

func TestBuildTrustReadOnly(t *testing.T) {
	files := pluginDocs()
	for p, c := range layoutFiles() {
		if p != "README.md" && !strings.Contains(p, "/broken/") {
			files[p] = c
		}
	}
	files["README.md"] = "# Home\n\n```hello\nx\n```\n\n[memo](memo.md) [fancy](fancy.md)\n"
	files["memo.md"] = "---\nlayout: memo\nto: Ada\n---\n# Memo\n"
	files["fancy.md"] = "---\nlayout: fancy\n---\n# Fancy\n"
	files["docs.md"] = "---\nlayout: docs\n---\n# Docs\n\n## Part\n"
	dir := pluginTree(t, files)
	cfg := t.TempDir()
	trustFile := filepath.Join(cfg, "trusted-plugins.yml")

	// Nothing decided yet: left out, with a warning each, and nothing written.
	r := runBuild(t, dir, cfg, cli.Build{Hosted: true})
	if r.err != nil {
		t.Fatal(r.err)
	}
	if exists(trustFile) {
		t.Error("the build wrote the trust file")
	}
	if exists(filepath.Join(dir, "ran.txt")) {
		t.Error("an unapproved backend ran")
	}
	if strings.Contains(r.read(t, "README.html"), "hello from the backend") {
		t.Error("unapproved plugin output in the page")
	}
	for _, id := range []string{`plugin "hello"`, `layout "memo"`, `layout "fancy"`} {
		if !hasLine(r.stderr, id, "allow it in the zen app first") {
			t.Errorf("no warning for %s:\n%s", id, r.stderr)
		}
	}
	for _, f := range []string{"_zen/plugins/hello/hello.js", "_zen/plugins/hello/hello.css", "_zen/layouts/memo/memo.css", "_zen/layouts/fancy/f.js", "_zen/layouts/fancy/f.css"} {
		if r.has(f) {
			t.Errorf("unapproved file %s copied", f)
		}
	}
	if strings.Contains(r.read(t, "memo.html"), `class="memo"`) {
		t.Error("an unapproved layout template was used")
	}
	// Built-in layouts need no approval.
	if !strings.Contains(r.read(t, "docs.html"), `data-zen-layout="docs"`) {
		t.Error("built-in layout missing")
	}

	// Approved in the app: used. The trust file is read, never changed.
	openWith(t, dir, cfg, true)
	before, err := os.ReadFile(trustFile)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(dir, "ran.txt"))
	r = runBuild(t, dir, cfg, cli.Build{Hosted: true})
	if r.err != nil || r.stderr != "" {
		t.Fatalf("%v %q", r.err, r.stderr)
	}
	if !strings.Contains(r.read(t, "README.html"), "hello from the backend") || !r.has("_zen/layouts/memo/memo.css") || !r.has("_zen/layouts/fancy/f.js") {
		t.Errorf("approved plugin and layouts not used: %v", r.files(t))
	}
	if after, _ := os.ReadFile(trustFile); !bytes.Equal(before, after) {
		t.Error("the build changed the trust file")
	}

	// A plugin edited after approval is left out again (hash check), still read-only.
	if err := os.WriteFile(filepath.Join(dir, ".zen/plugins/hello/hello.sh"), []byte("#!/bin/sh\nprintf '<p>evil</p>'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".zen/layouts/fancy/f.js"), []byte("export default { mount() { evil() } }"), 0o644); err != nil {
		t.Fatal(err)
	}
	r = runBuild(t, dir, cfg, cli.Build{Clean: true, Hosted: true})
	if r.err != nil {
		t.Fatal(r.err)
	}
	if strings.Contains(r.read(t, "README.html"), "evil") || r.has("_zen/layouts/fancy/f.js") {
		t.Error("an edited, unapproved file was built")
	}
	if !hasLine(r.stderr, `plugin "hello"`, "allow it in the zen app first") || !hasLine(r.stderr, `layout "fancy"`, "allow it in the zen app first") {
		t.Errorf("no warning after the edit:\n%s", r.stderr)
	}
	if after, _ := os.ReadFile(trustFile); !bytes.Equal(before, after) {
		t.Error("the build changed the trust file after an edit")
	}
}

// optPlugin shows its options, to see whether a user-scoped one reaches a build.
type optPlugin struct{}

func (optPlugin) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID: "opt", Version: "0.1.0", APIVersion: plugins.APIVersion, Default: true, Blocks: []string{"opt"},
		Config: map[string]plugins.Option{
			"token": {Type: "string", Default: "none", Scope: "user"},
			"label": {Type: "string", Default: "plain"},
		},
	}
}

func (optPlugin) RenderBlock(ctx plugins.Context, b plugins.Block) (string, error) {
	return "<p>token=" + ctx.Config["token"].(string) + " label=" + ctx.Config["label"].(string) + "</p>", nil
}

func TestBuildNoUserOptions(t *testing.T) {
	dir := tree(t, map[string]string{
		"README.md": "# Home\n\n:::opt\nx\n:::\n",
		".zen.yml":  "version: 1\nplugins:\n  opt:\n    label: folder\n",
	})
	cfg := t.TempDir()
	if err := os.WriteFile(filepath.Join(cfg, "config.yml"), []byte("version: 1\nplugins:\n  opt:\n    token: SECRET-TOKEN\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	reg := plugins.NewRegistry()
	if err := reg.Register(optPlugin{}); err != nil {
		t.Fatal(err)
	}
	r := runBuild(t, dir, cfg, cli.Build{}, func(e *buildEnv) { e.Registry = reg })
	if r.err != nil {
		t.Fatal(r.err)
	}
	// ... the build never does: not in a page, not in a file.
	for _, f := range r.files(t) {
		if b := r.read(t, f); strings.Contains(b, "SECRET-TOKEN") {
			t.Errorf("%s contains the user-scoped option", f)
		}
	}
	home := r.read(t, "README.html")
	if !strings.Contains(home, "token=none label=folder") {
		t.Errorf("home: %s", home)
	}
	if !hasLine(r.stderr, `plugin "opt"`, "token") || !strings.Contains(r.stderr, "user") {
		t.Errorf("no warning about the user-scoped option:\n%s", r.stderr)
	}
}

func TestBuildWorkspaceFilesStayInside(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.png"), []byte("SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := tree(t, map[string]string{
		"README.md": "# Home\n\n![s](leak.png) ![t](../secret.png) ![u](/_ws/../secret.png)\n",
	})
	if err := os.Symlink(filepath.Join(outside, "secret.png"), filepath.Join(dir, "leak.png")); err != nil {
		t.Fatal(err)
	}
	r := runBuild(t, dir, t.TempDir(), cli.Build{})
	if r.err != nil {
		t.Fatal(r.err)
	}
	for _, f := range r.files(t) {
		if strings.Contains(r.read(t, f), "SECRET") {
			t.Errorf("%s contains a file from outside the folder", f)
		}
	}
	if r.has("leak.png") || r.has("secret.png") {
		t.Errorf("copied: %v", r.files(t))
	}
	// The reads themselves refuse: a symlink out and a path out of the folder.
	z, _, _ := openWith(t, dir, t.TempDir(), true)
	in := &buildInput{s: z.state}
	for _, p := range []string{"leak.png", "../secret.png", "/etc/hosts"} {
		if f, err := in.WorkspaceFile(p); err == nil {
			f.Close()
			t.Errorf("WorkspaceFile(%q) opened a file", p)
		}
	}
}

func TestBuildConfig(t *testing.T) {
	dir := tree(t, map[string]string{
		"README.md":   "# Home\n\n[d](docs.md) [w](wip.md) [s](secret/x.md)\n",
		"docs.md":     "---\nlayout: docs\n---\n# Docs\n\ntext\n",
		"wip.md":      "---\ndraft: true\n---\n# WIP\n",
		"secret/x.md": "# X\n",
		".zen.yml":    "version: 1\nbuild:\n  out: public\n  id: my-notes\n  editURL: https://github.com/me/notes/edit/main/{path}\n  exclude: [\"secret/**\"]\n",
	})
	cfg := t.TempDir()
	r := runBuild(t, dir, cfg, cli.Build{})
	if r.err != nil {
		t.Fatal(r.err)
	}
	r.out = filepath.Join(dir, "public") // from build.out
	if !r.has("docs.html") || r.has("wip.html") || r.has("secret/x.html") {
		t.Fatalf("files: %v", r.files(t))
	}
	if !strings.Contains(r.read(t, "docs.html"), `href="https://github.com/me/notes/edit/main/docs.md"`) {
		t.Errorf("edit link: %s", r.read(t, "docs.html"))
	}
	// Portable is the default; build.portable: false or --hosted loads scripts as files.
	csp := func(r buildRun) string { return strings.ReplaceAll(r.read(t, "README.html"), "&#39;", "'") }
	if !strings.Contains(csp(r), "script-src 'self' file: blob:") {
		t.Errorf("default build is not portable")
	}
	if !strings.Contains(r.stdout, "Built") || strings.Contains(r.stdout, "web server") {
		t.Errorf("portable summary: %q", r.stdout)
	}
	if h := runBuild(t, dir, cfg, cli.Build{Out: filepath.Join(t.TempDir(), "h"), Hosted: true}); h.err != nil ||
		strings.Contains(csp(h), "blob:") || !strings.Contains(h.stdout, "open it through a web server") {
		t.Errorf("--hosted: %v %q", h.err, h.stdout)
	}
	// Flags win: --drafts brings the draft back; -o beats build.out.
	flagOut := filepath.Join(t.TempDir(), "flag")
	r = runBuild(t, dir, cfg, cli.Build{Out: flagOut, Drafts: true})
	if r.err != nil || !r.has("wip.html") {
		t.Fatalf("drafts: %v %v", r.err, r.files(t))
	}
	// build.drafts and build.portable from the folder.
	if err := os.WriteFile(filepath.Join(dir, ".zen.yml"), []byte("version: 1\nbuild:\n  drafts: true\n  portable: true\n  zzz: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r = runBuild(t, dir, cfg, cli.Build{})
	if r.err != nil || !r.has("wip.html") || !strings.Contains(strings.ReplaceAll(r.read(t, "README.html"), "&#39;", "'"), "script-src 'self' file: blob:") || !strings.Contains(r.stderr, "build.zzz") {
		t.Errorf("config build section: %v %q", r.err, r.stderr)
	}
	// A wrong type is an error naming the key.
	if err := os.WriteFile(filepath.Join(dir, ".zen.yml"), []byte("version: 1\nbuild:\n  drafts: soon\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := runBuild(t, dir, cfg, cli.Build{}); r.err == nil || !strings.Contains(r.err.Error(), "build.drafts") {
		t.Errorf("wrong type: %v", r.err)
	}
}

// TestBuildDemo builds examples/demo (its plugins and layout approved first,
// as a user would in the app) and expects no errors and no warnings.
func TestBuildDemo(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	demo := filepath.Join(filepath.Dir(file), "..", "..", "examples", "demo")
	cfg := t.TempDir()
	openWith(t, demo, cfg, true)

	for _, portable := range []bool{false, true} {
		out := filepath.Join(t.TempDir(), "site")
		r := runBuild(t, demo, cfg, cli.Build{Out: out, Portable: portable, Hosted: !portable})
		if r.err != nil {
			t.Fatalf("portable=%v: %v", portable, r.err)
		}
		if r.stderr != "" {
			t.Errorf("portable=%v: warnings:\n%s", portable, r.stderr)
		}
		t.Logf("portable=%v: %s", portable, strings.TrimSpace(r.stdout))
		if !r.has("index.html") || !r.has("layouts/memo.html") || !r.has("_zen/layouts/memo/memo.css") || !r.has("build/index.html") {
			t.Errorf("files: %v", r.files(t))
		}
		if portable {
			if !strings.Contains(r.read(t, "plugins/writing-plugins.html"), `data-zen-module="plugin:progress"`) {
				t.Error("progress script not inlined")
			}
		} else if !r.has("_zen/plugins/progress/progress.js") {
			t.Error("progress script missing")
		}
		// The demo sidebar reaches every page; the deliberate broken link stays broken.
		if !strings.Contains(r.read(t, "reading/navigation.html"), `data-zen-link="broken"`) {
			t.Error("broken link lost")
		}
	}
	// Without approval the same folder still builds: warnings, not errors.
	r := runBuild(t, demo, t.TempDir(), cli.Build{Out: filepath.Join(t.TempDir(), "site")})
	if r.err != nil || !strings.Contains(r.stderr, "allow it in the zen app first") {
		t.Errorf("unapproved demo: %v\n%s", r.err, r.stderr)
	}
}

func TestBuildSiteWithoutBuilder(t *testing.T) {
	// BuildSite (the one main.go wires) fails cleanly when no runtime was set.
	StaticAssets = nil
	err := BuildSite(cli.Build{Dir: t.TempDir()}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "runtime") {
		t.Errorf("BuildSite: %v", err)
	}
}
