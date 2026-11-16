package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/dragosh/zen/internal/core/extplugins"
	"github.com/dragosh/zen/internal/core/plugins"
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

// candidate is an external plugin that loaded and can be registered.
type candidate struct {
	p      *extplugins.Plugin
	source string // folder | user
	status string // allowed | blocked | undecided
}

// extState is everything derived from the external plugins of the open folder.
type extState struct {
	reg     *plugins.Registry
	root    string
	cands   []candidate
	invalid []PluginInfo
	warns   []string
	active  map[string]*extplugins.Plugin // allowed and registered
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

func trustPrompt(root string, cands []candidate) string {
	var b strings.Builder
	b.WriteString("This folder uses plugins. A plugin runs programs and scripts on your Mac, so zen asks before using one for the first time.\n")
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
			fmt.Fprintf(&b, "  runs: %s\n", r)
		}
	}
	b.WriteString("\nAllow them? Blocked plugins stay off; Settings › Plugins changes the answer later.")
	return b.String()
}

// loadExternal discovers the external plugins of the folder at root, asks
// the user about new ones (when prompt is set and the dialogs can show it),
// and builds the registry. Nothing external runs here.
func (s *state) loadExternal(root string, prompt bool) *extState {
	e := &extState{root: root, active: map[string]*extplugins.Plugin{}}
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
		p      *extplugins.Plugin
		source string
	}
	var all []found
	seen := map[string]bool{}
	addInvalid := func(id, source, problem string) {
		e.invalid = append(e.invalid, PluginInfo{ID: id, Source: source, Status: "invalid", Runs: []string{}, Problem: problem})
		e.warns = append(e.warns, fmt.Sprintf("plugin %q: %s", id, problem))
	}
	for _, d := range []struct{ dir, source string }{{folderDir, "folder"}, {userDir, "user"}} {
		if d.dir == "" {
			continue
		}
		ps, inv := extplugins.Scan(d.dir)
		for _, in := range inv {
			if !seen[in.ID] { // the folder's plugin shadows the user's of the same id
				seen[in.ID] = true
				addInvalid(in.ID, d.source, in.Problem)
			}
		}
		for _, p := range ps {
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
			addInvalid(id, f.source, "a built-in plugin already uses this id")
			continue
		}
		if err := scratch.Register(f.p); err != nil {
			addInvalid(id, f.source, strings.TrimPrefix(err.Error(), fmt.Sprintf("plugin %q: ", id)))
			continue
		}
		e.cands = append(e.cands, candidate{p: f.p, source: f.source})
	}

	decide := func() {
		for i := range e.cands {
			switch allowed, decided := s.trust.Decision(e.cands[i].p); {
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
				_ = s.trust.Set(c.p, allow)
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
		c.p.Bind(extplugins.Runtime{Workspace: root, DataDir: data})
		if err := e.reg.Register(c.p); err == nil {
			e.active[m.ID] = c.p
		}
	}
	return e
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
		out = append(out, PluginInfo{ID: m.ID, Version: m.Version, Description: m.Description, Source: "builtin",
			Status: onOff(enabled(m.ID, m.Default)), Runs: []string{}})
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
		out = append(out, PluginInfo{ID: m.ID, Version: m.Version, Description: m.Description, Source: c.source,
			Status: st, Runs: append([]string{}, c.p.Runs()...)})
	}
	return append(out, s.ext.invalid...)
}

// PluginTrust records the user's decision about an external plugin and
// reloads the plugins of the open folder.
func (z *Zen) PluginTrust(id string, allow bool) error { return z.state.pluginTrust(id, allow) }

func (s *state) pluginTrust(id string, allow bool) error {
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
	if err := s.trust.Set(c.p, allow); err != nil {
		return err
	}
	s.reloadExternal(false, true)
	return nil
}

// reloadExternal rebuilds the registry (asking about undecided plugins when
// prompt is set), re-renders and tells the frontend.
func (s *state) reloadExternal(prompt, always bool) {
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
		return
	}
	s.ext, s.reg = ext, ext.reg
	s.rebuildLocked()
	w := s.workspaceLocked()
	current := s.current
	s.mu.Unlock()
	if w != nil && always {
		s.events.Emit(EventWorkspaceChanged, w)
	}
	if current != "" {
		s.events.Emit(EventPageChanged, PageChanged{Path: current})
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

// target finds plugin id for the open folder. Built-in plugins must be
// switched on; external ones only when needOn (invoke and stream), storage
// stays reachable for an allowed plugin that is switched off.
func (s *state) target(id string, needOn bool) (target, error) {
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
	if s.reg != nil && s.cfg != nil {
		set, _ := s.reg.Enabled(s.cfg.ForPage(nil).Plugin, nil)
		on := false
		for _, ep := range set.Plugins() {
			if ep == p {
				on = true
			}
		}
		if !on && (builtin || needOn) {
			return target{}, fmt.Errorf("plugin %q is switched off", id)
		}
		ctx = set.Context(id, s.current)
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
	t, err := s.target(id, false)
	if err != nil {
		return "", err
	}
	dir := s.dataDir(t.root, id)
	if dir == "" {
		return "", errors.New("no settings folder on this system")
	}
	return filepath.Join(dir, "storage.json"), nil
}

func readStore(path string) map[string]json.RawMessage {
	m := map[string]json.RawMessage{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &m)
		if m == nil {
			m = map[string]json.RawMessage{}
		}
	}
	return m
}

// PluginStorageGet returns the JSON text stored under key by an external
// plugin, "null" when there is none.
func (z *Zen) PluginStorageGet(id, key string) (string, error) {
	if err := validKey(key); err != nil {
		return "", err
	}
	path, err := z.state.storagePath(id)
	if err != nil {
		return "", err
	}
	mu := lockFor(path)
	mu.Lock()
	defer mu.Unlock()
	if v, ok := readStore(path)[key]; ok {
		return string(v), nil
	}
	return "null", nil
}

// PluginStorageSet stores a JSON value for an external plugin (1 MB per
// plugin and folder, written atomically).
func (z *Zen) PluginStorageSet(id, key, value string) error {
	if err := validKey(key); err != nil {
		return err
	}
	if !json.Valid([]byte(value)) {
		return errors.New("the value is not valid JSON")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(value)); err != nil {
		return errors.New("the value is not valid JSON")
	}
	path, err := z.state.storagePath(id)
	if err != nil {
		return err
	}
	mu := lockFor(path)
	mu.Lock()
	defer mu.Unlock()
	m := readStore(path)
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
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return errors.New("could not store the value")
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".storage.*")
	if err != nil {
		return errors.New("could not store the value")
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr == nil && cerr == nil {
		werr = os.Chmod(tmp.Name(), 0o600)
	}
	if werr == nil && cerr == nil {
		werr = os.Rename(tmp.Name(), path)
	}
	if werr != nil || cerr != nil {
		os.Remove(tmp.Name())
		return errors.New("could not store the value")
	}
	return nil
}

// PluginInvoke runs a plugin's action hook (a built-in ActionHook, or an
// external backend with ZEN_HOOK=action) with a JSON payload and returns the
// JSON it answers ("null" for no output).
func (z *Zen) PluginInvoke(id, payload string) (string, error) {
	if !json.Valid([]byte(payload)) {
		return "", errors.New("the payload is not valid JSON")
	}
	t, err := z.state.target(id, true)
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
	if prefix != "" {
		if err := validKey(prefix); err != nil {
			return nil, err
		}
	}
	path, err := z.state.storagePath(id)
	if err != nil {
		return nil, err
	}
	mu := lockFor(path)
	mu.Lock()
	defer mu.Unlock()
	keys := []string{}
	for k := range readStore(path) {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}
