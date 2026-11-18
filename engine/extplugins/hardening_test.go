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

	"github.com/zenmark-org/zenmd/engine/plugins"
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

// H1: a .DS_Store can never be run or served (so leaving Finder's files out
// of the hash is safe), and Finder touching one does not ask to approve again.
func TestH1DSStoreIsNeverUsedAndNotHashed(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct{ name, yml string }{
		{"command", blockYML + "backend: {command: [./.DS_Store], hooks: [block]}\n"},
		{"argument", blockYML + "backend: {command: [sh, .DS_Store], hooks: [block]}\n"},
		{"symlinked command", blockYML + "backend: {command: [./run.sh], hooks: [block]}\n"},
		{"frontend", blockYML + "frontend: {js: [.DS_Store]}\n"},
	} {
		pdir := filepath.Join(root, "plugins", strings.ReplaceAll(tc.name, " ", "-"))
		mkfile(t, pdir, "plugin.yml", strings.Replace(tc.yml, "id: x", "id: "+filepath.Base(pdir), 1), 0o644)
		mkfile(t, pdir, ".DS_Store", "#!/bin/sh\necho EVIL\n", 0o755)
		if tc.name == "symlinked command" {
			if err := os.Symlink(".DS_Store", filepath.Join(pdir, "run.sh")); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := Load(pdir); err == nil || !strings.Contains(err.Error(), ".DS_Store") {
			t.Errorf("%s: want a .DS_Store refusal, got %v", tc.name, err)
		}
	}

	// A plugin that does not use it: Finder rewriting .DS_Store changes nothing.
	pdir := filepath.Join(root, "plugins", "ok")
	mkfile(t, pdir, "plugin.yml", strings.Replace(blockYML, "id: x", "id: ok", 1)+"backend: {command: [./run.sh], hooks: [block]}\n", 0o644)
	mkfile(t, pdir, "run.sh", "#!/bin/sh\necho ok\n", 0o755)
	p, err := Load(pdir)
	if err != nil {
		t.Fatal(err)
	}
	h1, _ := p.Hash()
	mkfile(t, pdir, ".DS_Store", "finder state", 0o644)
	p2, err := Load(pdir)
	if err != nil {
		t.Fatal(err)
	}
	if h2, _ := p2.Hash(); h1 != h2 {
		t.Fatal("Finder's .DS_Store changed the hash")
	}
	if p.Changed() { // loaded before Finder wrote the file
		t.Fatal("Finder's .DS_Store marked the plugin changed")
	}
}

// H1: an argument that names an existing file outside the folder is refused.
func TestH1ArgumentsOutsideRefused(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "secret.sh", "#!/bin/sh\n", 0o755)
	for name, arg := range map[string]string{"relative": "../../secret.sh", "absolute": filepath.Join(root, "secret.sh"), "etc": "/etc/hosts"} {
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

// M1: a plugin whose folder changed after load does not run and serves nothing.
func TestM1ChangedPluginDoesNotRunOrServe(t *testing.T) {
	dir := writePlugin(t, t.TempDir(), "x", blockYML+"backend: {command: [./run.sh], hooks: [block]}\nfrontend: {js: [x.js]}\n",
		map[string]string{"run.sh": "#!/bin/sh\necho ok\n", "x.js": "1"})
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	p.OnChange(func() { calls++ })
	if p.Changed() {
		t.Fatal("unchanged folder reported as changed")
	}
	if out, err := p.RenderBlock(plugins.Context{}, plugins.Block{Name: "x"}); err != nil || !strings.Contains(out, "ok") {
		t.Fatalf("before: %q %v", out, err)
	}
	if _, err := p.Assets().Open("x.js"); err != nil {
		t.Fatal(err)
	}
	mkfile(t, dir, "run.sh", "#!/bin/sh\necho EVIL\n", 0o755)
	if _, err := p.RenderBlock(plugins.Context{}, plugins.Block{Name: "x"}); err != ErrChanged {
		t.Fatalf("after the edit: %v", err)
	}
	if _, err := p.Assets().Open("x.js"); err == nil {
		t.Fatal("an asset of a changed plugin was served")
	}
	if !p.Changed() || calls != 1 {
		t.Fatalf("OnChange must be called once, got %d", calls)
	}
	// Even if the file is put back, the loaded plugin stays refused until it is reloaded.
	mkfile(t, dir, "run.sh", "#!/bin/sh\necho ok\n", 0o755)
	if _, err := p.RenderBlock(plugins.Context{}, plugins.Block{Name: "x"}); err == nil {
		t.Fatal("ran after a change was seen")
	}
}

// M1: adding a file or an argument file outside after load is caught.
func TestM1NewFileAndLateArgument(t *testing.T) {
	root := t.TempDir()
	late := filepath.Join(root, "late.sh")
	dir := writePlugin(t, filepath.Join(root, "p"), "x", blockYML+"backend: {command: [sh, "+late+"], hooks: [block]}\n", nil)
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	mkfile(t, root, "late.sh", "echo EVIL\n", 0o644) // names a file only now
	if _, err := p.RenderBlock(plugins.Context{}, plugins.Block{Name: "x"}); err == nil || !strings.Contains(err.Error(), "outside the plugin folder") {
		t.Fatalf("late argument file: %v", err)
	}
}

// M2: approval is for the files as loaded; an allow after a change is refused.
func TestM2ApprovalIsForTheLoadedHash(t *testing.T) {
	p := tinyPlugin(t)
	tr := LoadTrust("")
	os.WriteFile(filepath.Join(p.Dir(), "run.sh"), []byte("#!/bin/sh\nevil\n"), 0o755)
	if err := tr.Set(p, "", true); err != ErrChanged {
		t.Fatalf("Set after a change: %v", err)
	}
	if _, decided := tr.Decision(p, ""); decided {
		t.Fatal("recorded anyway")
	}
	if v := p.AssetVersion(); len(v) != 12 {
		t.Fatalf("asset version %q", v)
	}
}

// L5: the same plugin folder is a different plugin in another workspace.
func TestL5TrustIsPerWorkspace(t *testing.T) {
	p := tinyPlugin(t)
	file := filepath.Join(t.TempDir(), "trusted-plugins.yml")
	tr := LoadTrust(file)
	if err := tr.Set(p, "/work/a", true); err != nil {
		t.Fatal(err)
	}
	if a, d := LoadTrust(file).Decision(p, "/work/a"); !a || !d {
		t.Fatal("allowed in its own workspace")
	}
	if _, d := LoadTrust(file).Decision(p, "/work/b"); d {
		t.Fatal("decision leaked to another workspace")
	}
	if _, d := LoadTrust(file).Decision(p, ""); d {
		t.Fatal("decision leaked to the user scope")
	}
}

// L3: a call is cancelled with the plugin's context.
func TestL3RunIsCancelled(t *testing.T) {
	p, _, _ := scripted(t, `{command: ["./run.sh"], hooks: [block], timeout: 30s}`, "sleep 20")
	c, cancel := context.WithCancel(context.Background())
	p.Bind(Runtime{Workspace: t.TempDir(), Ctx: c})
	go func() { time.Sleep(200 * time.Millisecond); cancel() }()
	start := time.Now()
	_, err := p.RenderBlock(plugins.Context{}, plugins.Block{Name: "p"})
	if err == nil || !strings.Contains(err.Error(), "cancelled") || time.Since(start) > 3*time.Second {
		t.Fatalf("%v after %v", err, time.Since(start))
	}
}

// L8: a corrupt trust file is set aside and reported once, never overwritten;
// saves are atomic and private.
func TestL8CorruptTrustFileIsSetAside(t *testing.T) {
	p := tinyPlugin(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "trusted-plugins.yml")
	os.WriteFile(file, []byte("{{{ not yaml"), 0o600)
	tr := LoadTrust(file)
	n := tr.Notice()
	if !strings.Contains(n, "corrupt") || tr.Notice() != "" {
		t.Fatalf("notice once: %q", n)
	}
	matches, _ := filepath.Glob(file + ".corrupt-*")
	if len(matches) != 1 {
		t.Fatalf("the corrupt file must be kept aside: %v", matches)
	}
	if b, _ := os.ReadFile(matches[0]); string(b) != "{{{ not yaml" {
		t.Fatal("the set-aside file changed")
	}
	if err := tr.Set(p, "", true); err != nil {
		t.Fatal(err)
	}
	if a, d := LoadTrust(file).Decision(p, ""); !a || !d {
		t.Fatal("new decision lost")
	}
	if tmp, _ := filepath.Glob(filepath.Join(dir, ".tmp-*")); len(tmp) != 0 {
		t.Fatalf("temporary files left: %v", tmp)
	}
}
