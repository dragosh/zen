package plugins

import (
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

// fake is a minimal plugin used to exercise the registry.
type fake struct{ m Manifest }

func (f fake) Manifest() Manifest { return f.m }

// fakeAssets is a fake plugin that also ships assets.
type fakeAssets struct {
	fake
	fsys  fs.FS
	entry Assets
}

func (f fakeAssets) Assets() fs.FS { return f.fsys }
func (f fakeAssets) Entry() Assets { return f.entry }

func manifest(id string) Manifest {
	return Manifest{ID: id, Version: "0.1.0", APIVersion: APIVersion}
}

func TestRegisterValidates(t *testing.T) {
	bad := []struct {
		name string
		m    Manifest
	}{
		{"empty id", Manifest{Version: "1.0.0", APIVersion: APIVersion}},
		{"uppercase id", Manifest{ID: "Diagram", Version: "1.0.0", APIVersion: APIVersion}},
		{"id starts with digit", Manifest{ID: "1x", Version: "1.0.0", APIVersion: APIVersion}},
		{"id with underscore", Manifest{ID: "a_b", Version: "1.0.0", APIVersion: APIVersion}},
		{"old api", Manifest{ID: "old", Version: "1.0.0", APIVersion: APIVersion - 1}},
		{"new api", Manifest{ID: "new", Version: "1.0.0", APIVersion: APIVersion + 1}},
		{"no version", Manifest{ID: "nov", APIVersion: APIVersion}},
		{"bad version", Manifest{ID: "badv", Version: "1.0", APIVersion: APIVersion}},
		{"v-prefixed version", Manifest{ID: "vv", Version: "v1.0.0", APIVersion: APIVersion}},
		{"bad block name", Manifest{ID: "bb", Version: "1.0.0", APIVersion: APIVersion, Blocks: []string{"Bad Name"}}},
		{"bad inline name", Manifest{ID: "bi", Version: "1.0.0", APIVersion: APIVersion, Inlines: []string{""}}},
		{"bad option type", Manifest{ID: "bo", Version: "1.0.0", APIVersion: APIVersion, Config: map[string]Option{"x": {Type: "float"}}}},
	}
	for _, tc := range bad {
		r := NewRegistry()
		err := r.Register(fake{tc.m})
		if err == nil {
			t.Errorf("%s: expected error", tc.name)
			continue
		}
		if tc.m.ID != "" && !strings.Contains(err.Error(), tc.m.ID) {
			t.Errorf("%s: error %q does not name plugin %q", tc.name, err, tc.m.ID)
		}
		if len(r.All()) != 0 {
			t.Errorf("%s: rejected plugin was stored", tc.name)
		}
	}

	r := NewRegistry()
	if err := r.Register(nil); err == nil {
		t.Error("nil plugin: expected error")
	}
	for _, v := range []string{"0.1.0", "1.2.3-alpha.1", "1.0.0+build.5", "0.1.0-alpha.1+x"} {
		rr := NewRegistry()
		m := manifest("ok")
		m.Version = v
		if err := rr.Register(fake{m}); err != nil {
			t.Errorf("version %q rejected: %v", v, err)
		}
	}

	a := manifest("alpha")
	a.Blocks = []string{"mermaid", "diagram"}
	a.Inlines = []string{"badge"}
	if err := r.Register(fake{a}); err != nil {
		t.Fatalf("valid plugin rejected: %v", err)
	}
	// duplicate id
	if err := r.Register(fake{manifest("alpha")}); err == nil || !strings.Contains(err.Error(), "alpha") {
		t.Errorf("duplicate id: got %v", err)
	}
	// block owned by another plugin
	b := manifest("beta")
	b.Blocks = []string{"diagram"}
	err := r.Register(fake{b})
	if err == nil || !strings.Contains(err.Error(), "beta") || !strings.Contains(err.Error(), "diagram") {
		t.Errorf("stolen block: got %v", err)
	}
	// inline owned by another plugin
	c := manifest("gamma")
	c.Inlines = []string{"badge"}
	if err := r.Register(fake{c}); err == nil || !strings.Contains(err.Error(), "gamma") {
		t.Errorf("stolen inline: got %v", err)
	}
	// block and inline namespaces are separate
	d := manifest("delta")
	d.Blocks = []string{"badge"}
	d.Inlines = []string{"mermaid"}
	if err := r.Register(fake{d}); err != nil {
		t.Errorf("separate namespaces: %v", err)
	}
	// a failed registration must not leave names claimed
	e := manifest("epsilon")
	e.Blocks = []string{"fresh", "diagram"}
	if err := r.Register(fake{e}); err == nil {
		t.Error("expected error for epsilon")
	}
	f := manifest("zeta")
	f.Blocks = []string{"fresh"}
	if err := r.Register(fake{f}); err != nil {
		t.Errorf("name leaked from failed registration: %v", err)
	}

	var ids []string
	for _, m := range r.All() {
		ids = append(ids, m.ID)
	}
	if want := []string{"alpha", "delta", "zeta"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("All() = %v, want %v", ids, want)
	}
}

func testRegistry(t *testing.T) *Registry {
	t.Helper()
	r := NewRegistry()
	on := manifest("on")
	on.Default = true
	on.Blocks = []string{"onblock"}
	on.Inlines = []string{"online"}
	on.Allow = []AllowRule{{Element: "svg", Attrs: []string{"viewBox"}}}
	on.Config = map[string]Option{
		"theme": {Type: "string", Default: "neutral"},
		"lines": {Type: "bool", Default: false},
		"depth": {Type: "int", Default: 3},
	}
	off := manifest("off")
	off.Blocks = []string{"offblock"}
	off.Inlines = []string{"offline"}
	off.Allow = []AllowRule{{Element: "video"}}
	off.Config = map[string]Option{"speed": {Type: "int", Default: 1}}
	for _, m := range []Manifest{on, off} {
		if err := r.Register(fake{m}); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func setIDs(s *Set) []string {
	ids := []string{}
	for _, p := range s.Plugins() {
		ids = append(ids, p.Manifest().ID)
	}
	return ids
}

type entry struct {
	enabled bool
	opts    map[string]any
}

func lookupFrom(m map[string]entry) func(string) (bool, map[string]any, bool) {
	return func(id string) (bool, map[string]any, bool) {
		e, ok := m[id]
		return e.enabled, e.opts, ok
	}
}

func TestEnabled(t *testing.T) {
	r := testRegistry(t)

	// defaults
	s, warns := r.Enabled(lookupFrom(nil), nil)
	if got := setIDs(s); !reflect.DeepEqual(got, []string{"on"}) {
		t.Errorf("defaults: %v", got)
	}
	if len(warns) != 0 {
		t.Errorf("defaults: warnings %v", warns)
	}
	want := map[string]any{"theme": "neutral", "lines": false, "depth": 3}
	ctx := s.Context("on", "docs/a.md")
	if ctx.Path != "docs/a.md" || !reflect.DeepEqual(ctx.Config, want) {
		t.Errorf("default context: %+v", ctx)
	}
	// nil lookup behaves like "nothing mentioned"
	s, _ = r.Enabled(nil, nil)
	if got := setIDs(s); !reflect.DeepEqual(got, []string{"on"}) {
		t.Errorf("nil lookup: %v", got)
	}

	// config override: registration order is kept, options merge over defaults
	s, warns = r.Enabled(lookupFrom(map[string]entry{
		"off": {true, map[string]any{"speed": 5}},
		"on":  {true, map[string]any{"theme": "dark"}},
	}), []string{"off", "on"})
	if got := setIDs(s); !reflect.DeepEqual(got, []string{"on", "off"}) {
		t.Errorf("override order: %v", got)
	}
	if len(warns) != 0 {
		t.Errorf("override: warnings %v", warns)
	}
	want = map[string]any{"theme": "dark", "lines": false, "depth": 3}
	if got := s.Context("on", "p.md").Config; !reflect.DeepEqual(got, want) {
		t.Errorf("merged options: %v", got)
	}
	if got := s.Context("off", "p.md").Config; !reflect.DeepEqual(got, map[string]any{"speed": 5}) {
		t.Errorf("off options: %v", got)
	}
	// contexts are independent copies
	s.Context("on", "p.md").Config["theme"] = "mutated"
	if got := s.Context("on", "p.md").Config["theme"]; got != "dark" {
		t.Errorf("context config is shared: %v", got)
	}

	// switching a default plugin off
	s, _ = r.Enabled(lookupFrom(map[string]entry{"on": {false, nil}}), []string{"on"})
	if got := setIDs(s); len(got) != 0 {
		t.Errorf("disabled: %v", got)
	}
	if ctx := s.Context("on", "p.md"); ctx.Path != "p.md" || len(ctx.Config) != 0 {
		t.Errorf("context of disabled plugin: %+v", ctx)
	}

	// warnings: unknown option, wrong option type, unknown id
	s, warns = r.Enabled(lookupFrom(map[string]entry{
		"on":     {true, map[string]any{"nope": 1, "depth": "deep", "lines": true}},
		"ghost":  {true, nil},
		"ghost2": {false, nil},
	}), []string{"on", "ghost", "ghost2", "ghost"})
	if len(warns) != 4 {
		t.Fatalf("want 4 warnings, got %d: %v", len(warns), warns)
	}
	joined := strings.Join(warns, "\n")
	for _, frag := range []string{"nope", "depth", "ghost", "ghost2"} {
		if !strings.Contains(joined, frag) {
			t.Errorf("warnings %q miss %q", joined, frag)
		}
	}
	want = map[string]any{"theme": "neutral", "lines": true, "depth": 3}
	if got := s.Context("on", "p.md").Config; !reflect.DeepEqual(got, want) {
		t.Errorf("options after warnings: %v", got)
	}

	// YAML/JSON decoders may hand over other integer shapes
	s, warns = r.Enabled(lookupFrom(map[string]entry{
		"on": {true, map[string]any{"depth": float64(7)}},
	}), []string{"on"})
	if got := s.Context("on", "p.md").Config["depth"]; got != 7 || len(warns) != 0 {
		t.Errorf("integral float: %v (%T), warnings %v", got, got, warns)
	}
}

func TestLookupByName(t *testing.T) {
	r := testRegistry(t)
	s, _ := r.Enabled(nil, nil)
	if p := s.Block("onblock"); p == nil || p.Manifest().ID != "on" {
		t.Errorf("Block(onblock) = %v", p)
	}
	if p := s.Inline("online"); p == nil || p.Manifest().ID != "on" {
		t.Errorf("Inline(online) = %v", p)
	}
	if p := s.Block("offblock"); p != nil {
		t.Errorf("disabled plugin owns block: %v", p)
	}
	if p := s.Inline("offline"); p != nil {
		t.Errorf("disabled plugin owns inline: %v", p)
	}
	if s.Block("online") != nil || s.Inline("onblock") != nil || s.Block("nothing") != nil || s.Block("") != nil {
		t.Error("lookup crossed namespaces or matched unknown names")
	}
	if got := s.Allow(); !reflect.DeepEqual(got, []AllowRule{{Element: "svg", Attrs: []string{"viewBox"}}}) {
		t.Errorf("Allow() = %v", got)
	}

	s, _ = r.Enabled(lookupFrom(map[string]entry{"on": {false, nil}, "off": {true, nil}}), nil)
	if s.Block("onblock") != nil || s.Inline("online") != nil {
		t.Error("plugin disabled by config still owns names")
	}
	if p := s.Block("offblock"); p == nil || p.Manifest().ID != "off" {
		t.Errorf("Block(offblock) = %v", p)
	}
	if got := s.Allow(); !reflect.DeepEqual(got, []AllowRule{{Element: "video"}}) {
		t.Errorf("Allow() = %v", got)
	}
}

func TestErrorBoxEscapes(t *testing.T) {
	got := ErrorBox("diagram", errors.New(`bad <script>alert("x")</script> & 'more'`))
	if !strings.HasPrefix(got, `<div class="zen-plugin-error">`) || !strings.HasSuffix(got, "</div>") {
		t.Errorf("unexpected shape: %s", got)
	}
	if !strings.Contains(got, "diagram") {
		t.Errorf("plugin not named: %s", got)
	}
	if strings.Contains(got, "<script") || strings.Contains(got, `"x"`) || strings.Contains(got, "'more'") {
		t.Errorf("message not escaped: %s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") || !strings.Contains(got, "&amp;") {
		t.Errorf("escaped message missing: %s", got)
	}
	// the id is escaped too, and a nil error does not panic
	got = ErrorBox("<b>", nil)
	if strings.Contains(got, "<b>") {
		t.Errorf("id not escaped: %s", got)
	}
}

func TestAssets(t *testing.T) {
	r := NewRegistry()
	fsys := fstest.MapFS{
		"diagram.css":   {Data: []byte("css")},
		"js/diagram.js": {Data: []byte("js")},
		"js/mermaid.js": {Data: []byte("m")},
	}
	d := manifest("diagram")
	d.Default = true
	x := manifest("extra")
	plain := manifest("plain")
	plain.Default = true
	must := func(p Plugin) {
		t.Helper()
		if err := r.Register(p); err != nil {
			t.Fatal(err)
		}
	}
	must(fakeAssets{fake{d}, fsys, Assets{
		CSS: []string{"diagram.css"},
		JS:  []string{"js/mermaid.js", "./js/diagram.js", "../secret.js", "/abs.js", "https://evil.example/x.js", ""},
	}})
	must(fake{plain})
	must(fakeAssets{fake{x}, fstest.MapFS{"x.css": {Data: []byte("x")}}, Assets{CSS: []string{"x.css"}}})

	s, _ := r.Enabled(nil, nil)
	want := Assets{
		CSS: []string{"/_plugins/diagram/diagram.css"},
		JS:  []string{"/_plugins/diagram/js/mermaid.js", "/_plugins/diagram/js/diagram.js"},
	}
	if got := s.Assets(); !reflect.DeepEqual(got, want) {
		t.Errorf("Assets() = %+v, want %+v", got, want)
	}

	// enabling the second provider appends its files in registration order
	s, _ = r.Enabled(lookupFrom(map[string]entry{"extra": {true, nil}}), nil)
	if got := s.Assets().CSS; !reflect.DeepEqual(got, []string{"/_plugins/diagram/diagram.css", "/_plugins/extra/x.css"}) {
		t.Errorf("CSS = %v", got)
	}

	// nothing enabled: no assets
	s, _ = r.Enabled(lookupFrom(map[string]entry{"diagram": {false, nil}, "plain": {false, nil}}), nil)
	if got := s.Assets(); len(got.CSS) != 0 || len(got.JS) != 0 {
		t.Errorf("assets of disabled plugins: %+v", got)
	}

	// AssetFS works regardless of enablement
	got, ok := r.AssetFS("diagram")
	if !ok {
		t.Fatal("AssetFS(diagram) not found")
	}
	data, err := fs.ReadFile(got, "js/diagram.js")
	if err != nil || string(data) != "js" {
		t.Errorf("read asset: %q, %v", data, err)
	}
	if _, ok := r.AssetFS("extra"); !ok {
		t.Error("AssetFS(extra) not found")
	}
	if _, ok := r.AssetFS("plain"); ok {
		t.Error("plugin without assets has an FS")
	}
	if _, ok := r.AssetFS("missing"); ok {
		t.Error("unknown plugin has an FS")
	}
}

type versioned struct {
	fakeAssets
	v string
}

func (v versioned) AssetVersion() string { return v.v }

// Contract 2: asset URLs carry ?v=: the plugin's own version, else the registry's.
func TestAssetURLsAreVersioned(t *testing.T) {
	r := NewRegistry()
	mk := func(id string) fakeAssets {
		return fakeAssets{fake{Manifest{ID: id, Version: "1.0.0", APIVersion: APIVersion, Default: true}},
			fstest.MapFS{"a.js": {Data: []byte("1")}}, Assets{JS: []string{"a.js"}}}
	}
	if err := r.Register(mk("builtin")); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(versioned{mk("ext"), "0123456789ab"}); err != nil {
		t.Fatal(err)
	}
	if s, _ := r.Enabled(nil, nil); !reflect.DeepEqual(s.Assets().JS, []string{"/_plugins/builtin/a.js", "/_plugins/ext/a.js?v=0123456789ab"}) {
		t.Errorf("without a registry version: %v", s.Assets().JS)
	}
	r.SetAssetVersion("0.2.0")
	if s, _ := r.Enabled(nil, nil); !reflect.DeepEqual(s.Assets().JS, []string{"/_plugins/builtin/a.js?v=0.2.0", "/_plugins/ext/a.js?v=0123456789ab"}) {
		t.Errorf("with a registry version: %v", s.Assets().JS)
	}
}
