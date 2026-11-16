package scriptplugins_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dragosh/zen/internal/core/scriptplugins"
)

// crasher is a plugin that is fine for most input and makes Tengo recurse
// until Go's stack overflows (a fatal error nothing can recover) for one.
func crasher(crash string) string {
	return withHeader(`if hook == "block" {
    if body == "boom" {
` + crash + `
    }
    out = "fine:" + body
}`)
}

// The four ways to make Tengo overflow the stack with a self-containing value.
var crashers = map[string]string{
	"strmap":  `m := {}; m.a = m; out = string(m)`,
	"eqmaps":  `m := {}; m.a = m; n := {}; n.a = n; out = string(m == n)`,
	"copymap": `m := {}; m.a = m; out = string(copy(m))`,
	"addarr":  `a := [1]; a[0] = a; out = "x" + a`,
}

const spinner = `plugin := {version: "1.0.0", blocks: ["x"], timeout: "2s"}
if hook == "block" {
    if body == "spin" {
        for { }
    }
    out = "fine:" + body
}
`

// crashDuring calls p while the helper is killed with SIGKILL under it.
func crashDuring(p *scriptplugins.Plugin) (string, error) {
	time.AfterFunc(80*time.Millisecond, scriptplugins.CrashHelpers)
	return block(p, "x", "spin", nil)
}

func TestHelperKilledMidCallIsACrash(t *testing.T) {
	fresh(t)
	s := newSite(t)
	p := s.plugin("victim", spinner)
	other := s.plugin("other", withHeader(`if hook == "block" { out = "other:" + body }`))
	_, err := crashDuring(p)
	errContains(t, err, "crashed")
	if out := mustBlock(t, p, "x", "again", nil); out != "fine:again" {
		t.Errorf("same plugin after the crash: %q", out)
	}
	if out := mustBlock(t, other, "x", "later", nil); out != "other:later" {
		t.Errorf("other plugin after the crash: %q", out)
	}
}

func fresh(t *testing.T) {
	t.Helper()
	scriptplugins.ResetCrashes()
	scriptplugins.Reset()
}

func TestHelperCrashOneLinersGiveACleanError(t *testing.T) {
	fresh(t)
	s := newSite(t)
	other := s.plugin("other", withHeader(`if hook == "block" { out = "other:" + body }`))
	for name, line := range crashers {
		t.Run(name, func(t *testing.T) {
			p := s.plugin("c-"+name, crasher(line))
			_, err := block(p, "x", "boom", nil)
			// Whichever limit or crash gets there first, the answer is a clean
			// one-line error, not a dead app.
			if err == nil || strings.Contains(err.Error(), "\n") ||
				!(strings.Contains(err.Error(), "crashed") || strings.Contains(err.Error(), "memory") || strings.Contains(err.Error(), "timed out")) {
				t.Fatalf("error %v", err)
			}
			t.Logf("%s -> %v", name, err)
			// The same plugin and another one go on working.
			if out := mustBlock(t, p, "x", "again", nil); out != "fine:again" {
				t.Errorf("same plugin after the crash: %q", out)
			}
			if out := mustBlock(t, other, "x", "later", nil); out != "other:later" {
				t.Errorf("other plugin after the crash: %q", out)
			}
		})
	}
}

func TestHelperCrashLoopSwitchesThePluginOff(t *testing.T) {
	fresh(t)
	s := newSite(t)
	other := s.plugin("other", withHeader(`if hook == "block" { out = "other:" + body }`))
	src := spinner
	p := s.plugin("loopy", src)
	for i := 0; i < 2; i++ {
		_, err := crashDuring(p)
		errContains(t, err, "crashed")
	}
	_, err := crashDuring(p)
	errContains(t, err, "switched off")
	// Even a harmless call is refused now: no restart loop.
	_, err = block(p, "x", "fine", nil)
	errContains(t, err, "switched off")
	if out := mustBlock(t, other, "x", "ok", nil); out != "other:ok" {
		t.Errorf("other plugin: %q", out)
	}
	// Editing the script is a new version of the plugin: it runs again.
	p2 := s.plugin("loopy", src+"\n// fixed\n")
	if out := mustBlock(t, p2, "x", "ok", nil); out != "fine:ok" {
		t.Errorf("edited plugin: %q", out)
	}
}

func TestHelperTimeoutKeepsTheHelperServing(t *testing.T) {
	fresh(t)
	s := newSite(t)
	p := s.plugin("slow", withHeader("if hook == \"block\" {\n if body == \"spin\" { for { } }\n out = \"ok\"\n}"))
	_, err := block(p, "x", "spin", nil)
	errContains(t, err, "timed out")
	if out := mustBlock(t, p, "x", "go", nil); out != "ok" {
		t.Errorf("after the timeout: %q", out)
	}
	// A timeout is not a crash: it never switches the plugin off.
	for i := 0; i < 4; i++ {
		_, err = block(p, "x", "spin", nil)
		errContains(t, err, "timed out")
	}
	if out := mustBlock(t, p, "x", "go", nil); out != "ok" {
		t.Errorf("after many timeouts: %q", out)
	}
}

// fakeHelper makes the pool start `sh -c script` instead of zen.
func fakeHelper(t *testing.T, script string, args ...string) {
	t.Helper()
	scriptplugins.SetHelperCommand("/bin/sh", append([]string{"-c", script}, args...)...)
	t.Cleanup(func() {
		scriptplugins.SetHelperCommand(os.Args[0], scriptplugins.HostArg)
		scriptplugins.ResetCrashes()
	})
}

func TestHelperThatNeverAnswersIsKilledAtTheDeadline(t *testing.T) {
	fresh(t)
	fakeHelper(t, `exec sleep 30`)
	s := newSite(t)
	path := s.file(".zen/plugins/mute.tengo", header)
	var err error
	d := timed(func() { _, err = scriptplugins.Load(path) })
	errContains(t, err, "timed out")
	if d > 3*time.Second {
		t.Errorf("the wedged helper held the call for %v", d)
	}
	if n := scriptplugins.HelperCount(); n != 0 {
		t.Errorf("%d helpers alive after the kill", n)
	}
}

func TestHelperGarbageOnStdoutIsACrashNotAPanic(t *testing.T) {
	fresh(t)
	for _, script := range []string{
		`while read l; do echo 'not json'; done`,
		`while read l; do echo '{"t":"result","seq":999}'; done`,
		`while read l; do echo '{"t":"callback","seq":0,"cb":1,"fn":"nope"}'; done`,
		`while read l; do printf '\000\377\376{{{\n'; done`,
		`exit 3`,
		`head -c 100000 /dev/urandom; sleep 5`,
	} {
		fakeHelper(t, script)
		s := newSite(t)
		path := s.file(".zen/plugins/g.tengo", header)
		_, err := scriptplugins.Load(path)
		if err == nil {
			t.Fatalf("%q: Load succeeded", script)
		}
		t.Logf("%q -> %v", script, err)
	}
}

func TestHelperServesGarbageInput(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	var in bytes.Buffer
	for i := 0; i < 200; i++ {
		b := make([]byte, rnd.Intn(200))
		rnd.Read(b)
		in.Write(b)
		in.WriteByte('\n')
	}
	in.WriteString(`{"t":"call","seq":1,"hash":"nope"}` + "\n")
	in.WriteString(`{"t":"call","seq":2,"hash":"nope","globals":{},"limits":{"timeoutMs":5}}` + "\n")
	in.WriteString(`{"t":"reply","cb":77}` + "\n")
	sum := sha256.Sum256([]byte("y"))
	in.WriteString(`{"t":"load","hash":"` + hex.EncodeToString(sum[:]) + `","source":"y"}` + "\n")
	var out bytes.Buffer
	scriptplugins.ServeHost(&in, &out) // returns at the end of input, no panic
	if !strings.Contains(out.String(), "not loaded") || !strings.Contains(out.String(), `"t":"loaded"`) {
		t.Errorf("answers: %s", out.String())
	}
}

func TestHelperRefusesATerminalLikeStdin(t *testing.T) {
	null, err := os.Open(os.DevNull) // a character device, as a terminal is
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	errFile, err := os.CreateTemp(t.TempDir(), "err")
	if err != nil {
		t.Fatal(err)
	}
	defer errFile.Close()
	if code := scriptplugins.HostMain(null, os.Stdout, errFile); code == 0 {
		t.Errorf("exit code %d for a character device", code)
	}
	msg, _ := os.ReadFile(errFile.Name())
	if !strings.Contains(string(msg), scriptplugins.HostArg) {
		t.Errorf("message %q", msg)
	}
}

func TestHelperHasATinyEnvironmentAndAPrivateDirectory(t *testing.T) {
	fresh(t)
	t.Setenv("ZEN_SECRET_TOKEN", "hunter2")
	t.Setenv("LANG", "en_US.UTF-8")
	report := filepath.Join(t.TempDir(), "report")
	fakeHelper(t, `pwd > "$0"; env >> "$0"; exec sleep 30`, report)
	s := newSite(t)
	_, _ = scriptplugins.Load(s.file(".zen/plugins/env.tengo", header)) // times out; the report is written
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if !strings.Contains(lines[0], "zen-script-") {
		t.Errorf("working directory %q is not a private temp dir", lines[0])
	}
	allowed := map[string]bool{"PATH": true, "HOME": true, "TMPDIR": true, "LANG": true, "PWD": true, "SHLVL": true, "_": true, "OLDPWD": true}
	for _, l := range lines[1:] {
		k, _, _ := strings.Cut(l, "=")
		if !allowed[k] {
			t.Errorf("the helper sees %q", k)
		}
	}
	if strings.Contains(string(data), "hunter2") {
		t.Error("the secret reached the helper")
	}
}

func TestHelperMemoryHardKill(t *testing.T) {
	fresh(t)
	s := newSite(t)
	p := s.plugin("fat", `plugin := {version: "1.0.0", blocks: ["x"], timeout: "2s"}`+"\n"+"if hook == \"block\" {\n if body == \"spin\" { for { } }\n out = \"ok\"\n}")
	defer scriptplugins.SetRSSLimit(1, 20*time.Millisecond)() // any helper is over the limit
	var err error
	d := timed(func() { _, err = block(p, "x", "spin", nil) })
	errContains(t, err, "memory")
	if d > time.Second {
		t.Errorf("the hard kill took %v", d)
	}
	scriptplugins.SetRSSLimit(1<<30, time.Second)
	if out := mustBlock(t, p, "x", "go", nil); out != "ok" {
		t.Errorf("after the kill: %q", out)
	}
	if _, err = block(p, "x", "spin", nil); err == nil {
		t.Error("no error")
	}
}

func TestHelpersAreGoneAfterResetAndShutdown(t *testing.T) {
	fresh(t)
	s := newSite(t)
	p := s.plugin("a", withHeader(`if hook == "block" { out = "a" }`))
	mustBlock(t, p, "x", "", nil)
	if scriptplugins.HelperCount() == 0 {
		t.Fatal("no helper is running after a call")
	}
	scriptplugins.Reset() // a folder change
	if n := scriptplugins.HelperCount(); n != 0 {
		t.Errorf("%d helpers after Reset", n)
	}
	if out := mustBlock(t, p, "x", "", nil); out != "a" {
		t.Errorf("after Reset: %q", out)
	}
	scriptplugins.Shutdown() // quit
	if n := scriptplugins.HelperCount(); n != 0 {
		t.Errorf("%d helpers after Shutdown", n)
	}
}

func TestResetRetiresABusyHelper(t *testing.T) {
	fresh(t)
	s := newSite(t)
	p := s.plugin("busy", `plugin := {version: "1.0.0", blocks: ["x"], timeout: "1s"}`+"\n"+"if hook == \"block\" {\n for i := 0; i < 3000000; i++ { }\n out = \"done\"\n}")
	done := make(chan string, 1)
	go func() { out, _ := block(p, "x", "", nil); done <- out }()
	time.Sleep(30 * time.Millisecond)
	scriptplugins.Reset()
	<-done
	time.Sleep(50 * time.Millisecond)
	if n := scriptplugins.HelperCount(); n != 0 {
		t.Errorf("%d helpers after a busy one was retired", n)
	}
}

func TestHelperConcurrentCallsAcrossThePool(t *testing.T) {
	fresh(t)
	s := newSite(t)
	st := &memStore{}
	p := s.plugin("many", withHeader(`if hook == "block" {
    zen.storage.set("k"+body, body)
    out = zen.storage.get("k"+body) + ":" + string(len(zen.storage.keys("k")) > 0)
}`), scriptplugins.Runtime{Storage: st})
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := fmt.Sprint(i)
			out, err := block(p, "x", body, nil)
			if err != nil || out != body+":true" {
				t.Errorf("call %d: %q, %v", i, out, err)
			}
		}(i)
	}
	wg.Wait()
	if n := scriptplugins.HelperCount(); n < 1 || n > 4 {
		t.Errorf("%d helpers after a burst (want 1 to 4)", n)
	}
}

func TestNestedMarkdownDoesNotStarveASmallPool(t *testing.T) {
	fresh(t)
	scriptplugins.SetPoolSize(1)
	defer scriptplugins.SetPoolSize(2)
	s := newSite(t)
	loop := s.plugin("loop", `plugin := {version: "1.0.0", blocks: ["loop"]}
if hook == "block" { out = zen.html("div", {class: "loop"}, zen.markdown(":::loop\n" + body + "\n:::")) }`)
	var pg = renderPage(t, loopOnly(loop), ":::loop\nx\n:::\n")
	if depth := strings.Count(pg.HTML, `<div class="loop">`); depth < 5 {
		t.Errorf("only %d levels rendered with a pool of one: %v", depth, pg.Warnings)
	}
}

func loopOnly(p *scriptplugins.Plugin) []*scriptplugins.Plugin { return []*scriptplugins.Plugin{p} }

func BenchmarkHelperCallWarm(b *testing.B) {
	s := &site{t: nil, dir: b.TempDir()}
	path := writeFile(b, s.dir, ".zen/plugins/note.tengo", header+`if hook == "block" { out = zen.html("div", {class: "note"}, zen.escape(body)) }`)
	p, err := scriptplugins.Load(path)
	if err != nil {
		b.Fatal(err)
	}
	p.Bind(scriptplugins.Runtime{Workspace: s.dir})
	if _, err := block(p, "x", "warm", nil); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := block(p, "x", "hello", nil); err != nil {
			b.Fatal(err)
		}
	}
}

var _ = io.EOF
