package lint

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// exe writes an executable shell script at path (creating parent dirs).
func exe(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func noEnv(string) string { return "" }

func noPath(string) (string, error) { return "", errors.New("not found") }

func TestFind(t *testing.T) {
	dir := t.TempDir()

	// Bundle: Zen.app/Contents/MacOS/zen next to Contents/Helpers/vale.
	app := filepath.Join(dir, "Zen.app", "Contents")
	appExe := exe(t, filepath.Join(app, "MacOS", "zen"), "")
	bundled := exe(t, filepath.Join(app, "Helpers", "vale"), "")

	// Dev build: <repo>/bin/zen next to <repo>/build/darwin/helpers/<arch>/vale.
	repo := filepath.Join(dir, "repo")
	devExe := exe(t, filepath.Join(repo, "bin", "zen"), "")
	devVale := exe(t, filepath.Join(repo, "build", "darwin", "helpers", runtime.GOARCH, "vale"), "")

	// Elsewhere: a binary with no helper anywhere near it.
	lonely := exe(t, filepath.Join(dir, "lonely", "zen"), "")
	envVale := exe(t, filepath.Join(dir, "env", "vale"), "")
	notExec := filepath.Join(dir, "env", "plain")
	if err := os.WriteFile(notExec, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	pathVale := exe(t, filepath.Join(dir, "path", "vale"), "")

	env := func(v string) func(string) string {
		return func(k string) string {
			if k == "ZEN_VALE" {
				return v
			}
			return ""
		}
	}
	onPath := func(name string) (string, error) {
		if name != "vale" {
			return "", errors.New("unexpected name " + name)
		}
		return pathVale, nil
	}

	cases := []struct {
		name   string
		exe    string
		getenv func(string) string
		look   func(string) (string, error)
		want   string
	}{
		{"bundle first", appExe, env(envVale), onPath, bundled},
		{"ZEN_VALE before dev and PATH", devExe, env(envVale), onPath, envVale},
		{"dev build helpers", devExe, noEnv, onPath, devVale},
		{"PATH last", lonely, noEnv, onPath, pathVale},
		{"ZEN_VALE missing falls through", lonely, env(filepath.Join(dir, "nope")), onPath, pathVale},
		{"ZEN_VALE not executable falls through", lonely, env(notExec), onPath, pathVale},
		{"nothing", lonely, noEnv, noPath, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := find(c.exe, c.getenv, c.look)
			if c.want == "" {
				if !errors.Is(err, ErrUnavailable) {
					t.Fatalf("got %q, %v; want ErrUnavailable", got, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}

	// Find wraps find with the real executable and environment.
	if p, err := Find(); err == nil && p == "" {
		t.Error("Find returned an empty path without error")
	} else if err != nil && !errors.Is(err, ErrUnavailable) {
		t.Errorf("Find error %v is not ErrUnavailable", err)
	}
}

const fixtureJSON = `{
  "stdin.md": [
    {"Span": [33, 35], "Check": "Vale.Spelling", "Message": "Did you really mean 'teh'?", "Severity": "error", "Match": "teh", "Line": 7},
    {"Span": [20, 26], "Check": "Vale.Spelling", "Message": "Did you really mean 'speling'?", "Severity": "error", "Match": "speling", "Line": 2},
    {"Span": [13, 19], "Check": "Vale.Spelling", "Message": "Did you really mean 'speling'?", "Severity": "error", "Match": "speling", "Line": 7},
    {"Span": [1, 4], "Check": "Vale.Terms", "Message": "Use 'Zen'", "Severity": "suggestion", "Match": "zen!", "Line": 9},
    {"Span": [1, 3], "Check": "Vale.Repetition", "Message": "odd", "Severity": "warning", "Match": "", "Line": 9}
  ]
}`

const fixtureContent = "---\ntitle: héllo wörld speling\n---\n\n# Tïtle\n\nÉté ünïcode speling here and an teh error.\n\nzen! ok\n"

var fixtureAlerts = []Alert{
	{Line: 2, Col: 20, EndCol: 26, Severity: "error", Check: "Vale.Spelling", Message: "Did you really mean 'speling'?", Match: "speling"},
	{Line: 7, Col: 13, EndCol: 19, Severity: "error", Check: "Vale.Spelling", Message: "Did you really mean 'speling'?", Match: "speling"},
	{Line: 7, Col: 33, EndCol: 35, Severity: "error", Check: "Vale.Spelling", Message: "Did you really mean 'teh'?", Match: "teh"},
	{Line: 9, Col: 1, EndCol: 3, Severity: "warning", Check: "Vale.Repetition", Message: "odd", Match: ""},
	{Line: 9, Col: 1, EndCol: 4, Severity: "suggestion", Check: "Vale.Terms", Message: "Use 'Zen'", Match: "zen!"},
}

func TestParse(t *testing.T) {
	got, err := parse([]byte(fixtureJSON), []byte(fixtureContent))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, fixtureAlerts) {
		t.Errorf("parse:\n got %+v\nwant %+v", got, fixtureAlerts)
	}

	t.Run("empty", func(t *testing.T) {
		for _, in := range []string{"{}", "", "  \n", `{"stdin.md": []}`} {
			got, err := parse([]byte(in), []byte("x"))
			if err != nil || len(got) != 0 {
				t.Errorf("parse(%q) = %v, %v", in, got, err)
			}
		}
	})
	t.Run("malformed", func(t *testing.T) {
		if _, err := parse([]byte("{nope"), []byte("x")); err == nil {
			t.Error("accepted malformed JSON")
		}
	})
	t.Run("byte spans are converted to runes", func(t *testing.T) {
		// "Été ü " is 6 runes / 9 bytes: rune span 7..13, byte span 10..16.
		js := `{"stdin.md":[{"Span":[7,13],"Check":"C","Message":"m","Severity":"error","Match":"speling","Line":1}]}`
		got, err := parse([]byte(js), []byte("Été ü speling"))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Col != 7 || got[0].EndCol != 13 {
			t.Fatalf("got %+v", got)
		}
		js = `{"stdin.md":[{"Span":[10,16],"Check":"C","Message":"m","Severity":"error","Match":"speling","Line":1}]}`
		got, _ = parse([]byte(js), []byte("Été ü speling"))
		if len(got) != 1 || got[0].Col != 7 || got[0].EndCol != 13 {
			t.Fatalf("byte span: got %+v", got)
		}
	})
	t.Run("out of range is clamped", func(t *testing.T) {
		js := `{"stdin.md":[{"Span":[50,60],"Check":"C","Message":"m","Severity":"bogus","Match":"zz","Line":99}]}`
		got, err := parse([]byte(js), []byte("ab\ncd"))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("got %+v", got)
		}
		a := got[0]
		if a.Line != 2 || a.Col < 1 || a.EndCol < a.Col || a.EndCol > 3 || a.Severity != "suggestion" {
			t.Errorf("not clamped: %+v", a)
		}
	})
}

// fake writes a fake vale into dir that records its arguments, working
// directory, environment and stdin into dir/rec.*, then prints out.
func fake(t *testing.T, dir, out string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "out.json"), []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	q := func(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
	body := `for a in "$@"; do printf '%s\n' "$a"; done > ` + q(filepath.Join(dir, "rec.args")) + `
/bin/pwd -P > ` + q(filepath.Join(dir, "rec.cwd")) + `
/usr/bin/env > ` + q(filepath.Join(dir, "rec.env")) + `
/bin/cat > ` + q(filepath.Join(dir, "rec.stdin")) + `
/bin/cat ` + q(filepath.Join(dir, "out.json")) + `
`
	return exe(t, filepath.Join(dir, "vale"), body)
}

func readRec(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "rec."+name))
	if err != nil {
		t.Fatalf("fake vale did not record %s: %v", name, err)
	}
	return string(b)
}

func TestLintWithFake(t *testing.T) {
	t.Setenv("ZEN_SECRET_TOKEN", "hunter2")
	t.Setenv("VALE_STYLES_PATH", "/styles")
	dir := t.TempDir()
	root := t.TempDir()
	l := New(fake(t, dir, fixtureJSON))

	got, err := l.Lint(context.Background(), root, "notes/page.md", []byte(fixtureContent))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, fixtureAlerts) {
		t.Errorf("alerts:\n got %+v\nwant %+v", got, fixtureAlerts)
	}

	if s := readRec(t, dir, "stdin"); s != fixtureContent {
		t.Errorf("stdin = %q", s)
	}
	args := strings.Split(strings.TrimSuffix(readRec(t, dir, "args"), "\n"), "\n")
	want := []string{"--output=JSON", "--ext=.md", "--no-exit", "--no-global"}
	if len(args) != len(want)+1 || !reflect.DeepEqual(args[:len(want)], want) || !strings.HasPrefix(args[len(want)], "--config=") {
		t.Errorf("args = %q, want %q + --config=...", args, want)
	}
	for _, a := range args {
		if !strings.HasPrefix(a, "--") {
			t.Errorf("file argument passed to vale: %q", a)
		}
	}
	realRoot, _ := filepath.EvalSymlinks(root)
	if cwd := strings.TrimSpace(readRec(t, dir, "cwd")); cwd != realRoot {
		t.Errorf("cwd = %q, want workspace root %q", cwd, realRoot)
	}

	allowed := map[string]bool{"PATH": true, "HOME": true, "TMPDIR": true,
		// added by /bin/sh itself
		"PWD": true, "SHLVL": true, "_": true, "OLDPWD": true}
	envs := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(readRec(t, dir, "env")), "\n") {
		k, v, _ := strings.Cut(line, "=")
		envs[k] = v
		if !allowed[k] && !strings.HasPrefix(k, "VALE_") && !strings.HasPrefix(k, "__CF") {
			t.Errorf("vale got environment variable %s", k)
		}
	}
	if _, ok := envs["ZEN_SECRET_TOKEN"]; ok {
		t.Error("secret environment leaked to vale")
	}
	if envs["PATH"] == "" {
		t.Error("PATH not passed to vale")
	}
	if envs["VALE_STYLES_PATH"] != "/styles" {
		t.Errorf("VALE_STYLES_PATH = %q", envs["VALE_STYLES_PATH"])
	}

	// Nothing is written to the workspace.
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Errorf("workspace was written: %v", entries)
	}

	t.Run("vale failure is an error", func(t *testing.T) {
		d := t.TempDir()
		l := New(exe(t, filepath.Join(d, "vale"), "echo boom >&2\nexit 2\n"))
		if _, err := l.Lint(context.Background(), root, "a.md", []byte("x")); err == nil || !strings.Contains(err.Error(), "boom") {
			t.Errorf("err = %v, want it to carry stderr", err)
		}
	})
}

func TestConfigChoice(t *testing.T) {
	configArg := func(t *testing.T, root string) string {
		t.Helper()
		dir := t.TempDir()
		l := New(fake(t, dir, "{}"))
		if _, err := l.Lint(context.Background(), root, "a.md", []byte("hello")); err != nil {
			t.Fatal(err)
		}
		for _, a := range strings.Split(readRec(t, dir, "args"), "\n") {
			if v, ok := strings.CutPrefix(a, "--config="); ok {
				return v
			}
		}
		t.Fatal("no --config argument")
		return ""
	}

	t.Run("workspace .vale.ini", func(t *testing.T) {
		root := t.TempDir()
		ini := filepath.Join(root, ".vale.ini")
		if err := os.WriteFile(ini, []byte("MinAlertLevel = error\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := configArg(t, root); got != ini {
			t.Errorf("--config=%q, want %q", got, ini)
		}
	})
	t.Run("default when absent", func(t *testing.T) {
		root := t.TempDir()
		got := configArg(t, root)
		if strings.HasPrefix(got, root) {
			t.Errorf("default config %q written into the workspace", got)
		}
		b, err := os.ReadFile(got)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range []string{"MinAlertLevel = suggestion", "[*.{md,markdown}]", "BasedOnStyles = Vale"} {
			if !strings.Contains(string(b), s) {
				t.Errorf("default config lacks %q:\n%s", s, b)
			}
		}
		if fi, err := os.Stat(filepath.Dir(got)); err != nil || fi.Mode().Perm()&0o077 != 0 {
			t.Errorf("default config dir is not private: %v %v", fi.Mode(), err)
		}
		if again := configArg(t, t.TempDir()); again != got {
			t.Errorf("default config written twice: %q then %q", got, again)
		}
	})
	t.Run(".vale.ini directory is ignored", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, ".vale.ini"), 0o755); err != nil {
			t.Fatal(err)
		}
		if got := configArg(t, root); strings.HasPrefix(got, root) {
			t.Errorf("used %q", got)
		}
	})
}

func TestLimits(t *testing.T) {
	root := t.TempDir()

	t.Run("defaults", func(t *testing.T) {
		l := New("/x/vale")
		if l.timeout != 5*time.Second || MaxContent != 1<<20 || cap(l.sem) != 2 {
			t.Errorf("timeout %v, max %d, concurrency %d", l.timeout, MaxContent, cap(l.sem))
		}
	})

	t.Run("timeout", func(t *testing.T) {
		d := t.TempDir()
		l := New(exe(t, filepath.Join(d, "vale"), "exec /bin/sleep 30\n"))
		l.timeout = 300 * time.Millisecond
		start := time.Now()
		_, err := l.Lint(context.Background(), root, "a.md", []byte("x"))
		if err == nil {
			t.Fatal("no error from a hanging vale")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("err = %v, want DeadlineExceeded", err)
		}
		if el := time.Since(start); el > 5*time.Second {
			t.Errorf("took %v", el)
		}
	})

	t.Run("caller cancel", func(t *testing.T) {
		d := t.TempDir()
		l := New(exe(t, filepath.Join(d, "vale"), "exec /bin/sleep 30\n"))
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		if _, err := l.Lint(ctx, root, "a.md", []byte("x")); err == nil {
			t.Fatal("no error after cancel")
		}
	})

	t.Run("size cap", func(t *testing.T) {
		d := t.TempDir()
		l := New(fake(t, d, fixtureJSON))
		big := []byte(strings.Repeat("speling ", MaxContent/8+1))
		got, err := l.Lint(context.Background(), root, "big.md", big)
		if err != nil || len(got) != 0 {
			t.Errorf("got %v, %v; want no alerts", got, err)
		}
		if _, err := os.Stat(filepath.Join(d, "rec.args")); err == nil {
			t.Error("vale ran on content over the cap")
		}
		// Exactly at the cap still runs.
		if _, err := l.Lint(context.Background(), root, "ok.md", big[:MaxContent]); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(d, "rec.args")); err != nil {
			t.Error("vale did not run on content at the cap")
		}
	})

	t.Run("unavailable", func(t *testing.T) {
		for _, p := range []string{"", filepath.Join(t.TempDir(), "missing")} {
			if _, err := New(p).Lint(context.Background(), root, "a.md", []byte("x")); !errors.Is(err, ErrUnavailable) {
				t.Errorf("New(%q).Lint err = %v, want ErrUnavailable", p, err)
			}
		}
		var l *Linter
		if _, err := l.Lint(context.Background(), root, "a.md", []byte("x")); !errors.Is(err, ErrUnavailable) {
			t.Errorf("nil Linter err = %v", err)
		}
	})

	t.Run("at most 2 concurrent", func(t *testing.T) {
		d := t.TempDir()
		locks := filepath.Join(d, "locks")
		if err := os.Mkdir(locks, 0o755); err != nil {
			t.Fatal(err)
		}
		counts := filepath.Join(d, "counts")
		l := New(exe(t, filepath.Join(d, "vale"), `/bin/mkdir '`+locks+`/'$$
/bin/ls '`+locks+`' | /usr/bin/wc -l >> '`+counts+`'
/bin/sleep 0.3
/bin/rmdir '`+locks+`/'$$
echo '{}'
`))
		var wg sync.WaitGroup
		errs := make(chan error, 6)
		for i := 0; i < 6; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := l.Lint(context.Background(), root, "a.md", []byte("x"))
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		b, _ := os.ReadFile(counts)
		lines := strings.Fields(string(b))
		if len(lines) != 6 {
			t.Fatalf("vale ran %d times, want 6", len(lines))
		}
		for _, s := range lines {
			if n, _ := strconv.Atoi(s); n > 2 {
				t.Errorf("%d vale processes ran at once", n)
			}
		}
	})
}

// realVale finds a real vale: Find, then the repo's fetched helper.
func realVale(t *testing.T) string {
	t.Helper()
	if p, err := Find(); err == nil {
		return p
	}
	p, _ := filepath.Abs(filepath.Join("..", "..", "..", "build", "darwin", "helpers", runtime.GOARCH, "vale"))
	if isExec(p) {
		return p
	}
	t.Skip("vale not found (run `task darwin:fetch:helpers` or set ZEN_VALE)")
	return ""
}

func TestRealVale(t *testing.T) {
	if testing.Short() {
		t.Skip("runs vale; skipped with -short")
	}
	l := New(realVale(t))
	content := "---\ntitle: Notes\n---\n\n# Héllo\n\nÉté: a speling mistake.\n\n```\ncode speling here\n```\n\nAlso `inline speling`.\n"
	got, err := l.Lint(context.Background(), t.TempDir(), "page.md", []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	var spell []Alert
	for _, a := range got {
		if a.Check == "Vale.Spelling" {
			spell = append(spell, a)
		}
	}
	want := Alert{Line: 7, Col: 8, EndCol: 14, Severity: "error", Check: "Vale.Spelling", Match: "speling"}
	if len(spell) != 1 {
		t.Fatalf("spelling alerts = %+v, want exactly one (code ignored)", spell)
	}
	a := spell[0]
	a.Message = ""
	if a != want {
		t.Errorf("got %+v, want %+v", a, want)
	}
}
