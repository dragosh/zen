package extplugins

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zenmark-org/zenmd/engine/plugins"
)

type chunks struct {
	mu sync.Mutex
	v  []string
}

func (c *chunks) emit(b []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.v = append(c.v, string(b))
}
func (c *chunks) get() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.v...)
}

func TestStreamChunksInOrder(t *testing.T) {
	p, _, _ := scripted(t, `{ command: ["./run.sh"], hooks: [stream] }`,
		`echo "hook=$ZEN_HOOK in=$(cat)"; echo two; printf 'three'`)
	var c chunks
	err := p.Stream(context.Background(), ctx, []byte(`{"a":1}`), c.emit)
	if err != nil {
		t.Fatal(err)
	}
	got := c.get()
	want := []string{`hook=stream in={"a":1}`, "two", "three"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("chunks %q, want %q", got, want)
	}
	var _ plugins.StreamHook = p
}

func TestStreamArrivesWhileRunning(t *testing.T) {
	p, _, _ := scripted(t, `{ command: ["./run.sh"], hooks: [stream] }`, `echo first; sleep 1; echo second`)
	first := make(chan time.Time, 1)
	done := make(chan time.Time, 1)
	go func() {
		_ = p.Stream(context.Background(), ctx, nil, func(b []byte) {
			if string(b) == "first" {
				first <- time.Now()
			}
		})
		done <- time.Now()
	}()
	f, d := <-first, <-done
	if d.Sub(f) < 700*time.Millisecond {
		t.Fatalf("first chunk arrived only %v before the end", d.Sub(f))
	}
}

func TestStreamErrors(t *testing.T) {
	p, _, _ := scripted(t, `{ command: ["./run.sh"], hooks: [stream] }`, `echo partial; echo boom >&2; exit 3`)
	var c chunks
	err := p.Stream(context.Background(), ctx, nil, c.emit)
	if err == nil || !strings.Contains(err.Error(), "exit code 3") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error: %v", err)
	}
	if got := c.get(); len(got) != 1 || got[0] != "partial" {
		t.Fatalf("chunks before the failure: %q", got)
	}
	// Without the declared hook it refuses.
	q, _, _ := scripted(t, `{ command: ["./run.sh"], hooks: [block] }`, `echo x`)
	if err := q.Stream(context.Background(), ctx, nil, func([]byte) {}); err == nil || !strings.Contains(err.Error(), "stream") {
		t.Fatalf("undeclared hook: %v", err)
	}
	// Not an action-only plugin either: plugins.StreamHookOf honours declarations.
	if _, ok := plugins.StreamHookOf(q); ok {
		t.Error("StreamHookOf found an undeclared hook")
	}
	if _, ok := plugins.StreamHookOf(p); !ok {
		t.Error("StreamHookOf missed the declared hook")
	}
}

func waitDead(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var pid int
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
			if pid > 1 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid <= 1 {
		t.Fatalf("no pid in %s", path)
	}
	for alive(pid) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if alive(pid) {
		t.Fatalf("process %d survived", pid)
	}
}

func TestStreamCancelKillsTheGroup(t *testing.T) {
	p, _, data := scripted(t, `{ command: ["./run.sh"], hooks: [stream] }`,
		`sleep 60 & echo $! > "$ZEN_DATA_DIR/child"; echo $$ > "$ZEN_DATA_DIR/parent"; echo started; wait`)
	c, cancel := context.WithCancel(context.Background())
	var ch chunks
	started := make(chan struct{})
	var once sync.Once
	res := make(chan error, 1)
	go func() {
		res <- p.Stream(c, ctx, nil, func(b []byte) { ch.emit(b); once.Do(func() { close(started) }) })
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("stream never started")
	}
	cancel()
	select {
	case err := <-res:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not end the stream")
	}
	waitDead(t, filepath.Join(data, "child"))
	waitDead(t, filepath.Join(data, "parent"))
}

func TestStreamTimeout(t *testing.T) {
	p, _, data := scripted(t, `{ command: ["./run.sh"], hooks: [stream], timeout: 400ms }`,
		`sleep 60 & echo $! > "$ZEN_DATA_DIR/child"; wait`)
	start := time.Now()
	err := p.Stream(context.Background(), ctx, nil, func([]byte) {})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("want timeout, got %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("too slow")
	}
	waitDead(t, filepath.Join(data, "child"))
}

func TestStreamOutputLimit(t *testing.T) {
	// 5 MB in 1 KB lines.
	p, _, _ := scripted(t, `{ command: ["./run.sh"], hooks: [stream], timeout: 30s }`,
		`head -c 5242880 /dev/zero | tr '\0' 'a' | fold -w 1023`)
	var c chunks
	err := p.Stream(context.Background(), ctx, nil, c.emit)
	if !errors.Is(err, plugins.ErrOutputLimit) {
		t.Fatalf("error %v, want output limit", err)
	}
	total := 0
	for _, s := range c.get() {
		total += len(s) + 1
	}
	if total > plugins.MaxStreamOutput || total < plugins.MaxStreamOutput-4096 {
		t.Fatalf("delivered %d bytes, want just under %d", total, plugins.MaxStreamOutput)
	}
	if plugins.MaxStreamOutput != 4<<20 || plugins.ErrOutputLimit.Error() != "output limit" {
		t.Fatal("limit constants changed")
	}
	// Exactly at the limit is fine.
	ok, _, _ := scripted(t, `{ command: ["./run.sh"], hooks: [stream], timeout: 30s }`,
		`head -c 4194303 /dev/zero | tr '\0' 'a'; echo`)
	if err := ok.Stream(context.Background(), ctx, nil, func([]byte) {}); err != nil {
		t.Fatalf("4 MB exactly: %v", err)
	}
}

func TestStreamEnvironmentAndData(t *testing.T) {
	p, ws, data := scripted(t, `{ command: ["./run.sh"], hooks: [stream] }`,
		`echo "$ZEN_PLUGIN|$ZEN_PAGE|$ZEN_WORKSPACE|$ZEN_DATA_DIR|$ZEN_CONFIG_LABEL"`)
	var c chunks
	if err := p.Stream(context.Background(), ctx, nil, c.emit); err != nil {
		t.Fatal(err)
	}
	want := "p|guides/install.md|" + ws + "|" + data + "|a b"
	if got := c.get(); len(got) != 1 || got[0] != want {
		t.Fatalf("env: %q, want %q", got, want)
	}
}

func TestMapAndListConfigInEnvironmentIsJSON(t *testing.T) {
	p, _, _ := scripted(t, `{ command: ["./run.sh"], hooks: [block] }`, `echo "$ZEN_CONFIG_IMAGES|$ZEN_CONFIG_TAGS"`)
	cx := plugins.Context{Path: "a.md", Config: map[string]any{"images": map[string]string{"py": "python:3"}, "tags": []string{"a", "b"}}}
	out, err := p.RenderBlock(cx, plugins.Block{Name: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != `{"py":"python:3"}|["a","b"]` {
		t.Fatalf("env: %q", out)
	}
}

func TestOtherHooksKeepTheir60sCap(t *testing.T) {
	p, _, _ := scripted(t, `{ command: ["./run.sh"], hooks: [stream, block], timeout: 10m }`, `echo hi`)
	if p.callTimeout() != maxTimeout {
		t.Fatalf("block timeout %v, want %v", p.callTimeout(), maxTimeout)
	}
}
