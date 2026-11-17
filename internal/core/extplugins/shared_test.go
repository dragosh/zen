package extplugins

// Tests of the pieces other plugin kinds (script plugins) reuse: Trust works
// on any Subject, Guard is the cheap "did it change" check, ScanFolder and
// HashFile are the hardened hashing.

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeSubject struct {
	dir, hash string
	changed   atomic.Bool
}

func (f *fakeSubject) Dir() string           { return f.dir }
func (f *fakeSubject) Hash() (string, error) { return f.hash, nil }
func (f *fakeSubject) Changed() bool         { return f.changed.Load() }

func TestTrustWorksOnAnySubject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trusted-plugins.yml")
	tr := LoadTrust(path)
	s := &fakeSubject{dir: "/w/.zen/plugins/x.tengo", hash: "abc"}
	if _, decided := tr.Decision(s, "/w"); decided {
		t.Fatal("decided before asking")
	}
	if err := tr.Set(s, "/w", true); err != nil {
		t.Fatal(err)
	}
	if allowed, decided := LoadTrust(path).Decision(s, "/w"); !allowed || !decided {
		t.Fatalf("not remembered: %v %v", allowed, decided)
	}
	if _, decided := tr.Decision(s, "/other"); decided {
		t.Error("a decision leaked to another folder")
	}
	s2 := &fakeSubject{dir: s.dir, hash: "different"}
	if _, decided := tr.Decision(s2, "/w"); decided {
		t.Error("a decision survived a change of the hash")
	}
	s2.changed.Store(true)
	if err := tr.Set(s2, "/w", true); err != ErrChanged {
		t.Errorf("allowing a changed subject: %v", err)
	}
	if err := tr.Set(s2, "/w", false); err != nil {
		t.Errorf("blocking a changed subject: %v", err)
	}
}

func TestGuard(t *testing.T) {
	cur := "a"
	g := NewGuard("a", func() (string, error) { return cur, nil })
	calls := 0
	g.OnChange(func() { calls++ })
	if g.Changed() || calls != 0 {
		t.Fatal("changed without a change")
	}
	cur = "b"
	if !g.Changed() || !g.Changed() || calls != 1 {
		t.Fatalf("change not noticed once: calls=%d", calls)
	}
	cur = "a"
	if !g.Changed() {
		t.Error("Changed went back to false")
	}
	// A fingerprint that cannot be taken counts as a change.
	g2 := NewGuard("a", func() (string, error) { return "", os.ErrNotExist })
	if !g2.Changed() {
		t.Error("an unreadable subject is not changed")
	}
}

func TestScanFolderHelper(t *testing.T) {
	dir := writePlugin(t, t.TempDir(), "p", tinyYML, map[string]string{"run.sh": "#!/bin/sh\n"})
	real, hash, fp, err := ScanFolder(dir)
	if err != nil || len(hash) != 64 || fp == "" || real == "" {
		t.Fatalf("ScanFolder: %q %q %q %v", real, hash, fp, err)
	}
	if got, err := FolderFingerprint(real); err != nil || got != fp {
		t.Errorf("FolderFingerprint: %q %v", got, err)
	}
	os.WriteFile(filepath.Join(dir, "run.sh"), []byte("#!/bin/sh\necho more\n"), 0o755)
	if got, _ := FolderFingerprint(real); got == fp {
		t.Error("fingerprint did not change")
	}
	// The hardening is the same: a symlink out of the folder is refused.
	outside := filepath.Join(t.TempDir(), "x")
	os.WriteFile(outside, []byte("x"), 0o644)
	os.Symlink(outside, filepath.Join(dir, "evil"))
	if _, _, _, err := ScanFolder(dir); err == nil || !strings.Contains(err.Error(), "outside the plugin folder") {
		t.Errorf("symlink out: %v", err)
	}
}

func TestHashFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.tengo")
	os.WriteFile(path, []byte("one"), 0o644)
	data, h1, fp1, err := HashFile(path, 1024)
	if err != nil || string(data) != "one" || len(h1) != 64 || fp1 == "" {
		t.Fatalf("HashFile: %q %q %q %v", data, h1, fp1, err)
	}
	if _, h, _, _ := HashFile(path, 1024); h != h1 {
		t.Error("hash not stable")
	}
	os.WriteFile(path, []byte("two"), 0o644)
	os.Chtimes(path, time.Now().Add(2*time.Second), time.Now().Add(2*time.Second))
	_, h2, fp2, _ := HashFile(path, 1024)
	if h2 == h1 || fp2 == fp1 {
		t.Error("hash or fingerprint did not change")
	}
	if got, err := FileFingerprint(path); err != nil || got != fp2 {
		t.Errorf("FileFingerprint: %q %v", got, err)
	}
	// Too large, a symlink (even to a file in the same folder), a folder, a missing file.
	if _, _, _, err := HashFile(path, 2); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Errorf("too large: %v", err)
	}
	link := filepath.Join(dir, "l.tengo")
	os.Symlink(path, link)
	if _, _, _, err := HashFile(link, 1024); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Errorf("symlink: %v", err)
	}
	if _, _, _, err := HashFile(dir, 1024); err == nil {
		t.Error("a folder was hashed as a file")
	}
	if _, _, _, err := HashFile(filepath.Join(dir, "none"), 1024); err == nil {
		t.Error("a missing file was hashed")
	}
}

func TestKind(t *testing.T) {
	if got := tinyPlugin(t).Kind(); got != "program" {
		t.Errorf("backend plugin kind %q", got)
	}
	dir := writePlugin(t, t.TempDir(), "pg", "id: pg\nversion: 1.0.0\napiVersion: 1\nblocks: [pg]\nfrontend: { js: [a.js] }\n", map[string]string{"a.js": "export default {}"})
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Kind(); got != "page" {
		t.Errorf("frontend-only plugin kind %q", got)
	}
}

func TestParseOption(t *testing.T) {
	o, err := ParseOption("bool", true, "d", "")
	if err != nil || o.Default != true || o.Type != "bool" || o.Description != "d" {
		t.Fatalf("%+v %v", o, err)
	}
	if _, err := ParseOption("bool", "yes", "", ""); err == nil {
		t.Error("a default of the wrong type")
	}
	if _, err := ParseOption("float", nil, "", ""); err == nil {
		t.Error("an unknown type")
	}
	if _, err := ParseOption("string", nil, "", "folder"); err == nil {
		t.Error("an unknown scope")
	}
	if o, err := ParseOption("list", []any{"a"}, "", "user"); err != nil || o.Scope != "user" {
		t.Errorf("%+v %v", o, err)
	}
}

func TestContainmentHelpers(t *testing.T) {
	real, _ := filepath.EvalSymlinks(t.TempDir())
	os.MkdirAll(filepath.Join(real, "sub"), 0o755)
	os.WriteFile(filepath.Join(real, "sub", "f.txt"), []byte("x"), 0o644)
	outside := filepath.Join(t.TempDir(), "o.txt")
	os.WriteFile(outside, []byte("o"), 0o644)
	os.Symlink(outside, filepath.Join(real, "out"))
	os.Symlink("sub/f.txt", filepath.Join(real, "in"))

	if _, err := ResolveInside(real, "sub/f.txt", "file"); err != nil {
		t.Error(err)
	}
	if _, err := ResolveInside(real, "in", "file"); err != nil {
		t.Errorf("symlink inside: %v", err)
	}
	for _, bad := range []string{"out", "../x", "/etc/hosts", "sub/../../x"} {
		if _, err := ResolveInside(real, bad, "file"); err == nil {
			t.Errorf("ResolveInside accepted %q", bad)
		}
	}
	if err := CheckInside(real, "sub/f.txt", "file"); err != nil {
		t.Error(err)
	}
	if err := CheckInside(real, "sub", "file"); err == nil {
		t.Error("a folder passed CheckInside")
	}
	if !InsideRel(real, filepath.Join(real, "sub")) || InsideRel(real, filepath.Dir(real)) {
		t.Error("InsideRel")
	}
	if data, err := ReadRegular(filepath.Join(real, "sub", "f.txt"), 10); err != nil || string(data) != "x" {
		t.Errorf("ReadRegular: %q %v", data, err)
	}
	if _, err := ReadRegular(filepath.Join(real, "sub", "f.txt"), 0); err == nil {
		t.Error("ReadRegular ignored the limit")
	}
}
