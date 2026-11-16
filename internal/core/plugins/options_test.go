package plugins

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func optRegistry(t *testing.T) *Registry {
	t.Helper()
	r := NewRegistry()
	m := manifest("box")
	m.Default = true
	m.Config = map[string]Option{
		"images": {Type: "map", Scope: "user", Default: map[string]string{"py": "python:3"}},
		"tags":   {Type: "list", Default: []string{"a"}},
		"names":  {Type: "map"},
		"theme":  {Type: "string", Default: "x"},
	}
	if err := r.Register(fake{m}); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRegisterOptionTypesAndScope(t *testing.T) {
	for _, typ := range []string{"map", "list"} {
		m := manifest("ok")
		m.Config = map[string]Option{"o": {Type: typ}}
		if err := NewRegistry().Register(fake{m}); err != nil {
			t.Errorf("type %s refused: %v", typ, err)
		}
	}
	m := manifest("bad")
	m.Config = map[string]Option{"o": {Type: "string", Scope: "folder"}}
	if err := NewRegistry().Register(fake{m}); err == nil || !strings.Contains(err.Error(), "scope") {
		t.Errorf("unknown scope accepted: %v", err)
	}
	m.Config = map[string]Option{"o": {Type: "float"}}
	if err := NewRegistry().Register(fake{m}); err == nil {
		t.Error("unknown type accepted")
	}
}

func TestMapAndListOptions(t *testing.T) {
	r := optRegistry(t)
	s, warns := r.Enabled(lookupFrom(map[string]entry{"box": {true, map[string]any{
		"tags":  []any{"x", "y"},
		"names": map[string]any{"k": "v"},
	}}}), []string{"box"})
	if len(warns) != 0 {
		t.Fatalf("warnings: %v", warns)
	}
	cfg := s.Context("box", "p.md").Config
	if !reflect.DeepEqual(cfg["tags"], []string{"x", "y"}) || !reflect.DeepEqual(cfg["names"], map[string]string{"k": "v"}) {
		t.Errorf("coerced: %#v", cfg)
	}
	// defaults are copies: mutating one context does not change the next
	s.Context("box", "p.md").Config["tags"].([]string)[0] = "mutated"
	if got := s.Context("box", "p.md").Config["tags"]; !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Errorf("shared value: %v", got)
	}
	s, _ = r.Enabled(nil, nil)
	if got := s.Context("box", "p.md").Config["tags"]; !reflect.DeepEqual(got, []string{"a"}) {
		t.Errorf("default list: %#v", got)
	}
	s.Context("box", "p.md").Config["tags"].([]string)[0] = "mutated"
	s2, _ := r.Enabled(nil, nil)
	if got := s2.Context("box", "p.md").Config["tags"]; !reflect.DeepEqual(got, []string{"a"}) {
		t.Errorf("default list was mutated: %#v", got)
	}

	// wrong shapes keep the default and warn
	for name, bad := range map[string]any{
		"tags":  []any{"x", 3},
		"names": map[string]any{"k": 1},
	} {
		_, warns := r.Enabled(lookupFrom(map[string]entry{"box": {true, map[string]any{name: bad}}}), []string{"box"})
		if len(warns) != 1 || !strings.Contains(warns[0], name) {
			t.Errorf("%s=%v: warnings %v", name, bad, warns)
		}
	}
	for _, bad := range []any{"str", 5, []any{[]any{}}} {
		if _, ok := CoerceOption("map", bad); ok {
			t.Errorf("map accepted %v", bad)
		}
	}
	if v, ok := CoerceOption("list", []string{"a"}); !ok || !reflect.DeepEqual(v, []string{"a"}) {
		t.Errorf("list of strings: %v %v", v, ok)
	}
}

func TestUserScopedOptions(t *testing.T) {
	r := optRegistry(t)
	global := map[string]map[string]any{"box": {"images": map[string]any{"node": "node:20"}}}
	r.SetUserOptions(func(id string) map[string]any { return global[id] })

	// The global value wins; a folder or page value is ignored with a warning.
	s, warns := r.Enabled(lookupFrom(map[string]entry{"box": {true, map[string]any{
		"images": map[string]any{"evil": "attacker:latest"},
		"theme":  "dark",
	}}}), []string{"box"})
	cfg := s.Context("box", "p.md").Config
	if !reflect.DeepEqual(cfg["images"], map[string]string{"node": "node:20"}) || cfg["theme"] != "dark" {
		t.Errorf("config: %#v", cfg)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "images") || !strings.Contains(warns[0], "global") {
		t.Errorf("warnings: %v", warns)
	}

	// The same value as the global one is not a conflict.
	_, warns = r.Enabled(lookupFrom(map[string]entry{"box": {true, map[string]any{"images": map[string]any{"node": "node:20"}}}}), []string{"box"})
	if len(warns) != 0 {
		t.Errorf("same value warned: %v", warns)
	}

	// Without a global value the manifest default applies, not the folder's.
	delete(global, "box")
	s, warns = r.Enabled(lookupFrom(map[string]entry{"box": {true, map[string]any{"images": map[string]any{"evil": "x"}}}}), []string{"box"})
	if got := s.Context("box", "p.md").Config["images"]; !reflect.DeepEqual(got, map[string]string{"py": "python:3"}) {
		t.Errorf("default expected, got %#v", got)
	}
	if len(warns) != 1 {
		t.Errorf("warnings: %v", warns)
	}

	// With no provider at all nothing from the merged config is trusted.
	r2 := optRegistry(t)
	s, warns = r2.Enabled(lookupFrom(map[string]entry{"box": {true, map[string]any{"images": map[string]any{"evil": "x"}}}}), []string{"box"})
	if got := s.Context("box", "p.md").Config["images"]; !reflect.DeepEqual(got, map[string]string{"py": "python:3"}) || len(warns) != 1 {
		t.Errorf("no provider: %#v %v", got, warns)
	}

	// A malformed global value keeps the default and warns.
	r.SetUserOptions(func(string) map[string]any { return map[string]any{"images": "oops"} })
	_, warns = r.Enabled(nil, nil)
	if len(warns) != 1 || !strings.Contains(warns[0], "images") {
		t.Errorf("bad global value: %v", warns)
	}
}

func TestActionAndStreamHookOf(t *testing.T) {
	var p Plugin = fake{manifest("x")}
	if _, ok := ActionHookOf(p); ok {
		t.Error("plain plugin has an action hook")
	}
	if _, ok := StreamHookOf(p); ok {
		t.Error("plain plugin has a stream hook")
	}
	both := actingFake{fake{manifest("y")}}
	if _, ok := ActionHookOf(both); !ok {
		t.Error("ActionHook not found")
	}
	if _, ok := StreamHookOf(both); !ok {
		t.Error("StreamHook not found")
	}
}

type actingFake struct{ fake }

func (actingFake) Action(ctx Context, payload []byte) ([]byte, error) { return payload, nil }
func (actingFake) Stream(c context.Context, ctx Context, payload []byte, emit func([]byte)) error {
	return nil
}
