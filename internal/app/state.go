package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dragosh/zen/internal/core/config"
	"github.com/dragosh/zen/internal/core/extplugins"
	"github.com/dragosh/zen/internal/core/index"
	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/core/render"
	"github.com/dragosh/zen/internal/core/watch"
	"github.com/dragosh/zen/internal/core/workspace"
)

// Event names emitted to the frontend.
const (
	EventWorkspaceChanged = "workspace:changed"
	EventPageChanged      = "page:changed"
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

type Page struct {
	Layout      string         `json:"layout"` // layout plugin of the page, "" when none
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
	ext        *extState // guarded by mu

	mu      sync.Mutex
	ws      *workspace.Workspace
	cfg     *config.Config
	r       *render.Renderer
	idx     *index.Index
	tree    []NavNode
	slugs   map[string]string   // frontmatter slug -> path
	navWarn map[string][]string // lower-cased path -> duplicate slug warnings
	current string              // last page served

	watchMu sync.Mutex // serialises watcher start/stop
	w       *watch.Watcher
}

func newState(reg *plugins.Registry, globalPath string, events EventSink) *state {
	if reg == nil {
		reg = plugins.NewRegistry()
	}
	return &state{base: reg, reg: reg, globalPath: globalPath, events: events, trust: extplugins.LoadTrust("")}
}

// open replaces the workspace with dir ("" closes it) and starts watching.
// It must not be called from the watch callback.
func (s *state) open(dir string) error {
	var ws *workspace.Workspace
	if dir != "" {
		var err error
		l := config.Load(s.globalPath, "").Limits() // global only: a folder cannot raise its own limits
		if ws, err = workspace.OpenWithLimits(dir, l.Files, l.Entries); err != nil {
			return err
		}
	}
	root := ""
	if ws != nil {
		root = ws.Root()
	}
	ext := s.loadExternal(root, true) // may ask the user; no lock is held
	s.watchMu.Lock()
	defer s.watchMu.Unlock()
	if s.w != nil {
		s.w.Stop()
		s.w = nil
	}
	s.mu.Lock()
	s.ws, s.current, s.idx = ws, "", nil
	s.ext, s.reg = ext, ext.reg
	s.rebuildLocked()
	s.mu.Unlock()
	if ws == nil {
		return nil
	}
	w, err := watch.Start(ws.Root(), s.handle)
	if err != nil {
		return err // the workspace stays usable without live reload
	}
	s.w = w
	return nil
}

func (s *state) stopWatch() {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()
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
	s.r = render.New(s.reg, s.cfg, &slugResolver{ws: s.ws, slugs: s.slugs}) // reindex refreshes the slugs
	return s.reindexLocked()
}

// reindexLocked rebuilds the index and reports whether the tree changed.
func (s *state) reindexLocked() bool {
	files, err := s.ws.Files()
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
	for _, p := range files {
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
	s.r = render.New(s.reg, s.cfg, &slugResolver{ws: s.ws, slugs: slugs})
	meta := func(p string) index.Meta {
		m := metas[p]
		if m.Parent != "" && !known[strings.ToLower(m.Parent)] {
			if sp, ok := slugs[m.Parent]; ok {
				m.Parent = sp // files win over slugs
			}
		}
		return m
	}

	links := func(p string) []string {
		var out []string
		for _, l := range s.r.Links(p, read(p)) {
			if l.Kind == "internal" && l.Path != "" {
				out = append(out, l.Path)
			}
		}
		return out
	}
	titles := func(p string) string { return s.r.Title(p, read(p)) }
	idx := index.BuildWithMeta(files, s.ws.Home(), links, titles, meta)
	changed := s.idx == nil || !s.idx.Equal(idx)
	s.idx = idx
	s.tree = navNodes(idx.Tree())
	return changed
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

func (s *state) page(path string) (Page, error) {
	if !isMarkdownPath(path) {
		return Page{}, s.cleanErr(path, workspace.ErrNotMarkdown)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ws == nil {
		return Page{}, errNoWorkspace
	}
	src, err := s.ws.Read(path)
	if err != nil {
		return Page{}, s.cleanErr(path, err)
	}
	p, err := s.renderLocked(path, src)
	if err == nil {
		s.current = path
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
	defer s.mu.Unlock()
	if s.ws == nil {
		return Page{}, errNoWorkspace
	}
	if _, err := s.ws.Resolve(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Page{}, s.cleanErr(path, err)
	}
	return s.renderLocked(path, []byte(content))
}

func (s *state) save(path, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ws == nil {
		return errNoWorkspace
	}
	return s.cleanErr(path, s.ws.Write(path, []byte(content)))
}

func (s *state) renderLocked(path string, src []byte) (Page, error) {
	rp, err := s.r.Render(path, src)
	if err != nil {
		return Page{}, s.cleanErr(path, err)
	}
	p := Page{
		Path: path, Title: rp.Title, HTML: rp.HTML, Layout: rp.Layout,
		Frontmatter: rp.Frontmatter, Breadcrumb: []Crumb{},
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
	if s.ext != nil {
		p.Warnings = append(p.Warnings, s.ext.warns...)
	}
	_, _, navWarn := index.ParseMeta(p.Frontmatter)
	p.Warnings = append(p.Warnings, navWarn...)
	p.Warnings = append(p.Warnings, s.navWarn[strings.ToLower(path)]...)
	p.Warnings = append(p.Warnings, s.idx.Warnings(path)...)
	if p.Warnings == nil {
		p.Warnings = []string{}
	}
	for _, c := range s.idx.Breadcrumb(path) {
		p.Breadcrumb = append(p.Breadcrumb, Crumb{Title: c.Title, Path: c.Path})
	}
	return p, nil
}

// cleanErr turns an error into a message without absolute paths.
func (s *state) cleanErr(rel string, err error) error {
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
	if s.ws != nil {
		msg = strings.ReplaceAll(msg, s.ws.Root()+string(os.PathSeparator), "")
		msg = strings.ReplaceAll(msg, s.ws.Root(), ".")
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
}

func isMarkdownPath(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".md", ".markdown":
		return true
	}
	return false
}
