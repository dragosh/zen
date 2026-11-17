package app

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/dragosh/zen/engine/session"
	"github.com/dragosh/zen/internal/cli"
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
	// DeferOpen leaves Dir to the first Startup call (or OpenInitial), so the
	// window can show progress while a big folder opens.
	DeferOpen bool
}

// Zen is the service bound to the frontend (docs/PLAN.md "Service API"). It is
// a thin layer over the headless session of engine/session: bindings,
// dialogs, windows and the launch state.
type Zen struct {
	s       *session.Session
	events  EventSink // the same sink as the session's, for the events of the app itself
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
	so := session.Options{
		GlobalPath: o.GlobalPath,
		ConfigDir:  o.ConfigDir,
		Registry:   o.Registry,
		Events:     o.Events,
		Runtime:    staticRuntime{},
		Presets:    siteBuiltins,
		Watch:      true,
		Version:    cli.Version,
	}
	if o.Dialogs != nil {
		so.Prompter = o.Dialogs
	}
	z := &Zen{
		s:       session.New(so),
		events:  o.Events,
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
	if err := z.s.Open(o.Dir); err != nil {
		z.s.Close()
		return nil, err
	}
	return z, nil
}

func (z *Zen) close() { z.s.Close() }

// middleware serves the session's routes (the open folder's files, plugin
// and layout assets, the print document) and sends everything else to next
// with the frontend's content security policy.
func (z *Zen) middleware(next http.Handler) http.Handler {
	h := z.s.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, prefix := range session.Routes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				h.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Security-Policy", frontendCSP)
		next.ServeHTTP(w, r)
	})
}

// frontendCSP is sent with everything that is not a workspace or plugin file.
const frontendCSP = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' https:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'"

// Startup returns the workspace, the page to show first and the mode.
func (z *Zen) Startup() Startup {
	z.openInitial()
	if err := z.launchErr(); err != nil {
		return Startup{Mode: z.mode, Error: err.Error()}
	}
	ws := z.s.Workspace()
	page := z.page
	if page == "" || ws == nil {
		page = z.s.Home()
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
		if err := z.s.Open(z.initialDir); err != nil && !errors.Is(err, ErrOpenCancelled) {
			z.setLaunchErr(z.s.OpenError(err))
		}
	})
}

// PickFolder shows the folder dialog and opens the chosen folder; null on
// cancel, also when the opening itself was cancelled (CancelOpen).
func (z *Zen) PickFolder() (*Workspace, error) {
	dir, err := z.dialogs.PickFolder()
	if err != nil || dir == "" {
		return nil, err
	}
	if err := z.s.Open(dir); err != nil {
		if errors.Is(err, ErrOpenCancelled) {
			return nil, nil
		}
		return nil, z.s.OpenError(err)
	}
	z.setLaunchErr(nil) // a folder is open now
	z.page = ""
	return z.s.Workspace(), nil
}

// openPath opens a folder or markdown file handed over by macOS (Open With,
// `open -a Zen <path>`), using the same rules as the zen command line, and
// returns what the frontend should show. Not bound to the frontend.
func (z *Zen) openPath(path string) (Startup, error) {
	var l cli.Launch
	code := cli.Run([]string{path}, "/", io.Discard, io.Discard, func(got cli.Launch) error {
		l = got
		return nil
	}, nil, nil)
	if code != 0 {
		return Startup{}, errors.New("not a folder or markdown file")
	}
	if err := z.s.Open(l.Dir); err != nil {
		return Startup{}, z.s.OpenError(err)
	}
	z.setLaunchErr(nil)
	z.page = l.Page
	return z.Startup(), nil
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

// What follows is one line each: the service's methods that the session
// implements. Names and payloads are the frontend's contract.

// CancelOpen stops the folder that is opening; the one open before stays.
func (z *Zen) CancelOpen() { z.s.CancelOpen() }

// Page renders a workspace page.
func (z *Zen) Page(path string) (Page, error) { return z.s.Page(path) }

// Source returns the markdown of a page, or of a settings file.
func (z *Zen) Source(path string) (string, error) { return z.s.Source(path) }

// Preview renders unsaved text as if it were path.
func (z *Zen) Preview(path, content string) (Page, error) { return z.s.Preview(path, content) }

// Save writes a page, or a settings file.
func (z *Zen) Save(path, content string) error { return z.s.Save(path, content) }

// Lint returns Vale's alerts for a page: content, or the file on disk when
// content is "". Unavailable without vale or with `lint: false`.
func (z *Zen) Lint(path, content string) (LintResult, error) { return z.s.Lint(path, content) }

// CreatePage creates a markdown page from the template of layout and returns
// its workspace path; an existing file is never overwritten.
func (z *Zen) CreatePage(name, layout string) (string, error) { return z.s.CreatePage(name, layout) }

// Plugins lists the built-in and external plugins for Settings › Plugins.
func (z *Zen) Plugins() []PluginInfo { return z.s.Plugins() }

// PluginTrust records the user's decision about an external plugin; allowing
// asks first.
func (z *Zen) PluginTrust(id string, allow bool) error { return z.s.PluginTrust(id, allow) }

// PluginStorageGet returns the JSON text stored under key by an external
// plugin, "null" when there is none.
func (z *Zen) PluginStorageGet(id, key string) (string, error) { return z.s.PluginStorageGet(id, key) }

// PluginStorageSet stores a JSON value for an external plugin.
func (z *Zen) PluginStorageSet(id, key, value string) error {
	return z.s.PluginStorageSet(id, key, value)
}

// PluginStorageKeys lists the plugin's storage keys that start with prefix.
func (z *Zen) PluginStorageKeys(id, prefix string) ([]string, error) {
	return z.s.PluginStorageKeys(id, prefix)
}

// PluginInvoke runs a plugin's action hook with a JSON payload.
func (z *Zen) PluginInvoke(id, page, payload string) (string, error) {
	return z.s.PluginInvoke(id, page, payload)
}

// PluginStream starts a plugin's stream hook and returns the id of the call.
func (z *Zen) PluginStream(id, page, payload string) (string, error) {
	return z.s.PluginStream(id, page, payload)
}

// PluginCancel cancels a running stream; a finished or unknown call is not an
// error.
func (z *Zen) PluginCancel(call string) error { return z.s.PluginCancel(call) }

// Layouts lists the built-in, folder and user layouts for Settings › Layouts.
func (z *Zen) Layouts() []LayoutInfo { return z.s.Layouts() }

// LayoutTrust records the user's decision about a folder or user layout;
// allowing asks first.
func (z *Zen) LayoutTrust(id string, allow bool) error { return z.s.LayoutTrust(id, allow) }

// LayoutStorageGet returns the JSON text stored under key by a layout.
func (z *Zen) LayoutStorageGet(id, key string) (string, error) { return z.s.LayoutStorageGet(id, key) }

// LayoutStorageSet stores a JSON value for a layout.
func (z *Zen) LayoutStorageSet(id, key, value string) error {
	return z.s.LayoutStorageSet(id, key, value)
}

// LayoutStorageKeys lists the layout's storage keys that start with prefix.
func (z *Zen) LayoutStorageKeys(id, prefix string) ([]string, error) {
	return z.s.LayoutStorageKeys(id, prefix)
}

// BuildOptions returns the form's starting values for the open folder.
func (z *Zen) BuildOptions() (BuildOptions, error) { return z.s.BuildOptions() }

// BuildWebsite builds the open folder like `zen build`; build:progress events
// report each page.
func (z *Zen) BuildWebsite(o BuildOptions) (BuildResult, error) { return z.s.BuildWebsite(o) }

// CancelBuild stops the running build after the current page.
func (z *Zen) CancelBuild() { z.s.CancelBuild() }

// SaveBuildDefaults writes the options into the build: section of the
// folder's .zen.yml; without confirm it only returns the preview text.
func (z *Zen) SaveBuildDefaults(o BuildOptions, confirm bool) (string, error) {
	return z.s.SaveBuildDefaults(o, confirm)
}
