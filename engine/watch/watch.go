// Package watch reports changes to workspace files, debounced into batches.
package watch

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/zenmark-org/zenmd/engine/config"
)

// Kind is the kind of change: "changed", "created" or "removed".
type Kind string

// The kinds of change.
const (
	Changed Kind = "changed"
	Created Kind = "created"
	Removed Kind = "removed"
)

// Event is one change to a workspace-relative, "/"-separated path.
type Event struct {
	Path string
	Kind Kind
}

const (
	debounce   = 100 * time.Millisecond
	maxLatency = time.Second
)

// Watcher watches a workspace root recursively.
type Watcher struct {
	root    string
	fw      *fsnotify.Watcher
	onEvent func([]Event)

	known   map[string]bool // relevant files believed to exist
	pending map[string]bool // path -> existed at start of window
	order   []string

	done     chan struct{}
	finished chan struct{}
	stopOnce sync.Once

	mu  sync.Mutex
	err error
}

// Start watches root recursively and calls onEvent with batches of events
// from a single goroutine. onEvent must not call Stop.
func Start(root string, onEvent func([]Event)) (*Watcher, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("watch: %s is not a directory", root)
	}
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		root: abs, fw: fw, onEvent: onEvent,
		known: map[string]bool{}, pending: map[string]bool{},
		done: make(chan struct{}), finished: make(chan struct{}),
	}
	if err := w.addTree(abs, false); err != nil {
		fw.Close()
		return nil, err
	}
	go w.loop()
	return w, nil
}

// Stop releases all watches; no callback runs after it returns. Safe to call twice.
func (w *Watcher) Stop() {
	w.stopOnce.Do(func() { close(w.done) })
	<-w.finished
}

// Err returns the cause if the watcher failed, else nil.
func (w *Watcher) Err() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}

func (w *Watcher) loop() {
	defer close(w.finished)
	defer w.fw.Close()
	var timer *time.Timer
	var timerC <-chan time.Time
	var windowStart time.Time
	for {
		select {
		case <-w.done:
			if timer != nil {
				timer.Stop()
			}
			return
		case ev, ok := <-w.fw.Events:
			if !ok {
				w.fail(errors.New("watch: watcher closed"))
				return
			}
			if err := w.handle(ev); err != nil {
				w.fail(err)
				return
			}
			if len(w.pending) == 0 {
				continue
			}
			now := time.Now()
			if timer == nil {
				windowStart = now
				timer = time.NewTimer(debounce)
				timerC = timer.C
			} else if now.Sub(windowStart) < maxLatency {
				timer.Stop()
				timer = time.NewTimer(debounce)
				timerC = timer.C
			}
		case err, ok := <-w.fw.Errors:
			if !ok {
				err = errors.New("watch: watcher closed")
			}
			w.fail(err)
			return
		case <-timerC:
			timer, timerC = nil, nil
			batch := w.flush()
			if len(batch) == 0 {
				continue
			}
			select {
			case <-w.done:
				return
			default:
			}
			w.onEvent(batch)
		}
	}
}

func (w *Watcher) fail(err error) {
	w.mu.Lock()
	if w.err == nil {
		w.err = err
	}
	w.mu.Unlock()
}

func (w *Watcher) flush() []Event {
	var out []Event
	for _, p := range w.order {
		was, now := w.pending[p], w.known[p]
		switch {
		case !was && now:
			out = append(out, Event{p, Created})
		case was && !now:
			out = append(out, Event{p, Removed})
		case was && now:
			out = append(out, Event{p, Changed})
		}
	}
	w.pending = map[string]bool{}
	w.order = nil
	return out
}

func (w *Watcher) touch(rel string, exists bool) {
	if _, ok := w.pending[rel]; !ok {
		w.pending[rel] = w.known[rel]
		w.order = append(w.order, rel)
	}
	if exists {
		w.known[rel] = true
	} else {
		delete(w.known, rel)
	}
}

func (w *Watcher) handle(ev fsnotify.Event) error {
	name := filepath.Clean(ev.Name)
	if name == w.root {
		if ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename) {
			return fmt.Errorf("watch: root %s removed", w.root)
		}
		return nil
	}
	rel, ok := w.rel(name)
	if !ok || ignoredDir(filepath.Dir(rel)) {
		return nil
	}
	if ev.Op == fsnotify.Chmod {
		return nil
	}
	fi, err := os.Lstat(name)
	exists := err == nil
	if exists && fi.IsDir() {
		if ignoredName(fi.Name()) {
			return nil
		}
		if ev.Has(fsnotify.Create) {
			return w.addTree(name, true)
		}
		return nil
	}
	if !exists {
		// Possibly a removed directory: drop every known file under it.
		prefix := filepath.ToSlash(rel) + "/"
		for p := range w.known {
			if strings.HasPrefix(p, prefix) {
				w.touch(p, false)
			}
		}
		_ = w.fw.Remove(name)
	}
	r := filepath.ToSlash(rel)
	if !relevant(r) {
		return nil
	}
	if !exists && !w.known[r] {
		if _, ok := w.pending[r]; !ok {
			return nil
		}
	}
	w.touch(r, exists)
	return nil
}

// addTree watches dir and its non-ignored subdirectories (symlinks are not
// followed) and records relevant files. report marks found files as created.
func (w *Watcher) addTree(dir string, report bool) error {
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == dir && !report {
				return err
			}
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		rel, ok := w.rel(p)
		if !ok {
			return nil
		}
		if d.IsDir() {
			if p != w.root && ignoredName(d.Name()) {
				return fs.SkipDir
			}
			if err := w.fw.Add(p); err != nil {
				if p == dir && !report {
					return err
				}
				if errors.Is(err, fs.ErrNotExist) {
					return fs.SkipDir
				}
				return err // e.g. watch limit reached
			}
			return nil
		}
		r := filepath.ToSlash(rel)
		if !relevant(r) {
			return nil
		}
		if report {
			if !w.known[r] {
				w.touch(r, true)
			}
		} else {
			w.known[r] = true
		}
		return nil
	})
}

func (w *Watcher) rel(p string) (string, bool) {
	rel, err := filepath.Rel(w.root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

// ignoredName mirrors workspace.Files: dot dirs, node_modules, vendor.
func ignoredName(name string) bool {
	return strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor"
}

func ignoredDir(rel string) bool {
	if rel == "." || rel == "" {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if ignoredName(part) {
			return true
		}
	}
	return false
}

func relevant(rel string) bool {
	if rel == config.WorkspaceFile || rel == config.LegacyWorkspaceFile {
		return true
	}
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".md", ".markdown", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".avif":
		return true
	}
	return false
}
