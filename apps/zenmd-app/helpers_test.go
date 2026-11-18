package app

import (
	"github.com/zenmark-org/zenmd/engine/sitetemplates"
	"io/fs"

	"github.com/zenmark-org/zenmd/engine/session"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/zenmark-org/zenmd/engine/plugins"
)

type event struct {
	Name string
	Data any
}

type sink struct {
	mu  sync.Mutex
	evs []event
}

func (s *sink) Emit(name string, data any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evs = append(s.evs, event{name, data})
}

func (s *sink) all() []event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]event(nil), s.evs...)
}

func (s *sink) reset() {
	s.mu.Lock()
	s.evs = nil
	s.mu.Unlock()
}

type fakeDialogs struct {
	folder  string
	confirm bool
	asked   []string
}

func (d *fakeDialogs) PickFolder() (string, error) { return d.folder, nil }
func (d *fakeDialogs) Confirm(msg string) bool {
	d.asked = append(d.asked, msg)
	return d.confirm
}

func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for p, c := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func newTestZen(t *testing.T, dir, page, mode string) (*Zen, *sink, *fakeDialogs, *[]string) {
	t.Helper()
	ev := &sink{}
	dl := &fakeDialogs{}
	var opened []string
	z, err := newZen(zenOptions{
		Registry: plugins.NewRegistry(),
		Dir:      dir,
		Page:     page,
		Mode:     mode,
		Events:   ev,
		Dialogs:  dl,
		Open:     func(u string) error { opened = append(opened, u); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	return z, ev, dl, &opened
}

var docs = map[string]string{
	"README.md":        "# Home\n\n[Guide](docs/guide.md)\n",
	"docs/guide.md":    "# Guide\n\n[Deep](deep.md) ![i](../img/a.png)\n",
	"docs/deep.md":     "# Deep\n",
	"img/a.png":        "\x89PNG\r\n\x1a\n",
	"img/x.svg":        `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
	"notes/secret.txt": "nope",
}

// siteEnv is a settings folder and a set of built-in presets, as the template
// tests want them.
type siteEnv struct {
	globalPath string
	builtins   fs.FS
}

func (e siteEnv) api() session.TemplatesAPI {
	return session.New(session.Options{GlobalPath: e.globalPath, Presets: e.builtins}).Templates()
}

func (e siteEnv) list() session.TemplateListing { return e.api().Listing() }

func hasEvent(ev *sink, name string) bool {
	for _, e := range ev.all() {
		if e.Name == name {
			return true
		}
	}
	return false
}

func (e siteEnv) find(id string) (sitetemplates.Template, session.TemplateListing, bool) {
	l := e.list()
	for _, t := range l.Templates {
		if t.ID == id {
			return t, l, true
		}
	}
	return sitetemplates.Template{}, l, false
}
