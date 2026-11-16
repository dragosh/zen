// Package sandbox runs the code of a ```sandbox block inside a container with
// macOS's `container` tool (Apple's container platform). The image is chosen
// by a key of the user's global `images` option, never by the page; the plugin
// never starts the container service, pulls an image or touches the workspace.
//
// Stream chunks are JSON lines (one chunk per output line):
//
//	{"s":"out","t":"a line of stdout"}
//	{"s":"err","t":"a line of stderr"}
//	{"s":"exit","code":0,"ms":1530}                // always the last chunk of a run
//	{"s":"exit","code":-1,"ms":30000,"timedOut":true}
//
// Problems before the program starts (bad input, service stopped, tool
// missing) end the stream with an error instead, which the frontend shows.
package sandbox

import (
	"bufio"
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dragosh/zen/internal/core/plugins"
)

//go:embed assets/sandbox.js assets/sandbox.css
var assets embed.FS

// ContainerPath is the `container` executable. Tests point it at a fake script.
var ContainerPath = defaultContainerPath()

func defaultContainerPath() string {
	if p, err := exec.LookPath("container"); err == nil {
		return p
	}
	return "/opt/homebrew/bin/container"
}

const (
	maxCmd      = 2000
	maxCode     = 1 << 20
	maxLine     = 64 << 10
	maxTimeout  = 600
	statusLimit = 10 * time.Second
	killLimit   = 10 * time.Second
)

var (
	fileRe   = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]{0,99}$`)
	memoryRe = regexp.MustCompile(`^[1-9][0-9]{0,5}[KkMmGg]?$`)
)

type plugin struct{}

// New returns the sandbox plugin.
func New() plugins.Plugin { return plugin{} }

func (plugin) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID:          "sandbox",
		Version:     "0.1.0",
		APIVersion:  plugins.APIVersion,
		Description: "Run a block's code in a container (starts containers with `container run`)",
		Default:     true,
		Blocks:      []string{"sandbox"},
		Config: map[string]plugins.Option{
			"images":  {Type: "map", Default: map[string]string{}, Scope: plugins.ScopeUser, Description: "image keys a block may use, mapped to image references"},
			"timeout": {Type: "int", Default: 30, Scope: plugins.ScopeUser, Description: "seconds a run may take (1 to 600)"},
			"network": {Type: "bool", Default: false, Scope: plugins.ScopeUser, Description: "give the container network access"},
			"memory":  {Type: "string", Default: "512M", Scope: plugins.ScopeUser, Description: "memory limit, e.g. 512M or 1G"},
			"cpus":    {Type: "int", Default: 1, Scope: plugins.ScopeUser, Description: "CPUs for the container (1 to 8)"},
		},
	}
}

func (plugin) Assets() fs.FS {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err)
	}
	return sub
}

func (plugin) Entry() plugins.Assets {
	return plugins.Assets{CSS: []string{"sandbox.css"}, JS: []string{"sandbox.js"}}
}

// ---- options ----

func images(ctx plugins.Context) map[string]string {
	out := map[string]string{}
	switch m := ctx.Config["images"].(type) {
	case map[string]string:
		for k, v := range m {
			out[k] = v
		}
	case map[string]any:
		for k, v := range m {
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func intOpt(ctx plugins.Context, name string, def, lo, hi int) int {
	n := def
	switch v := ctx.Config[name].(type) {
	case int:
		n = v
	case int64:
		n = int(v)
	case float64:
		n = int(v)
	}
	if n < lo || n > hi {
		return def
	}
	return n
}

func memoryOpt(ctx plugins.Context) string {
	if s, ok := ctx.Config["memory"].(string); ok && memoryRe.MatchString(s) {
		return s
	}
	return "512M"
}

func networkOpt(ctx plugins.Context) bool {
	b, _ := ctx.Config["network"].(bool)
	return b
}

// ---- block ----

const snippet = "plugins:\n  sandbox:\n    images:\n      python: \"python:3.12-slim\"\n"

func (plugin) RenderBlock(ctx plugins.Context, b plugins.Block) (string, error) {
	imgs := images(ctx)
	key := b.Attrs["image"]
	code := `<pre class="zen-sandbox-code">` + html.EscapeString(b.Body) + `</pre>`
	if _, ok := imgs[key]; ok && key != "" {
		return `<div class="zen-sandbox zen-sandbox-ready">` + code + `</div>`, nil
	}
	var msg string
	switch {
	case len(imgs) == 0:
		msg = "No sandbox images are configured. Add the images you allow under plugins.sandbox.images in your global config.yml (cog menu, Open global config), for example:"
	case key == "":
		msg = "This block has no image. Name one of the configured images (" + strings.Join(sortedKeys(imgs), ", ") + "), for example {image=" + sortedKeys(imgs)[0] + "}, or add one under plugins.sandbox.images in your global config.yml, for example:"
	default:
		msg = "Unknown image \"" + key + "\". Configured: " + strings.Join(sortedKeys(imgs), ", ") + ". Add it under plugins.sandbox.images in your global config.yml, for example:"
	}
	return `<div class="zen-sandbox zen-sandbox-setup"><div class="zen-sandbox-note">` + html.EscapeString(msg) +
		`<pre class="zen-sandbox-snippet">` + html.EscapeString(snippet) + `</pre></div>` + code + `</div>`, nil
}

// ---- service status ----

// serviceState is "running", "stopped" or "missing" (the tool is not installed).
func serviceState(c context.Context) string {
	if _, err := os.Stat(ContainerPath); err != nil {
		return "missing"
	}
	c, cancel := context.WithTimeout(c, statusLimit)
	defer cancel()
	if err := exec.CommandContext(c, ContainerPath, "system", "status").Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "stopped"
		}
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
			return "missing"
		}
		return "stopped"
	}
	return "running"
}

func (plugin) Action(ctx plugins.Context, payload []byte) ([]byte, error) {
	var req struct {
		Op string `json:"op"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, errors.New("the request is not valid JSON")
	}
	if req.Op != "status" {
		return nil, fmt.Errorf("unknown op %q", req.Op)
	}
	return json.Marshal(map[string]any{
		"service": serviceState(context.Background()),
		"images":  sortedKeys(images(ctx)),
	})
}

// ---- stream ----

type runRequest struct {
	Op    string `json:"op"`
	Image string `json:"image"`
	Cmd   string `json:"cmd"`
	File  string `json:"file"`
	Code  string `json:"code"`
}

func (plugin) Stream(c context.Context, ctx plugins.Context, payload []byte, emit func([]byte)) error {
	var req runRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return errors.New("the request is not valid JSON")
	}
	if req.Op != "run" {
		return fmt.Errorf("unknown op %q", req.Op)
	}
	imgs := images(ctx)
	ref, ok := imgs[req.Image]
	if !ok || req.Image == "" {
		return fmt.Errorf("unknown image %q: add it under plugins.sandbox.images in your global config.yml", req.Image)
	}
	if ref == "" || strings.HasPrefix(ref, "-") || strings.ContainsAny(ref, " \t\r\n\x00") {
		return fmt.Errorf("the image reference for %q is not valid", req.Image)
	}
	if req.Cmd == "" {
		return errors.New("the block has no cmd")
	}
	if len(req.Cmd) > maxCmd {
		return fmt.Errorf("cmd is longer than %d characters", maxCmd)
	}
	if strings.ContainsRune(req.Cmd, 0) {
		return errors.New("cmd contains a NUL byte")
	}
	if len(req.Code) > maxCode {
		return errors.New("the code is larger than 1 MB")
	}
	file := req.File
	if file == "" {
		file = "main.txt"
	}
	if !fileRe.MatchString(file) || file == ".." {
		return fmt.Errorf("file %q must be a plain file name", req.File)
	}

	switch serviceState(c) {
	case "missing":
		return fmt.Errorf("Apple's container tool is not installed (looked for %s). Install it, then run `container system start` in a terminal.", ContainerPath)
	case "stopped":
		return errors.New("the container service is not running. Run `container system start` in a terminal, then try again")
	}
	if c.Err() != nil {
		return c.Err()
	}

	tmp, err := os.MkdirTemp("", "zen-sandbox-")
	if err != nil {
		return errors.New("could not create a temporary folder")
	}
	defer os.RemoveAll(tmp)
	if err := os.Chmod(tmp, 0o755); err != nil {
		return errors.New("could not prepare the temporary folder")
	}
	if err := os.WriteFile(filepath.Join(tmp, file), []byte(req.Code), 0o644); err != nil {
		return errors.New("could not write the code")
	}
	var rnd [6]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return errors.New("could not name the container")
	}
	name := "zen-" + hex.EncodeToString(rnd[:])

	args := []string{"run", "--rm", "--name", name}
	if !networkOpt(ctx) {
		args = append(args, "--network", "none", "--no-dns")
	}
	args = append(args, "--read-only", "--progress", "none",
		"-c", fmt.Sprint(intOpt(ctx, "cpus", 1, 1, 8)),
		"-m", memoryOpt(ctx),
		"-v", tmp+":/work", "-w", "/work",
		ref, "sh", "-c", req.Cmd)

	limit := time.Duration(intOpt(ctx, "timeout", 30, 1, maxTimeout)) * time.Second
	tc, cancel := context.WithTimeout(c, limit)
	defer cancel()
	return runContainer(c, tc, name, args, emit)
}

func runContainer(parent, tc context.Context, name string, args []string, emit func([]byte)) error {
	cmd := exec.Command(ContainerPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.New("could not start the container tool")
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return errors.New("could not start the container tool")
	}
	start := time.Now()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start the container tool: %w", err)
	}

	var mu sync.Mutex
	send := func(v any) {
		b, err := json.Marshal(v)
		if err != nil {
			return
		}
		mu.Lock()
		emit(b)
		mu.Unlock()
	}
	var readers sync.WaitGroup
	pump := func(r io.Reader, tag string) {
		defer readers.Done()
		br := bufio.NewReaderSize(r, maxLine)
		for {
			line, err := br.ReadSlice('\n')
			if len(line) > 0 {
				send(map[string]string{"s": tag, "t": strings.TrimRight(string(line), "\r\n")})
			}
			if errors.Is(err, bufio.ErrBufferFull) {
				continue
			}
			if err != nil {
				return
			}
		}
	}
	readers.Add(2)
	go pump(stdout, "out")
	go pump(stderr, "err")

	waited := make(chan error, 1)
	go func() {
		readers.Wait() // the pipes must be drained before Wait closes them
		waited <- cmd.Wait()
	}()

	var waitErr error
	select {
	case waitErr = <-waited:
	case <-tc.Done():
		killContainer(name)
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		select {
		case waitErr = <-waited:
		case <-time.After(5 * time.Second):
		}
	}

	if parent.Err() != nil {
		return parent.Err()
	}
	ms := time.Since(start).Milliseconds()
	if tc.Err() != nil {
		send(map[string]any{"s": "exit", "code": -1, "ms": ms, "timedOut": true})
		return nil
	}
	code := 0
	if waitErr != nil {
		var ee *exec.ExitError
		if !errors.As(waitErr, &ee) {
			return fmt.Errorf("the container tool failed: %w", waitErr)
		}
		code = ee.ExitCode()
	}
	send(map[string]any{"s": "exit", "code": code, "ms": ms})
	return nil
}

// killContainer stops the named container; errors are ignored (it may be gone).
func killContainer(name string) {
	c, cancel := context.WithTimeout(context.Background(), killLimit)
	defer cancel()
	_ = exec.CommandContext(c, ContainerPath, "kill", name).Run()
}
