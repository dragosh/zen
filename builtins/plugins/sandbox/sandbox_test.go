package sandbox

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zenmark-org/zenmd/engine/plugins"
)

// fake writes a fake `container` script into a temp dir and points the plugin
// at it. runBody is the shell run for `container run …`; system status exits 0
// unless the file "stopped" exists. Every call is appended to calls.log.
func fake(t *testing.T, runBody string) string {
	t.Helper()
	d := t.TempDir()
	script := `#!/bin/sh
d="$(dirname "$0")"
echo "$*" >> "$d/calls.log"
case "$1" in
system)
  if [ -f "$d/stopped" ]; then echo "apiserver is not running" >&2; exit 1; fi
  echo running; exit 0;;
kill)
  printf '%s\n' "$@" > "$d/kill.args"; exit 0;;
run)
  printf '%s\n' "$@" > "$d/run.args"
  prev=""; host=""
  for a in "$@"; do if [ "$prev" = "-v" ]; then host="${a%%:*}"; fi; prev="$a"; done
  echo "$host" > "$d/host.txt"
  mkdir -p "$d/mounted"; cp "$host"/* "$d/mounted/" 2>/dev/null
` + runBody + `
  ;;
esac
`
	p := filepath.Join(d, "container")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	old := ContainerPath
	ContainerPath = p
	t.Cleanup(func() { ContainerPath = old })
	return d
}

func cfg(extra map[string]any) plugins.Context {
	c := map[string]any{"images": map[string]string{"python": "python:3.12-slim", "node": "node:22-alpine"}}
	for k, v := range extra {
		c[k] = v
	}
	return plugins.Context{Config: c}
}

type chunk struct {
	S        string `json:"s"`
	T        string `json:"t"`
	Code     int    `json:"code"`
	MS       int64  `json:"ms"`
	TimedOut bool   `json:"timedOut"`
}

func runStream(t *testing.T, c context.Context, ctx plugins.Context, payload string) ([]chunk, error) {
	t.Helper()
	var out []chunk
	err := New().(plugins.StreamHook).Stream(c, ctx, []byte(payload), func(b []byte) {
		var ch chunk
		if e := json.Unmarshal(b, &ch); e != nil {
			t.Fatalf("chunk %q is not JSON: %v", b, e)
		}
		out = append(out, ch)
	})
	return out, err
}

func lines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

const goodPayload = `{"op":"run","image":"python","cmd":"python main.py","file":"main.py","code":"print('hi')\n"}`

func TestManifest(t *testing.T) {
	m := New().Manifest()
	if m.ID != "sandbox" || !m.Default || m.APIVersion != plugins.APIVersion || m.Version == "" {
		t.Fatalf("manifest %+v", m)
	}
	if strings.Join(m.Blocks, ",") != "sandbox" {
		t.Errorf("blocks %v", m.Blocks)
	}
	want := map[string]string{"images": "map", "timeout": "int", "network": "bool", "memory": "string", "cpus": "int"}
	for k, typ := range want {
		o, ok := m.Config[k]
		if !ok || o.Type != typ || o.Scope != plugins.ScopeUser {
			t.Errorf("option %s: %+v", k, o)
		}
	}
	if m.Config["timeout"].Default != 30 || m.Config["network"].Default != false || m.Config["memory"].Default != "512M" || m.Config["cpus"].Default != 1 {
		t.Errorf("defaults %+v", m.Config)
	}
	p := New()
	if _, ok := p.(plugins.BlockHook); !ok {
		t.Error("not a BlockHook")
	}
	if _, ok := p.(plugins.ActionHook); !ok {
		t.Error("not an ActionHook")
	}
	if _, ok := p.(plugins.StreamHook); !ok {
		t.Error("not a StreamHook")
	}
	ap, ok := p.(plugins.AssetProvider)
	if !ok {
		t.Fatal("not an AssetProvider")
	}
	e := ap.Entry()
	if strings.Join(e.JS, ",") != "sandbox.js" || strings.Join(e.CSS, ",") != "sandbox.css" {
		t.Errorf("entry %+v", e)
	}
	for _, f := range []string{"sandbox.js", "sandbox.css"} {
		if _, err := fs.Stat(ap.Assets(), f); err != nil {
			t.Errorf("asset %s: %v", f, err)
		}
	}
}

func TestRenderBlockReady(t *testing.T) {
	h := New().(plugins.BlockHook)
	out, err := h.RenderBlock(cfg(nil), plugins.Block{Name: "sandbox", Fenced: true, Body: "print('<b>&')", Attrs: map[string]string{"image": "python"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "zen-sandbox-ready") || strings.Contains(out, "zen-sandbox-setup") {
		t.Errorf("not ready: %s", out)
	}
	if !strings.Contains(out, `<pre class="zen-sandbox-code">print(&#39;&lt;b&gt;&amp;&#39;)</pre>`) || strings.Contains(out, "<b>") {
		t.Errorf("code not escaped: %s", out)
	}
	// the images map may arrive as map[string]any
	out, _ = h.RenderBlock(plugins.Context{Config: map[string]any{"images": map[string]any{"python": "p"}}}, plugins.Block{Body: "x", Attrs: map[string]string{"image": "python"}})
	if !strings.Contains(out, "zen-sandbox-ready") {
		t.Errorf("map[string]any images: %s", out)
	}
}

func TestRenderBlockNotConfigured(t *testing.T) {
	h := New().(plugins.BlockHook)
	for name, c := range map[string]plugins.Context{"nil": {}, "empty": {Config: map[string]any{"images": map[string]string{}}}} {
		out, err := h.RenderBlock(c, plugins.Block{Body: "x<y", Attrs: map[string]string{"image": "python"}})
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"zen-sandbox-setup", "plugins.sandbox.images", "config.yml", "python: ", "x&lt;y"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: missing %q in %s", name, want, out)
			}
		}
		if strings.Contains(out, "zen-sandbox-ready") {
			t.Errorf("%s: ready", name)
		}
	}
}

func TestRenderBlockUnknownKey(t *testing.T) {
	out, _ := New().(plugins.BlockHook).RenderBlock(cfg(nil), plugins.Block{Body: "x", Attrs: map[string]string{"image": `evil"><script>`}})
	if !strings.Contains(out, "zen-sandbox-setup") || strings.Contains(out, "<script>") {
		t.Errorf("unknown key: %s", out)
	}
	if !strings.Contains(out, "node, python") || !strings.Contains(out, "plugins.sandbox.images") {
		t.Errorf("message does not name the keys and the setting: %s", out)
	}
	out, _ = New().(plugins.BlockHook).RenderBlock(cfg(nil), plugins.Block{Body: "x", Attrs: map[string]string{}})
	if !strings.Contains(out, "zen-sandbox-setup") {
		t.Errorf("missing image attr: %s", out)
	}
}

func TestActionStatus(t *testing.T) {
	a := New().(plugins.ActionHook)
	status := func(ctx plugins.Context) (string, []string) {
		b, err := a.Action(ctx, []byte(`{"op":"status"}`))
		if err != nil {
			t.Fatal(err)
		}
		var r struct {
			Service string   `json:"service"`
			Images  []string `json:"images"`
		}
		if err := json.Unmarshal(b, &r); err != nil {
			t.Fatal(err)
		}
		return r.Service, r.Images
	}
	d := fake(t, "")
	if s, imgs := status(cfg(nil)); s != "running" || strings.Join(imgs, ",") != "node,python" {
		t.Errorf("running: %s %v", s, imgs)
	}
	os.WriteFile(filepath.Join(d, "stopped"), nil, 0o644)
	if s, _ := status(cfg(nil)); s != "stopped" {
		t.Errorf("stopped: %s", s)
	}
	ContainerPath = filepath.Join(d, "does-not-exist")
	if s, imgs := status(plugins.Context{}); s != "missing" || len(imgs) != 0 {
		t.Errorf("missing: %s %v", s, imgs)
	}
	if _, err := a.Action(cfg(nil), []byte(`{"op":"start"}`)); err == nil {
		t.Error("unknown op accepted")
	}
	if b := lines(t, filepath.Join(d, "calls.log")); strings.Contains(strings.Join(b, "|"), "system start") {
		t.Error("the plugin started the service")
	}
}

func TestStreamRun(t *testing.T) {
	d := fake(t, `echo "hello"; echo "oops" >&2; echo ""; exit 3`)
	out, err := runStream(t, context.Background(), cfg(map[string]any{"cpus": 2, "memory": "256M"}), goodPayload)
	if err != nil {
		t.Fatal(err)
	}
	var o, e []string
	var exit *chunk
	for i := range out {
		switch out[i].S {
		case "out":
			o = append(o, out[i].T)
		case "err":
			e = append(e, out[i].T)
		case "exit":
			exit = &out[i]
		}
	}
	if strings.Join(o, "|") != "hello|" || strings.Join(e, "|") != "oops" {
		t.Errorf("chunks %+v", out)
	}
	if exit == nil || exit.Code != 3 || exit.TimedOut || out[len(out)-1].S != "exit" {
		t.Errorf("exit chunk %+v (last must be exit)", out)
	}
	args := lines(t, filepath.Join(d, "run.args"))
	host := lines(t, filepath.Join(d, "host.txt"))[0]
	if len(args) < 3 || args[0] != "run" || !strings.HasPrefix(args[3], "zen-") {
		t.Fatalf("args %v", args)
	}
	name := args[3]
	want := []string{"run", "--rm", "--name", name, "--network", "none", "--no-dns", "--read-only", "--progress", "none", "-c", "2", "-m", "256M", "-v", host + ":/work", "-w", "/work", "python:3.12-slim", "sh", "-c", "python main.py"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("args\n got %q\nwant %q", args, want)
	}
	if b, _ := os.ReadFile(filepath.Join(d, "mounted", "main.py")); string(b) != "print('hi')\n" {
		t.Errorf("mounted file %q", b)
	}
	if exists(host) {
		t.Error("temp dir not removed")
	}
	if strings.Contains(strings.Join(lines(t, filepath.Join(d, "calls.log")), "|"), "system start") {
		t.Error("started the service")
	}
}

func TestStreamNetwork(t *testing.T) {
	d := fake(t, "exit 0")
	if _, err := runStream(t, context.Background(), cfg(map[string]any{"network": true}), goodPayload); err != nil {
		t.Fatal(err)
	}
	args := strings.Join(lines(t, filepath.Join(d, "run.args")), " ")
	if strings.Contains(args, "--network") || strings.Contains(args, "--no-dns") {
		t.Errorf("network on, args %s", args)
	}
	if !strings.Contains(args, "--read-only") || !strings.Contains(args, "-c 1 -m 512M") {
		t.Errorf("defaults missing: %s", args)
	}
}

func TestStreamValidation(t *testing.T) {
	d := fake(t, "exit 0")
	long := strings.Repeat("x", 2001)
	big := strings.Repeat("x", 1<<20+1)
	bad := map[string]string{
		"unknown key": `{"op":"run","image":"ruby","cmd":"x","file":"a","code":""}`,
		"no key":      `{"op":"run","cmd":"x","file":"a","code":""}`,
		"path file":   `{"op":"run","image":"python","cmd":"x","file":"../a.py","code":""}`,
		"slash file":  `{"op":"run","image":"python","cmd":"x","file":"a/b.py","code":""}`,
		"dot file":    `{"op":"run","image":"python","cmd":"x","file":"..","code":""}`,
		"hidden file": `{"op":"run","image":"python","cmd":"x","file":".x","code":""}`,
		"long cmd":    `{"op":"run","image":"python","cmd":"` + long + `","file":"a","code":""}`,
		"empty cmd":   `{"op":"run","image":"python","cmd":"","file":"a","code":""}`,
		"big code":    `{"op":"run","image":"python","cmd":"x","file":"a","code":"` + big + `"}`,
		"unknown op":  `{"op":"exec","image":"python"}`,
		"not json":    `nope`,
	}
	for name, p := range bad {
		out, err := runStream(t, context.Background(), cfg(nil), p)
		if err == nil {
			t.Errorf("%s: accepted (%v)", name, out)
		}
	}
	// an image reference that looks like a flag is refused
	_, err := runStream(t, context.Background(), cfg(map[string]any{"images": map[string]string{"x": "--privileged"}}), `{"op":"run","image":"x","cmd":"x","file":"a","code":""}`)
	if err == nil {
		t.Error("flag-like image ref accepted")
	}
	if exists(filepath.Join(d, "run.args")) {
		t.Error("container run was invoked for invalid input")
	}
}

func TestStreamServiceStopped(t *testing.T) {
	d := fake(t, "exit 0")
	os.WriteFile(filepath.Join(d, "stopped"), nil, 0o644)
	_, err := runStream(t, context.Background(), cfg(nil), goodPayload)
	if err == nil || !strings.Contains(err.Error(), "container system start") {
		t.Fatalf("err %v", err)
	}
	if exists(filepath.Join(d, "run.args")) {
		t.Error("ran with the service stopped")
	}
	if strings.Contains(strings.Join(lines(t, filepath.Join(d, "calls.log")), "|"), "system start") {
		t.Error("started the service")
	}
}

func TestStreamToolMissing(t *testing.T) {
	old := ContainerPath
	ContainerPath = filepath.Join(t.TempDir(), "nope")
	t.Cleanup(func() { ContainerPath = old })
	_, err := runStream(t, context.Background(), cfg(nil), goodPayload)
	if err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("err %v", err)
	}
}

func TestStreamTimeout(t *testing.T) {
	d := fake(t, `echo started; exec sleep 30`)
	start := time.Now()
	out, err := runStream(t, context.Background(), cfg(map[string]any{"timeout": 1}), goodPayload)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 10*time.Second {
		t.Error("timeout not enforced")
	}
	last := out[len(out)-1]
	if last.S != "exit" || !last.TimedOut {
		t.Errorf("last chunk %+v", last)
	}
	args := lines(t, filepath.Join(d, "run.args"))
	k := lines(t, filepath.Join(d, "kill.args"))
	if len(k) != 2 || k[0] != "kill" || k[1] != args[3] {
		t.Errorf("kill args %v (name %s)", k, args[3])
	}
	if exists(lines(t, filepath.Join(d, "host.txt"))[0]) {
		t.Error("temp dir not removed")
	}
}

func TestStreamCancel(t *testing.T) {
	d := fake(t, `echo started; exec sleep 30`)
	c, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(500 * time.Millisecond); cancel() }()
	_, err := runStream(t, c, cfg(nil), goodPayload)
	if err == nil {
		t.Fatal("cancel returned nil")
	}
	if !exists(filepath.Join(d, "kill.args")) {
		t.Error("container kill not run")
	}
	if exists(lines(t, filepath.Join(d, "host.txt"))[0]) {
		t.Error("temp dir not removed")
	}
}

func TestStreamLongLine(t *testing.T) {
	fake(t, `head -c 200000 /dev/zero | tr '\0' 'a'; echo`)
	out, err := runStream(t, context.Background(), cfg(nil), goodPayload)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, c := range out {
		if c.S == "out" {
			total += len(c.T)
		}
	}
	if total != 200000 {
		t.Errorf("lost output: %d", total)
	}
}
