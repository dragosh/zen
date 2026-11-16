package app

import (
	"errors"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/dragosh/zen/internal/cli"
	"github.com/dragosh/zen/internal/core/extplugins"
	"github.com/dragosh/zen/internal/core/plugins"
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
}

// Zen is the service bound to the frontend (docs/PLAN.md "Service API").
type Zen struct {
	state   *state
	page    string
	mode    string
	dialogs Dialogs
	open    func(string) error
	windows *windowManager // nil until the GUI runs (ui/editor-window)
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
	if err := z.state.open(o.Dir); err != nil {
		return nil, err
	}
	return z, nil
}

func (z *Zen) close() {
	z.state.stopWatch()
	z.state.streams.cancelAll()
	z.state.streams.wait(3 * time.Second)
}

// Startup returns the workspace, the page to show first and the mode.
func (z *Zen) Startup() Startup {
	ws := z.state.workspace()
	page := z.page
	if page == "" || ws == nil {
		page = z.state.home()
	}
	return Startup{Workspace: ws, Page: page, Mode: z.mode}
}

// PickFolder shows the folder dialog and opens the chosen folder; null on cancel.
func (z *Zen) PickFolder() (*Workspace, error) {
	dir, err := z.dialogs.PickFolder()
	if err != nil || dir == "" {
		return nil, err
	}
	if err := z.state.open(dir); err != nil {
		return nil, z.state.cleanErr(".", err)
	}
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
	})
	if code != 0 {
		return Startup{}, errors.New("not a folder or markdown file")
	}
	if err := z.state.open(l.Dir); err != nil {
		return Startup{}, z.state.cleanErr(".", err)
	}
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
