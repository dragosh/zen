package plugins

import (
	"strings"
	"testing"
)

type bare struct{ m Manifest }

func (b bare) Manifest() Manifest { return b.m }

type declared struct {
	bare
	hooks string
}

func (d declared) RenderBlock(Context, Block) (string, error)   { return "", nil }
func (d declared) RenderInline(Context, Inline) (string, error) { return "", nil }
func (d declared) Source(_ Context, s []byte) ([]byte, error)   { return s, nil }
func (d declared) HTML(_ Context, s string) (string, error)     { return s, nil }
func (d declared) HasHook(name string) bool                     { return strings.Contains(d.hooks, name) }

func mf(id string) Manifest {
	return Manifest{ID: id, Version: "1.0.0", APIVersion: APIVersion, Default: true}
}

func TestLayouts(t *testing.T) {
	r := NewRegistry()
	a := mf("a")
	a.Layouts = []string{"slides", "course"}
	if err := r.Register(bare{a}); err != nil {
		t.Fatal(err)
	}
	if p := r.Layout("slides"); p == nil || p.Manifest().ID != "a" {
		t.Fatal("registry layout lookup")
	}
	if r.Layout("nope") != nil {
		t.Fatal("unknown layout")
	}
	b := mf("b")
	b.Layouts = []string{"course"}
	if err := r.Register(bare{b}); err == nil || !strings.Contains(err.Error(), `layout "course" is already owned by plugin "a"`) {
		t.Fatalf("duplicate layout: %v", err)
	}
	c := mf("c")
	c.Layouts = []string{"Bad"}
	if err := r.Register(bare{c}); err == nil {
		t.Fatal("invalid layout name accepted")
	}
	// A layout may share its name with the plugin's own block.
	d := mf("d")
	d.Layouts, d.Blocks = []string{"deck"}, []string{"deck"}
	if err := r.Register(bare{d}); err != nil {
		t.Fatal(err)
	}

	set, _ := r.Enabled(func(id string) (bool, map[string]any, bool) { return id != "d", nil, true }, nil)
	if p := set.Layout("slides"); p == nil || p.Manifest().ID != "a" {
		t.Fatal("set layout lookup")
	}
	if set.Layout("deck") != nil {
		t.Fatal("layout of a disabled plugin")
	}
	if got := r.Plugins(); len(got) != 2 || got[0].Manifest().ID != "a" || got[1].Manifest().ID != "d" {
		t.Fatalf("Plugins() = %v", got)
	}
}

func TestDeny(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(bare{mf("ok")}); err != nil {
		t.Fatal(err)
	}
	x := mf("x")
	x.Blocks, x.Inlines, x.Layouts = []string{"secret"}, []string{"sec"}, []string{"slay"}
	r.Deny(x)
	set, warnings := r.Enabled(nil, []string{"x", "ghost"})
	if len(warnings) != 1 || warnings[0] != `unknown plugin "ghost"` {
		t.Fatalf("warnings: %q", warnings)
	}
	if set.Denied("block", "secret") != "x" || set.Denied("inline", "sec") != "x" || set.Denied("layout", "slay") != "x" {
		t.Fatal("denied names not reported")
	}
	if set.Denied("block", "sec") != "" || set.Denied("block", "other") != "" {
		t.Fatal("wrong kind reported")
	}
	if set.Block("secret") != nil {
		t.Fatal("a denied plugin must not own names")
	}
	// A name owned by a registered plugin is never denied.
	y := mf("y")
	y.Blocks = []string{"ok-block"}
	if err := r.Register(bare{y}); err != nil {
		t.Fatal(err)
	}
	z := mf("z")
	z.Blocks = []string{"ok-block"}
	r.Deny(z)
	set, _ = r.Enabled(nil, nil)
	if set.Denied("block", "ok-block") != "" {
		t.Fatal("registered name reported as denied")
	}
}

func TestDeclaredHooks(t *testing.T) {
	all := declared{bare{mf("all")}, "block inline source html"}
	none := declared{bare{mf("none")}, ""}
	if _, ok := BlockHookOf(all); !ok {
		t.Fatal("declared block hook")
	}
	if _, ok := InlineHookOf(all); !ok {
		t.Fatal("declared inline hook")
	}
	if _, ok := SourceHookOf(all); !ok {
		t.Fatal("declared source hook")
	}
	if _, ok := HTMLHookOf(all); !ok {
		t.Fatal("declared html hook")
	}
	if _, ok := BlockHookOf(none); ok {
		t.Fatal("undeclared block hook")
	}
	if _, ok := InlineHookOf(none); ok {
		t.Fatal("undeclared inline hook")
	}
	if _, ok := SourceHookOf(none); ok {
		t.Fatal("undeclared source hook")
	}
	if _, ok := HTMLHookOf(none); ok {
		t.Fatal("undeclared html hook")
	}
	if _, ok := BlockHookOf(bare{mf("bare")}); ok {
		t.Fatal("a plugin without the method has no hook")
	}
}
