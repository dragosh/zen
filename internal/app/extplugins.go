package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/dragosh/zen/internal/core/atomicfile"
	"github.com/dragosh/zen/internal/core/extplugins"
	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/core/scriptplugins"
)

// PluginInfo is one row of Settings › Plugins.
type PluginInfo struct {
	ID          string   `json:"id"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Source      string   `json:"source"` // builtin | folder | user
	Status      string   `json:"status"` // on | off | blocked | undecided | invalid
	Runs        []string `json:"runs"`
	Problem     string   `json:"problem"`
	// Kind says what the plugin is made of: builtin (Go, in zen), program (a
	// backend zen runs), page (scripts and styles only) or script (one Tengo
	// file zen runs itself). For a rejected plugin it is a best guess.
	Kind string `json:"kind"`
	// Help is what the plugin says about itself; Blocks, Inlines and
	// Options are what it adds. They are empty lists, never null.
	Help    PluginHelp     `json:"help"`
	Blocks  []string       `json:"blocks"`
	Inlines []string       `json:"inlines"`
	Options []PluginOption `json:"options"`
}

// PluginHelp is a plugin's summary and its snippets (Settings › Plugins).
type PluginHelp struct {
	Summary  string          `json:"summary"`
	Snippets []PluginSnippet `json:"snippets"`
}

// PluginSnippet is one example to copy into a page.
type PluginSnippet struct {
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
}

// PluginOption is one option a plugin reads from the config or a page.
type PluginOption struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Default     any    `json:"default"`
	Description string `json:"description"`
}

// pluginInfo builds a list row from a manifest; the caller sets the rest.
func pluginInfo(m plugins.Manifest) PluginInfo {
	info := PluginInfo{
		ID: m.ID, Version: m.Version, Description: m.Description, Runs: []string{},
		Help:    PluginHelp{Summary: m.Help.Summary, Snippets: []PluginSnippet{}},
		Blocks:  append([]string{}, m.Blocks...),
		Inlines: append([]string{}, m.Inlines...),
		Options: []PluginOption{},
	}
	for _, sn := range m.Help.Snippets {
		info.Help.Snippets = append(info.Help.Snippets, PluginSnippet{Title: sn.Title, Markdown: sn.Markdown})
	}
	names := make([]string, 0, len(m.Config))
	for n := range m.Config {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		o := m.Config[n]
		info.Options = append(info.Options, PluginOption{Name: n, Type: o.Type, Default: o.Default, Description: o.Description})
	}
	return info
}

const (
	storageLimit = 1 << 20 // bytes of storage per plugin and folder
	maxKeyLen    = 64
)

// trustAsker is implemented by dialogs that have a dedicated trust prompt.
type trustAsker interface{ ConfirmTrust(message string) bool }

// readyDialogs is implemented by dialogs that cannot show anything yet at
// start-up (before the application window exists).
type readyDialogs interface{ Ready() bool }

// extPlugin is what the app needs of an external plugin of any kind: a
// folder plugin (extplugins) or a script plugin (scriptplugins).
type extPlugin interface {
	plugins.Plugin
	extplugins.Subject // Dir, Hash, Changed: what trust decides on
	OnChange(func())
	Runs() []string
	Kind() string // program | page | script
}

// candidate is an external plugin that loaded and can be registered.
type candidate struct {
	p      extPlugin
	source string // folder | user
	status string // allowed | blocked | undecided
}

// extState is everything derived from the external plugins of the open folder.
type extState struct {
	ctx     context.Context // ends the calls of these plugins when they are replaced or the app quits
	cancel  context.CancelFunc
	reg     *plugins.Registry
	root    string
	cands   []candidate
	invalid []PluginInfo
	warns   []string
	active  map[string]extPlugin // allowed and registered
}

func (e *extState) candidate(id string) *candidate {
	for i := range e.cands {
		if e.cands[i].p.Manifest().ID == id {
			return &e.cands[i]
		}
	}
	return nil
}

func (e *extState) undecided() bool {
	for _, c := range e.cands {
		if c.status == "undecided" {
			return true
		}
	}
	return false
}

// cloneBase returns a registry holding the built-in plugins.
func (s *state) cloneBase() *plugins.Registry {
	r := plugins.NewRegistry()
	r.SetAssetVersion(s.base.AssetVersion())
	for _, p := range s.base.Plugins() {
		_ = r.Register(p)
	}
	return r
}

func folderKey(root string) string {
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:])[:16]
}

// dataDir is the per-folder, per-plugin folder under the settings folder.
func (s *state) dataDir(root, id string) string {
	if s.configDir == "" {
		return ""
	}
	return filepath.Join(s.configDir, "data", folderKey(root), id)
}

func (s *state) canAsk() bool {
	if s.dialogs == nil {
		return false
	}
	if r, ok := s.dialogs.(readyDialogs); ok {
		return r.Ready()
	}
	return true
}

func (s *state) ask(msg string) bool {
	if t, ok := s.dialogs.(trustAsker); ok {
		return t.ConfirmTrust(msg)
	}
	return s.dialogs.Confirm(msg)
}

func tilde(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

// pageScripts is the line of Runs that says a plugin ships scripts for the page.
const pageScripts = "scripts in the page"

func trustPrompt(root string, cands []candidate) string {
	// Script plugins that ship no page script cannot run programs: say so,
	// instead of the warning that fits programs.
	scriptsOnly := true
	for _, c := range cands {
		if c.p.Kind() != "script" {
			scriptsOnly = false
		}
		for _, r := range c.p.Runs() {
			if r == pageScripts {
				scriptsOnly = false
			}
		}
	}
	var b strings.Builder
	if len(cands) == 1 {
		b.WriteString("Allow this plugin? ")
	} else {
		b.WriteString("This folder uses plugins. ")
	}
	if scriptsOnly {
		b.WriteString("Script plugins run inside zen, with a time and memory limit, and reach nothing zen does not give them. " +
			"zen asks before using one for the first time and again when its files change.\n")
	} else {
		b.WriteString("A plugin runs programs and scripts on your Mac with your own access to files and the network, " +
			"and its script in the page can use zen's functions. zen asks before using one for the first time and again when its files change.\n")
	}
	for _, c := range cands {
		m := c.p.Manifest()
		where := tilde(c.p.Dir())
		if rel, err := filepath.Rel(root, c.p.Dir()); err == nil && !strings.HasPrefix(rel, "..") {
			where = rel
		}
		fmt.Fprintf(&b, "\n%s %s (%s)\n", m.ID, m.Version, where)
		if m.Description != "" {
			fmt.Fprintf(&b, "  %s\n", m.Description)
		}
		for _, r := range c.p.Runs() {
			if c.p.Kind() == "script" && r != pageScripts {
				fmt.Fprintf(&b, "  %s\n", r) // what a script plugin can do, as a sentence
			} else {
				fmt.Fprintf(&b, "  runs: %s\n", r)
			}
		}
	}
	if len(cands) == 1 {
		b.WriteString("\nAllow it? Settings › Plugins changes the answer later.")
	} else {
		b.WriteString("\nAllow them? Blocked plugins stay off; Settings › Plugins changes the answer later.")
	}
	return b.String()
}

// scopeOf is the trust scope of a plugin: the folder root for a plugin of the
// folder, "" for one of the user's own (those apply to every folder).
func scopeOf(root string, c candidate) string {
	if c.source == "folder" {
		return root
	}
	return ""
}

// loadExternal discovers the external plugins of the folder at root, asks
// the user about new ones (when prompt is set and the dialogs can show it),
// and builds the registry. Nothing external runs here.
func (s *state) loadExternal(root string, prompt bool) *extState {
	e := &extState{root: root, active: map[string]extPlugin{}}
	var cancel context.CancelFunc
	e.ctx, cancel = context.WithCancel(s.life)
	// The previous folder's calls end and its idle script helpers go (busy
	// ones go when their call ends).
	e.cancel = func() { cancel(); scriptplugins.Reset() }
	if n := s.trust.Notice(); n != "" {
		e.warns = append(e.warns, n)
	}
	if root == "" {
		e.reg = s.base
		return e
	}
	folderDir := filepath.Join(root, ".zen", "plugins")
	userDir := ""
	if s.configDir != "" {
		userDir = filepath.Join(s.configDir, "plugins")
	}
	type found struct {
		p      extPlugin
		source string
	}
	var all []found
	seen := map[string]bool{}
	addInvalid := func(id, source, kind, problem string) {
		e.invalid = append(e.invalid, invalidInfo(id, source, kind, problem))
		e.warns = append(e.warns, fmt.Sprintf("plugin %q: %s", id, problem))
	}
	for _, d := range []struct{ dir, source string }{{folderDir, "folder"}, {userDir, "user"}} {
		if d.dir == "" {
			continue
		}
		ps, inv := extplugins.Scan(d.dir)
		sps, sinv := scriptplugins.Scan(d.dir)
		// An id defined by a plugin.yml folder and by a .tengo plugin of the same
		// folder is rejected, not guessed.
		inExt, inScript := map[string]bool{}, map[string]bool{}
		for _, p := range ps {
			inExt[p.Manifest().ID] = true
		}
		for _, in := range inv {
			inExt[in.ID] = true
		}
		for _, p := range sps {
			inScript[p.Manifest().ID] = true
		}
		for _, in := range sinv {
			inScript[in.ID] = true
		}
		clash := func(id string) bool { return inExt[id] && inScript[id] }
		type invalidItem struct{ ID, Problem, Kind string }
		var bad []invalidItem
		for _, in := range inv {
			if !clash(in.ID) {
				bad = append(bad, invalidItem{in.ID, in.Problem, "program"})
			}
		}
		for _, in := range sinv {
			if !clash(in.ID) {
				bad = append(bad, invalidItem{in.ID, in.Problem, "script"})
			}
		}
		for id := range inExt {
			if clash(id) {
				bad = append(bad, invalidItem{id, "both a plugin.yml folder and a .tengo plugin define this id; keep one", "script"})
			}
		}
		sort.Slice(bad, func(i, j int) bool { return bad[i].ID < bad[j].ID })
		for _, in := range bad {
			if !seen[in.ID] { // the folder's plugin shadows the user's of the same id
				seen[in.ID] = true
				addInvalid(in.ID, d.source, in.Kind, in.Problem)
			}
		}
		var good []extPlugin
		for _, p := range ps {
			if !clash(p.Manifest().ID) {
				good = append(good, p)
			}
		}
		for _, p := range sps {
			if !clash(p.Manifest().ID) {
				good = append(good, p)
			}
		}
		sort.Slice(good, func(i, j int) bool { return good[i].Manifest().ID < good[j].Manifest().ID })
		for _, p := range good {
			id := p.Manifest().ID
			if seen[id] {
				continue
			}
			seen[id] = true
			all = append(all, found{p, d.source})
		}
	}

	// Which ones could be registered at all? Only those are ever asked about.
	scratch := s.cloneBase()
	builtin := map[string]bool{}
	for _, m := range s.base.All() {
		builtin[m.ID] = true
	}
	for _, f := range all {
		id := f.p.Manifest().ID
		if builtin[id] {
			addInvalid(id, f.source, f.p.Kind(), "a built-in plugin already uses this id")
			continue
		}
		if err := scratch.Register(f.p); err != nil {
			addInvalid(id, f.source, f.p.Kind(), strings.TrimPrefix(err.Error(), fmt.Sprintf("plugin %q: ", id)))
			continue
		}
		e.cands = append(e.cands, candidate{p: f.p, source: f.source})
	}

	decide := func() {
		for i := range e.cands {
			switch allowed, decided := s.trust.Decision(e.cands[i].p, scopeOf(root, e.cands[i])); {
			case !decided:
				e.cands[i].status = "undecided"
			case allowed:
				e.cands[i].status = "allowed"
			default:
				e.cands[i].status = "blocked"
			}
		}
	}
	decide()
	if prompt && s.canAsk() {
		var ask []candidate
		for _, c := range e.cands {
			if c.status == "undecided" {
				ask = append(ask, c)
			}
		}
		if len(ask) > 0 {
			allow := s.ask(trustPrompt(root, ask))
			for _, c := range ask {
				if err := s.trust.Set(c.p, scopeOf(root, c), allow); err != nil {
					e.warns = append(e.warns, fmt.Sprintf("plugin %q: %v", c.p.Manifest().ID, err))
				}
			}
			decide()
		}
	}

	e.reg = s.cloneBase()
	for _, c := range e.cands {
		m := c.p.Manifest()
		if c.status != "allowed" {
			e.reg.Deny(m)
			continue
		}
		data := s.dataDir(root, m.ID)
		if data != "" {
			data = filepath.Join(data, "files")
		}
		switch pl := c.p.(type) {
		case *extplugins.Plugin:
			pl.Bind(extplugins.Runtime{Workspace: root, DataDir: data, Ctx: e.ctx})
		case *scriptplugins.Plugin:
			pl.Bind(scriptplugins.Runtime{Workspace: root, Ctx: e.ctx, Storage: pluginStore{s: s, id: m.ID}})
		}
		c.p.OnChange(func() { go s.pluginFilesChanged(e) })
		if err := e.reg.Register(c.p); err == nil {
			e.active[m.ID] = c.p
		}
	}
	return e
}

func invalidInfo(id, source, kind, problem string) PluginInfo {
	info := pluginInfo(plugins.Manifest{ID: id})
	info.Source, info.Status, info.Problem, info.Kind = source, "invalid", problem, kind
	return info
}

// Plugins lists the built-in and external plugins for Settings › Plugins.
func (z *Zen) Plugins() []PluginInfo { return z.state.pluginList() }

func (s *state) pluginList() []PluginInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	enabled := func(id string, def bool) bool {
		if s.cfg == nil {
			return def
		}
		if on, _, mentioned := s.cfg.ForPage(nil).Plugin(id); mentioned {
			return on
		}
		return def
	}
	onOff := func(on bool) string {
		if on {
			return "on"
		}
		return "off"
	}
	out := []PluginInfo{}
	for _, m := range s.base.All() {
		info := pluginInfo(m)
		info.Source, info.Status, info.Kind = "builtin", onOff(enabled(m.ID, m.Default)), "builtin"
		out = append(out, info)
	}
	if s.ext == nil {
		return out
	}
	for _, c := range s.ext.cands {
		m := c.p.Manifest()
		st := c.status
		switch st {
		case "allowed":
			st = onOff(enabled(m.ID, m.Default))
		}
		info := pluginInfo(m)
		info.Source, info.Status, info.Runs, info.Kind = c.source, st, append([]string{}, c.p.Runs()...), c.p.Kind()
		out = append(out, info)
	}
	return append(out, s.ext.invalid...)
}

// PluginTrust records the user's decision about an external plugin and
// reloads the plugins of the open folder. Allowing shows the native Allow /
// Block dialog first and records Allow only when the user confirms there
// (nothing a page script does can allow a plugin); declining changes nothing.
// Blocking needs no dialog.
func (z *Zen) PluginTrust(id string, allow bool) error { return z.state.pluginTrust(id, allow) }

func (s *state) pluginTrust(id string, allow bool) error {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()
	s.mu.Lock()
	ext := s.ext
	s.mu.Unlock()
	if ext == nil || !plugins.ValidName(id) {
		return errors.New("unknown plugin")
	}
	c := ext.candidate(id)
	if c == nil {
		return errors.New("unknown plugin")
	}
	if allow {
		if !s.canAsk() {
			return errors.New("zen cannot ask for confirmation now; try again in a moment")
		}
		if !s.ask(trustPrompt(ext.root, []candidate{*c})) {
			return nil
		}
	}
	err := s.trust.Set(c.p, scopeOf(ext.root, *c), allow)
	if err != nil && !errors.Is(err, extplugins.ErrChanged) {
		return err
	}
	if !allow {
		s.streams.cancelPlugin(id)
	}
	s.reloadHeld(false, true)
	if err != nil {
		return errors.New("the plugin changed; check Settings › Plugins and allow it again")
	}
	return nil
}

// pluginFilesChanged is called when a plugin that was allowed is found
// changed on disk: it is loaded again, and asks again.
func (s *state) pluginFilesChanged(was *extState) {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()
	s.mu.Lock()
	current := s.ext
	s.mu.Unlock()
	if current != was { // already replaced
		return
	}
	s.reloadHeld(false, true)
}

// reloadExternal rebuilds the registry (asking about undecided plugins when
// prompt is set), re-renders and tells the frontend. Reloads are serialised.
func (s *state) reloadExternal(prompt, always bool) {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()
	s.reloadHeld(prompt, always)
}

// reloadHeld is reloadExternal for callers that hold reloadMu.
func (s *state) reloadHeld(prompt, always bool) {
	s.mu.Lock()
	ws := s.ws
	s.mu.Unlock()
	if ws == nil {
		return
	}
	ext := s.loadExternal(ws.Root(), prompt)
	s.mu.Lock()
	if s.ws != ws { // another folder was opened meanwhile
		s.mu.Unlock()
		ext.cancel()
		return
	}
	old := s.ext
	s.ext, s.reg = ext, ext.reg
	s.rebuildLocked()
	w := s.workspaceLocked()
	current := s.current
	s.mu.Unlock()
	if old != nil {
		old.cancel()
	}
	s.reconcileStreams()
	s.emit(EventPluginsChanged, nil)
	if w != nil && always {
		s.emit(EventWorkspaceChanged, w)
	}
	if current != "" {
		s.emit(EventPageChanged, PageChanged{Path: current})
	}
}

// promptPending asks about plugins that could not be asked about while the
// application was starting.
func (s *state) promptPending() {
	s.mu.Lock()
	need := s.ext != nil && s.ext.undecided()
	s.mu.Unlock()
	if need && s.canAsk() {
		s.reloadExternal(true, true)
	}
}

// target is a plugin the frontend talks to: a built-in one that is switched
// on, or an external one the user allowed.
type target struct {
	p    plugins.Plugin
	root string
	ctx  plugins.Context // with the folder and the plugin's data folder
}

// target finds plugin id for the open folder, for the page at page (its
// frontmatter decides which plugins are on and their options; ZEN_PAGE is
// page). Built-in plugins must be switched on; external ones only when needOn
// (invoke and stream), storage stays reachable for an allowed plugin that is
// switched off. An empty page means no page: the folder's settings.
func (s *state) target(id, page string, needOn bool) (target, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ws == nil || s.ext == nil {
		return target{}, errNoWorkspace
	}
	if !plugins.ValidName(id) {
		return target{}, errors.New("unknown plugin")
	}
	var p plugins.Plugin
	builtin := false
	for _, bp := range s.base.Plugins() {
		if bp.Manifest().ID == id {
			p, builtin = bp, true
		}
	}
	if !builtin {
		ep := s.ext.active[id]
		if ep == nil {
			return target{}, fmt.Errorf("plugin %q is not an allowed plugin of this folder", id)
		}
		p = ep
	}
	var ctx plugins.Context
	if s.r != nil {
		var src []byte
		if page != "" {
			if !isMarkdownPath(page) {
				return target{}, errors.New("the page is not a markdown file")
			}
			b, err := s.ws.Read(page)
			switch {
			case err == nil:
				src = b
			case !errors.Is(err, fs.ErrNotExist): // a page that is not saved yet has no frontmatter on disk
				return target{}, s.cleanErr(page, err)
			}
		}
		set := s.r.PluginSet(src)
		on := false
		for _, ep := range set.Plugins() {
			if ep == p {
				on = true
			}
		}
		if !on && (builtin || needOn) {
			return target{}, fmt.Errorf("plugin %q is switched off", id)
		}
		ctx = set.Context(id, page)
		r, path := s.r, page
		// An action renders markdown through the page's own pipeline too.
		ctx.Markdown = func(src string) (string, error) {
			pg, err := r.Render(path, []byte(src))
			return pg.HTML, err
		}
	}
	ctx.Workspace = s.ext.root
	if d := s.dataDir(s.ext.root, id); d != "" {
		ctx.DataDir = filepath.Join(d, "files")
	}
	return target{p: p, root: s.ext.root, ctx: ctx}, nil
}

var storeLocks sync.Map // storage file -> *sync.Mutex

func lockFor(path string) *sync.Mutex {
	m, _ := storeLocks.LoadOrStore(path, &sync.Mutex{})
	return m.(*sync.Mutex)
}

var keyRE = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func validKey(k string) error {
	if len(k) < 1 || len(k) > maxKeyLen || !keyRE.MatchString(k) || strings.ContainsRune(k, '\n') {
		return errors.New("storage keys use letters, digits, . _ - and are 1 to 64 characters long")
	}
	return nil
}

func (s *state) storagePath(id string) (string, error) {
	t, err := s.target(id, "", false)
	if err != nil {
		return "", err
	}
	dir := s.dataDir(t.root, id)
	if dir == "" {
		return "", errors.New("no settings folder on this system")
	}
	return filepath.Join(dir, "storage.json"), nil
}

// readStore reads the plugin's storage file. A file that cannot be parsed is
// set aside as storage.json.corrupt-<time> (never overwritten) and the error
// says so; the next call starts from an empty store.
func readStore(path string) (map[string]json.RawMessage, error) {
	m := map[string]json.RawMessage{}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, errors.New("plugin storage could not be read")
	}
	if err := json.Unmarshal(b, &m); err != nil || m == nil {
		if to, serr := atomicfile.SetAside(path); serr == nil {
			return nil, fmt.Errorf("plugin storage was damaged and was moved to %s; it starts empty now", filepath.Base(to))
		}
		return nil, errors.New("plugin storage is damaged and could not be set aside")
	}
	return m, nil
}

// PluginStorageGet returns the JSON text stored under key by an external
// plugin, "null" when there is none.
func (z *Zen) PluginStorageGet(id, key string) (string, error) { return z.state.storageGet(id, key) }

func (s *state) storageGet(id, key string) (string, error) {
	if err := validKey(key); err != nil {
		return "", err
	}
	path, err := s.storagePath(id)
	if err != nil {
		return "", err
	}
	return getAt(path, key)
}

// getAt reads one key of the storage file at path.
func getAt(path, key string) (string, error) {
	mu := lockFor(path)
	mu.Lock()
	defer mu.Unlock()
	m, err := readStore(path)
	if err != nil {
		return "", err
	}
	if v, ok := m[key]; ok {
		return string(v), nil
	}
	return "null", nil
}

// PluginStorageSet stores a JSON value for an external plugin (1 MB per
// plugin and folder, written atomically).
func (z *Zen) PluginStorageSet(id, key, value string) error {
	return z.state.storageSet(id, key, value)
}

func (s *state) storageSet(id, key, value string) error {
	if err := validKey(key); err != nil {
		return err
	}
	if !json.Valid([]byte(value)) {
		return errors.New("the value is not valid JSON")
	}
	path, err := s.storagePath(id)
	if err != nil {
		return err
	}
	return setAt(path, key, value)
}

// setAt stores a JSON value under key in the storage file at path.
func setAt(path, key, value string) error {
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(value)); err != nil {
		return errors.New("the value is not valid JSON")
	}
	mu := lockFor(path)
	mu.Lock()
	defer mu.Unlock()
	m, err := readStore(path)
	if err != nil {
		return err
	}
	m[key] = json.RawMessage(compact.String())
	total := 0
	for k, v := range m {
		total += len(k) + len(v)
	}
	if total > storageLimit {
		return errors.New("plugin storage is limited to 1 MB")
	}
	data, err := json.Marshal(m)
	if err != nil {
		return errors.New("could not store the value")
	}
	if err := atomicfile.Write(path, data); err != nil {
		return errors.New("could not store the value")
	}
	return nil
}

// pluginStore is the storage a script plugin reaches as zen.storage: the same
// store, file and limits as ctx.storage.
type pluginStore struct {
	s  *state
	id string
}

func (p pluginStore) Get(key string) (string, error)       { return p.s.storageGet(p.id, key) }
func (p pluginStore) Set(key, value string) error          { return p.s.storageSet(p.id, key, value) }
func (p pluginStore) Keys(prefix string) ([]string, error) { return p.s.storageKeys(p.id, prefix) }

// PluginInvoke runs a plugin's action hook (a built-in ActionHook, or an
// external backend with ZEN_HOOK=action) with a JSON payload and returns the
// JSON it answers ("null" for no output).
//
// page is the workspace path of the page the block is on: its frontmatter
// decides whether the plugin is on and its options, and it is ZEN_PAGE.
func (z *Zen) PluginInvoke(id, page, payload string) (string, error) {
	if !json.Valid([]byte(payload)) {
		return "", errors.New("the payload is not valid JSON")
	}
	t, err := z.state.target(id, page, true)
	if err != nil {
		return "", err
	}
	h, ok := plugins.ActionHookOf(t.p)
	if !ok {
		return "", fmt.Errorf("plugin %q does not declare the action hook", id)
	}
	out, err := h.Action(t.ctx, []byte(payload))
	if err != nil {
		return "", fmt.Errorf("plugin %q: %v", id, err)
	}
	out = []byte(strings.TrimSpace(string(out)))
	if len(out) == 0 {
		return "null", nil
	}
	if !json.Valid(out) {
		return "", fmt.Errorf("plugin %q did not answer with JSON", id)
	}
	return string(out), nil
}

// PluginStorageKeys lists the plugin's storage keys that start with prefix
// ("" for all), sorted.
func (z *Zen) PluginStorageKeys(id, prefix string) ([]string, error) {
	return z.state.storageKeys(id, prefix)
}

func (s *state) storageKeys(id, prefix string) ([]string, error) {
	if prefix != "" {
		if err := validKey(prefix); err != nil {
			return nil, err
		}
	}
	path, err := s.storagePath(id)
	if err != nil {
		return nil, err
	}
	return keysAt(path, prefix)
}

// keysAt lists the keys of the storage file at path that start with prefix.
func keysAt(path, prefix string) ([]string, error) {
	mu := lockFor(path)
	mu.Lock()
	defer mu.Unlock()
	m, err := readStore(path)
	if err != nil {
		return nil, err
	}
	keys := []string{}
	for k := range m {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}
