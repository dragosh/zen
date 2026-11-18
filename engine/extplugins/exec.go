package extplugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/dragosh/zenmd/engine/plugins"
)

const (
	maxStdout   = 2 << 20  // bytes of stdout a backend may print
	maxStderr   = 16 << 10 // bytes of stderr kept
	maxBackends = 4        // backend processes running at once, per app
	errLines    = 5        // lines of stderr shown in an error
	errBytes    = 600      // bytes of stderr shown in an error
)

var slots = make(chan struct{}, maxBackends)

// call is one backend invocation.
type call struct {
	hook, name string
	fenced     *bool
	attrs      map[string]string
	ctx        plugins.Context
	stdin      []byte
}

func (p *Plugin) declared(hook string) error {
	if !p.HasHook(hook) {
		return fmt.Errorf("the plugin does not declare the %s hook", hook)
	}
	return nil
}

func (p *Plugin) finish(out []byte) string {
	if p.back.text {
		return html.EscapeString(string(out))
	}
	return string(out)
}

// RenderBlock implements plugins.BlockHook (ZEN_HOOK=block).
func (p *Plugin) RenderBlock(ctx plugins.Context, b plugins.Block) (string, error) {
	if err := p.declared("block"); err != nil {
		return "", err
	}
	fenced := b.Fenced
	out, err := p.run(call{hook: "block", name: b.Name, fenced: &fenced, attrs: b.Attrs, ctx: ctx, stdin: []byte(b.Body)})
	if err != nil {
		return "", err
	}
	if p.back.text {
		return `<pre class="zen-plugin-output">` + p.finish(out) + "</pre>", nil
	}
	return string(out), nil
}

// RenderInline implements plugins.InlineHook (ZEN_HOOK=inline).
func (p *Plugin) RenderInline(ctx plugins.Context, i plugins.Inline) (string, error) {
	if err := p.declared("inline"); err != nil {
		return "", err
	}
	out, err := p.run(call{hook: "inline", name: i.Name, attrs: i.Attrs, ctx: ctx, stdin: []byte(i.Text)})
	if err != nil {
		return "", err
	}
	return p.finish(out), nil
}

// Source implements plugins.SourceHook (ZEN_HOOK=source).
func (p *Plugin) Source(ctx plugins.Context, src []byte) ([]byte, error) {
	if err := p.declared("source"); err != nil {
		return nil, err
	}
	return p.run(call{hook: "source", ctx: ctx, stdin: src})
}

// HTML implements plugins.HTMLHook (ZEN_HOOK=html).
func (p *Plugin) HTML(ctx plugins.Context, h string) (string, error) {
	if err := p.declared("html"); err != nil {
		return "", err
	}
	out, err := p.run(call{hook: "html", ctx: ctx, stdin: []byte(h)})
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Action runs the action hook (ZEN_HOOK=action): payload in, stdout out.
func (p *Plugin) Action(ctx plugins.Context, payload []byte) ([]byte, error) {
	if err := p.declared("action"); err != nil {
		return nil, err
	}
	return p.run(call{hook: "action", ctx: ctx, stdin: payload})
}

// envKey turns an attribute or option name into the suffix of an
// environment variable: upper case, anything but [A-Z0-9_] becomes "_".
func envKey(k string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(k) {
		if r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func noNUL(s string) string { return strings.ReplaceAll(s, "\x00", "") }

func jsonOrEmpty(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func (p *Plugin) env(c call, rt Runtime) []string {
	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if home == "" {
		home = "/"
	}
	lang := os.Getenv("LANG")
	if lang == "" {
		lang = "en_US.UTF-8"
	}
	env := []string{
		"PATH=" + toolPath(),
		"HOME=" + home,
		"TMPDIR=" + os.TempDir(),
		"LANG=" + lang,
		"ZEN_HOOK=" + c.hook,
		"ZEN_PLUGIN=" + p.man.ID,
		"ZEN_API=" + strconv.Itoa(plugins.APIVersion),
		"ZEN_PAGE=" + noNUL(c.ctx.Path),
		"ZEN_WORKSPACE=" + noNUL(rt.Workspace),
		"ZEN_PLUGIN_DIR=" + p.dir,
		"ZEN_DATA_DIR=" + noNUL(rt.DataDir),
	}
	if c.name != "" {
		env = append(env, "ZEN_NAME="+c.name)
	}
	if c.fenced != nil {
		f := "0"
		if *c.fenced {
			f = "1"
		}
		env = append(env, "ZEN_FENCED="+f)
	}
	attrs := c.attrs
	if attrs == nil {
		attrs = map[string]string{}
	}
	env = append(env, "ZEN_ATTRS_JSON="+noNUL(jsonOrEmpty(attrs)))
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	env = append(env, prefixed("ZEN_ATTR_", keys, func(k string) string { return attrs[k] })...)
	cfg := c.ctx.Config
	if cfg == nil {
		cfg = map[string]any{}
	}
	env = append(env, "ZEN_CONFIG_JSON="+noNUL(jsonOrEmpty(cfg)))
	keys = keys[:0]
	for k := range cfg {
		keys = append(keys, k)
	}
	env = append(env, prefixed("ZEN_CONFIG_", keys, func(k string) string { return configEnv(cfg[k]) })...)
	return env
}

// prefixed makes the PREFIX<KEY>=value entries for names. Names are taken in
// sorted order; when several map to the same variable (a-b and a_b are both
// A_B) the one that sorts last wins and only one entry is emitted. The
// *_JSON variables always carry every name.
func prefixed(prefix string, names []string, value func(name string) string) []string {
	sort.Strings(names)
	vals := map[string]string{}
	var order []string
	for _, k := range names {
		ek := envKey(k)
		if ek == "" {
			continue
		}
		if _, seen := vals[ek]; !seen {
			order = append(order, ek)
		}
		vals[ek] = noNUL(value(k))
	}
	out := make([]string, 0, len(order))
	for _, ek := range order {
		out = append(out, prefix+ek+"="+vals[ek])
	}
	return out
}

// configEnv is an option's value for ZEN_CONFIG_<KEY>: JSON for maps and lists.
func configEnv(v any) string {
	switch v.(type) {
	case map[string]string, []string, map[string]any, []any:
		return jsonOrEmpty(v)
	}
	return fmt.Sprint(v)
}

// toolPath is PATH for backends: the app's PATH plus the usual places of
// command line tools on a Mac (an app started from the Finder has a short one).
func toolPath() string {
	parts := strings.Split(os.Getenv("PATH"), ":")
	have := map[string]bool{}
	var out []string
	for _, d := range parts {
		if d != "" && !have[d] {
			have[d] = true
			out = append(out, d)
		}
	}
	home, _ := os.UserHomeDir()
	extra := []string{}
	if home != "" {
		extra = append(extra, filepath.Join(home, ".local", "bin"), filepath.Join(home, ".cargo", "bin"))
	}
	for _, d := range append([]string{"/usr/bin", "/bin", "/usr/sbin", "/sbin", "/opt/homebrew/bin", "/usr/local/bin", "/opt/local/bin"}, extra...) {
		if !have[d] {
			have[d] = true
			out = append(out, d)
		}
	}
	return strings.Join(out, ":")
}

// program resolves command[0]: a path inside the plugin folder, or a program
// on the backend's PATH.
func (p *Plugin) program(path string) (string, error) {
	prog := p.back.command[0]
	if strings.Contains(prog, "/") {
		full, err := resolveInside(p.dir, prog, "command")
		if err != nil {
			return "", err
		}
		return full, nil
	}
	for _, d := range strings.Split(path, ":") {
		cand := filepath.Join(d, prog)
		if fi, err := os.Stat(cand); err == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0o111 != 0 {
			return cand, nil
		}
	}
	return "", fmt.Errorf("program %q not found (is it installed?)", prog)
}

func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// collect reads r into memory, keeping at most limit bytes; over reports
// whether more arrived. It keeps draining so the writer never blocks.
type collector struct {
	limit int
	buf   []byte
	over  bool
	onOvr func()
	done  chan struct{}
}

func (c *collector) run(r io.Reader) {
	defer close(c.done)
	chunk := make([]byte, 32<<10)
	for {
		n, err := r.Read(chunk)
		if n > 0 {
			if room := c.limit - len(c.buf); room >= n {
				c.buf = append(c.buf, chunk[:n]...)
			} else {
				if room > 0 {
					c.buf = append(c.buf, chunk[:room]...)
				}
				if !c.over {
					c.over = true
					if c.onOvr != nil {
						c.onOvr()
					}
				}
			}
		}
		if err != nil {
			return
		}
	}
}

var errTimeout = errors.New("timed out")

// proc is a started backend: its pipes are the caller's to read and close.
type proc struct {
	cmd        *exec.Cmd
	inW        *os.File
	outR, errR *os.File
}

// spawn starts the backend for c in its own process group and feeds c.stdin.
func (p *Plugin) spawn(c call) (*proc, error) {
	if p.back == nil {
		return nil, errors.New("the plugin has no backend")
	}
	if p.Changed() {
		return nil, ErrChanged
	}
	if err := checkArgs(p.dir, p.back.command); err != nil {
		return nil, err
	}
	rt := Runtime{}
	if r := p.rt.Load(); r != nil {
		rt = *r
	}
	if rt.DataDir != "" {
		if err := os.MkdirAll(rt.DataDir, 0o700); err != nil {
			return nil, errors.New("could not create the plugin's data folder")
		}
	}
	env := p.env(c, rt)
	var path string
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, "PATH="); ok {
			path = v
		}
	}
	prog, err := p.program(path)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(prog, p.back.command[1:]...)
	cmd.Dir = p.dir
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	inR, inW, err := os.Pipe()
	if err != nil {
		return nil, errors.New("could not start the plugin")
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		inR.Close()
		inW.Close()
		return nil, errors.New("could not start the plugin")
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		inR.Close()
		inW.Close()
		outR.Close()
		outW.Close()
		return nil, errors.New("could not start the plugin")
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = inR, outW, errW
	startErr := cmd.Start()
	inR.Close()
	outW.Close()
	errW.Close()
	if startErr != nil {
		inW.Close()
		outR.Close()
		errR.Close()
		return nil, fmt.Errorf("could not start %s", filepath.Base(prog))
	}
	go func() {
		_, _ = inW.Write(c.stdin) // a program that does not read its input is fine
		inW.Close()
	}()
	return &proc{cmd: cmd, inW: inW, outR: outR, errR: errR}, nil
}

// ctx is the context that ends the plugin's calls (Runtime.Ctx).
func (p *Plugin) ctx() context.Context {
	if r := p.rt.Load(); r != nil && r.Ctx != nil {
		return r.Ctx
	}
	return context.Background()
}

var errCancelled = errors.New("cancelled")

// result is how a started backend ended.
type result struct {
	waitErr   error // from Wait: nil, an *exec.ExitError or another error
	timedOut  bool  // the deadline passed (the process or a pipe was still open)
	cancelled bool  // the context was cancelled
	stderr    []byte
}

// spawnWait is the one place that waits for a started backend, for run and
// Stream. It reads stdout with readOut (in its own goroutine) and keeps the
// first maxStderr bytes of stderr. One deadline, timeout from now, covers the
// process and both pipes: when it passes, or ctx is cancelled, the process
// group is killed, the pipes are closed (a child that left the group and still
// holds them cannot keep anything open) and the result says which. Normally
// it returns when the process exited and both pipes reached EOF. On return no
// goroutine of the call is running: readOut is never called again.
func spawnWait(ctx context.Context, sp *proc, timeout time.Duration, readOut func(io.Reader)) result {
	dctx, stop := context.WithTimeout(ctx, timeout)
	defer stop()
	errc := &collector{limit: maxStderr, done: make(chan struct{})}
	go errc.run(sp.errR)
	outDone := make(chan struct{})
	go func() { defer close(outDone); readOut(sp.outR) }()
	waited := make(chan error, 1)
	go func() { waited <- sp.cmd.Wait() }()

	var res result
	interrupted := false
	select {
	case res.waitErr = <-waited:
	case <-dctx.Done():
		interrupted = true
		killGroup(sp.cmd)
		res.waitErr = <-waited
	}
	killGroup(sp.cmd) // children left behind do not outlive the call
	for _, d := range []chan struct{}{outDone, errc.done} {
		select {
		case <-d:
		case <-dctx.Done():
			interrupted = true
		}
	}
	sp.outR.Close() // wakes a reader that a leftover process keeps waiting
	sp.errR.Close()
	sp.inW.Close()
	<-outDone
	<-errc.done
	if interrupted {
		if ctx.Err() != nil {
			res.cancelled = true
		} else {
			res.timedOut = true
		}
	}
	res.stderr = errc.buf
	return res
}

func (p *Plugin) run(c call) ([]byte, error) {
	if p.back == nil {
		return nil, errors.New("the plugin has no backend")
	}
	ctx := p.ctx()
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	case <-ctx.Done():
		return nil, errCancelled
	}

	sp, err := p.spawn(c)
	if err != nil {
		return nil, err
	}
	out := &collector{limit: maxStdout, done: make(chan struct{})}
	out.onOvr = func() { killGroup(sp.cmd) }
	res := spawnWait(ctx, sp, p.callTimeout(), out.run)

	switch {
	case res.cancelled:
		return nil, errCancelled
	case res.timedOut:
		return nil, fmt.Errorf("%w after %s", errTimeout, p.callTimeout())
	case out.over:
		return nil, errors.New("the output is larger than 2 MB")
	case res.waitErr != nil:
		var ee *exec.ExitError
		if errors.As(res.waitErr, &ee) {
			return nil, fmt.Errorf("exit code %d%s", ee.ExitCode(), stderrSummary(res.stderr))
		}
		return nil, errors.New("the plugin did not finish")
	}
	return out.buf, nil
}

// stderrSummary is ": " and the first lines of stderr, or "".
func stderrSummary(b []byte) string {
	s := strings.TrimSpace(strings.ToValidUTF8(string(b), ""))
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > errLines {
		lines = lines[:errLines]
	}
	s = strings.Join(lines, "\n")
	if len(s) > errBytes {
		s = s[:errBytes]
		for len(s) > 0 && !utf8.ValidString(s) {
			s = s[:len(s)-1]
		}
		s += "…"
	}
	return ": " + s
}

// assetFS serves only the files the manifest lists, from the plugin folder,
// refusing anything that resolves outside it.
type assetFS struct {
	p     *Plugin
	dir   string
	files map[string]bool
}

func (a assetFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) || !a.files[name] || a.p.Changed() {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	full, err := resolveInside(a.dir, name, "file")
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	f, err := os.Open(full)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() {
		f.Close()
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return f, nil
}

// Assets implements plugins.AssetProvider: the frontend files, nothing else.
// It is nil for a plugin without frontend files.
func (p *Plugin) Assets() fs.FS {
	if len(p.frontend.JS)+len(p.frontend.CSS) == 0 {
		return nil
	}
	files := map[string]bool{}
	for _, f := range append(append([]string(nil), p.frontend.JS...), p.frontend.CSS...) {
		files[f] = true
	}
	return assetFS{p: p, dir: p.dir, files: files}
}
