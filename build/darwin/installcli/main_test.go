package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeApp makes <tmp>/Zenmark.app/Contents/MacOS/zenmd and returns the app path.
func fakeApp(t *testing.T, root string) string {
	t.Helper()
	app := filepath.Join(root, "Zenmark.app")
	macos := filepath.Join(app, "Contents", "MacOS")
	if err := os.MkdirAll(macos, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"zenmd", "zen"} { // zen: what an older bundle had
		if err := os.WriteFile(filepath.Join(macos, n), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return app
}

func do(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var o, e bytes.Buffer
	code = run(args, &o, &e)
	return code, o.String(), e.String()
}

func target(t *testing.T, p string) string {
	t.Helper()
	got, err := os.Readlink(p)
	if err != nil {
		t.Fatalf("%s is not a link: %v", p, err)
	}
	return got
}

func TestInstallLinks(t *testing.T) {
	root := t.TempDir()
	app := fakeApp(t, root)
	dir := filepath.Join(root, "bin", "nested") // created when missing
	want := filepath.Join(app, "Contents", "MacOS", "zenmd")
	for i := 0; i < 2; i++ { // twice: idempotent
		if code, _, errOut := do(t, "-app", app, "-dir", dir); code != 0 || errOut != "" {
			t.Fatalf("run %d: code %d, stderr %q", i, code, errOut)
		}
		for _, n := range []string{"zenmd", "zen"} {
			if got := target(t, filepath.Join(dir, n)); got != want {
				t.Errorf("%s -> %s, want %s", n, got, want)
			}
		}
	}
}

func TestInstallNeedsTheApp(t *testing.T) {
	root := t.TempDir()
	if code, _, errOut := do(t, "-app", filepath.Join(root, "Zenmark.app"), "-dir", filepath.Join(root, "bin")); code == 0 || !strings.Contains(errOut, "Zenmark.app") {
		t.Fatalf("code %d, stderr %q", code, errOut)
	}
	if _, err := os.Lstat(filepath.Join(root, "bin", "zenmd")); err == nil {
		t.Fatal("linked a missing app")
	}
}

func TestInstallReplacesOldLinks(t *testing.T) {
	root := t.TempDir()
	app := fakeApp(t, root)
	oldApp := filepath.Join(root, "old", "Zenmark.app") // gone: a dangling link
	dir := filepath.Join(root, "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(app, "Contents", "MacOS", "zen")
	if err := os.Symlink(old, filepath.Join(dir, "zen")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(oldApp, "Contents", "MacOS", "zenmd"), filepath.Join(dir, "zenmd")); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := do(t, "-app", app, "-dir", dir); code != 0 || errOut != "" {
		t.Fatalf("code %d, stderr %q", code, errOut)
	}
	want := filepath.Join(app, "Contents", "MacOS", "zenmd")
	for _, n := range []string{"zenmd", "zen"} {
		if got := target(t, filepath.Join(dir, n)); got != want {
			t.Errorf("%s -> %s, want %s", n, got, want)
		}
	}
}

func TestLinkChainIsOurs(t *testing.T) {
	root := t.TempDir()
	app := fakeApp(t, root)
	dir := filepath.Join(root, "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// zen -> zenmd -> the bundle
	if err := os.Symlink(filepath.Join(app, "Contents", "MacOS", "zenmd"), filepath.Join(dir, "zenmd")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("zenmd", filepath.Join(dir, "zen")); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := do(t, "-app", app, "-dir", dir, "-uninstall"); code != 0 {
		t.Fatalf("code %d", code)
	}
	for _, n := range []string{"zenmd", "zen"} {
		if _, err := os.Lstat(filepath.Join(dir, n)); err == nil {
			t.Errorf("%s was not removed", n)
		}
	}
}

func TestForeignZenLeftAlone(t *testing.T) {
	root := t.TempDir()
	app := fakeApp(t, root)
	dir := filepath.Join(root, "bin")
	other := filepath.Join(root, "tools", "zen")
	for _, p := range []string{other, filepath.Join(dir, "x")} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(other, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, filepath.Join(dir, "zen")); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := do(t, "-app", app, "-dir", dir)
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, errOut)
	}
	if !strings.Contains(errOut, "zen") || !strings.Contains(strings.ToLower(errOut), "warning") {
		t.Errorf("no warning: %q", errOut)
	}
	if got := target(t, filepath.Join(dir, "zen")); got != other {
		t.Errorf("foreign zen now -> %s", got)
	}
	if got, want := target(t, filepath.Join(dir, "zenmd")), filepath.Join(app, "Contents", "MacOS", "zenmd"); got != want {
		t.Errorf("zenmd -> %s, want %s", got, want)
	}

	// A regular file called zen is also left alone.
	dir2 := filepath.Join(root, "bin2")
	if err := os.MkdirAll(dir2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir2, "zen"), []byte("mine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := do(t, "-app", app, "-dir", dir2); code != 0 {
		t.Fatalf("code %d", code)
	}
	if b, _ := os.ReadFile(filepath.Join(dir2, "zen")); string(b) != "mine" {
		t.Errorf("regular file replaced: %q", b)
	}
}

func TestUninstallOnlyOurs(t *testing.T) {
	root := t.TempDir()
	app := fakeApp(t, root)
	dir := filepath.Join(root, "bin")
	if code, _, _ := do(t, "-app", app, "-dir", dir); code != 0 {
		t.Fatalf("install: %d", code)
	}
	// Our links go; a foreign zen stays.
	other := filepath.Join(root, "tools", "zen")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "zen")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, filepath.Join(dir, "zen")); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := do(t, "-app", app, "-dir", dir, "-uninstall"); code != 0 {
		t.Fatalf("uninstall: %d", code)
	}
	if _, err := os.Lstat(filepath.Join(dir, "zenmd")); err == nil {
		t.Error("zenmd not removed")
	}
	if got := target(t, filepath.Join(dir, "zen")); got != other {
		t.Errorf("foreign zen -> %s", got)
	}
	// Nothing there at all is fine.
	if code, _, _ := do(t, "-app", app, "-dir", filepath.Join(root, "empty"), "-uninstall"); code != 0 {
		t.Errorf("uninstall of nothing: %d", code)
	}
}

// Switching from a development link (bin/zenmd of a checkout, or the bin/zen
// made before the rename) to the installed app needs no manual rm.
func TestDevLinksAreOurs(t *testing.T) {
	root := t.TempDir()
	app := fakeApp(t, root)
	checkout := filepath.Join(root, "src", "zen")
	for _, p := range []string{filepath.Join(checkout, "bin", "zenmd"), filepath.Join(checkout, "bin", "zen")} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(checkout, "build", "darwin", "installcli"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "local", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(checkout, "bin", "zenmd"), filepath.Join(dir, "zenmd")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(checkout, "bin", "zen"), filepath.Join(dir, "zen")); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := do(t, "-app", app, "-dir", dir)
	if code != 0 || strings.Contains(strings.ToLower(errOut), "warning") {
		t.Fatalf("code %d, stderr %q", code, errOut)
	}
	want := filepath.Join(app, "Contents", "MacOS", "zenmd")
	for _, n := range []string{"zenmd", "zen"} {
		if got := target(t, filepath.Join(dir, n)); got != want {
			t.Errorf("%s -> %s, want %s", n, got, want)
		}
	}
}

// Links into the bundle's old name (Zen.app, before the Zenmark rename) are replaced.
func TestOldBundleNameIsOurs(t *testing.T) {
	root := t.TempDir()
	app := fakeApp(t, root)
	old := filepath.Join(root, "Zen.app", "Contents", "MacOS", "zenmd")
	if err := os.MkdirAll(filepath.Dir(old), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"zenmd", "zen"} {
		if err := os.Symlink(old, filepath.Join(dir, n)); err != nil {
			t.Fatal(err)
		}
	}
	code, _, errOut := do(t, "-app", app, "-dir", dir)
	if code != 0 || strings.Contains(strings.ToLower(errOut), "warning") {
		t.Fatalf("code %d, stderr %q", code, errOut)
	}
	want := filepath.Join(app, "Contents", "MacOS", "zenmd")
	for _, n := range []string{"zenmd", "zen"} {
		if got := target(t, filepath.Join(dir, n)); got != want {
			t.Errorf("%s -> %s, want %s", n, got, want)
		}
	}
}
