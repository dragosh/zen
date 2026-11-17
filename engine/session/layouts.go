package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dragosh/zen/internal/core/config"
	"github.com/dragosh/zen/internal/core/extplugins"
	"github.com/dragosh/zen/internal/core/index"
	"github.com/dragosh/zen/internal/core/layouts"
	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/core/workspace"
)

// EventLayoutsChanged (no payload) follows any change to the layout set or to
// a layout trust decision, and a folder open: the page side unmounts layout
// scripts and loads them again, and Settings › Layouts refreshes.
const EventLayoutsChanged = "layouts:changed"

// LayoutInfo is one row of Settings › Layouts.
type LayoutInfo struct {
	ID          string         `json:"id"`
	Version     string         `json:"version"`
	Description string         `json:"description"`
	Source      string         `json:"source"` // builtin | folder | user
	Status      string         `json:"status"` // on | blocked | undecided | invalid
	Kind        string         `json:"kind"`   // always "layout"
	HasTemplate bool           `json:"hasTemplate"`
	HasScript   bool           `json:"hasScript"`
	Fields      []LayoutField  `json:"fields"`
	Options     []LayoutOption `json:"options"`
	Help        PluginHelp     `json:"help"`
	Problem     string         `json:"problem"`
}

// LayoutField is a frontmatter field a layout reads.
type LayoutField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// LayoutOption is an option of a layout (frontmatter layoutOptions).
type LayoutOption struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Default     any    `json:"default"`
	Description string `json:"description"`
}

func layoutInfo(l *layouts.Layout) LayoutInfo {
	m := l.Manifest()
	info := LayoutInfo{
		ID: m.ID, Version: m.Version, Description: m.Description, Source: l.Source(), Kind: "layout",
		HasTemplate: l.HasTemplate(), HasScript: l.HasScript(),
		Fields: []LayoutField{}, Options: []LayoutOption{},
		Help: PluginHelp{Summary: m.Help.Summary, Snippets: []PluginSnippet{}},
	}
	for _, f := range m.Fields {
		info.Fields = append(info.Fields, LayoutField{Name: f.Name, Type: f.Type, Description: f.Description})
	}
	for _, o := range m.Options {
		info.Options = append(info.Options, LayoutOption{Name: o.Name, Type: o.Type, Default: o.Default, Description: o.Description})
	}
	for _, sn := range m.Help.Snippets {
		info.Help.Snippets = append(info.Help.Snippets, PluginSnippet{Title: sn.Title, Markdown: sn.Markdown})
	}
	return info
}

func invalidLayoutInfo(id, source, problem string) LayoutInfo {
	return LayoutInfo{
		ID: id, Source: source, Status: "invalid", Kind: "layout", Problem: problem,
		Fields: []LayoutField{}, Options: []LayoutOption{}, Help: PluginHelp{Snippets: []PluginSnippet{}},
	}
}

// layoutCand is a folder or user layout that loaded and can be registered.
type layoutCand struct {
	l      *layouts.Layout
	status string // allowed | blocked | undecided
}

// layoutState is everything derived from the layouts of the open folder.
type layoutState struct {
	root    string
	reg     *layouts.Registry
	cands   []layoutCand
	invalid []LayoutInfo
}

func (ls *layoutState) candidate(id string) *layoutCand {
	for i := range ls.cands {
		if ls.cands[i].l.Manifest().ID == id {
			return &ls.cands[i]
		}
	}
	return nil
}

func (ls *layoutState) undecided() bool {
	for _, c := range ls.cands {
		if c.status == "undecided" {
			return true
		}
	}
	return false
}

func layoutScope(root string, l *layouts.Layout) string {
	if l.Source() == layouts.SourceFolder {
		return root
	}
	return ""
}

func layoutTrustPrompt(root string, cands []layoutCand) string {
	var b strings.Builder
	if len(cands) == 1 {
		b.WriteString("Allow this layout? ")
	} else {
		b.WriteString("This folder uses layouts. ")
	}
	b.WriteString("A layout changes how pages look. zen asks before using one for the first time and again when its files change.\n")
	for _, c := range cands {
		m := c.l.Manifest()
		where := tilde(c.l.Dir())
		if rel, err := filepath.Rel(root, c.l.Dir()); err == nil && !strings.HasPrefix(rel, "..") {
			where = rel
		}
		fmt.Fprintf(&b, "\n%s %s (%s)\n", m.ID, m.Version, where)
		if m.Description != "" {
			fmt.Fprintf(&b, "  %s\n", m.Description)
		}
		for _, r := range c.l.Runs() {
			fmt.Fprintf(&b, "  %s\n", r)
		}
	}
	if len(cands) == 1 {
		b.WriteString("\nAllow it? Settings › Layouts changes the answer later.")
	} else {
		b.WriteString("\nAllow them? Blocked layouts stay off; Settings › Layouts changes the answer later.")
	}
	return b.String()
}

// loadLayouts discovers the layouts of the folder at root and the user's own,
// asks about new ones (when prompt is set and the dialogs can show it) and
// builds the registry. Nothing here runs a layout script.
func (s *Session) loadLayouts(root string, prompt bool) *layoutState {
	ls := &layoutState{root: root, reg: s.lbase.Clone()}
	if root == "" {
		return ls
	}
	var all []*layouts.Layout
	seen := map[string]bool{}
	for _, b := range s.lbase.All() {
		seen[b.Manifest().ID] = true
	}
	dirs := []struct{ dir, source string }{{filepath.Join(root, ".zen", "layouts"), layouts.SourceFolder}}
	if s.configDir != "" {
		dirs = append(dirs, struct{ dir, source string }{filepath.Join(s.configDir, "layouts"), layouts.SourceUser})
	}
	builtin := map[string]bool{}
	for id := range seen {
		builtin[id] = true
	}
	for _, d := range dirs {
		good, bad := layouts.Scan(d.dir, d.source)
		for _, in := range bad {
			if seen[in.ID] {
				continue
			}
			seen[in.ID] = true
			ls.invalid = append(ls.invalid, invalidLayoutInfo(in.ID, d.source, in.Problem))
			ls.reg.Invalidate(in.ID, in.Problem)
		}
		for _, l := range good {
			id := l.Manifest().ID
			if builtin[id] {
				msg := "a built-in layout already uses this id"
				ls.invalid = append(ls.invalid, invalidLayoutInfo(id, d.source, msg))
				continue
			}
			if seen[id] { // the folder's layout shadows the user's of the same id
				continue
			}
			seen[id] = true
			all = append(all, l)
		}
	}
	for _, l := range all {
		ls.cands = append(ls.cands, layoutCand{l: l})
	}
	decide := func() {
		for i := range ls.cands {
			l := ls.cands[i].l
			switch allowed, decided := s.trust.Decision(l, layoutScope(root, l)); {
			case !decided:
				ls.cands[i].status = "undecided"
			case allowed:
				ls.cands[i].status = "allowed"
			default:
				ls.cands[i].status = "blocked"
			}
		}
	}
	decide()
	if prompt && s.canAsk() {
		var ask []layoutCand
		for _, c := range ls.cands {
			if c.status == "undecided" {
				ask = append(ask, c)
			}
		}
		if len(ask) > 0 {
			allow := s.ask(layoutTrustPrompt(root, ask))
			for _, c := range ask {
				_ = s.trust.Set(c.l, layoutScope(root, c.l), allow)
			}
			decide()
		}
	}
	for _, c := range ls.cands {
		id := c.l.Manifest().ID
		if c.status != "allowed" {
			ls.reg.Deny(id)
			continue
		}
		cur := ls
		c.l.OnChange(func() { go s.layoutFilesChanged(cur) })
		_ = ls.reg.Register(c.l)
	}
	return ls
}

// Layouts lists the built-in, folder and user layouts for Settings › Layouts.
func (s *Session) Layouts() []LayoutInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []LayoutInfo{}
	for _, l := range s.lbase.All() {
		info := layoutInfo(l)
		info.Status = "on"
		out = append(out, info)
	}
	if s.lay == nil {
		return out
	}
	for _, c := range s.lay.cands {
		info := layoutInfo(c.l)
		info.Status = c.status
		if c.status == "allowed" {
			info.Status = "on"
		}
		out = append(out, info)
	}
	return append(out, s.lay.invalid...)
}

// LayoutTrust records the user's decision about a folder or user layout and
// reloads the layouts of the open folder. Allowing shows the native Allow /
// Block dialog first and records Allow only when the user confirms there;
// declining changes nothing. Blocking needs no dialog.
func (s *Session) LayoutTrust(id string, allow bool) error {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()
	s.mu.Lock()
	ls := s.lay
	s.mu.Unlock()
	if ls == nil || !plugins.ValidName(id) {
		return errors.New("unknown layout")
	}
	c := ls.candidate(id)
	if c == nil {
		return errors.New("unknown layout")
	}
	if allow {
		if !s.canAsk() {
			return errors.New("zen cannot ask for confirmation now; try again in a moment")
		}
		if !s.ask(layoutTrustPrompt(ls.root, []layoutCand{*c})) {
			return nil
		}
	}
	err := s.trust.Set(c.l, layoutScope(ls.root, c.l), allow)
	if err != nil && !errors.Is(err, extplugins.ErrChanged) {
		return err
	}
	s.reloadLayoutsHeld(false)
	if err != nil {
		return errors.New("the layout changed; check Settings › Layouts and allow it again")
	}
	return nil
}

// layoutFilesChanged is called when an allowed layout is found changed on
// disk: the layouts are loaded again, and ask again.
func (s *Session) layoutFilesChanged(was *layoutState) {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()
	s.mu.Lock()
	current := s.lay
	s.mu.Unlock()
	if current != was {
		return
	}
	s.reloadLayoutsHeld(false)
}

// reloadLayoutsHeld rebuilds the layout registry (asking about undecided
// layouts when prompt is set), re-renders and tells the frontend. The caller
// holds reloadMu.
func (s *Session) reloadLayoutsHeld(prompt bool) {
	s.mu.Lock()
	ws := s.ws
	s.mu.Unlock()
	if ws == nil {
		return
	}
	ls := s.loadLayouts(ws.Root(), prompt)
	s.mu.Lock()
	if s.ws != ws {
		s.mu.Unlock()
		return
	}
	s.lay, s.lreg = ls, ls.reg
	s.rebuildLocked()
	w := s.workspaceLocked()
	current := s.current
	s.mu.Unlock()
	s.emit(EventLayoutsChanged, nil)
	if w != nil {
		s.emit(EventWorkspaceChanged, w)
	}
	if current != "" {
		s.emit(EventPageChanged, PageChanged{Path: current})
	}
}

// PromptPendingLayouts asks about layouts that could not be asked about while
// the application was starting.
func (s *Session) PromptPendingLayouts() {
	s.mu.Lock()
	need := s.lay != nil && s.lay.undecided()
	s.mu.Unlock()
	if need && s.canAsk() {
		s.reloadMu.Lock()
		defer s.reloadMu.Unlock()
		s.reloadLayoutsHeld(true)
	}
}

// ---- storage ---------------------------------------------------------------

// layoutStoragePath is the storage file of layout id for the open folder:
// the layouts of the app and the folder's allowed layouts only. The course
// layout keeps using the file of the course plugin it replaces, so progress
// saved before survives.
func (s *Session) layoutStoragePath(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ws == nil || s.lay == nil {
		return "", errNoWorkspace
	}
	if !plugins.ValidName(id) {
		return "", errors.New("unknown layout")
	}
	l := s.lreg.Get(id)
	if l == nil {
		return "", fmt.Errorf("layout %q is not an allowed layout of this folder", id)
	}
	name := "_layouts/" + id
	if l.Source() == layouts.SourceBuiltin && id == "course" {
		name = "course" // legacy: the file the course plugin used
	}
	dir := s.dataDir(s.lay.root, name)
	if dir == "" {
		return "", errors.New("no settings folder on this system")
	}
	return filepath.Join(dir, "storage.json"), nil
}

// LayoutStorageGet returns the JSON text stored under key by a layout, "null"
// when there is none.
func (s *Session) LayoutStorageGet(id, key string) (string, error) {
	if err := validKey(key); err != nil {
		return "", err
	}
	path, err := s.layoutStoragePath(id)
	if err != nil {
		return "", err
	}
	return getAt(path, key)
}

// LayoutStorageSet stores a JSON value for a layout (1 MB per layout and
// folder, written atomically).
func (s *Session) LayoutStorageSet(id, key, value string) error {
	if err := validKey(key); err != nil {
		return err
	}
	if !json.Valid([]byte(value)) {
		return errors.New("the value is not valid JSON")
	}
	path, err := s.layoutStoragePath(id)
	if err != nil {
		return err
	}
	return setAt(path, key, value)
}

// LayoutStorageKeys lists the layout's storage keys that start with prefix
// ("" for all), sorted.
func (s *Session) LayoutStorageKeys(id, prefix string) ([]string, error) {
	if prefix != "" {
		if err := validKey(prefix); err != nil {
			return nil, err
		}
	}
	path, err := s.layoutStoragePath(id)
	if err != nil {
		return nil, err
	}
	return keysAt(path, prefix)
}

// ---- where a page sits: Prev, Next, Children, Breadcrumb -------------------

// layoutSite answers template layouts' questions from the sidebar tree. It
// is immutable: a new one is made whenever the index is rebuilt.
type layoutSite struct {
	ws       *workspace.Workspace
	idx      *index.Index
	flat     []NavNode            // the pages in sidebar order
	pos      map[string]int       // page path -> position in flat
	children map[string][]NavNode // page path -> pages under it
}

func newLayoutSite(ws *workspace.Workspace, idx *index.Index, tree []NavNode) *layoutSite {
	ls := &layoutSite{ws: ws, idx: idx, pos: map[string]int{}, children: map[string][]NavNode{}}
	var pagesUnder func(ns []NavNode) []NavNode
	pagesUnder = func(ns []NavNode) []NavNode { // pages, looking through groups
		var out []NavNode
		for _, n := range ns {
			if n.Kind == "page" {
				out = append(out, n)
			} else {
				out = append(out, pagesUnder(n.Children)...)
			}
		}
		return out
	}
	var walk func(ns []NavNode)
	walk = func(ns []NavNode) {
		for _, n := range ns {
			if n.Kind == "page" {
				ls.pos[n.Path] = len(ls.flat)
				ls.flat = append(ls.flat, n)
				ls.children[n.Path] = pagesUnder(n.Children)
			} else {
				// a folder group: its pages follow it in order
				ls.children[n.Path] = pagesUnder(n.Children)
			}
			walk(n.Children)
		}
	}
	walk(tree)
	return ls
}

func (ls *layoutSite) Breadcrumb(path string) []layouts.Crumb {
	if ls == nil || ls.idx == nil {
		return nil
	}
	var out []layouts.Crumb
	for _, c := range ls.idx.Breadcrumb(path) {
		out = append(out, layouts.Crumb{Title: c.Title, Path: c.Path})
	}
	return out
}

func (ls *layoutSite) Neighbours(path string) (prev, next *layouts.Link) {
	if ls == nil {
		return nil, nil
	}
	i, ok := ls.pos[path]
	if !ok {
		return nil, nil
	}
	if i > 0 {
		prev = &layouts.Link{Title: ls.flat[i-1].Title, Path: ls.flat[i-1].Path}
	}
	if i+1 < len(ls.flat) {
		next = &layouts.Link{Title: ls.flat[i+1].Title, Path: ls.flat[i+1].Path}
	}
	return prev, next
}

func (ls *layoutSite) Children(path string) []layouts.Child {
	if ls == nil {
		return nil
	}
	var out []layouts.Child
	for _, n := range ls.children[path] {
		c := layouts.Child{Title: n.Title, Path: n.Path}
		if b, err := ls.ws.Read(n.Path); err == nil {
			c.Summary = firstParagraph(b)
		}
		out = append(out, c)
	}
	return out
}

var (
	mdLinkRE = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	mdMarkRE = regexp.MustCompile("[*_`~]+")
)

var orderedRE = regexp.MustCompile(`^\d+[.)] `)

// firstParagraph is the plain text of the first paragraph of a page: the first
// block that is not a heading, code, a list, a table, a quote, a container or
// HTML, without markdown marks, at most 180 characters.
func firstParagraph(src []byte) string {
	_, body, _ := config.SplitFrontmatter(src)
	var block []string
	skipping, inFence := false, false
	for _, line := range strings.Split(string(body), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			inFence = !inFence
			block, skipping = nil, false
			continue
		}
		if inFence {
			continue
		}
		if t == "" {
			if len(block) > 0 {
				return summarise(strings.Join(block, " "))
			}
			skipping = false
			continue
		}
		if skipping {
			continue
		}
		if len(block) == 0 && (strings.HasPrefix(t, "#") || strings.HasPrefix(t, ":::") || strings.HasPrefix(t, "<") ||
			strings.HasPrefix(t, "|") || strings.HasPrefix(t, ">") || strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") ||
			strings.HasPrefix(t, "+ ") || strings.HasPrefix(t, "---") || orderedRE.MatchString(t)) {
			skipping = true
			continue
		}
		block = append(block, t)
	}
	if len(block) > 0 {
		return summarise(strings.Join(block, " "))
	}
	return ""
}

func summarise(text string) string {
	text = mdLinkRE.ReplaceAllString(text, "$1")
	text = mdMarkRE.ReplaceAllString(text, "")
	text = strings.Join(strings.Fields(text), " ")
	r := []rune(text)
	if len(r) <= 180 {
		return text
	}
	cut := string(r[:180])
	if i := strings.LastIndex(cut, " "); i > 100 {
		cut = cut[:i]
	}
	return cut + "…"
}
