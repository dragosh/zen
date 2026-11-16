// Package scriptplugins loads script plugins: one-file plugins written in
// Tengo that zen runs itself, inside the app, under a hard time and
// allocation budget and with no access to files, programs or the network
// unless the plugin asks for the "read" permission (and then only to the
// markdown and text files of the open folder). See docs/PLUGINS.md, "Script
// plugins (Tengo)".
//
// A script plugin is `<id>.tengo`, or `<id>/plugin.tengo` with files beside
// it. The script defines a top-level `plugin` map (read by a run with no
// hook) and answers hooks through the globals hook, name, body, attrs,
// config, page and payload and the `zen` module; it assigns `out`.
//
// Discovery, hashing, the fingerprint re-check, trust and plugins:changed are
// the ones of folder plugins: the hashing is core/extplugins', used as it is.
package scriptplugins

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/d5/tengo/v2"

	"github.com/dragosh/zen/internal/core/extplugins"
	"github.com/dragosh/zen/internal/core/plugins"
)

// Storage is the plugin's persistent store (the one ctx.storage reaches from
// the page): JSON text by key.
type Storage interface {
	Get(key string) (string, error) // "null" when there is none
	Set(key, valueJSON string) error
	Keys(prefix string) ([]string, error)
}

// Runtime is what a plugin learns about the folder it runs for. Bind it
// before any hook runs.
type Runtime struct {
	Workspace string // folder root (what zen.read and zen.pages reach)
	// Ctx ends every call of the plugin when cancelled (the folder changed,
	// the app quits); nil means never.
	Ctx     context.Context
	Storage Storage // zen.storage; nil makes it fail
}

// Plugin is a loaded script plugin. It implements plugins.Plugin, the hook
// interfaces it declares (HasHook says which) and plugins.AssetProvider.
type Plugin struct {
	path   string // the .tengo file, or the plugin folder
	real   string // real path of the plugin folder, "" for a single file
	man    plugins.Manifest
	spec   spec
	script *compiled
	hash   string
	guard  *extplugins.Guard
	rt     atomic.Pointer[boundRuntime]

	titles     sync.Map // page path -> titleEntry
	titleCount atomic.Int64
}

type boundRuntime struct {
	Runtime
	root string // resolved Workspace, "" for none
}

// Bind sets the folder and store the plugin runs for.
func (p *Plugin) Bind(rt Runtime) {
	b := &boundRuntime{Runtime: rt}
	if rt.Workspace != "" {
		if real, err := filepath.EvalSymlinks(rt.Workspace); err == nil {
			b.root = real
		}
	}
	p.rt.Store(b)
}

func (p *Plugin) runtime() *boundRuntime {
	if b := p.rt.Load(); b != nil {
		return b
	}
	return &boundRuntime{}
}

// Manifest implements plugins.Plugin.
func (p *Plugin) Manifest() plugins.Manifest { return p.man }

// Kind is "script".
func (p *Plugin) Kind() string { return "script" }

// Dir is the plugin's path: the .tengo file or the plugin folder.
func (p *Plugin) Dir() string { return p.path }

// Hash is the SHA-256 taken when the plugin was loaded (what the user
// approves): of the file, or of every file of the plugin folder.
func (p *Plugin) Hash() (string, error) { return p.hash, nil }

// AssetVersion is the first 12 hex digits of the hash: the ?v= of the
// plugin's asset URLs, so a changed plugin loads under a new URL.
func (p *Plugin) AssetVersion() string {
	if len(p.hash) > 12 {
		return p.hash[:12]
	}
	return p.hash
}

// Changed reports whether the plugin's file or folder differs from what was
// loaded (no file is read). Once true it stays true.
func (p *Plugin) Changed() bool { return p.guard.Changed() }

// OnChange sets f to be called (once, in the calling goroutine) when the
// plugin is found changed.
func (p *Plugin) OnChange(f func()) { p.guard.OnChange(f) }

// HasHook reports whether the plugin declares the hook.
func (p *Plugin) HasHook(name string) bool { return p.spec.hooks[name] }

// Runs describes what the plugin can do, one line each, for the trust prompt.
func (p *Plugin) Runs() []string {
	line := "renders blocks inside zen; cannot read files, run programs or use the network"
	if p.spec.read {
		line = "renders blocks inside zen and reads files in this folder; cannot run programs or use the network"
	}
	out := []string{line}
	if len(p.spec.js) > 0 {
		out = append(out, "scripts in the page")
	}
	return out
}

// Invalid describes a plugin that was rejected.
type Invalid = extplugins.Invalid

// Load reads and validates the script plugin at path: a `.tengo` file or a
// folder with a plugin.tengo. The id is the file or folder name. The error
// never contains an absolute path.
func Load(path string) (*Plugin, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, errors.New("plugin not found")
	}
	base := filepath.Base(path)
	p := &Plugin{}
	var src []byte
	var id string
	if fi.IsDir() || (fi.Mode()&os.ModeSymlink != 0 && isDir(path)) {
		id = base
		if !plugins.ValidName(id) {
			return nil, fmt.Errorf("invalid id %q (want [a-z][a-z0-9-]*)", id)
		}
		real, hash, fp, err := extplugins.ScanFolder(path)
		if err != nil {
			return nil, err
		}
		if _, err := os.Lstat(filepath.Join(real, "plugin.tengo")); err != nil {
			return nil, errors.New("plugin.tengo not found")
		}
		full, err := extplugins.ResolveInside(real, "plugin.tengo", "plugin.tengo")
		if err != nil {
			return nil, errors.New("plugin.tengo points outside the plugin folder")
		}
		if src, err = extplugins.ReadRegular(full, maxScript); err != nil {
			return nil, scriptReadErr(err)
		}
		p.path, p.real, p.hash = real, real, hash
		p.guard = extplugins.NewGuard(fp, func() (string, error) { return extplugins.FolderFingerprint(real) })
	} else {
		if !strings.HasSuffix(base, ".tengo") {
			return nil, errors.New("a script plugin is a .tengo file or a folder with plugin.tengo")
		}
		id = strings.TrimSuffix(base, ".tengo")
		if !plugins.ValidName(id) {
			return nil, fmt.Errorf("invalid id %q (want [a-z][a-z0-9-]*)", id)
		}
		var hash, fp string
		if src, hash, fp, err = extplugins.HashFile(path, maxScript); err != nil {
			return nil, scriptReadErr(err)
		}
		dir, err := filepath.EvalSymlinks(filepath.Dir(path))
		if err != nil {
			return nil, errors.New("plugin folder not readable")
		}
		file := filepath.Join(dir, base)
		p.path, p.hash = file, hash
		p.guard = extplugins.NewGuard(fp, func() (string, error) { return extplugins.FileFingerprint(file) })
	}
	if err := p.setup(id, src); err != nil {
		return nil, err
	}
	// What was hashed is what was read: a change in between is refused.
	if p.Changed() {
		return nil, errors.New("the plugin changed while it was loaded")
	}
	return p, nil
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func scriptReadErr(err error) error {
	if err.Error() == "too large" {
		return errors.New("the script is larger than 256 KB")
	}
	return err
}

// setup compiles the script, runs it once with no hook to read the plugin map
// and validates it.
func (p *Plugin) setup(id string, src []byte) error {
	c, err := compile(src)
	if err != nil {
		return err
	}
	p.script = c
	run, err := c.exec(nil, defaultTimeout, input{hook: "", config: map[string]any{}}, &zenCall{p: p})
	if err != nil {
		return err
	}
	obj := run.Get("plugin").Object()
	if err := p.parse(id, obj); err != nil {
		return err
	}
	return nil
}

// Scan loads every `<id>.tengo` and `<id>/plugin.tengo` in the given plugin
// folders, in order; the first folder wins for an id. A folder with a
// plugin.yml is a program or page plugin, not a script plugin, and is left
// alone. Valid plugins and rejected ones are each sorted by id.
func Scan(dirs ...string) ([]*Plugin, []Invalid) {
	var found []*Plugin
	var invalid []Invalid
	seen := map[string]bool{}
	for _, base := range dirs {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		type cand struct{ id, path string }
		var cands []cand
		for _, e := range entries { // sorted by name
			name := e.Name()
			path := filepath.Join(base, name)
			if strings.HasSuffix(name, ".tengo") && !isDir(path) {
				cands = append(cands, cand{strings.TrimSuffix(name, ".tengo"), path})
				continue
			}
			if !isDir(path) {
				continue
			}
			if _, err := os.Lstat(filepath.Join(path, "plugin.yml")); err == nil {
				continue
			}
			if _, err := os.Lstat(filepath.Join(path, "plugin.tengo")); err == nil {
				cands = append(cands, cand{name, path})
			}
		}
		count := map[string]int{}
		for _, c := range cands {
			count[c.id]++
		}
		done := map[string]bool{}
		for _, c := range cands {
			if seen[c.id] || done[c.id] {
				continue
			}
			if count[c.id] > 1 {
				done[c.id] = true
				seen[c.id] = true
				invalid = append(invalid, Invalid{ID: c.id, Dir: base, Problem: fmt.Sprintf("both %s.tengo and a folder %s/ define this plugin; keep one", c.id, c.id)})
				continue
			}
			seen[c.id] = true
			p, err := Load(c.path)
			if err != nil {
				invalid = append(invalid, Invalid{ID: c.id, Dir: c.path, Problem: err.Error()})
				continue
			}
			found = append(found, p)
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].man.ID < found[j].man.ID })
	sort.Slice(invalid, func(i, j int) bool { return invalid[i].ID < invalid[j].ID })
	return found, invalid
}

// ---- hooks ---------------------------------------------------------------

func (p *Plugin) declared(hook string) error {
	if !p.HasHook(hook) {
		return fmt.Errorf("the plugin does not declare the %s hook", hook)
	}
	return nil
}

// call runs the script for a hook and returns the value of out.
func (p *Plugin) call(hook string, ctx plugins.Context, in input) (tengo.Object, error) {
	if err := p.declared(hook); err != nil {
		return nil, err
	}
	if p.Changed() {
		return nil, extplugins.ErrChanged
	}
	rt := p.runtime()
	in.hook = hook
	in.config = ctx.Config
	in.page.path = ctx.Path
	in.page.title = p.titleOf(rt.root, ctx.Path)
	z := &zenCall{p: p, ctx: ctx, rt: rt.Runtime, root: rt.root, read: p.spec.read}
	run, err := p.script.exec(rt.Ctx, p.spec.timeout, in, z)
	if err != nil {
		return nil, err
	}
	return run.Get("out").Object(), nil
}

// text returns the string a text hook answered, and whether it set out.
func (p *Plugin) text(hook string, ctx plugins.Context, in input) (string, bool, error) {
	out, err := p.call(hook, ctx, in)
	if err != nil {
		return "", false, err
	}
	return outString(out)
}

// RenderBlock implements plugins.BlockHook.
func (p *Plugin) RenderBlock(ctx plugins.Context, b plugins.Block) (string, error) {
	s, _, err := p.text("block", ctx, input{name: b.Name, body: b.Body, attrs: b.Attrs})
	return s, err
}

// RenderInline implements plugins.InlineHook.
func (p *Plugin) RenderInline(ctx plugins.Context, i plugins.Inline) (string, error) {
	s, _, err := p.text("inline", ctx, input{name: i.Name, body: i.Text, attrs: i.Attrs})
	return s, err
}

// Source implements plugins.SourceHook: out replaces the markdown; a script
// that sets no out leaves it as it is.
func (p *Plugin) Source(ctx plugins.Context, src []byte) ([]byte, error) {
	s, set, err := p.text("source", ctx, input{body: string(src)})
	if err != nil || !set {
		return src, err
	}
	return []byte(s), nil
}

// HTML implements plugins.HTMLHook: out replaces the HTML; a script that sets
// no out leaves it as it is.
func (p *Plugin) HTML(ctx plugins.Context, h string) (string, error) {
	s, set, err := p.text("html", ctx, input{body: h})
	if err != nil || !set {
		return h, err
	}
	return s, nil
}

// Action implements plugins.ActionHook: payload is the decoded JSON and out
// any value, answered as JSON (nothing for no out).
func (p *Plugin) Action(ctx plugins.Context, payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		payload = []byte("null")
	}
	out, err := p.call("action", ctx, input{payload: payload})
	if err != nil {
		return nil, err
	}
	return outJSON(out)
}
