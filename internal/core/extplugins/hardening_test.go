package extplugins

// Regression tests for docs/plugin-review-2026-11.md (the reviewer's probes,
// asserting the fixed behaviour).

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/dragosh/zen/internal/core/plugins"
)

func mkfile(t *testing.T, dir, name, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

const blockYML = "id: x\nversion: 1.0.0\napiVersion: 1\nblocks: [x]\n"

// within fails the test when f does not return in d; a blocked f is unblocked
// with unblock.
func within(t *testing.T, d time.Duration, unblock func(), f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { f(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
		if unblock != nil {
			unblock()
		}
		t.Fatalf("still blocked after %v", d)
	}
}

// H1: a plugin.yml that is a symlink to a file outside the folder is refused.
func TestH1ManifestSymlinkOutsideRefused(t *testing.T) {
	root := t.TempDir()
	pdir := filepath.Join(root, "plugins", "x")
	os.MkdirAll(pdir, 0o755)
	mkfile(t, root, "shared.yml", blockYML+"backend: {command: [echo, benign], hooks: [block]}\n", 0o644)
	os.Symlink("../../shared.yml", filepath.Join(pdir, "plugin.yml"))
	if _, err := Load(pdir); err == nil || !strings.Contains(err.Error(), "outside the plugin folder") {
		t.Fatalf("want an outside-the-folder rejection, got %v", err)
	}
}

// H1: a frontend file that is a symlink out of the folder is refused; one that
// stays inside is hashed by its target's content.
func TestH1SymlinksInsideAndOutside(t *testing.T) {
	root := t.TempDir()
	pdir := writePlugin(t, root, "x", blockYML+"frontend: {js: [link.js]}\n", map[string]string{"real.js": "1"})
	os.Symlink("real.js", filepath.Join(pdir, "link.js"))
	p, err := Load(pdir)
	if err != nil {
		t.Fatal(err)
	}
	h1, _ := p.Hash()
	mkfile(t, pdir, "real.js", "2", 0o644)
	p2, _ := Load(pdir)
	if h2, _ := p2.Hash(); h1 == h2 {
		t.Fatal("changing a symlink's target must change the hash")
	}
	os.Remove(filepath.Join(pdir, "link.js"))
	mkfile(t, root, "out.js", "x", 0o644)
	os.Symlink("../out.js", filepath.Join(pdir, "link.js"))
	if _, err := Load(pdir); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("symlink out of the folder: %v", err)
	}
}

// H1: .DS_Store is hashed like any file (it can be the command).
func TestH1DSStoreIsHashed(t *testing.T) {
	root := t.TempDir()
	pdir := filepath.Join(root, "plugins", "x")
	mkfile(t, pdir, "plugin.yml", blockYML+"backend: {command: [./.DS_Store], hooks: [block]}\n", 0o644)
	mkfile(t, pdir, ".DS_Store", "#!/bin/sh\necho benign\n", 0o755)
	p, err := Load(pdir)
	if err != nil {
		t.Fatal(err)
	}
	h1, _ := p.Hash()
	mkfile(t, pdir, ".DS_Store", "#!/bin/sh\necho EVIL\n", 0o755)
	p2, err := Load(pdir)
	if err != nil {
		t.Fatal(err)
	}
	if h2, _ := p2.Hash(); h1 == h2 {
		t.Fatal("the hash ignores .DS_Store")
	}
	tr := LoadTrust("")
	tr.Set(p, true)
	if _, decided := tr.Decision(p2); decided {
		t.Fatal("a changed .DS_Store must ask again")
	}
}

// H1: an argument that names an existing file outside the folder is refused.
func TestH1ArgumentsOutsideRefused(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "secret.sh", "#!/bin/sh\n", 0o755)
	for name, arg := range map[string]string{"relative": "../secret.sh", "absolute": filepath.Join(root, "secret.sh"), "etc": "/etc/hosts"} {
		pdir := writePlugin(t, filepath.Join(root, name), "x", blockYML+"backend: {command: [sh, \""+arg+"\"], hooks: [block]}\n", nil)
		if _, err := Load(pdir); err == nil || !strings.Contains(err.Error(), "outside the plugin folder") {
			t.Errorf("%s argument %q: %v", name, arg, err)
		}
	}
	// A file inside the folder, and plain words, are fine.
	pdir := writePlugin(t, filepath.Join(root, "ok"), "x", blockYML+"backend: {command: [sh, ./run.sh, -c, hello], hooks: [block]}\n", map[string]string{"run.sh": "#!/bin/sh\n"})
	if _, err := Load(pdir); err != nil {
		t.Fatalf("inside argument: %v", err)
	}
}

// H3: a FIFO as plugin.yml or inside the folder never blocks and rejects the plugin.
func TestH3FIFOIsRejectedNotOpened(t *testing.T) {
	root := t.TempDir()
	pdir := filepath.Join(root, "x")
	os.MkdirAll(pdir, 0o755)
	fifo := filepath.Join(pdir, "plugin.yml")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Fatal(err)
	}
	unblock := func() { f, _ := os.OpenFile(fifo, os.O_WRONLY, 0); f.Close() }
	var inv []Invalid
	within(t, 2*time.Second, unblock, func() { _, inv = Scan(root) })
	if len(inv) != 1 || inv[0].Problem == "" {
		t.Fatalf("want one rejected folder, got %v", inv)
	}

	pdir2 := writePlugin(t, root, "y", strings.Replace(blockYML, "id: x", "id: y", 1), nil)
	fifo2 := filepath.Join(pdir2, "pipe")
	syscall.Mkfifo(fifo2, 0o644)
	unblock2 := func() { f, _ := os.OpenFile(fifo2, os.O_WRONLY, 0); f.Close() }
	var err error
	within(t, 2*time.Second, unblock2, func() { _, err = Load(pdir2) })
	if err == nil || !strings.Contains(err.Error(), "special file") {
		t.Fatalf("a FIFO in the folder: %v", err)
	}
}

// H3: plugin.yml is read through a size limit.
func TestH3ManifestIsLimited(t *testing.T) {
	root := t.TempDir()
	pdir := filepath.Join(root, "x")
	mkfile(t, pdir, "plugin.yml", blockYML+"description: "+strings.Repeat("a", maxManifest)+"\n", 0o644)
	if _, err := Load(pdir); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("big manifest: %v", err)
	}
	// /dev/zero as plugin.yml: refused at once (it also resolves outside the folder).
	pdir2 := filepath.Join(root, "z")
	os.MkdirAll(pdir2, 0o755)
	os.Symlink("/dev/zero", filepath.Join(pdir2, "plugin.yml"))
	within(t, 3*time.Second, nil, func() {
		_, err := Load(pdir2)
		if err == nil {
			t.Error("/dev/zero accepted")
		}
	})
}

const escapeScript = "#!/usr/bin/perl\nuse POSIX;\n$|=1; print \"hello\\n\";\nif (fork()==0) { POSIX::setsid(); for (1..80) { print \"x\" x 1000, \"\\n\"; select(undef,undef,undef,0.05);} exit 0; }\nexit 0;\n"

// H2/L2: a child that left the process group and keeps stdout open ends at the
// deadline with a timeout error, never a truncated success.
func TestH2L2EscapedGrandchildTimesOut(t *testing.T) {
	root := t.TempDir()
	pdir := writePlugin(t, root, "x", blockYML+"backend: {command: [./run.pl], hooks: [block], timeout: 1s}\n", nil)
	mkfile(t, pdir, "run.pl", escapeScript, 0o755)
	p, err := Load(pdir)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	out, err := p.RenderBlock(plugins.Context{}, plugins.Block{Name: "x"})
	took := time.Since(start)
	if err == nil || !strings.Contains(err.Error(), "timed out") || out != "" {
		t.Fatalf("want a timeout error, got %q %v", out, err)
	}
	if took > 1800*time.Millisecond {
		t.Fatalf("returned after %v, the timeout is 1s", took)
	}
}

// H2: cancelling a stream whose child escaped the group returns at once and
// nothing is emitted afterwards.
func TestH2StreamCancelWithEscapedChild(t *testing.T) {
	root := t.TempDir()
	pdir := writePlugin(t, root, "x", "id: x\nversion: 1.0.0\napiVersion: 1\nbackend: {command: [./run.pl], hooks: [stream], timeout: 5s}\n", nil)
	mkfile(t, pdir, "run.pl", "#!/usr/bin/perl\nuse POSIX;\n$|=1;\nif (fork()==0) { POSIX::setsid(); for (1..80) { print \"tick\\n\"; select(undef,undef,undef,0.05);} exit 0; }\nexit 0;\n", 0o755)
	p, err := Load(pdir)
	if err != nil {
		t.Fatal(err)
	}
	c, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(300 * time.Millisecond); cancel() }()
	var mu sync.Mutex
	var last time.Time
	start := time.Now()
	err = p.Stream(c, plugins.Context{}, nil, func([]byte) { mu.Lock(); last = time.Now(); mu.Unlock() })
	returned := time.Now()
	if err != context.Canceled || returned.Sub(start) > 1200*time.Millisecond {
		t.Fatalf("returned after %v with %v", returned.Sub(start), err)
	}
	time.Sleep(500 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if last.After(returned) {
		t.Fatal("emit was called after Stream returned")
	}
}

// L7: names that map to the same variable give one entry; the one that sorts last wins.
func TestL7DuplicateEnvKeys(t *testing.T) {
	p := &Plugin{man: plugins.Manifest{ID: "x"}, back: &backend{}}
	env := p.env(call{hook: "block", attrs: map[string]string{"a-b": "1", "a_b": "2"}, ctx: plugins.Context{Config: map[string]any{"c-d": 1, "c_d": 2}}}, Runtime{})
	n := map[string]string{}
	for _, e := range env {
		if k, v, _ := strings.Cut(e, "="); strings.HasPrefix(k, "ZEN_ATTR_") || strings.HasPrefix(k, "ZEN_CONFIG_") {
			if _, dup := n[k]; dup {
				t.Errorf("duplicate %s", k)
			}
			n[k] = v
		}
	}
	if n["ZEN_ATTR_A_B"] != "2" || n["ZEN_CONFIG_C_D"] != "2" {
		t.Fatalf("last name wins: %v", n)
	}
}

// L10: tools installed for the user are found from the Finder.
func TestL10ToolPath(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	t.Setenv("PATH", "/usr/bin")
	got := ":" + toolPath() + ":"
	for _, d := range []string{"/home/u/.local/bin", "/home/u/.cargo/bin", "/opt/local/bin", "/opt/homebrew/bin"} {
		if !strings.Contains(got, ":"+d+":") {
			t.Errorf("%s missing from %s", d, got)
		}
	}
}
