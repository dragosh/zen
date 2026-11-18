// Package lint runs the Vale prose linter (bundled in Zenmark.app as
// Contents/Helpers/vale) on markdown content and returns its alerts with
// positions in the original content.
//
// Content goes to vale on stdin; vale runs with the workspace root as its
// working directory, no file arguments, a minimal environment, a timeout and
// at most two processes at once per Linter.
package lint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// ErrUnavailable is returned when no vale binary can be found or run.
// Callers treat it as "linting off", not as an error to show.
var ErrUnavailable = errors.New("lint: vale is not available")

// MaxContent is the largest content Lint passes to vale (1 MB). Larger
// content gets no alerts.
const MaxContent = 1 << 20

const (
	defaultTimeout = 5 * time.Second
	maxConcurrent  = 2
	maxStderr      = 4 << 10
)

// defaultConfig is used when the workspace has no .vale.ini: spelling and
// Vale's built-in checks.
const defaultConfig = `MinAlertLevel = suggestion

[*.{md,markdown}]
BasedOnStyles = Vale
`

// Alert is one vale finding. Line is 1-based; Col and EndCol are 1-based
// rune columns within that line of the original content, EndCol inclusive.
type Alert struct {
	Line, Col, EndCol int    // 1-based; columns in runes
	Severity          string // "error" | "warning" | "suggestion"
	Check, Message    string
	Match             string
	Description       string // why the check exists (may be empty)
	Link              string // https link to the rule's documentation (may be empty)
}

// Linter runs one vale binary. It is safe for concurrent use.
type Linter struct {
	path    string
	timeout time.Duration
	sem     chan struct{}
}

// Find locates vale: Contents/Helpers/vale next to the running executable
// (Zenmark.app), else $ZEN_VALE, else build/darwin/helpers/<arch>/vale in the
// repository of a dev build (bin/zen), else vale on PATH.
func Find() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		exe = ""
	} else if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	return find(exe, os.Getenv, exec.LookPath)
}

func find(exe string, getenv func(string) string, lookPath func(string) (string, error)) (string, error) {
	dir := ""
	if exe != "" {
		dir = filepath.Dir(exe)
		if filepath.Base(dir) == "MacOS" {
			if p := filepath.Join(filepath.Dir(dir), "Helpers", "vale"); isExec(p) {
				return p, nil
			}
		}
	}
	if p := getenv("ZEN_VALE"); p != "" && isExec(p) {
		return p, nil
	}
	if dir != "" && filepath.Base(dir) == "bin" {
		if p := filepath.Join(filepath.Dir(dir), "build", "darwin", "helpers", runtime.GOARCH, "vale"); isExec(p) {
			return p, nil
		}
	}
	if p, err := lookPath("vale"); err == nil && p != "" {
		return p, nil
	}
	return "", ErrUnavailable
}

// isExec reports whether p is a regular file with an execute bit.
func isExec(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0o111 != 0
}

// New returns a Linter for the vale binary at valePath. An empty or missing
// path gives a Linter whose Lint returns ErrUnavailable.
func New(valePath string) *Linter {
	return &Linter{path: valePath, timeout: defaultTimeout, sem: make(chan struct{}, maxConcurrent)}
}

// Lint runs vale on content (the text of path, a workspace-relative page,
// possibly unsaved) with the workspace root as working directory. It uses
// root/.vale.ini when present, else zen's default config.
func (l *Linter) Lint(ctx context.Context, root, path string, content []byte) ([]Alert, error) {
	if l == nil || l.path == "" || !isExec(l.path) {
		return nil, ErrUnavailable
	}
	if len(content) > MaxContent {
		slog.Warn(fmt.Sprintf("lint: %s is larger than %d bytes; not linted", path, MaxContent))
		return []Alert{}, nil
	}
	if fi, err := os.Stat(root); root == "" || err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("lint: workspace root %q is not a directory", root)
	}
	cfg, err := configFor(root)
	if err != nil {
		return nil, err
	}

	select {
	case l.sem <- struct{}{}:
		defer func() { <-l.sem }()
	case <-ctx.Done():
		return nil, fmt.Errorf("lint: %w", ctx.Err())
	}

	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, l.path,
		"--output=JSON", "--ext=.md", "--no-exit", "--no-global", "--config="+cfg)
	cmd.Dir = root
	cmd.Env = valeEnv()
	cmd.Stdin = bytes.NewReader(content)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &limited{buf: &stderr, n: maxStderr}
	cmd.WaitDelay = time.Second
	err = cmd.Run()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("lint: vale: %w", ctx.Err())
	}
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
			return nil, ErrUnavailable
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(truncate(stdout.String(), maxStderr))
		}
		return nil, fmt.Errorf("lint: vale: %v: %s", err, msg)
	}
	return parse(stdout.Bytes(), content)
}

// limited is a writer that keeps the first n bytes.
type limited struct {
	buf *bytes.Buffer
	n   int
}

func (w *limited) Write(p []byte) (int, error) {
	if room := w.n - w.buf.Len(); room > 0 {
		w.buf.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// valeEnv is the environment vale runs with: PATH, HOME, TMPDIR and VALE_*
// (except VALE_CONFIG_PATH: the config is always given with --config).
func valeEnv() []string {
	var env []string
	hasPath := false
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch {
		case k == "PATH":
			hasPath = true
			env = append(env, kv)
		case k == "HOME", k == "TMPDIR":
			env = append(env, kv)
		case strings.HasPrefix(k, "VALE_") && k != "VALE_CONFIG_PATH":
			env = append(env, kv)
		}
	}
	if !hasPath {
		env = append(env, "PATH=/usr/bin:/bin")
	}
	return env
}

// configFor returns root/.vale.ini when it is a file, else the default config.
func configFor(root string) (string, error) {
	ini := filepath.Join(root, ".vale.ini")
	if fi, err := os.Stat(ini); err == nil && fi.Mode().IsRegular() {
		return ini, nil
	}
	return defaultConfigPath()
}

var defaultCfg struct {
	sync.Mutex
	path string
}

// defaultConfigPath writes the default config once to a private temp dir
// (again only if something removed or changed it) and returns its path.
func defaultConfigPath() (string, error) {
	defaultCfg.Lock()
	defer defaultCfg.Unlock()
	if defaultCfg.path != "" {
		if b, err := os.ReadFile(defaultCfg.path); err == nil && string(b) == defaultConfig {
			return defaultCfg.path, nil
		}
	}
	dir, err := os.MkdirTemp("", "zen-vale-")
	if err != nil {
		return "", fmt.Errorf("lint: %w", err)
	}
	p := filepath.Join(dir, ".vale.ini")
	if err := os.WriteFile(p, []byte(defaultConfig), 0o600); err != nil {
		return "", fmt.Errorf("lint: %w", err)
	}
	defaultCfg.path = p
	return p, nil
}

// valeAlert is one alert in vale's JSON output.
type valeAlert struct {
	Span        [2]int
	Check       string
	Message     string
	Severity    string
	Match       string
	Line        int
	Description string
	Link        string
}

// parse turns vale's JSON output for stdin into alerts positioned in
// content, sorted by line then column.
func parse(data, content []byte) ([]Alert, error) {
	alerts := []Alert{}
	if len(bytes.TrimSpace(data)) == 0 {
		return alerts, nil
	}
	var files map[string][]valeAlert
	if err := json.Unmarshal(data, &files); err != nil {
		return nil, fmt.Errorf("lint: vale output: %w", err)
	}
	lines := strings.Split(string(content), "\n")
	for _, va := range files {
		for _, v := range va {
			alerts = append(alerts, position(v, lines))
		}
	}
	sort.SliceStable(alerts, func(i, j int) bool {
		a, b := alerts[i], alerts[j]
		switch {
		case a.Line != b.Line:
			return a.Line < b.Line
		case a.Col != b.Col:
			return a.Col < b.Col
		case a.EndCol != b.EndCol:
			return a.EndCol < b.EndCol
		case a.Check != b.Check:
			return a.Check < b.Check
		}
		return a.Message < b.Message
	})
	return alerts, nil
}

// position resolves a vale alert to rune columns of the original line.
// Vale's spans are 1-based inclusive rune columns; when the match is not
// found there (byte columns, or drift), the occurrence of Match nearest to
// the span is used, and otherwise the span is clamped to the line.
func position(v valeAlert, lines []string) Alert {
	a := Alert{Severity: severity(v.Severity), Check: v.Check, Message: v.Message, Match: v.Match,
		Description: v.Description, Link: httpsLink(v.Link)}
	a.Line = clamp(v.Line, 1, len(lines))
	line := strings.TrimSuffix(lines[a.Line-1], "\r")
	runes := []rune(line)
	start, end := v.Span[0], v.Span[1]

	if v.Match != "" {
		m := len([]rune(v.Match))
		at := func(col int) bool { // Match starts at 1-based rune column col
			return col >= 1 && col-1+m <= len(runes) && string(runes[col-1:col-1+m]) == v.Match
		}
		col := 0
		switch {
		case at(start):
			col = start
		case start >= 1 && start-1 <= len(line) && strings.HasPrefix(line[start-1:], v.Match) && utf8.ValidString(line[:start-1]):
			col = utf8.RuneCountInString(line[:start-1]) + 1 // byte column
		default:
			for c := 1; c+m-1 <= len(runes); c++ {
				if at(c) && (col == 0 || abs(c-start) < abs(col-start)) {
					col = c
				}
			}
		}
		if col > 0 {
			a.Col, a.EndCol = col, col+m-1
			return a
		}
	}
	n := len(runes)
	a.Col = clamp(start, 1, max(n, 1))
	a.EndCol = clamp(end, a.Col, max(n, a.Col))
	return a
}

func severity(s string) string {
	switch s = strings.ToLower(s); s {
	case "error", "warning", "suggestion":
		return s
	}
	return "suggestion"
}

func clamp(v, lo, hi int) int {
	return max(lo, min(v, hi))
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// httpsLink keeps only https documentation links; anything else becomes "".
func httpsLink(s string) string {
	if strings.HasPrefix(s, "https://") && !strings.ContainsAny(s, " \t\n<>\"") {
		return s
	}
	return ""
}
