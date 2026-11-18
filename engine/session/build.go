package session

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dragosh/zenmd/engine/config"
	"github.com/dragosh/zenmd/engine/plugins"
	"github.com/dragosh/zenmd/engine/site"
)

// SiteOptions are the options of a site build, the flags of `zenmd build`.
type SiteOptions struct {
	Dir      string // absolute folder
	Out      string // absolute output folder; "" = from config / default
	Portable bool   // opens from disk (the default)
	Hosted   bool   // scripts as files, for a web server
	Drafts   bool
	NoDrafts bool // leave drafts out even when build.drafts is true (the app's dialog)
	Clean    bool
	NoPrint  bool // no print.html
}

// SiteEnv is what a build needs from outside; the CLI fills in the real ones
// and tests their own.
type SiteEnv struct {
	GlobalPath string            // global config file ("" for none); its folder holds trust, user plugins and layouts
	Runtime    fs.FS             // frontend/dist: the static runtime of a built site is static/zen.js and static/zen.css in it
	Registry   *plugins.Registry // built-in plugins; nil for the default set
	Version    string            // asset versioning of the built-ins
	// Wrap, when set, decorates the site input before the build reads it
	// (the app counts pages for progress and cancels through it).
	Wrap func(site.Input) site.Input
}

// WriteSite is `zenmd build`: it writes the static site of b.Dir and prints the
// summary to stdout and the warnings to stderr. It opens no window and asks
// nothing; trust decisions are only read.
func WriteSite(env SiteEnv, b SiteOptions, stdout, stderr io.Writer) error {
	js, css, err := staticRuntime(env.Runtime)
	if err != nil {
		return err
	}
	root, err := filepath.Abs(b.Dir)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return fmt.Errorf("%s is not a folder", b.Dir)
	}

	cfg := config.Load(env.GlobalPath, root)
	bc, bwarn, err := cfg.Build()
	if err != nil {
		return err
	}
	// A headless session: no events, no watcher, no prompter, so nothing can
	// ask and the trust file is only read.
	st := New(Options{GlobalPath: env.GlobalPath, Registry: env.Registry, Version: env.Version})
	defer st.Close()
	if err := st.Open(root); err != nil {
		return st.cleanErr(".", err)
	}

	var warns []string
	add := func(format string, args ...any) { warns = append(warns, "site: "+fmt.Sprintf(format, args...)) }
	for _, w := range cfg.Warnings() {
		add("%s", strings.ReplaceAll(w, root+string(os.PathSeparator), ""))
	}
	for _, w := range bwarn {
		add("%s", w)
	}
	in := newBuildInput(st, js, css)
	for _, w := range in.statusWarnings(config.Load(env.GlobalPath, "")) {
		add("%s", w)
	}

	out := b.Out
	if out == "" {
		out = bc.Out
	}
	if out == "" {
		out = "_site"
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(root, out)
	}
	id := bc.ID
	if id == "" {
		id = filepath.Base(root)
	}
	// Portable output opens from disk and from any web server, so it is the
	// default; --hosted (or build.portable: false) loads scripts as files.
	portable := true
	if bc.Portable != nil {
		portable = *bc.Portable
	}
	if b.Portable {
		portable = true
	}
	if b.Hosted {
		portable = false
	}
	drafts := (bc.Drafts || b.Drafts) && !b.NoDrafts
	var input site.Input = in
	if env.Wrap != nil {
		input = env.Wrap(in)
	}
	rep, err := site.Build(input, site.Options{
		Out: filepath.Clean(out), SiteID: id, EditURL: bc.EditURL,
		Drafts: drafts, Portable: portable, NoPrint: b.NoPrint,
		Clean: b.Clean, Exclude: bc.Exclude,
	})
	if err != nil {
		return err
	}
	all := append(warns, rep.Warnings...)
	sort.Strings(all)
	for _, w := range all {
		fmt.Fprintln(stderr, w)
	}
	shown := out
	if rel, err := filepath.Rel(root, out); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		shown = rel
	}
	summary := fmt.Sprintf("Built %d pages and %d files into %s", rep.Pages, rep.Files, shown)
	switch n := len(all); n {
	case 0:
	case 1:
		summary += " (1 warning)"
	default:
		summary += fmt.Sprintf(" (%d warnings)", n)
	}
	fmt.Fprintln(stdout, summary)
	if rep.Print != "" {
		fmt.Fprintf(stdout, "Print version: %s\n", filepath.Join(shown, rep.Print))
	}
	if !portable {
		fmt.Fprintln(stdout, "Hosted output: open it through a web server; browsers block its scripts from disk.")
	}
	return nil
}

// staticRuntime reads the static runtime out of frontend/dist.
func staticRuntime(assets fs.FS) (js, css []byte, err error) {
	missing := errors.New("the static runtime (static/zen.js and static/zen.css) is missing from this build of zen; build the frontend first")
	if assets == nil {
		return nil, nil, missing
	}
	if js, err = fs.ReadFile(assets, "static/zen.js"); err != nil {
		return nil, nil, missing
	}
	if css, err = fs.ReadFile(assets, "static/zen.css"); err != nil {
		return nil, nil, missing
	}
	return js, css, nil
}

// buildInput adapts a session to site.Input. It renders exactly like the
// app does (Session.snapLocked and renderSnap.render).
type buildInput struct {
	s       *Session
	js, css []byte
}

func newBuildInput(s *Session, js, css []byte) *buildInput {
	// Options with scope user (tokens, images) belong to the reader of the
	// app, never to a site that is published: render-time plugins get none.
	s.mu.Lock()
	s.reg.SetUserOptions(func(string) map[string]any { return nil })
	s.mu.Unlock()
	return &buildInput{s: s, js: js, css: css}
}

// statusWarnings names what the build leaves out or withholds.
func (in *buildInput) statusWarnings(global *config.Config) []string {
	s := in.s
	s.mu.Lock()
	ext, lay, reg := s.ext, s.lay, s.reg
	s.mu.Unlock()
	var out []string
	if ext != nil {
		out = append(out, ext.warns...)
		for _, c := range ext.cands {
			if c.status != "allowed" {
				out = append(out, fmt.Sprintf("plugin %q is not allowed for this folder; allow it in the zen app first (Settings › Plugins)", c.p.Manifest().ID))
			}
		}
	}
	if lay != nil {
		for _, c := range lay.cands {
			if c.status != "allowed" {
				out = append(out, fmt.Sprintf("layout %q is not allowed for this folder; allow it in the zen app first (Settings › Layouts)", c.l.Manifest().ID))
			}
		}
		for _, inv := range lay.invalid {
			out = append(out, fmt.Sprintf("layout %q: %s", inv.ID, inv.Problem))
		}
	}
	// A user-scoped option that is set in the global config is withheld.
	gp := global.ForPage(nil)
	for _, m := range reg.All() {
		_, opts, _ := gp.Plugin(m.ID)
		var names []string
		for name, o := range m.Config {
			if _, set := opts[name]; set && o.Scope == plugins.ScopeUser {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		for _, name := range names {
			out = append(out, fmt.Sprintf("plugin %q: option %q is user-scoped (global config only) and is not available in a build", m.ID, name))
		}
	}
	return out
}

func (in *buildInput) Pages() ([]string, error) {
	in.s.mu.Lock()
	ws := in.s.ws
	in.s.mu.Unlock()
	return ws.Files()
}

func (in *buildInput) Root() string {
	in.s.mu.Lock()
	defer in.s.mu.Unlock()
	return in.s.ws.Root()
}

func (in *buildInput) Home() string { return in.s.Home() }

func (in *buildInput) Tree() []site.NavNode {
	in.s.mu.Lock()
	defer in.s.mu.Unlock()
	return siteNodes(in.s.tree)
}

func siteNodes(ns []NavNode) []site.NavNode {
	out := make([]site.NavNode, 0, len(ns))
	for _, n := range ns {
		out = append(out, site.NavNode{Title: n.Title, Path: n.Path, Kind: n.Kind, Children: siteNodes(n.Children)})
	}
	return out
}

func (in *buildInput) Render(path string) (site.Page, error) {
	s := in.s
	s.mu.Lock()
	src, err := s.ws.Read(path)
	if err != nil {
		err = s.cleanErr(path, err)
		s.mu.Unlock()
		return site.Page{}, err
	}
	sn := s.snapLocked(path)
	s.mu.Unlock()
	sn.extWarns = nil // reported once for the site, not on every page
	p, err := sn.render(path, src)
	if err != nil {
		return site.Page{}, err
	}
	out := site.Page{
		Path: p.Path, Title: p.Title, HTML: p.HTML, Layout: p.Layout.Name,
		CSS: p.Assets.CSS, JS: p.Assets.JS, Warnings: p.Warnings,
	}
	out.Draft, _ = p.Frontmatter["draft"].(bool)
	out.Lang, _ = p.Frontmatter["lang"].(string)
	for _, c := range p.Breadcrumb {
		out.Breadcrumb = append(out.Breadcrumb, site.Crumb{Title: c.Title, Path: c.Path})
	}
	return out, nil
}

func (in *buildInput) Slug(slug string) (string, bool) {
	in.s.mu.Lock()
	defer in.s.mu.Unlock()
	p, ok := in.s.slugs[slug]
	return p, ok
}

func (in *buildInput) WorkspaceFile(path string) (fs.File, error) {
	in.s.mu.Lock()
	ws := in.s.ws
	in.s.mu.Unlock()
	f, err := ws.Asset(path) // root-confined, no symlinks out, images only
	if err != nil {
		return nil, err
	}
	return f, nil
}

// AssetFiles lists every file the app serves for a plugin or layout (kind
// "plugins" | "layouts"): all of a built-in's embedded assets; for external
// plugins, whose files cannot be listed, nothing beyond what pages name.
func (in *buildInput) AssetFiles(kind, id string) []string {
	in.s.mu.Lock()
	reg, lreg := in.s.reg, in.s.lreg
	in.s.mu.Unlock()
	var fsys fs.FS
	var ok bool
	switch {
	case kind == "plugins" && reg != nil:
		fsys, ok = reg.AssetFS(id)
	case kind == "layouts" && lreg != nil:
		fsys, ok = lreg.AssetFS(id)
	}
	if !ok {
		return nil
	}
	var out []string
	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return fs.SkipDir
		}
		if d.Type().IsRegular() && path.Base(p) != ".DS_Store" {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// Asset opens a /_plugins/ or /_layouts/ URL the way the app serves it.
func (in *buildInput) Asset(u string) (fs.File, error) {
	p := u
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	if _, err := url.PathUnescape(p); err != nil {
		return nil, fs.ErrNotExist
	}
	in.s.mu.Lock()
	reg, lreg := in.s.reg, in.s.lreg
	in.s.mu.Unlock()
	var fsys fs.FS
	var file string
	var ok bool
	switch {
	case strings.HasPrefix(p, "/_plugins/"):
		fsys, file, ok = assetOf(reg, strings.TrimPrefix(p, "/_plugins/"))
	case strings.HasPrefix(p, "/_layouts/") && lreg != nil:
		fsys, file, ok = assetOf(lreg, strings.TrimPrefix(p, "/_layouts/"))
	}
	if !ok {
		return nil, fs.ErrNotExist
	}
	f, err := fsys.Open(file)
	if err != nil {
		return nil, err
	}
	if fi, err := f.Stat(); err != nil || fi.IsDir() {
		f.Close()
		return nil, fs.ErrNotExist
	}
	return f, nil
}

func (in *buildInput) Runtime() (js, css []byte) { return in.js, in.css }
