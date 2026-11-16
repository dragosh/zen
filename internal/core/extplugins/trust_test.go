package extplugins

import (
	"os"
	"path/filepath"
	"testing"
)

const tinyYML = "id: p\nversion: 1.0.0\napiVersion: 1\nbackend: { command: [\"./run.sh\"], hooks: [block] }\n"

func tinyPlugin(t *testing.T) *Plugin {
	t.Helper()
	dir := writePlugin(t, t.TempDir(), "p", tinyYML, map[string]string{"run.sh": "#!/bin/sh\ncat\n", "lib/data.txt": "one"})
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestHashCoversEveryFile(t *testing.T) {
	p := tinyPlugin(t)
	h := func() string {
		t.Helper()
		s, err := p.Hash()
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	h0 := h()
	if len(h0) != 64 || h() != h0 {
		t.Fatalf("hash not stable: %q", h0)
	}
	seen := map[string]bool{h0: true}
	step := func(what string, f func()) {
		t.Helper()
		f()
		got := h()
		if seen[got] {
			t.Fatalf("%s: hash did not change", what)
		}
		seen[got] = true
	}
	step("script edited", func() { os.WriteFile(filepath.Join(p.Dir(), "run.sh"), []byte("#!/bin/sh\ncat; echo x\n"), 0o755) })
	step("data edited", func() { os.WriteFile(filepath.Join(p.Dir(), "lib", "data.txt"), []byte("two"), 0o644) })
	step("file added", func() { os.WriteFile(filepath.Join(p.Dir(), "new.txt"), nil, 0o644) })
	step("file renamed", func() { os.Rename(filepath.Join(p.Dir(), "new.txt"), filepath.Join(p.Dir(), "newer.txt")) })
	step("manifest edited", func() { os.WriteFile(filepath.Join(p.Dir(), "plugin.yml"), []byte(tinyYML+"description: x\n"), 0o644) })
	step("symlink added", func() { os.Symlink("/etc/hosts", filepath.Join(p.Dir(), "link")) })
	step("file removed", func() { os.Remove(filepath.Join(p.Dir(), "newer.txt")) })
	// Content moving between files changes the hash too.
	os.WriteFile(filepath.Join(p.Dir(), "a"), []byte("xy"), 0o644)
	os.WriteFile(filepath.Join(p.Dir(), "b"), []byte("z"), 0o644)
	h1 := h()
	os.WriteFile(filepath.Join(p.Dir(), "a"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(p.Dir(), "b"), []byte("yz"), 0o644)
	if h() == h1 {
		t.Fatal("hash ignores file boundaries")
	}
}

func TestTrustDecisions(t *testing.T) {
	p := tinyPlugin(t)
	file := filepath.Join(t.TempDir(), "zen", "trusted-plugins.yml")
	tr := LoadTrust(file)
	if allowed, decided := tr.Decision(p); allowed || decided {
		t.Fatal("a new plugin has no decision")
	}
	if err := tr.Set(p, true); err != nil {
		t.Fatal(err)
	}
	if allowed, decided := tr.Decision(p); !allowed || !decided {
		t.Fatal("allow not recorded")
	}
	fi, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v, want 0600", fi.Mode().Perm())
	}
	// Persisted.
	if allowed, decided := LoadTrust(file).Decision(p); !allowed || !decided {
		t.Fatal("decision not persisted")
	}
	// Any change to the files makes it undecided again.
	os.WriteFile(filepath.Join(p.Dir(), "run.sh"), []byte("#!/bin/sh\nrm -rf /\n"), 0o755)
	if allowed, decided := LoadTrust(file).Decision(p); allowed || decided {
		t.Fatal("a changed plugin kept its approval")
	}
	// Block is remembered as well.
	if err := tr.Set(p, false); err != nil {
		t.Fatal(err)
	}
	if allowed, decided := LoadTrust(file).Decision(p); allowed || !decided {
		t.Fatal("block not recorded")
	}
	// The same files in another place are another plugin.
	other := writePlugin(t, t.TempDir(), "p", tinyYML, map[string]string{"run.sh": "#!/bin/sh\nrm -rf /\n", "lib/data.txt": "one"})
	q, err := Load(other)
	if err != nil {
		t.Fatal(err)
	}
	if hp, _ := p.Hash(); func() string { h, _ := q.Hash(); return h }() != hp {
		t.Fatal("test setup: hashes should match")
	}
	if _, decided := LoadTrust(file).Decision(q); decided {
		t.Fatal("decision leaked to another folder")
	}
	// Two plugins keep separate decisions.
	if err := tr.Set(q, true); err != nil {
		t.Fatal(err)
	}
	tr2 := LoadTrust(file)
	if a, d := tr2.Decision(q); !a || !d {
		t.Fatal("second plugin lost")
	}
	if a, d := tr2.Decision(p); a || !d {
		t.Fatal("first plugin lost")
	}
}

func TestTrustWithoutFile(t *testing.T) {
	p := tinyPlugin(t)
	tr := LoadTrust("") // nowhere to store: decisions last for the session
	if err := tr.Set(p, true); err != nil {
		t.Fatal(err)
	}
	if a, d := tr.Decision(p); !a || !d {
		t.Fatal("session decision lost")
	}
	// A corrupt file is the same as no decisions.
	bad := filepath.Join(t.TempDir(), "trusted-plugins.yml")
	os.WriteFile(bad, []byte("{{{"), 0o600)
	if _, d := LoadTrust(bad).Decision(p); d {
		t.Fatal("corrupt file produced a decision")
	}
}
