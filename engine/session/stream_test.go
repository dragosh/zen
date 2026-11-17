package session

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/plugins/codeblock"
	"github.com/dragosh/zen/internal/plugins/diagram"
)

// actor is a built-in plugin with an action and a stream hook.
type actor struct {
	id      string
	started chan struct{} // receives one value per stream that started waiting
	mu      sync.Mutex
	ctxs    []plugins.Context
}

func newActor(id string) *actor { return &actor{id: id, started: make(chan struct{}, 16)} }

func (a *actor) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID: a.id, Version: "0.1.0", APIVersion: plugins.APIVersion, Default: true,
		Config: map[string]plugins.Option{
			"images": {Type: "map", Scope: "user", Default: map[string]string{"py": "python:3"}},
			"label":  {Type: "string", Default: "hi"},
		},
	}
}

func (a *actor) Action(ctx plugins.Context, payload []byte) ([]byte, error) {
	if string(payload) == `"fail"` {
		return nil, errors.New("action failed")
	}
	return json.Marshal(map[string]any{
		"payload": json.RawMessage(payload), "data": ctx.DataDir, "workspace": ctx.Workspace,
		"page": ctx.Path, "config": ctx.Config,
	})
}

func (a *actor) Stream(c context.Context, ctx plugins.Context, payload []byte, emit func([]byte)) error {
	a.mu.Lock()
	a.ctxs = append(a.ctxs, ctx)
	a.mu.Unlock()
	switch string(payload) {
	case `"three"`:
		for _, s := range []string{"one", "two", "three"} {
			emit([]byte(s))
		}
		return nil
	case `"fail"`:
		emit([]byte("partial"))
		return errors.New("it broke")
	case `"wait"`:
		a.started <- struct{}{}
		<-c.Done()
		emit([]byte("late chunk after cancel"))
		return c.Err()
	case `"flood"`:
		big := []byte(strings.Repeat("x", 1023))
		for i := 0; i < 6000 && c.Err() == nil; i++ {
			emit(big)
		}
		return nil
	}
	return nil
}

// openActing opens dir with the built-ins plus extra, a global config.yml
// in cfgDir, and returns the service.
func openActing(t *testing.T, dir, cfgDir string, extra ...plugins.Plugin) (*Session, *sink) {
	t.Helper()
	reg := plugins.NewRegistry()
	for _, p := range append([]plugins.Plugin{codeblock.New(), diagram.New()}, extra...) {
		if err := reg.Register(p); err != nil {
			t.Fatal(err)
		}
	}
	ev := &sink{}
	z, err := newZen(zenOptions{
		Registry: reg, ConfigDir: cfgDir, GlobalPath: filepath.Join(cfgDir, "config.yml"),
		Dir: dir, Events: ev, Dialogs: &fakeDialogs{Allow: true}, Open: func(string) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	return z, ev
}

func streamEvents(ev *sink, call string) []PluginStreamEvent {
	var out []PluginStreamEvent
	for _, e := range ev.all() {
		if e.Name != EventPluginStream {
			continue
		}
		pe, ok := e.Data.(PluginStreamEvent)
		if !ok {
			continue
		}
		if call == "" || pe.Call == call {
			out = append(out, pe)
		}
	}
	return out
}

// finish waits for the done event of call and returns its events.
func finish(t *testing.T, ev *sink, call string) []PluginStreamEvent {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		es := streamEvents(ev, call)
		if n := len(es); n > 0 && es[n-1].Done {
			return es
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("stream %s did not finish: %+v", call, streamEvents(ev, call))
	return nil
}

// chunksOf is the lines of all chunk events, joined with "|": events are
// batches of lines joined with "\n".
func chunksOf(es []PluginStreamEvent) string {
	var c []string
	for _, e := range es {
		if !e.Done {
			c = append(c, strings.Split(e.Chunk, "\n")...)
		}
	}
	return strings.Join(c, "|")
}

func TestStreamEventShape(t *testing.T) {
	if EventPluginStream != "plugin:stream" {
		t.Fatal(EventPluginStream)
	}
	b, _ := json.Marshal(PluginStreamEvent{Call: "c", Chunk: "x"})
	if string(b) != `{"call":"c","chunk":"x","done":false,"error":""}` {
		t.Fatalf("json %s", b)
	}
}

func TestBuiltinStream(t *testing.T) {
	dir := pluginTree(t, pluginDocs())
	act := newActor("act")
	z, ev := openActing(t, dir, t.TempDir(), act)
	if _, err := z.Page("other.md"); err != nil {
		t.Fatal(err)
	}

	call, err := z.PluginStream("act", "other.md", `"three"`)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(call) {
		t.Fatalf("call id %q", call)
	}
	es := finish(t, ev, call)
	if chunksOf(es) != "one|two|three" {
		t.Fatalf("chunks: %+v", es)
	}
	last := es[len(es)-1]
	if !last.Done || last.Error != "" || last.Call != call || len(es) < 2 {
		t.Fatalf("end: %+v", es)
	}
	// The hook got the page, the folder and a data folder.
	act.mu.Lock()
	cx := act.ctxs[0]
	act.mu.Unlock()
	if cx.Path != "other.md" || cx.Workspace == "" || !strings.Contains(cx.DataDir, string(filepath.Separator)+"data"+string(filepath.Separator)) || filepath.Base(cx.DataDir) != "files" {
		t.Errorf("context: %+v", cx)
	}

	// Ids are random and distinct.
	c2, _ := z.PluginStream("act", "README.md", `"three"`)
	if c2 == call {
		t.Error("call ids repeat")
	}
	finish(t, ev, c2)

	// A returned error ends the stream with its text.
	c3, err := z.PluginStream("act", "README.md", `"fail"`)
	if err != nil {
		t.Fatal(err)
	}
	es = finish(t, ev, c3)
	if chunksOf(es) != "partial" || es[len(es)-1].Error != "it broke" || !es[len(es)-1].Done {
		t.Fatalf("failure: %+v", es)
	}
}

func TestStreamRefusals(t *testing.T) {
	files := pluginDocs()
	files[".zen.yml"] = "version: 1\nplugins:\n  off: false\n"
	dir := pluginTree(t, files)
	act, off := newActor("act"), newActor("off")
	z, ev := openActing(t, dir, t.TempDir(), act, off)

	for _, id := range []string{"codeblock", "diagram", "nope", "", "../act", "lint", "off", "hello"} {
		if _, err := z.PluginStream(id, "README.md", `1`); err == nil {
			t.Errorf("stream for %q accepted", id)
		}
	}
	if _, err := z.PluginStream("act", "README.md", `{bad`); err == nil {
		t.Error("payload that is not JSON accepted")
	}
	z2, _ := openActing(t, "", t.TempDir(), newActor("act"))
	if _, err := z2.PluginStream("act", "README.md", `1`); err == nil {
		t.Error("stream without a folder")
	}
	if n := len(streamEvents(ev, "")); n != 0 {
		t.Errorf("%d events from refused streams", n)
	}
}

func TestStreamCancel(t *testing.T) {
	dir := pluginTree(t, pluginDocs())
	act := newActor("act")
	z, ev := openActing(t, dir, t.TempDir(), act)
	call, err := z.PluginStream("act", "README.md", `"wait"`)
	if err != nil {
		t.Fatal(err)
	}
	<-act.started
	if err := z.PluginCancel(call); err != nil {
		t.Fatal(err)
	}
	es := finish(t, ev, call)
	last := es[len(es)-1]
	if !last.Done || last.Error != "cancelled" {
		t.Fatalf("end: %+v", last)
	}
	if chunksOf(es) != "" {
		t.Errorf("chunk delivered after cancel: %+v", es)
	}
	// A finished or unknown call is not an error.
	if err := z.PluginCancel(call); err != nil {
		t.Errorf("cancel of a finished call: %v", err)
	}
	if err := z.PluginCancel("nope"); err != nil {
		t.Errorf("cancel of an unknown call: %v", err)
	}
	if n := len(streamEvents(ev, call)); n != 1 {
		t.Errorf("%d events after cancel", n)
	}
}

func TestStreamAtMostFourAtOnce(t *testing.T) {
	dir := pluginTree(t, pluginDocs())
	act := newActor("act")
	z, ev := openActing(t, dir, t.TempDir(), act)
	var calls []string
	for i := 0; i < 4; i++ {
		c, err := z.PluginStream("act", "README.md", `"wait"`)
		if err != nil {
			t.Fatal(err)
		}
		calls = append(calls, c)
	}
	if _, err := z.PluginStream("act", "README.md", `"three"`); err == nil || !strings.Contains(err.Error(), "4") {
		t.Fatalf("fifth stream: %v", err)
	}
	// A slot frees up when one ends.
	_ = z.PluginCancel(calls[0])
	finish(t, ev, calls[0])
	c5, err := z.PluginStream("act", "README.md", `"three"`)
	if err != nil {
		t.Fatalf("after one ended: %v", err)
	}
	finish(t, ev, c5)
	for _, c := range calls[1:] {
		_ = z.PluginCancel(c)
		finish(t, ev, c)
	}
}

func TestStreamOutputLimit(t *testing.T) {
	dir := pluginTree(t, pluginDocs())
	z, ev := openActing(t, dir, t.TempDir(), newActor("act"))
	call, err := z.PluginStream("act", "README.md", `"flood"`)
	if err != nil {
		t.Fatal(err)
	}
	es := finish(t, ev, call)
	last := es[len(es)-1]
	if last.Error != "output limit" {
		t.Fatalf("end: %+v", last)
	}
	total := 0
	for _, e := range es {
		total += len(e.Chunk)
	}
	if total > plugins.MaxStreamOutput || total < plugins.MaxStreamOutput-2048 {
		t.Fatalf("delivered %d bytes", total)
	}
}

func TestStreamTimeoutOfBuiltins(t *testing.T) {
	old := builtinStreamTimeout
	builtinStreamTimeout = 150 * time.Millisecond
	t.Cleanup(func() { builtinStreamTimeout = old })
	dir := pluginTree(t, pluginDocs())
	act := newActor("act")
	z, ev := openActing(t, dir, t.TempDir(), act)
	call, err := z.PluginStream("act", "README.md", `"wait"`)
	if err != nil {
		t.Fatal(err)
	}
	es := finish(t, ev, call)
	if e := es[len(es)-1].Error; !strings.Contains(e, "timed out") {
		t.Fatalf("end: %+v", es[len(es)-1])
	}
}

func TestStreamsEndWhenTheFolderChangesOrTheAppQuits(t *testing.T) {
	dir := pluginTree(t, pluginDocs())
	act := newActor("act")
	z, ev := openActing(t, dir, t.TempDir(), act)
	call, err := z.PluginStream("act", "README.md", `"wait"`)
	if err != nil {
		t.Fatal(err)
	}
	<-act.started
	if err := z.Open(pluginTree(t, map[string]string{"README.md": "# Other\n"})); err != nil {
		t.Fatal(err)
	}
	es := finish(t, ev, call)
	if es[len(es)-1].Error != "cancelled" {
		t.Fatalf("folder change: %+v", es[len(es)-1])
	}

	act2 := newActor("act")
	z2, ev2 := openActing(t, dir, t.TempDir(), act2)
	call2, err := z2.PluginStream("act", "README.md", `"wait"`)
	if err != nil {
		t.Fatal(err)
	}
	<-act2.started
	z2.close()
	es = streamEvents(ev2, call2) // close waits for the streams to end
	if n := len(es); n == 0 || !es[n-1].Done || es[n-1].Error != "cancelled" {
		t.Fatalf("quit: %+v", es)
	}
}

// ---- external streams through the service ----

const streamYML = `id: tick
version: 0.1.0
apiVersion: 1
backend:
  command: ["./tick.sh"]
  hooks: [stream, action]
  timeout: 20s
`

const tickSH = `#!/bin/sh
case "$(cat)" in
  '"count"') echo 1; echo 2; echo 3;;
  '"fail"') echo oops >&2; exit 4;;
  '"hang"') sleep 60 & echo $! > "$ZEN_DATA_DIR/child"; echo up; wait;;
esac
`

func tickTree(t *testing.T) string {
	files := pluginDocs()
	files[".zen/plugins/tick/plugin.yml"] = streamYML
	files[".zen/plugins/tick/tick.sh"] = tickSH
	return pluginTree(t, files)
}

func TestExternalStream(t *testing.T) {
	dir := tickTree(t)
	z, ev, _ := openWith(t, dir, t.TempDir(), true)
	call, err := z.PluginStream("tick", "README.md", `"count"`)
	if err != nil {
		t.Fatal(err)
	}
	es := finish(t, ev, call)
	if chunksOf(es) != "1|2|3" || es[len(es)-1].Error != "" {
		t.Fatalf("events: %+v", es)
	}
	call, _ = z.PluginStream("tick", "README.md", `"fail"`)
	es = finish(t, ev, call)
	if e := es[len(es)-1].Error; !strings.Contains(e, "exit code 4") || !strings.Contains(e, "oops") || strings.Contains(e, dir) {
		t.Fatalf("failure: %+v", es[len(es)-1])
	}
	// hello declares no stream hook.
	if _, err := z.PluginStream("hello", "README.md", `1`); err == nil {
		t.Error("stream of a plugin without the hook")
	}
}

func TestExternalStreamCancelKillsTheProcess(t *testing.T) {
	dir := tickTree(t)
	cfg := t.TempDir()
	z, ev, _ := openWith(t, dir, cfg, true)
	call, err := z.PluginStream("tick", "README.md", `"hang"`)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(streamEvents(ev, call)) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if err := z.PluginCancel(call); err != nil {
		t.Fatal(err)
	}
	es := finish(t, ev, call)
	if es[len(es)-1].Error != "cancelled" {
		t.Fatalf("end: %+v", es[len(es)-1])
	}
	matches, _ := filepath.Glob(filepath.Join(cfg, "data", "*", "tick", "files", "child"))
	if len(matches) != 1 {
		t.Fatalf("child pid file: %v", matches)
	}
	b, _ := os.ReadFile(matches[0])
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	if pid <= 1 {
		t.Fatalf("pid %q", b)
	}
	deadline = time.Now().Add(3 * time.Second)
	for processAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if processAlive(pid) {
		t.Fatalf("child %d survived the cancel", pid)
	}
}

func TestExternalStreamBlockedOrOffIsRefused(t *testing.T) {
	dir := tickTree(t)
	z, _, _ := openWith(t, dir, t.TempDir(), true)
	if err := z.PluginTrust("tick", false); err != nil {
		t.Fatal(err)
	}
	if _, err := z.PluginStream("tick", "README.md", `"count"`); err == nil {
		t.Error("blocked plugin streamed")
	}
}

func TestStreamEventOrderAcrossCalls(t *testing.T) {
	dir := pluginTree(t, pluginDocs())
	z, ev := openActing(t, dir, t.TempDir(), newActor("act"))
	var calls []string
	for i := 0; i < 3; i++ {
		c, err := z.PluginStream("act", "README.md", `"three"`)
		if err != nil {
			t.Fatal(err)
		}
		calls = append(calls, c)
	}
	sort.Strings(calls)
	for _, c := range calls {
		if chunksOf(finish(t, ev, c)) != "one|two|three" {
			t.Fatalf("call %s out of order", c)
		}
	}
}

func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
