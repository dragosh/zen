package extplugins

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/dragosh/zenmd/engine/plugins"
)

// scripted loads a plugin "p" whose backend is run.sh with the given body.
func scripted(t *testing.T, backend, script string) (p *Plugin, workspace, data string) {
	t.Helper()
	yml := "id: p\nversion: 1.0.0\napiVersion: 1\nblocks: [p]\ninlines: [p]\n" +
		"options:\n  size: { type: int, default: 3 }\n  dark-mode: { type: bool, default: false }\n  label: { type: string, default: hi }\n" +
		"backend: " + backend + "\n"
	dir := writePlugin(t, t.TempDir(), "p", yml, map[string]string{"run.sh": "#!/bin/sh\n" + script + "\n"})
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	workspace = t.TempDir()
	data = filepath.Join(t.TempDir(), "data", "p", "files") // created on demand
	p.Bind(Runtime{Workspace: workspace, DataDir: data})
	return p, workspace, data
}

var ctx = plugins.Context{Path: "guides/install.md", Config: map[string]any{"size": 7, "dark-mode": true, "label": "a b"}}

func TestEnvironment(t *testing.T) {
	t.Setenv("ZEN_TEST_SECRET", "hunter2")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "hunter2")
	p, ws, data := scripted(t, `{ command: ["./run.sh"], hooks: [block, inline, source, html, action], output: text }`, `env; printf 'CWD=%s\n' "$(pwd -P)"`)
	out, err := p.RenderBlock(ctx, plugins.Block{Name: "p", Body: "x", Fenced: true,
		Attrs: map[string]string{"title": "Two words", "data-kind": "a$b", "x.y": "1"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, `<pre class="zen-plugin-output">`) || !strings.HasSuffix(out, "</pre>") {
		t.Fatalf("text output not wrapped: %q", out)
	}
	env := map[string]string{}
	body := strings.TrimSuffix(strings.TrimPrefix(out, `<pre class="zen-plugin-output">`), "</pre>")
	for _, line := range strings.Split(body, "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			env[k] = strings.NewReplacer("&#34;", `"`, "&amp;", "&", "&lt;", "<", "&gt;", ">", "&#39;", "'").Replace(v)
		}
	}
	want := map[string]string{
		"ZEN_HOOK": "block", "ZEN_PLUGIN": "p", "ZEN_API": "1", "ZEN_NAME": "p", "ZEN_FENCED": "1",
		"ZEN_PAGE": "guides/install.md", "ZEN_ATTR_TITLE": "Two words", "ZEN_ATTR_DATA_KIND": "a$b", "ZEN_ATTR_X_Y": "1",
		"ZEN_CONFIG_SIZE": "7", "ZEN_CONFIG_DARK_MODE": "true", "ZEN_CONFIG_LABEL": "a b",
		"ZEN_WORKSPACE": ws, "ZEN_PLUGIN_DIR": p.Dir(), "ZEN_DATA_DIR": data, "CWD": p.Dir(),
	}
	for k, v := range want {
		if env[k] != v {
			t.Errorf("%s = %q, want %q", k, env[k], v)
		}
	}
	var attrs map[string]string
	if err := json.Unmarshal([]byte(env["ZEN_ATTRS_JSON"]), &attrs); err != nil || attrs["title"] != "Two words" || len(attrs) != 3 {
		t.Errorf("ZEN_ATTRS_JSON = %q (%v)", env["ZEN_ATTRS_JSON"], err)
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(env["ZEN_CONFIG_JSON"]), &cfg); err != nil || cfg["size"] != float64(7) || cfg["dark-mode"] != true {
		t.Errorf("ZEN_CONFIG_JSON = %q (%v)", env["ZEN_CONFIG_JSON"], err)
	}
	if fi, err := os.Stat(data); err != nil || !fi.IsDir() {
		t.Errorf("data dir not created: %v", err)
	}
	// Nothing else: only PATH, HOME, TMPDIR, LANG and what sh adds itself.
	allowed := map[string]bool{"PATH": true, "HOME": true, "TMPDIR": true, "LANG": true, "PWD": true, "OLDPWD": true, "SHLVL": true, "_": true, "CWD": true}
	var extra []string
	for k := range env {
		if !allowed[k] && !strings.HasPrefix(k, "ZEN_") {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	if len(extra) > 0 || env["ZEN_TEST_SECRET"] != "" {
		t.Errorf("environment leaks: %v", extra)
	}
	for _, k := range []string{"PATH", "HOME", "TMPDIR", "LANG"} {
		if env[k] == "" {
			t.Errorf("%s is empty", k)
		}
	}

	// A container is ZEN_FENCED=0; an inline has a name and no ZEN_FENCED.
	out, _ = p.RenderBlock(ctx, plugins.Block{Name: "p", Body: "x"})
	if !strings.Contains(out, "ZEN_FENCED=0\n") {
		t.Error("container must be ZEN_FENCED=0")
	}
	out, err = p.RenderInline(ctx, plugins.Inline{Name: "p", Text: "x", Attrs: map[string]string{"color": "gray"}})
	if err != nil || !strings.Contains(out, "ZEN_HOOK=inline\n") || !strings.Contains(out, "ZEN_ATTR_COLOR=gray\n") || strings.Contains(out, "ZEN_FENCED") || !strings.Contains(out, "ZEN_NAME=p\n") {
		t.Errorf("inline env: %q %v", out, err)
	}
	if strings.Contains(out, "<pre") {
		t.Errorf("inline text output must not be a <pre>: %q", out)
	}
}

func TestHooksStdinStdout(t *testing.T) {
	// Prefixes stdin with the hook name; html output is passed through as is.
	p, _, _ := scripted(t, `{ command: ["./run.sh"], hooks: [block, inline, source, html, action] }`, `printf '%s:' "$ZEN_HOOK"; cat`)
	if out, err := p.RenderBlock(ctx, plugins.Block{Name: "p", Body: "<b>body</b>\nline 2\n"}); err != nil || out != "block:<b>body</b>\nline 2\n" {
		t.Errorf("block: %q %v", out, err)
	}
	if out, err := p.RenderInline(ctx, plugins.Inline{Name: "p", Text: "text"}); err != nil || out != "inline:text" {
		t.Errorf("inline: %q %v", out, err)
	}
	if out, err := p.Source(ctx, []byte("# md\n")); err != nil || string(out) != "source:# md\n" {
		t.Errorf("source: %q %v", out, err)
	}
	if out, err := p.HTML(ctx, "<p>x</p>"); err != nil || out != "html:<p>x</p>" {
		t.Errorf("html: %q %v", out, err)
	}

	// Action: JSON in, JSON out.
	a, _, _ := scripted(t, `{ command: ["./run.sh"], hooks: [action] }`, `printf '{"hook":"%s","got":' "$ZEN_HOOK"; cat; printf '}'`)
	out, err := a.Action(ctx, []byte(`{"n":1}`))
	if err != nil || string(out) != `{"hook":"action","got":{"n":1}}` {
		t.Errorf("action: %q %v", out, err)
	}
	// A hook that is not declared is refused without running anything.
	if _, err := a.RenderBlock(ctx, plugins.Block{Name: "p"}); err == nil {
		t.Error("undeclared block hook ran")
	}
	if _, err := p.Action(plugins.Context{}, nil); err != nil {
		t.Errorf("declared action: %v", err)
	}
	b, _, _ := scripted(t, `{ command: ["./run.sh"], hooks: [block] }`, `cat`)
	if _, err := b.Action(ctx, []byte(`1`)); err == nil {
		t.Error("undeclared action hook ran")
	}
}

func TestTextOutputIsEscaped(t *testing.T) {
	p, _, _ := scripted(t, `{ command: ["./run.sh"], hooks: [block, inline], output: text }`, `cat`)
	out, err := p.RenderBlock(ctx, plugins.Block{Name: "p", Body: `<script>alert(1)</script> & "q"`})
	if err != nil || strings.Contains(out, "<script") || !strings.Contains(out, "&lt;script&gt;") || !strings.Contains(out, "&amp;") {
		t.Fatalf("block: %q %v", out, err)
	}
	out, err = p.RenderInline(ctx, plugins.Inline{Name: "p", Text: "<i>"})
	if err != nil || out != "&lt;i&gt;" {
		t.Fatalf("inline: %q %v", out, err)
	}
}

func TestExitCode(t *testing.T) {
	p, _, _ := scripted(t, `{ command: ["./run.sh"], hooks: [block] }`, `echo partial; echo "bad <input>" >&2; echo second >&2; exit 3`)
	out, err := p.RenderBlock(ctx, plugins.Block{Name: "p"})
	if err == nil || out != "" {
		t.Fatalf("non-zero exit must fail: %q", out)
	}
	if !strings.Contains(err.Error(), "bad <input>") || !strings.Contains(err.Error(), "3") {
		t.Fatalf("error should carry the exit code and stderr: %q", err)
	}
	// The renderer shows it in an error box, escaped.
	box := plugins.ErrorBox("p", err)
	if strings.Contains(box, "<input>") || !strings.Contains(box, "bad &lt;input&gt;") {
		t.Fatalf("box: %q", box)
	}

	// A program that is not installed fails the same way.
	dir := writePlugin(t, t.TempDir(), "tool", "id: tool\nversion: 1.0.0\napiVersion: 1\nblocks: [tool]\nbackend: { command: [\"zen-no-such-program-xyz\"], hooks: [block] }\n", nil)
	tool, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tool.RenderBlock(ctx, plugins.Block{Name: "tool"}); err == nil || !strings.Contains(err.Error(), "zen-no-such-program-xyz") {
		t.Fatalf("missing program: %v", err)
	}
}

func TestToolOnPath(t *testing.T) {
	dir := writePlugin(t, t.TempDir(), "upper", "id: upper\nversion: 1.0.0\napiVersion: 1\nblocks: [upper]\nbackend: { command: [\"tr\", \"a-z\", \"A-Z\"], hooks: [block], output: text }\n", nil)
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.RenderBlock(ctx, plugins.Block{Name: "upper", Body: "shout"})
	if err != nil || out != `<pre class="zen-plugin-output">SHOUT</pre>` {
		t.Fatalf("%q %v", out, err)
	}
}

func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func TestTimeoutKillsProcessGroup(t *testing.T) {
	p, _, data := scripted(t, `{ command: ["./run.sh"], hooks: [block], timeout: 400ms }`,
		`sleep 60 & echo $! > "$ZEN_DATA_DIR/child"; echo $$ > "$ZEN_DATA_DIR/parent"; wait`)
	start := time.Now()
	_, err := p.RenderBlock(ctx, plugins.Block{Name: "p"})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("want a timeout error, got %v", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("returned after %v", d)
	}
	for _, name := range []string{"child", "parent"} {
		b, err := os.ReadFile(filepath.Join(data, name))
		if err != nil {
			t.Fatal(err)
		}
		pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
		if pid <= 1 {
			t.Fatalf("bad pid %q", b)
		}
		deadline := time.Now().Add(3 * time.Second)
		for alive(pid) && time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
		}
		if alive(pid) {
			syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("%s process %d survived the timeout", name, pid)
		}
	}
}

func TestBackgroundChildDoesNotHang(t *testing.T) {
	// The script exits at once but leaves a child holding stdout open.
	p, _, _ := scripted(t, `{ command: ["./run.sh"], hooks: [block], timeout: 10s }`, `echo done; sleep 60 &`)
	start := time.Now()
	out, err := p.RenderBlock(ctx, plugins.Block{Name: "p"})
	if err != nil || out != "done\n" {
		t.Fatalf("%q %v", out, err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("waited %v for a background child", d)
	}
}

func TestOutputCaps(t *testing.T) {
	p, _, _ := scripted(t, `{ command: ["./run.sh"], hooks: [block] }`, `head -c 3000000 /dev/zero | tr '\0' 'a'`)
	out, err := p.RenderBlock(ctx, plugins.Block{Name: "p"})
	if err == nil || out != "" || !strings.Contains(err.Error(), "2 MB") {
		t.Fatalf("stdout over the cap must fail: %d bytes, %v", len(out), err)
	}
	// Exactly at the cap is fine.
	ok, _, _ := scripted(t, `{ command: ["./run.sh"], hooks: [block] }`, `head -c 2097152 /dev/zero | tr '\0' 'a'`)
	if out, err := ok.RenderBlock(ctx, plugins.Block{Name: "p"}); err != nil || len(out) != 2<<20 {
		t.Fatalf("2 MB exactly: %d bytes, %v", len(out), err)
	}
	// stderr is cut; the error stays short.
	noisy, _, _ := scripted(t, `{ command: ["./run.sh"], hooks: [block] }`, `echo first line >&2; head -c 300000 /dev/zero | tr '\0' 'e' >&2; exit 1`)
	_, err = noisy.RenderBlock(ctx, plugins.Block{Name: "p"})
	if err == nil || !strings.Contains(err.Error(), "first line") || len(err.Error()) > 1024 {
		t.Fatalf("stderr: %d bytes: %.80q", len(err.Error()), err)
	}
	if maxStderr != 16<<10 || maxStdout != 2<<20 {
		t.Fatal("caps changed")
	}
}

func TestAtMostFourAtOnce(t *testing.T) {
	// Each run adds a marker, reports how many markers exist, waits, removes it.
	p, _, _ := scripted(t, `{ command: ["./run.sh"], hooks: [block] }`,
		`mkdir "$ZEN_DATA_DIR/run.$$"; ls "$ZEN_DATA_DIR" | wc -l; sleep 0.3; rmdir "$ZEN_DATA_DIR/run.$$"`)
	var wg sync.WaitGroup
	counts := make([]int, 12)
	for i := range counts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := p.RenderBlock(ctx, plugins.Block{Name: "p"})
			if err != nil {
				t.Error(err)
				return
			}
			counts[i], _ = strconv.Atoi(strings.TrimSpace(out))
		}()
	}
	wg.Wait()
	max := 0
	for _, c := range counts {
		if c > max {
			max = c
		}
	}
	if max > 4 {
		t.Fatalf("%d backends ran at once (limit 4): %v", max, counts)
	}
	if max < 2 {
		t.Fatalf("backends never ran in parallel: %v", counts)
	}
}

func TestNoShellInjection(t *testing.T) {
	// Arguments, attributes and the body reach the program as data.
	yml := "id: p\nversion: 1.0.0\napiVersion: 1\nblocks: [p]\n" +
		"backend: { command: [\"./run.sh\", \"a b; touch $ZEN_WORKSPACE/pwned-arg\", \"$(touch pwned-sub)\"], hooks: [block], output: text }\n"
	dir := writePlugin(t, t.TempDir(), "p", yml, map[string]string{
		"run.sh": "#!/bin/sh\nprintf 'ARGS=%s\\n' \"$#\"\nprintf 'A1=%s\\n' \"$1\"\nprintf 'T=%s\\n' \"$ZEN_ATTR_TITLE\"\ncat\n"})
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	ws := t.TempDir()
	p.Bind(Runtime{Workspace: ws, DataDir: filepath.Join(t.TempDir(), "d")})
	evil := "`touch " + ws + "/pwned-attr`; $(touch " + ws + "/pwned-attr2) \"' && rm -rf x"
	out, err := p.RenderBlock(ctx, plugins.Block{Name: "p", Body: "$(touch " + ws + "/pwned-body)\n", Attrs: map[string]string{"title": evil, "bad key\n=x": "v"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ARGS=2\n", "A1=a b; touch $ZEN_WORKSPACE/pwned-arg\n", "$(touch " + ws + "/pwned-body)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
	entries, _ := os.ReadDir(ws)
	if len(entries) != 0 {
		t.Fatalf("something was executed: %v", entries)
	}
	if es, _ := os.ReadDir(dir); len(es) != 2 {
		t.Fatalf("plugin folder changed: %v", es)
	}
}
