package extplugins

import (
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

	"github.com/dragosh/zen/internal/core/plugins"
)

const (
	maxStdout     = 2 << 20  // bytes of stdout a backend may print
	maxStderr     = 16 << 10 // bytes of stderr kept
	maxBackends   = 4        // backend processes running at once, per app
	errLines      = 5        // lines of stderr shown in an error
	errBytes      = 600      // bytes of stderr shown in an error
	pipeDrainWait = 500 * time.Millisecond
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
	sort.Strings(keys)
	for _, k := range keys {
		if ek := envKey(k); ek != "" {
			env = append(env, "ZEN_ATTR_"+ek+"="+noNUL(attrs[k]))
		}
	}
	cfg := c.ctx.Config
	if cfg == nil {
		cfg = map[string]any{}
	}
	env = append(env, "ZEN_CONFIG_JSON="+noNUL(jsonOrEmpty(cfg)))
	keys = keys[:0]
	for k := range cfg {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if ek := envKey(k); ek != "" {
			env = append(env, "ZEN_CONFIG_"+ek+"="+noNUL(fmt.Sprint(cfg[k])))
		}
	}
	return env
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
	for _, d := range []string{"/usr/bin", "/bin", "/usr/sbin", "/sbin", "/opt/homebrew/bin", "/usr/local/bin"} {
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

func (p *Plugin) run(c call) ([]byte, error) {
	if p.back == nil {
		return nil, errors.New("the plugin has no backend")
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

	slots <- struct{}{}
	defer func() { <-slots }()

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
	out := &collector{limit: maxStdout, done: make(chan struct{})}
	out.onOvr = func() { killGroup(cmd) }
	errc := &collector{limit: maxStderr, done: make(chan struct{})}
	go out.run(outR)
	go errc.run(errR)

	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	timer := time.NewTimer(p.back.timeout)
	defer timer.Stop()
	var waitErr error
	timedOut := false
	select {
	case waitErr = <-waited:
	case <-timer.C:
		timedOut = true
		killGroup(cmd)
		waitErr = <-waited
	}
	killGroup(cmd) // children left behind do not outlive the call
	grace := time.NewTimer(pipeDrainWait)
	for _, d := range []chan struct{}{out.done, errc.done} {
		select {
		case <-d:
		case <-grace.C:
		}
	}
	grace.Stop()
	outR.Close()
	errR.Close()
	inW.Close()

	switch {
	case timedOut:
		return nil, fmt.Errorf("%w after %s", errTimeout, p.back.timeout)
	case out.over:
		return nil, errors.New("the output is larger than 2 MB")
	case waitErr != nil:
		var ee *exec.ExitError
		if errors.As(waitErr, &ee) {
			return nil, fmt.Errorf("exit code %d%s", ee.ExitCode(), stderrSummary(errc.buf))
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
	dir   string
	files map[string]bool
}

func (a assetFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) || !a.files[name] {
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
	return assetFS{dir: p.dir, files: files}
}
