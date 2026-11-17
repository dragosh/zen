// Package build_test checks the version metadata described in
// docs/VERSIONING.md: the VERSION file, the changelog and build stamping.
package build_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// semverRE is the official SemVer 2.0.0 regular expression (semver.org).
var semverRE = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
	`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
	`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

func readVersion(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../VERSION")
	if err != nil {
		t.Fatalf("read VERSION: %v", err)
	}
	return strings.TrimSuffix(string(b), "\n")
}

func TestVersionFileIsSemver(t *testing.T) {
	v := readVersion(t)
	if strings.ContainsAny(v, "\r\n") {
		t.Fatalf("VERSION must be a single line, got %q", v)
	}
	if !semverRE.MatchString(v) {
		t.Fatalf("VERSION %q is not valid SemVer 2.0.0 (no v prefix)", v)
	}
}

func TestChangelogHasUnreleased(t *testing.T) {
	b, err := os.ReadFile("../CHANGELOG.md")
	if err != nil {
		t.Fatalf("read CHANGELOG.md: %v", err)
	}
	if !regexp.MustCompile(`(?m)^## \[Unreleased\]\s*$`).Match(b) {
		t.Fatal("CHANGELOG.md has no `## [Unreleased]` section")
	}
}

// findTask locates the `task` binary on PATH or in $(go env GOPATH)/bin.
func findTask(t *testing.T) string {
	t.Helper()
	if p, err := exec.LookPath("task"); err == nil {
		return p
	}
	if out, err := exec.Command("go", "env", "GOPATH").Output(); err == nil {
		p := filepath.Join(strings.TrimSpace(string(out)), "bin", "task")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("task is not installed; cannot read the Taskfile's version ldflags")
	return ""
}

// TestBuildStampsVersion builds a small program that runs the real CLI with
// the exact -X flag the Taskfiles add to every `go build`
// (`task version:ldflags`), and checks that `--version` reports VERSION.
func TestBuildStampsVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a binary; skipped with -short")
	}
	version := readVersion(t)

	cmd := exec.Command(findTask(t), "--silent", "version:ldflags")
	cmd.Dir = ".."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("task version:ldflags: %v", err)
	}
	ldflags := strings.TrimSpace(string(out))
	want := "-X github.com/dragosh/zen/internal/cli.Version=" + version
	if ldflags != want {
		t.Fatalf("task version:ldflags = %q, want %q", ldflags, want)
	}

	// The program lives inside the module (so it may import internal/cli) in
	// a directory `go build ./...` ignores because of the leading underscore.
	dir, err := os.MkdirTemp(".", "_stamp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	src := `package main

import (
	"os"

	"github.com/dragosh/zen/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], "/", os.Stdout, os.Stderr, func(cli.Launch) error { return nil }, nil, nil))
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "stamped")
	build := exec.Command("go", "build", "-buildvcs=false", "-ldflags", ldflags, "-o", bin, "./"+dir)
	if b, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, b)
	}
	got, err := exec.Command(bin, "--version").Output()
	if err != nil {
		t.Fatalf("run stamped binary: %v", err)
	}
	if string(got) != "zen "+version+"\n" {
		t.Fatalf("--version printed %q, want %q", got, "zen "+version+"\n")
	}
}
