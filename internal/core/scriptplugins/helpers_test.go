package scriptplugins_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/core/scriptplugins"
)

// writeFile writes rel (slash separated) under dir.
func writeFile(t testing.TB, dir, rel, body string) string {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return full
}

// memStore is the Storage of a test: values are JSON text.
type memStore struct {
	mu   sync.Mutex
	m    map[string]string
	fail error
}

func (s *memStore) Get(key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail != nil {
		return "", s.fail
	}
	if v, ok := s.m[key]; ok {
		return v, nil
	}
	return "null", nil
}

func (s *memStore) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail != nil {
		return s.fail
	}
	if s.m == nil {
		s.m = map[string]string{}
	}
	s.m[key] = value
	return nil
}

func (s *memStore) Keys(prefix string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := []string{}
	for k := range s.m {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

// site is a workspace folder with a plugin folder in it.
type site struct {
	t   *testing.T
	dir string // workspace root
}

func newSite(t *testing.T) *site {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &site{t: t, dir: dir}
}

func (s *site) file(rel, body string) string { return writeFile(s.t, s.dir, rel, body) }

// plugin writes .zen/plugins/<id>.tengo, loads and binds it.
func (s *site) plugin(id, src string, rt ...scriptplugins.Runtime) *scriptplugins.Plugin {
	s.t.Helper()
	path := s.file(".zen/plugins/"+id+".tengo", src)
	return s.bind(path, rt...)
}

// folder writes .zen/plugins/<id>/plugin.tengo (plus extra files), loads and binds it.
func (s *site) folder(id, src string, extra map[string]string, rt ...scriptplugins.Runtime) *scriptplugins.Plugin {
	s.t.Helper()
	path := s.file(".zen/plugins/"+id+"/plugin.tengo", src)
	for name, body := range extra {
		s.file(".zen/plugins/"+id+"/"+name, body)
	}
	return s.bind(filepath.Dir(path), rt...)
}

func (s *site) bind(path string, rt ...scriptplugins.Runtime) *scriptplugins.Plugin {
	s.t.Helper()
	p, err := scriptplugins.Load(path)
	if err != nil {
		s.t.Fatalf("Load: %v", err)
	}
	r := scriptplugins.Runtime{Workspace: s.dir}
	if len(rt) > 0 {
		r = rt[0]
		if r.Workspace == "" {
			r.Workspace = s.dir
		}
	}
	p.Bind(r)
	return p
}

// loadErr writes a plugin and returns the error Load gives.
func (s *site) loadErr(id, src string) error {
	s.t.Helper()
	path := s.file(".zen/plugins/"+id+".tengo", src)
	p, err := scriptplugins.Load(path)
	if err == nil {
		s.t.Fatalf("Load(%s) succeeded: %+v", id, p.Manifest())
	}
	return err
}

func block(p *scriptplugins.Plugin, name, body string, attrs map[string]string) (string, error) {
	return p.RenderBlock(plugins.Context{Path: "a.md"}, plugins.Block{Name: name, Body: body, Attrs: attrs})
}

func mustBlock(t testing.TB, p *scriptplugins.Plugin, name, body string, attrs map[string]string) string {
	t.Helper()
	out, err := block(p, name, body, attrs)
	if err != nil {
		t.Fatalf("RenderBlock: %v", err)
	}
	return out
}

// header is a valid plugin map for a plugin with one block called x.
const header = `plugin := {version: "1.0.0", blocks: ["x"]}` + "\n"

func withHeader(body string) string { return header + body }

func errContains(t testing.TB, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error %v, want it to contain %q", err, want)
	}
}

func noPath(t testing.TB, err error, dir string) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), dir) {
		t.Errorf("error names a path: %v", err)
	}
}

func sprintf(f string, a ...any) string { return fmt.Sprintf(f, a...) }

var _ = context.Background
