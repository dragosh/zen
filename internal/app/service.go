package app

import (
	"net/url"
	"strings"

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
}

func newZen(o zenOptions) (*Zen, error) {
	z := &Zen{
		state:   newState(o.Registry, o.GlobalPath, o.Events),
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

func (z *Zen) close() { z.state.stopWatch() }

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

// Page renders a workspace page.
func (z *Zen) Page(path string) (Page, error) { return z.state.page(path) }

// Source returns the markdown of a page.
func (z *Zen) Source(path string) (string, error) { return z.state.source(path) }

// Preview renders unsaved text as if it were path.
func (z *Zen) Preview(path, content string) (Page, error) { return z.state.preview(path, content) }

// Save writes a page.
func (z *Zen) Save(path, content string) error { return z.state.save(path, content) }

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
