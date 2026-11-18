package extplugins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/dragosh/zenmd/internal/core/atomicfile"
)

// Hash is the SHA-256 over the sorted relative paths and contents of every
// file in the plugin folder as it was when the plugin was loaded; nothing is
// skipped. It is computed once at load: the decision, the approval and the
// pre-spawn check all use this value.
func (p *Plugin) Hash() (string, error) { return p.hash, nil }

// AssetVersion is the first 12 hex digits of the hash: the ?v= of the
// plugin's asset URLs, so a changed plugin loads under a new URL.
func (p *Plugin) AssetVersion() string {
	if len(p.hash) > 12 {
		return p.hash[:12]
	}
	return p.hash
}

// OnChange sets f to be called (in the calling goroutine, once) when the
// plugin's folder is found changed.
func (p *Plugin) OnChange(f func()) { p.guard.OnChange(f) }

// Changed reports whether anything in the plugin folder differs from what was
// loaded (a path, mode, size or modification time: no file is read). Once
// true it stays true; the first time it also calls the OnChange function.
func (p *Plugin) Changed() bool { return p.guard.Changed() }

type trustEntry struct {
	Path     string `yaml:"path"`
	Scope    string `yaml:"scope,omitempty"` // folder root for a plugin of a folder, "" for a user plugin
	SHA256   string `yaml:"sha256"`
	Decision string `yaml:"decision"` // allow | block
}

type trustFile struct {
	Version int          `yaml:"version"`
	Plugins []trustEntry `yaml:"plugins"`
}

// Trust is the user's decisions about external plugins, kept in
// trusted-plugins.yml. A decision is for one plugin folder, in one scope (the
// workspace root for a plugin inside a workspace, so a folder that links to an
// approved plugin is asked about), and the exact contents it had when the
// user decided.
type Trust struct {
	path    string
	mu      sync.Mutex
	entries []trustEntry
	notice  string
}

// LoadTrust reads the decisions in path. A missing file means no decisions. A
// file that cannot be read or parsed is set aside as <path>.corrupt-<time>
// (never overwritten) and Notice says so. With path "" decisions last for the
// session only.
func LoadTrust(path string) *Trust {
	t := &Trust{path: path}
	if path == "" {
		return t
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			t.notice = "the trusted plugins file could not be read; every plugin asks again"
		}
		return t
	}
	var f trustFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		if to, serr := atomicfile.SetAside(path); serr == nil {
			t.notice = fmt.Sprintf("the trusted plugins file could not be read and was moved to %s; every plugin asks again", filepath.Base(to))
		} else {
			t.notice = "the trusted plugins file could not be read; every plugin asks again"
		}
		return t
	}
	for _, e := range f.Plugins {
		if e.Path != "" && e.SHA256 != "" && (e.Decision == "allow" || e.Decision == "block") {
			t.entries = append(t.entries, e)
		}
	}
	return t
}

// Notice returns the problem found while loading the file, once ("" after).
func (t *Trust) Notice() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := t.notice
	t.notice = ""
	return n
}

// Decision reports the user's decision for the plugin's loaded files in scope.
func (t *Trust) Decision(p Subject, scope string) (allowed, decided bool) {
	hash, _ := p.Hash()
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, e := range t.entries {
		if e.Path == p.Dir() && e.Scope == scope && e.SHA256 == hash {
			return e.Decision == "allow", true
		}
	}
	return false, false
}

// Set records a decision for the files the plugin had when it was loaded (what
// the user was shown) and saves it. Allowing a plugin whose folder has changed
// since is refused with ErrChanged.
func (t *Trust) Set(p Subject, scope string, allow bool) error {
	if allow && p.Changed() {
		return ErrChanged
	}
	d := "block"
	if allow {
		d = "allow"
	}
	hash, err := p.Hash()
	if err != nil {
		return errors.New("could not hash the plugin")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	kept := t.entries[:0:0]
	for _, e := range t.entries {
		if e.Path != p.Dir() || e.Scope != scope {
			kept = append(kept, e)
		}
	}
	old := t.entries
	t.entries = append(kept, trustEntry{Path: p.Dir(), Scope: scope, SHA256: hash, Decision: d})
	if err := t.saveLocked(); err != nil {
		t.entries = old
		return err
	}
	return nil
}

func (t *Trust) saveLocked() error {
	if t.path == "" {
		return nil
	}
	sort.Slice(t.entries, func(i, j int) bool {
		if t.entries[i].Path != t.entries[j].Path {
			return t.entries[i].Path < t.entries[j].Path
		}
		return t.entries[i].Scope < t.entries[j].Scope
	})
	data, err := yaml.Marshal(trustFile{Version: 1, Plugins: t.entries})
	if err != nil {
		return errors.New("could not save the decision")
	}
	if err := atomicfile.Write(t.path, data); err != nil {
		return errors.New("could not save the decision (is the settings folder writable?)")
	}
	return nil
}
