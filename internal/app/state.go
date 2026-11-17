package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dragosh/zen/internal/cli"
	"github.com/dragosh/zen/internal/core/config"
	"github.com/dragosh/zen/internal/core/extplugins"
	"github.com/dragosh/zen/internal/core/index"
	"github.com/dragosh/zen/internal/core/layouts"
	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/core/render"
	"github.com/dragosh/zen/internal/core/watch"
	"github.com/dragosh/zen/internal/core/workspace"
	builtinlayouts "github.com/dragosh/zen/internal/layouts"
)

// Event names emitted to the frontend.
const (
	EventWorkspaceChanged = "workspace:changed"
	EventPageChanged      = "page:changed"
	// EventPluginsChanged (no payload) follows any change to the external
	// plugin set or to a trust decision, and a folder open: the page side
	// unmounts its plugins and loads them again from the current page.
	EventPluginsChanged = "plugins:changed"
)

// EventSink delivers events to the frontend.
type EventSink interface {
	Emit(name string, data any)
}

// NavNode, Workspace, Crumb, Page and Startup are the JSON shapes of
// docs/PLAN.md "Service API".
type NavNode struct {
	Title    string    `json:"title"`
	Path     string    `json:"path"`
	Kind     string    `json:"kind"`
	Children []NavNode `json:"children"`
}

type Workspace struct {
	Root string    `json:"root"`
	Name string    `json:"name"`
	Home string    `json:"home"`
	Tree []NavNode `json:"tree"`
}

type Crumb struct {
	Title string `json:"title"`
	Path  string `json:"path"`
}

type PageAssets struct {
	CSS []string `json:"css"`
	JS  []string `json:"js"`
}

// PageLayout is the layout a page is shown with; article and builtin when it
// chose none.
type PageLayout struct {
	Name   string `json:"name"`
	Source string `json:"source"` // builtin | folder | user
}

type Page struct {
	Layout      PageLayout     `json:"layout"`
	Path        string         `json:"path"`
	Title       string         `json:"title"`
	HTML        string         `json:"html"`
	Frontmatter map[string]any `json:"frontmatter"`
	Breadcrumb  []Crumb        `json:"breadcrumb"`
	Assets      PageAssets     `json:"assets"`
	Warnings    []string       `json:"warnings"`
}

type Startup struct {
	Workspace *Workspace `json:"workspace"`
	Page      string     `json:"page"`
	Mode      string     `json:"mode"`
	Error     string     `json:"error,omitempty"` // the folder given at launch did not open: why
}

// PageChanged is the payload of page:changed.
type PageChanged struct {
	Path string `json:"path"`
}

var errNoWorkspace = errors.New("no folder is open")

// state holds the open workspace and everything derived from it. All fields
// below mu are swapped together.
type state struct {
	base       *plugins.Registry // built-in plugins
	reg        *plugins.Registry // base plus the allowed plugins of the open folder (guarded by mu)
	globalPath string
	configDir  string // settings folder: user plugins, trust, plugin data; "" for none
	events     EventSink
	dialogs    Dialogs
	trust      *extplugins.Trust
	lbase      *layouts.Registry // built-in layouts
	lreg       *layouts.Registry // lbase plus the allowed layouts of the open folder (guarded by mu)
	lay        *layoutState      // guarded by mu
	site       *layoutSite       // where pages sit in the folder (guarded by mu)
	ext        *extState         // guarded by mu
	streams    streams           // running plugin streams (stream.go)
	reloadMu   sync.Mutex        // serialises loading the external plugins (open, trust changes, plugin edits)
	life       context.Context
	stop       context.CancelFunc // ends every plugin call (the app quits)

	mu      sync.Mutex
	ws      *workspace.Workspace
	cfg     *config.Config
	r       *render.Renderer
	idx     *index.Index
	tree    []NavNode
	slugs   map[string]string   // frontmatter slug -> path
	navWarn map[string][]string // lower-cased path -> duplicate slug warnings
	current string              // last page served

	noWatch bool // open does not start a watcher (the static build)

	watchMu  sync.Mutex // serialises watcher start/stop
	w        *watch.Watcher
	watchGen int // bumped by every start and stop; a late async start compares it

	openMu      sync.Mutex
	openCancel  context.CancelFunc // cancels the open in progress, nil when none
	openGen     int
	openTimeout time.Duration // 0 = from limits.openTimeout (tests set it)

	indexCtx  context.Context       // set while open indexes: cancel stops the index (guarded by mu)
	indexStep func(done, total int) // progress of that index (guarded by mu)
	indexErr  error                 // why that index stopped (guarded by mu)
}

func newState(reg *plugins.Registry, globalPath string, events EventSink) *state {
	if reg == nil {
		reg = plugins.NewRegistry()
	}
	if reg.AssetVersion() == "" {
		reg.SetAssetVersion(cli.Version) // built-in assets are versioned by the app version
	}
	life, stop := context.WithCancel(context.Background())
	lbase := builtinlayouts.Registry(cli.Version)
	return &state{base: reg, reg: reg, lbase: lbase, lreg: lbase, globalPath: globalPath, events: events, trust: extplugins.LoadTrust(""), life: life, stop: stop}
}

// emit sends an event unless there is nobody to send it to.
func (s *state) emit(name string, data any) {
	if s.events != nil {
		s.events.Emit(name, data)
	}
}

// open replaces the workspace with dir ("" closes it) and starts watching.
// It must not be called from the watch callback.
func (s *state) stopWatch() {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()
	s.watchGen++ // a watcher still starting is dropped
	if s.w != nil {
		s.w.Stop()
		s.w = nil
	}
}

// rebuildLocked reloads config, renderer and index and reports whether the
// tree changed.
func (s *state) rebuildLocked() bool {
	if s.ws == nil {
		s.cfg, s.r, s.idx, s.tree, s.slugs, s.navWarn = nil, nil, nil, nil, nil, nil
		return true
	}
	s.cfg = config.Load(s.globalPath, s.ws.Root())
	// Options with scope user come from the global config only.
	global := config.Load(s.globalPath, "").ForPage(nil)
	s.reg.SetUserOptions(func(id string) map[string]any {
		_, opts, _ := global.Plugin(id)
		return opts
	})
	s.r = render.New(s.reg, s.cfg, &slugResolver{ws: s.ws, slugs: s.slugs}).WithLayouts(s.lreg, s.siteOrNil()) // reindex refreshes the slugs
	return s.reindexLocked()
}

// reindexLocked rebuilds the index and reports whether the tree changed.
func (s *state) reindexLocked() bool {
	files := s.ws.Opened() // the listing open made, once
	var err error
	if files == nil {
		files, err = s.ws.Files()
	}
	if err != nil && s.idx != nil {
		// The folder grew past the limits while open: keep the current tree.
		return false
	}
	srcs := make(map[string][]byte, len(files))
	read := func(p string) []byte {
		if b, ok := srcs[p]; ok {
			return b
		}
		b, _ := s.ws.Read(p)
		srcs[p] = b
		return b
	}
	// Frontmatter first: slugs decide which links are internal.
	metas := make(map[string]index.Meta, len(files))
	slugOf := map[string]string{}
	known := make(map[string]bool, len(files))
	for i, p := range files {
		if i&63 == 0 && s.indexStoppedLocked() {
			return false
		}
		known[strings.ToLower(p)] = true
		fm, _, _ := config.SplitFrontmatter(read(p))
		m, slug, _ := index.ParseMeta(fm) // warnings are added when the page renders
		metas[p] = m
		if slug != "" {
			slugOf[p] = slug
		}
	}
	slugs, dup := buildSlugs(slugOf)
	s.navWarn = make(map[string][]string, len(dup))
	for p, w := range dup {
		s.navWarn[strings.ToLower(p)] = w
	}
	s.slugs = slugs
	s.r = render.New(s.reg, s.cfg, &slugResolver{ws: s.ws, slugs: slugs}).WithLayouts(s.lreg, s.siteOrNil())
	meta := func(p string) index.Meta {
		m := metas[p]
		if m.Parent != "" && !known[strings.ToLower(m.Parent)] {
			if sp, ok := slugs[m.Parent]; ok {
				m.Parent = sp // files win over slugs
			}
		}
		return m
	}

	done := 0
	links := func(p string) []string {
		// Called once per page while the index is built: progress and cancel.
		if s.indexStoppedLocked() {
			return nil
		}
		done++
		if s.indexStep != nil {
			s.indexStep(done, len(files))
		}
		var out []string
		for _, l := range s.r.Links(p, read(p)) {
			if l.Kind == "internal" && l.Path != "" {
				out = append(out, l.Path)
			}
		}
		return out
	}
	titles := func(p string) string {
		if s.indexStoppedLocked() {
			return ""
		}
		return s.r.Title(p, read(p))
	}
	idx := index.BuildWithMeta(files, s.ws.Home(), links, titles, meta)
	if s.indexStoppedLocked() {
		return false
	}
	changed := s.idx == nil || !s.idx.Equal(idx)
	s.idx = idx
	s.tree = navNodes(idx.Tree())
	s.site = newLayoutSite(s.ws, idx, s.tree)
	s.r = s.r.WithLayouts(s.lreg, s.site)
	return changed
}

// indexStoppedLocked reports whether the index open is building was cancelled
// or ran out of time; the reason is kept in indexErr.
func (s *state) indexStoppedLocked() bool {
	if s.indexErr != nil {
		return true
	}
	if s.indexCtx != nil {
		s.indexErr = s.indexCtx.Err()
	}
	return s.indexErr != nil
}

// siteOrNil is the layout site as a layouts.Site, nil (not a typed nil) when
// there is none yet.
func (s *state) siteOrNil() layouts.Site {
	if s.site == nil {
		return nil
	}
	return s.site
}

func navNodes(ns []index.Node) []NavNode {
	out := make([]NavNode, 0, len(ns))
	for _, n := range ns {
		out = append(out, NavNode{Title: n.Title, Path: n.Path, Kind: n.Kind, Children: navNodes(n.Children)})
	}
	return out
}

func (s *state) workspaceLocked() *Workspace {
	if s.ws == nil {
		return nil
	}
	return &Workspace{Root: s.ws.Root(), Name: s.ws.Name(), Home: s.ws.Home(), Tree: s.tree}
}

func (s *state) workspace() *Workspace {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.workspaceLocked()
}

func (s *state) home() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ws == nil {
		return ""
	}
	return s.ws.Home()
}

// renderSnap is what a render needs from the state, taken under the lock so
// the backends of external plugins run without holding it.
type renderSnap struct {
	r        *render.Renderer
	root     string
	extWarns []string
	navWarn  []string
	idxWarn  []string
	crumbs   []Crumb
}

func (s *state) snapLocked(path string) renderSnap {
	sn := renderSnap{r: s.r, root: s.ws.Root()}
	if s.ext != nil {
		sn.extWarns = append(sn.extWarns, s.ext.warns...)
	}
	sn.navWarn = append(sn.navWarn, s.navWarn[strings.ToLower(path)]...)
	sn.idxWarn = s.idx.Warnings(path)
	sn.crumbs = []Crumb{}
	for _, c := range s.idx.Breadcrumb(path) {
		sn.crumbs = append(sn.crumbs, Crumb{Title: c.Title, Path: c.Path})
	}
	return sn
}

// render runs the pipeline on src outside the state lock.
func (sn renderSnap) render(path string, src []byte) (Page, error) {
	rp, err := sn.r.Render(path, src)
	if err != nil {
		return Page{}, cleanErrFor(sn.root, path, err)
	}
	p := Page{
		Path: path, Title: rp.Title, HTML: rp.HTML, Layout: PageLayout{Name: rp.Layout.Name, Source: rp.Layout.Source},
		Frontmatter: rp.Frontmatter, Breadcrumb: sn.crumbs,
		Assets:   PageAssets{CSS: rp.Assets.CSS, JS: rp.Assets.JS},
		Warnings: rp.Warnings,
	}
	if p.Path == "" {
		p.Path = path
	}
	if p.Frontmatter == nil {
		p.Frontmatter = map[string]any{}
	}
	if p.Assets.CSS == nil {
		p.Assets.CSS = []string{}
	}
	if p.Assets.JS == nil {
		p.Assets.JS = []string{}
	}
	p.Warnings = append(p.Warnings, sn.extWarns...)
	_, _, navWarn := index.ParseMeta(p.Frontmatter)
	p.Warnings = append(p.Warnings, navWarn...)
	p.Warnings = append(p.Warnings, sn.navWarn...)
	p.Warnings = append(p.Warnings, sn.idxWarn...)
	if p.Warnings == nil {
		p.Warnings = []string{}
	}
	return p, nil
}

func (s *state) page(path string) (Page, error) {
	if !isMarkdownPath(path) {
		return Page{}, s.cleanErr(path, workspace.ErrNotMarkdown)
	}
	s.mu.Lock()
	if s.ws == nil {
		s.mu.Unlock()
		return Page{}, errNoWorkspace
	}
	ws := s.ws
	src, err := s.ws.Read(path)
	if err != nil {
		err = s.cleanErr(path, err)
		s.mu.Unlock()
		return Page{}, err
	}
	sn := s.snapLocked(path)
	s.mu.Unlock()

	p, err := sn.render(path, src) // backends run here, not under the lock
	if err == nil {
		s.mu.Lock()
		if s.ws == ws {
			s.current = path
		}
		s.mu.Unlock()
	}
	return p, err
}

func (s *state) source(path string) (string, error) {
	if !isMarkdownPath(path) {
		return "", s.cleanErr(path, workspace.ErrNotMarkdown)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ws == nil {
		return "", errNoWorkspace
	}
	src, err := s.ws.Read(path)
	if err != nil {
		return "", s.cleanErr(path, err)
	}
	return string(src), nil
}

func (s *state) preview(path, content string) (Page, error) {
	if !isMarkdownPath(path) {
		return Page{}, s.cleanErr(path, workspace.ErrNotMarkdown)
	}
	s.mu.Lock()
	if s.ws == nil {
		s.mu.Unlock()
		return Page{}, errNoWorkspace
	}
	if _, err := s.ws.Resolve(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		err = s.cleanErr(path, err)
		s.mu.Unlock()
		return Page{}, err
	}
	sn := s.snapLocked(path)
	s.mu.Unlock()
	return sn.render(path, []byte(content))
}

func (s *state) save(path, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ws == nil {
		return errNoWorkspace
	}
	return s.cleanErr(path, s.ws.Write(path, []byte(content)))
}

// cleanErr turns an error into a message without absolute paths.
func (s *state) cleanErr(rel string, err error) error {
	root := ""
	if s.ws != nil {
		root = s.ws.Root()
	}
	return cleanErrFor(root, rel, err)
}

// cleanErrFor is cleanErr for a workspace at root ("" for none).
func cleanErrFor(root, rel string, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, workspace.ErrOutside), errors.Is(err, workspace.ErrTooLarge),
		errors.Is(err, workspace.ErrNotMarkdown), errors.Is(err, workspace.ErrNotAsset):
		return fmt.Errorf("%s: %s", rel, strings.TrimPrefix(rootErr(err).Error(), "workspace: "))
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%s: not found", rel)
	case errors.Is(err, fs.ErrPermission):
		return fmt.Errorf("%s: permission denied", rel)
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s: %v", rel, pe.Err)
	}
	msg := err.Error()
	if root != "" {
		msg = strings.ReplaceAll(msg, root+string(os.PathSeparator), "")
		msg = strings.ReplaceAll(msg, root, ".")
	}
	return errors.New(msg)
}

func rootErr(err error) error {
	for _, e := range []error{workspace.ErrOutside, workspace.ErrTooLarge, workspace.ErrNotMarkdown, workspace.ErrNotAsset} {
		if errors.Is(err, e) {
			return e
		}
	}
	return err
}

// handle is the watch callback. It never stops the watcher.
func (s *state) handle(evs []watch.Event) {
	s.mu.Lock()
	if s.ws == nil {
		s.mu.Unlock()
		return
	}
	var pages []string
	seen := map[string]bool{}
	reindex, assets, cfg := false, false, false
	for _, e := range evs {
		switch {
		case e.Path == config.WorkspaceFile:
			cfg = true
		case isMarkdownPath(e.Path):
			reindex = true
			if !seen[e.Path] {
				seen[e.Path] = true
				pages = append(pages, e.Path)
			}
		default:
			assets = true
		}
	}
	changed := false
	if cfg {
		changed = s.rebuildLocked() // plugins may change titles and links
	} else if reindex {
		changed = s.reindexLocked()
	}
	var ws *Workspace
	if changed {
		ws = s.workspaceLocked()
	}
	if (assets || cfg) && s.current != "" && !seen[s.current] {
		pages = append(pages, s.current)
	}
	s.mu.Unlock()

	if ws != nil {
		s.events.Emit(EventWorkspaceChanged, ws)
	}
	for _, p := range pages {
		s.events.Emit(EventPageChanged, PageChanged{Path: p})
	}
	if cfg {
		s.reconcileStreams() // a plugin may have been switched off
	}
}

// reloadConfig re-reads the config files after one was saved in the editor
// and re-renders: the tree may change (plugins) and the page always may.
func (s *state) reloadConfig() {
	s.mu.Lock()
	if s.ws == nil {
		s.mu.Unlock()
		return
	}
	changed := s.rebuildLocked()
	var ws *Workspace
	if changed {
		ws = s.workspaceLocked()
	}
	current := s.current
	s.mu.Unlock()
	if ws != nil {
		s.events.Emit(EventWorkspaceChanged, ws)
	}
	if current != "" {
		s.events.Emit(EventPageChanged, PageChanged{Path: current})
	}
	s.reconcileStreams()
}

func isMarkdownPath(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".md", ".markdown":
		return true
	}
	return false
}
