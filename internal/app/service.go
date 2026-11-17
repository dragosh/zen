package app

import (
	"errors"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dragosh/zen/internal/cli"
	"github.com/dragosh/zen/internal/core/extplugins"
	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/core/scriptplugins"
)

// Dialogs are the native dialogs the service needs.
type Dialogs interface {
	// PickFolder returns the chosen directory, "" on cancel.
	PickFolder() (string, error)
	// Confirm asks a yes/no question.
	Confirm(message string) bool
}

type zenOptions struct {
	Registry   *plugins.Registry
	GlobalPath string // global config file, "" to skip
	ConfigDir  string // settings folder (user plugins, trust, plugin data); default: the folder of GlobalPath
	Dir        string
	Page       string
	Mode       string
	Events     EventSink
	Dialogs    Dialogs
	Open       func(url string) error
	// DeferOpen leaves Dir to the first Startup call (or OpenInitial), so the
	// window can show progress while a big folder opens.
	DeferOpen bool
}

// Zen is the service bound to the frontend (docs/PLAN.md "Service API").
type Zen struct {
	state   *state
	page    string
	mode    string
	dialogs Dialogs
	open    func(string) error
	windows *windowManager // nil until the GUI runs (ui/editor-window)

	initialDir  string // opened by openInitial when DeferOpen
	initialOnce sync.Once
	initialMu   sync.Mutex
	initialErr  error // why the folder given at launch did not open (guarded by initialMu)
}

func (z *Zen) launchErr() error {
	z.initialMu.Lock()
	defer z.initialMu.Unlock()
	return z.initialErr
}

func (z *Zen) setLaunchErr(err error) {
	z.initialMu.Lock()
	z.initialErr = err
	z.initialMu.Unlock()
}

func newZen(o zenOptions) (*Zen, error) {
	configDir := o.ConfigDir
	if configDir == "" && o.GlobalPath != "" {
		configDir = filepath.Dir(o.GlobalPath)
	}
	st := newState(o.Registry, o.GlobalPath, o.Events)
	st.configDir, st.dialogs = configDir, o.Dialogs
	if configDir != "" {
		st.trust = extplugins.LoadTrust(filepath.Join(configDir, "trusted-plugins.yml"))
	}
	z := &Zen{
		state:   st,
		page:    o.Page,
		mode:    o.Mode,
		dialogs: o.Dialogs,
		open:    o.Open,
	}
	if z.mode != "edit" {
		z.mode = "read"
	}
	if o.DeferOpen {
		z.initialDir = o.Dir
		return z, nil
	}
	if err := z.state.open(o.Dir); err != nil {
		return nil, err
	}
	return z, nil
}

func (z *Zen) close() {
	z.state.stopWatch()
	z.state.stop()        // ends every plugin call
	scriptplugins.Reset() // the script helpers go with them
	z.state.streams.cancelAll()
	z.state.streams.wait(3 * time.Second)
}

// Startup returns the workspace, the page to show first and the mode.
func (z *Zen) Startup() Startup {
	z.openInitial()
	if err := z.launchErr(); err != nil {
		return Startup{Mode: z.mode, Error: err.Error()}
	}
	ws := z.state.workspace()
	page := z.page
	if page == "" || ws == nil {
		page = z.state.home()
	}
	return Startup{Workspace: ws, Page: page, Mode: z.mode}
}

// openInitial opens the folder given at launch, once, when DeferOpen; every
// caller waits for it.
func (z *Zen) openInitial() {
	z.initialOnce.Do(func() {
		if z.initialDir == "" {
			return
		}
		// Cancelled means a newer open (Open Folder, Open With) replaced it.
		if err := z.state.open(z.initialDir); err != nil && !errors.Is(err, ErrOpenCancelled) {
			z.setLaunchErr(openErr(z.state, err))
		}
	})
}

// openErr is the message for an open that failed, without absolute paths.
func openErr(s *state, err error) error {
	var te *OpenTimeoutError
	if errors.Is(err, ErrOpenCancelled) || errors.As(err, &te) {
		return err // no paths in these
	}
	return s.cleanErr(".", err)
}

// CancelOpen stops the folder that is opening; the one open before stays.
func (z *Zen) CancelOpen() { z.state.cancelOpen() }

// PickFolder shows the folder dialog and opens the chosen folder; null on
// cancel, also when the opening itself was cancelled (CancelOpen).
func (z *Zen) PickFolder() (*Workspace, error) {
	dir, err := z.dialogs.PickFolder()
	if err != nil || dir == "" {
		return nil, err
	}
	if err := z.state.open(dir); err != nil {
		if errors.Is(err, ErrOpenCancelled) {
			return nil, nil
		}
		return nil, openErr(z.state, err)
	}
	z.setLaunchErr(nil) // a folder is open now
	z.page = ""
	return z.state.workspace(), nil
}

// EventWorkspaceOpened tells the frontend to show a different workspace,
// payload Startup. Sent after Finder "Open With" and File > Open Folder.
const EventWorkspaceOpened = "workspace:opened"

// openPath opens a folder or markdown file handed over by macOS (Open With,
// `open -a Zen <path>`), using the same rules as the zen command line, and
// returns what the frontend should show. Not bound to the frontend.
func (z *Zen) openPath(path string) (Startup, error) {
	var l cli.Launch
	code := cli.Run([]string{path}, "/", io.Discard, io.Discard, func(got cli.Launch) error {
		l = got
		return nil
	}, nil)
	if code != 0 {
		return Startup{}, errors.New("not a folder or markdown file")
	}
	if err := z.state.open(l.Dir); err != nil {
		return Startup{}, openErr(z.state, err)
	}
	z.setLaunchErr(nil)
	z.page = l.Page
	return z.Startup(), nil
}

// Page renders a workspace page.
func (z *Zen) Page(path string) (Page, error) { return z.state.page(path) }

// Source returns the markdown of a page.
func (z *Zen) Source(path string) (string, error) {
	if which, ok := configWhich(path); ok {
		return z.configSource(which)
	}
	return z.state.source(path)
}

// Preview renders unsaved text as if it were path.
func (z *Zen) Preview(path, content string) (Page, error) { return z.state.preview(path, content) }

// Save writes a page.
func (z *Zen) Save(path, content string) error {
	if which, ok := configWhich(path); ok {
		return z.saveConfig(which, content)
	}
	return z.state.save(path, content)
}

// OpenExternal asks, then opens an http, https or mailto URL in the browser.
func (z *Zen) OpenExternal(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		if u.Host == "" {
			return false
		}
	case "mailto":
	default:
		return false
	}
	if !z.dialogs.Confirm("Open " + rawURL + " in your browser?") {
		return false
	}
	return z.open(rawURL) == nil
}

// Lint returns Vale's alerts for a page: content, or the file on disk when
// content is "". Unavailable without vale or with `lint: false` (ui/lint).
func (z *Zen) Lint(path, content string) (LintResult, error) {
	if _, ok := configWhich(path); ok {
		return lintOff(), nil // settings files are not prose
	}
	return z.state.lint(currentLinter(), path, content)
}
