package session

import (
	"io/fs"
	"net/http"
	"testing"

	"github.com/zenmark-org/zenmd/engine/plugins"
	"github.com/zenmark-org/zenmd/engine/session/sessiontest"
)

// Helpers shared by the tests of this package. They keep the shape the
// tests had when they drove the app's service: a folder tree, an event sink,
// a prompter that answers as told, and a constructor with the old options.

type fakeDialogs = sessiontest.Prompter

type event struct {
	Name string
	Data any
}

// sink is the events of a session; all and reset keep their old names.
type sink struct{ sessiontest.Sink }

func (s *sink) all() []event {
	var out []event
	for _, e := range s.All() {
		out = append(out, event{e.Name, e.Data})
	}
	return out
}

func (s *sink) reset() { s.Reset() }

func tree(t *testing.T, files map[string]string) string { return sessiontest.Tree(t, files) }

// zenOptions are the options the tests give newZen.
type zenOptions struct {
	Registry   *plugins.Registry
	GlobalPath string
	ConfigDir  string
	Dir        string
	Events     Events
	Dialogs    Prompter
	Runtime    fs.FS
	Open       func(string) error // unused: the session opens nothing outside the folder
	DeferOpen  bool
}

func newZen(o zenOptions) (*Session, error) {
	s := New(Options{
		GlobalPath: o.GlobalPath, ConfigDir: o.ConfigDir, Registry: o.Registry,
		Events: o.Events, Prompter: o.Dialogs, Runtime: o.Runtime, Watch: true,
	})
	if o.DeferOpen {
		return s, nil
	}
	if err := s.Open(o.Dir); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func (s *Session) close() { s.Close() }

func newTestZen(t *testing.T, dir, page, mode string) (*Session, *sink, *fakeDialogs, *[]string) {
	t.Helper()
	ev := &sink{}
	dl := &fakeDialogs{}
	z, err := newZen(zenOptions{Registry: plugins.NewRegistry(), Dir: dir, Events: ev, Dialogs: dl})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	return z, ev, dl, new([]string)
}

var docs = map[string]string{
	"README.md":        "# Home\n\n[Guide](docs/guide.md)\n",
	"docs/guide.md":    "# Guide\n\n[Deep](deep.md) ![i](../img/a.png)\n",
	"docs/deep.md":     "# Deep\n",
	"img/a.png":        "\x89PNG\r\n\x1a\n",
	"img/x.svg":        `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
	"notes/secret.txt": "nope",
}

// Startup is what the app's service answers at launch; the tests that read the
// folder through it only need the workspace.
func (s *Session) Startup() Startup {
	return Startup{Workspace: s.Workspace(), Page: s.Home(), Mode: "read"}
}

// middleware is the session's handler; the app wraps it with the frontend.
func (s *Session) middleware(_ http.Handler) http.Handler { return s.Handler() }
