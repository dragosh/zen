package scriptplugins_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zenmark-org/zenmd/engine/scriptplugins"
)

func timed(f func()) time.Duration {
	t0 := time.Now()
	f()
	return time.Since(t0)
}

func TestTimeoutDefaultIs200ms(t *testing.T) {
	s := newSite(t)
	p := s.plugin("loop", withHeader(`if hook == "block" { for { } }`))
	var err error
	d := timed(func() { _, err = block(p, "x", "", nil) })
	errContains(t, err, "timed out")
	errContains(t, err, "200ms")
	if d < 150*time.Millisecond || d > 1500*time.Millisecond {
		t.Errorf("stopped after %v, want about 200ms", d)
	}
	// The plugin keeps working afterwards.
	ok := s.plugin("ok", withHeader(`if hook == "block" { out = "fine" }`))
	if got := mustBlock(t, ok, "x", "", nil); got != "fine" {
		t.Error(got)
	}
}

func TestTimeoutCanBeRaisedUpTo2s(t *testing.T) {
	s := newSite(t)
	p := s.plugin("short", `plugin := {version: "1.0.0", blocks: ["x"], timeout: "50ms"}
if hook == "block" { for { } }`)
	var err error
	d := timed(func() { _, err = block(p, "x", "", nil) })
	errContains(t, err, "timed out")
	errContains(t, err, "50ms")
	if d > 700*time.Millisecond {
		t.Errorf("a 50ms timeout took %v", d)
	}
	// A longer budget really is longer.
	long := s.plugin("long", `plugin := {version: "1.0.0", blocks: ["x"], timeout: "1s"}
if hook == "block" { for { } }`)
	d = timed(func() { _, err = block(long, "x", "", nil) })
	errContains(t, err, "1s")
	if d < 900*time.Millisecond || d > 2500*time.Millisecond {
		t.Errorf("a 1s timeout took %v", d)
	}
	// 2s is the most that can be asked for.
	two := s.plugin("two", `plugin := {version: "1.0.0", blocks: ["x"], timeout: "2s"}
if hook == "block" { out = "ok" }`)
	if got := mustBlock(t, two, "x", "", nil); got != "ok" {
		t.Error(got)
	}
	for _, bad := range []string{`"3s"`, `"2001ms"`, `"0s"`, `"-1s"`, `"soon"`, `5`, `true`} {
		err := s.loadErr("bad", `plugin := {version: "1.0.0", blocks: ["x"], timeout: `+bad+`}`)
		errContains(t, err, "timeout")
	}
}

func TestTopLevelLoopFailsTheLoad(t *testing.T) {
	s := newSite(t)
	err := s.loadErr("spin", `plugin := {version: "1.0.0", blocks: ["x"]}`+"\nfor { }\n")
	errContains(t, err, "timed out")
}

func TestContextCancelStopsACall(t *testing.T) {
	s := newSite(t)
	ctx, cancel := context.WithCancel(context.Background())
	p := s.plugin("loop", withHeader(`if hook == "block" { for { } }`), scriptplugins.Runtime{Ctx: ctx})
	time.AfterFunc(50*time.Millisecond, cancel)
	var err error
	d := timed(func() { _, err = block(p, "x", "", nil) })
	errContains(t, err, "cancelled")
	if d > time.Second {
		t.Errorf("cancel took %v", d)
	}
	// A cancelled context refuses new calls at once.
	if _, err := block(p, "x", "", nil); err == nil {
		t.Error("call after cancel succeeded")
	}
}

func TestAllocationBudget(t *testing.T) {
	s := newSite(t)
	// A long timeout, so that the allocation budget is what stops it, even slowly (-race).
	p := s.plugin("alloc", `plugin := {version: "1.0.0", blocks: ["x"], timeout: "2s"}`+"\n"+`if hook == "block" {
    a := []
    for i := 0; i < 100000000; i++ { a = append(a, "xxxxxxxx") }
}`)
	var err error
	d := timed(func() { _, err = block(p, "x", "", nil) })
	errContains(t, err, "allocation")
	if d > time.Second {
		t.Errorf("the allocation budget took %v to hit", d)
	}
	// 500 000 objects is the budget (every number made counts): well under it works.
	under := s.plugin("under", withHeader(`if hook == "block" {
    n := 0
    for i := 0; i < 50000; i++ { s := string(i); n += len(s) }
    out = string(n)
}`))
	if _, err := block(under, "x", "", nil); err != nil {
		t.Errorf("work under the budget failed: %v", err)
	}
}

func TestMemoryBlowUpIsStopped(t *testing.T) {
	// Doubling an array costs few allocations but gigabytes.
	s := newSite(t)
	p := s.plugin("dbl", withHeader(`if hook == "block" {
    x := [1]
    for i := 0; i < 40; i++ { x = x + x }
}`))
	var err error
	d := timed(func() { _, err = block(p, "x", "", nil) })
	if err == nil {
		t.Fatal("an array grown to terabytes was accepted")
	}
	if d > 2*time.Second {
		t.Errorf("stopped after %v", d)
	}
}

func TestStringAndBytesDoublingAreStopped(t *testing.T) {
	s := newSite(t)
	for name, src := range map[string]string{
		"str":    withHeader("if hook == \"block\" {\ns := \"abcdefgh\"; for i := 0; i < 40; i++ { s = s + s }\n}"),
		"bytes":  withHeader("if hook == \"block\" {\nb := bytes(8); for i := 0; i < 40; i++ { b = b + b }\n}"),
		"repeat": "text := import(\"text\")\n" + withHeader("if hook == \"block\" { s := text.repeat(\"a\", 100000000) }"),
	} {
		p := s.plugin(name, src)
		var err error
		d := timed(func() { _, err = block(p, "x", "", nil) })
		if err == nil {
			t.Errorf("%s: accepted", name)
		}
		if d > 2*time.Second {
			t.Errorf("%s: stopped after %v", name, d)
		}
	}
}

func TestOutputLimit2MB(t *testing.T) {
	s := newSite(t)
	p := s.plugin("big", `text := import("text")
plugin := {version: "1.0.0", blocks: ["x"]}
if hook == "block" { out = text.repeat("a", int(body)) }`)
	if got, err := block(p, "x", "2097152", nil); err != nil || len(got) != 2<<20 {
		t.Errorf("exactly 2 MB: %d %v", len(got), err)
	}
	_, err := block(p, "x", "2097153", nil)
	errContains(t, err, "2 MB")
	_, err = block(p, "x", "3145728", nil)
	errContains(t, err, "2 MB")
}

func TestScriptSizeLimit256KB(t *testing.T) {
	s := newSite(t)
	pad := func(n int) string { return "// " + strings.Repeat("x", n-4) + "\n" }
	body := withHeader("")
	ok := body + pad(256<<10-len(body))
	path := s.file(".zen/plugins/fits.tengo", ok)
	if _, err := scriptplugins.Load(path); err != nil {
		t.Errorf("a 256 KB script: %v", err)
	}
	err := s.loadErr("huge", ok+"// one more\n")
	errContains(t, err, "256 KB")
	noPath(t, err, s.dir)
	// Folder form too.
	s.file(".zen/plugins/huge2/plugin.tengo", ok+"// one more\n")
	_, err = scriptplugins.Load(s.dir + "/.zen/plugins/huge2")
	errContains(t, err, "256 KB")
}

func TestNoSleepNoPrintAndTheStdlibThatIsThere(t *testing.T) {
	s := newSite(t)
	// times.sleep would block the call past the timeout (it cannot be interrupted).
	p := s.plugin("sleepy", `times := import("times")
plugin := {version: "1.0.0", blocks: ["x"]}
if hook == "block" { times.sleep(5 * times.second) }`)
	var err error
	d := timed(func() { _, err = block(p, "x", "", nil) })
	if err == nil || d > time.Second {
		t.Errorf("times.sleep: %v after %v", err, d)
	}
	q := s.plugin("printy", `fmt := import("fmt")
plugin := {version: "1.0.0", blocks: ["x"]}
if hook == "block" { fmt.println("to stdout") }`)
	if _, err := block(q, "x", "", nil); err == nil {
		t.Error("fmt.println is available (a plugin could write to zen's stdout)")
	}
	// The pure ones are there.
	ok := s.plugin("pure", `text := import("text"); math := import("math"); times := import("times"); fmt := import("fmt")
json := import("json"); enum := import("enum"); base64 := import("base64"); hex := import("hex"); rand := import("rand")
plugin := {version: "1.0.0", blocks: ["x"]}
if hook == "block" {
    out = fmt.sprintf("%s|%v|%s|%s|%s|%d|%v", text.to_upper("a"), math.abs(-2), json.encode({a: 1}), base64.encode("hi"), hex.encode("hi"), times.time_year(times.date(2026, 1, 2, 0, 0, 0, 0)), enum.all([1, 2], func(k, v) { return v > 0 }))
    r := rand.intn(10)
    if r < 0 || r > 9 { out = "bad rand" }
}`)
	got, err := block(ok, "x", "", nil)
	if err != nil || got != "A|2|{\"a\":1}|aGk=|6869|2026|true" {
		t.Errorf("stdlib: %q %v", got, err)
	}
}
