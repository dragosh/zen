package session_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zenmark-org/zenmd/engine/session"
	"github.com/zenmark-org/zenmd/engine/session/sessiontest"
)

const helloPlugin = `id: hello
version: 0.1.0
apiVersion: 1
description: Says hello
default: true
blocks: [hello]
backend:
  command: ["./hello.sh"]
  hooks: [block, action]
`

// With no prompter the session never asks and never decides: a folder's
// plugin stays undecided, it cannot be allowed, and it never runs.
func TestNilPrompterNeverAllows(t *testing.T) {
	dir := sessiontest.Tree(t, map[string]string{
		"README.md":                     "# Home\n\n```hello\nx\n```\n",
		".zen/plugins/hello/plugin.yml": helloPlugin,
		".zen/plugins/hello/hello.sh":   "#!/bin/sh\ntouch \"$ZEN_WORKSPACE/ran.txt\"\nprintf '<p>hi</p>'\n",
	})
	if err := os.Chmod(filepath.Join(dir, ".zen/plugins/hello/hello.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := session.New(session.Options{ConfigDir: t.TempDir()})
	t.Cleanup(s.Close)
	if err := s.Open(dir); err != nil {
		t.Fatal(err)
	}
	status := func() string {
		for _, p := range s.Plugins() {
			if p.ID == "hello" {
				return p.Status
			}
		}
		t.Fatal("hello is not listed")
		return ""
	}
	if got := status(); got != "undecided" {
		t.Fatalf("status %q, want undecided", got)
	}
	if err := s.PluginTrust("hello", true); err == nil {
		t.Error("allowing without a prompter did not fail")
	}
	if got := status(); got != "undecided" {
		t.Errorf("status %q after an allow without a prompter", got)
	}
	if _, err := s.Page("README.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ran.txt")); err == nil {
		t.Error("an undecided plugin ran")
	}
}
