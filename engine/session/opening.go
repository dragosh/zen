package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/zenmark-org/zenmd/engine/config"
	"github.com/zenmark-org/zenmd/engine/index"
	"github.com/zenmark-org/zenmd/engine/layouts"
	"github.com/zenmark-org/zenmd/engine/plugins"
	"github.com/zenmark-org/zenmd/engine/render"
	"github.com/zenmark-org/zenmd/engine/watch"
	"github.com/zenmark-org/zenmd/engine/workspace"
)

// EventWorkspaceOpening reports progress while a folder opens, payload
// OpenProgress. The last event of an open has Phase "done" or "failed".
const EventWorkspaceOpening = "workspace:opening"

// OpenProgress is the payload of workspace:opening.
type OpenProgress struct {
	Name  string `json:"name"`  // the folder's name
	Phase string `json:"phase"` // "reading" | "indexing" | "done" | "failed"
	Done  int    `json:"done"`  // reading: files and folders seen; indexing: pages indexed
	Total int    `json:"total"` // indexing: pages in all; 0 when not known
}

// ErrOpenCancelled is returned when CancelOpen stopped an open.
var ErrOpenCancelled = errors.New("opening the folder was cancelled")

// OpenTimeoutError is returned when opening took longer than
// limits.openTimeout. The folder that was open before stays open.
type OpenTimeoutError struct {
	Name    string
	Timeout time.Duration
}

// Error describes the timeout and how to raise it.
func (e *OpenTimeoutError) Error() string {
	return fmt.Sprintf("Opening %s took longer than %s. Open a smaller folder, or raise limits.openTimeout (seconds) in config.yml.",
		e.Name, e.Timeout.Round(time.Second))
}

// asyncWatchFrom is the folder size (files and folders) from which the
// watcher starts after the folder is shown: watching a big tree takes seconds.
const asyncWatchFrom = 2000

// progressEvery throttles workspace:opening events.
const progressEvery = 100 * time.Millisecond

type openProgress struct {
	s     *Session
	name  string
	mu    sync.Mutex
	last  time.Time
	phase string
}

// send reports progress at most every progressEvery, and always when the
// phase changes.
func (p *openProgress) send(phase string, done, total int) {
	p.mu.Lock()
	now := time.Now()
	if phase == p.phase && now.Sub(p.last) < progressEvery {
		p.mu.Unlock()
		return
	}
	p.last, p.phase = now, phase
	p.mu.Unlock()
	p.s.emit(EventWorkspaceOpening, OpenProgress{Name: p.name, Phase: phase, Done: done, Total: total})
}

func (p *openProgress) end(phase string) {
	p.s.emit(EventWorkspaceOpening, OpenProgress{Name: p.name, Phase: phase})
}

// CancelOpen stops the open in progress, if any; the previous folder stays.
func (s *Session) CancelOpen() {
	s.openMu.Lock()
	defer s.openMu.Unlock()
	if s.openCancel != nil {
		s.openCancel()
	}
}

// SetOpenTimeout overrides limits.openTimeout for the opens that follow; 0
// goes back to the config's value. For tests and embedders.
func (s *Session) SetOpenTimeout(d time.Duration) {
	s.openMu.Lock()
	s.openTimeout = d
	s.openMu.Unlock()
}

func (s *Session) timeoutOf(l config.Limits) time.Duration {
	s.openMu.Lock()
	override := s.openTimeout
	s.openMu.Unlock()
	if override > 0 {
		return override
	}
	secs := l.OpenTimeout
	if secs <= 0 {
		secs = config.DefaultOpenTimeout
	}
	return time.Duration(secs) * time.Second
}

// Open opens dir ("" closes the folder). It reports progress as
// workspace:opening events, stops when CancelOpen is called or when reading
// or indexing takes longer than limits.openTimeout (the time spent in a trust
// prompt does not count), and then leaves the folder that was open before as
// it was. A newer open cancels an older one.
func (s *Session) Open(dir string) error {
	l := config.Load(s.globalPath, "").Limits() // global only: a folder cannot raise its own limits
	parent, cancel := context.WithCancel(context.Background())
	s.openMu.Lock()
	if s.openCancel != nil {
		s.openCancel() // the newest open wins
	}
	s.openGen++
	gen := s.openGen
	s.openCancel = cancel
	s.openMu.Unlock()
	defer func() {
		cancel()
		s.openMu.Lock()
		if s.openGen == gen {
			s.openCancel = nil
		}
		s.openMu.Unlock()
	}()

	name := filepath.Base(dir)
	p := &openProgress{s: s, name: name}
	timeout := s.timeoutOf(l)
	s.mu.Lock()
	prevName := ""
	if s.ws != nil {
		prevName = s.ws.Name()
	}
	s.mu.Unlock()
	start := time.Now()
	err := s.openIn(parent, dir, l, timeout, p)
	s.logOpened(dir, name, prevName, err, time.Since(start))
	if dir != "" {
		if err != nil {
			p.end("failed")
		} else {
			p.end("done")
		}
	}
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.DeadlineExceeded):
		return &OpenTimeoutError{Name: name, Timeout: timeout}
	case errors.Is(err, context.Canceled):
		return ErrOpenCancelled
	}
	return err
}

func (s *Session) openIn(parent context.Context, dir string, l config.Limits, timeout time.Duration, p *openProgress) error {
	var ws *workspace.Workspace
	if dir != "" {
		ctx, stop := context.WithTimeout(parent, timeout)
		var err error
		ws, err = workspace.OpenContext(ctx, dir, l.Files, l.Entries, func(n int) { p.send("reading", n, 0) })
		stop()
		if err != nil {
			return err
		}
	}
	root := ""
	if ws != nil {
		root = ws.Root()
	}
	s.reloadMu.Lock()
	ext := s.loadExternal(root, true) // may ask the user; no state lock is held, the clock is not running
	lay := s.loadLayouts(root, true)
	if err := parent.Err(); err != nil {
		s.reloadMu.Unlock()
		ext.cancel()
		return err
	}

	// Index under a fresh deadline, keeping what was open in case it stops.
	ctx, stop := context.WithTimeout(parent, timeout)
	defer stop()
	s.stopWatch()
	s.mu.Lock()
	prev := *s.snapshotLocked()
	s.ws, s.current, s.idx = ws, "", nil
	s.ext, s.reg = ext, ext.reg
	s.lay, s.lreg, s.site = lay, lay.reg, nil
	s.indexCtx, s.indexErr = ctx, nil
	s.indexStep = func(done, total int) { p.send("indexing", done, total) }
	indexStart := time.Now()
	s.rebuildLocked()
	s.indexTook = time.Since(indexStart)
	err := s.indexErr
	s.indexCtx, s.indexStep, s.indexErr = nil, nil, nil
	if err != nil {
		s.restoreLocked(prev)
		s.mu.Unlock()
		ext.cancel()
		s.reloadMu.Unlock()
		if prev.ws != nil && !s.noWatch {
			s.startWatch(prev.ws.Root(), true)
		}
		return err
	}
	s.mu.Unlock()
	s.streams.cancelAll() // streams belong to the folder they started in
	if prev.ext != nil {
		prev.ext.cancel() // calls of the previous folder's plugins end
	}
	s.reloadMu.Unlock()
	s.emit(EventPluginsChanged, nil)
	s.emit(EventLayoutsChanged, nil)
	if ws != nil && !s.noWatch {
		// Watching a big tree takes seconds: show the folder first.
		s.startWatch(ws.Root(), ws.Entries() >= asyncWatchFrom)
	}
	return nil
}

// openSnapshot is what open replaces, kept to put back when an open stops.
type openSnapshot struct {
	ws      *workspace.Workspace
	current string
	idx     *index.Index
	ext     *extState
	reg     *plugins.Registry
	lay     *layoutState
	lreg    *layouts.Registry
	site    *layoutSite
	cfg     *config.Config
	r       *render.Renderer
	tree    []NavNode
	slugs   map[string]string
	navWarn map[string][]string
}

func (s *Session) snapshotLocked() *openSnapshot {
	return &openSnapshot{ws: s.ws, current: s.current, idx: s.idx, ext: s.ext, reg: s.reg, lay: s.lay, lreg: s.lreg, site: s.site,
		cfg: s.cfg, r: s.r, tree: s.tree, slugs: s.slugs, navWarn: s.navWarn}
}

func (s *Session) restoreLocked(p openSnapshot) {
	s.ws, s.current, s.idx = p.ws, p.current, p.idx
	s.ext, s.lay, s.site = p.ext, p.lay, p.site
	s.cfg, s.r, s.tree, s.slugs, s.navWarn = p.cfg, p.r, p.tree, p.slugs, p.navWarn
	s.reg, s.lreg = p.reg, p.lreg
}

// startWatch starts watching root, in the background when async; a stop or a
// newer start before it is ready drops it.
func (s *Session) startWatch(root string, async bool) {
	s.watchMu.Lock()
	s.watchGen++
	gen := s.watchGen
	s.watchMu.Unlock()
	start := func() {
		w, err := watch.Start(root, s.handle)
		if err != nil {
			// The workspace stays usable without live reload.
			logApp(slog.LevelWarn, "watcher failed to start; changes on disk will not show", "error", scrubRoot(root, err.Error()))
			return
		}
		s.watchMu.Lock()
		defer s.watchMu.Unlock()
		if gen != s.watchGen {
			w.Stop()
			return
		}
		s.w = w
	}
	if async {
		go start()
		return
	}
	start()
}

// OpenError is the message for an open that failed, without absolute paths.
func (s *Session) OpenError(err error) error {
	var te *OpenTimeoutError
	if errors.Is(err, ErrOpenCancelled) || errors.As(err, &te) {
		return err // no paths in these
	}
	return s.cleanErr(".", err)
}

// logOpened logs how an open went: what was opened, closed or switched to, with
// the page count and the time it took; a slow index is a warning.
func (s *Session) logOpened(dir, name, prevName string, err error, took time.Duration) {
	switch {
	case errors.Is(err, context.Canceled):
		logApp(slog.LevelInfo, "folder open cancelled", "name", name)
		return
	case errors.Is(err, context.DeadlineExceeded):
		logApp(slog.LevelWarn, "folder open timed out", "name", name, "ms", took.Milliseconds())
		return
	case err != nil:
		return // the caller reports it to the user, and the service logs it
	}
	level := slog.LevelInfo
	if s.noWatch {
		level = slog.LevelDebug // the build's own session, inside an export: not something the user did
	}
	if dir == "" {
		logApp(level, "folder closed", "name", prevName)
		return
	}
	s.mu.Lock()
	pages, indexTook := s.pageCount, s.indexTook
	s.mu.Unlock()
	if prevName != "" {
		logApp(level, "folder switched", "name", name, "from", prevName, "pages", pages, "ms", took.Milliseconds())
	} else {
		logApp(level, "folder opened", "name", name, "pages", pages, "ms", took.Milliseconds())
	}
	if indexTook > slowIndex {
		logApp(slog.LevelWarn, "slow index", "name", name, "pages", pages, "ms", indexTook.Milliseconds())
	}
}
