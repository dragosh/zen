package extplugins

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"

	"gopkg.in/yaml.v3"
)

// Hash returns the SHA-256 over the sorted relative paths and contents of
// every file in the plugin folder. Any change to any file changes it.
func (p *Plugin) Hash() (string, error) {
	h := sha256.New()
	put := func(s string) {
		h.Write([]byte(strconv.Itoa(len(s))))
		h.Write([]byte{':'})
		h.Write([]byte(s))
	}
	err := filepath.WalkDir(p.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == p.dir || d.Name() == ".DS_Store" {
			return nil
		}
		rel, _ := filepath.Rel(p.dir, path)
		put(filepath.ToSlash(rel))
		switch {
		case d.IsDir():
			put("d")
		case d.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			put("l")
			put(target)
		default:
			fi, err := d.Info()
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if fi.Mode().Perm()&0o111 != 0 {
				put("x")
			} else {
				put("f")
			}
			put(string(data))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type trustEntry struct {
	Path     string `yaml:"path"`
	SHA256   string `yaml:"sha256"`
	Decision string `yaml:"decision"` // allow | block
}

type trustFile struct {
	Version int          `yaml:"version"`
	Plugins []trustEntry `yaml:"plugins"`
}

// Trust is the user's decisions about external plugins, kept in
// trusted-plugins.yml. A decision is for one plugin folder and the exact
// contents it had when the user decided.
type Trust struct {
	path    string
	mu      sync.Mutex
	entries []trustEntry
}

// LoadTrust reads the decisions in path. A missing or unreadable file means
// no decisions. With path "" decisions last for the session only.
func LoadTrust(path string) *Trust {
	t := &Trust{path: path}
	if path == "" {
		return t
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return t
	}
	var f trustFile
	if yaml.Unmarshal(data, &f) != nil {
		return t
	}
	for _, e := range f.Plugins {
		if e.Path != "" && e.SHA256 != "" && (e.Decision == "allow" || e.Decision == "block") {
			t.entries = append(t.entries, e)
		}
	}
	return t
}

// Decision reports the user's decision for the plugin's current files.
func (t *Trust) Decision(p *Plugin) (allowed, decided bool) {
	hash, err := p.Hash()
	if err != nil {
		return false, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, e := range t.entries {
		if e.Path == p.dir && e.SHA256 == hash {
			return e.Decision == "allow", true
		}
	}
	return false, false
}

// Set records a decision for the plugin's current files and saves it.
func (t *Trust) Set(p *Plugin, allow bool) error {
	hash, err := p.Hash()
	if err != nil {
		return errors.New("could not read the plugin's files")
	}
	d := "block"
	if allow {
		d = "allow"
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	kept := t.entries[:0:0]
	for _, e := range t.entries {
		if e.Path != p.dir {
			kept = append(kept, e)
		}
	}
	t.entries = append(kept, trustEntry{Path: p.dir, SHA256: hash, Decision: d})
	return t.saveLocked()
}

func (t *Trust) saveLocked() error {
	if t.path == "" {
		return nil
	}
	sort.Slice(t.entries, func(i, j int) bool { return t.entries[i].Path < t.entries[j].Path })
	data, err := yaml.Marshal(trustFile{Version: 1, Plugins: t.entries})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(t.path), 0o700); err != nil {
		return errors.New("could not save the decision")
	}
	tmp, err := os.CreateTemp(filepath.Dir(t.path), ".trusted-plugins.*")
	if err != nil {
		return errors.New("could not save the decision")
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr == nil && cerr == nil {
		werr = os.Chmod(tmp.Name(), 0o600)
	}
	if werr == nil && cerr == nil {
		werr = os.Rename(tmp.Name(), t.path)
	}
	if werr != nil || cerr != nil {
		os.Remove(tmp.Name())
		return errors.New("could not save the decision")
	}
	return nil
}
