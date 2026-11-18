package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLimits(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "config.yml")
	ws := t.TempDir()

	if got := Load("", "").Limits(); got != (Limits{}) {
		t.Fatalf("default: %+v", got)
	}

	writeFile(t, global, "limits:\n  files: 5000\n  entries: 200000\n  openTimeout: 90\n")
	if got := Load(global, "").Limits(); got != (Limits{Files: 5000, Entries: 200000, OpenTimeout: 90}) {
		t.Fatalf("global: %+v", got)
	}

	writeFile(t, filepath.Join(ws, WorkspaceFile), "limits:\n  files: 99999\n")
	c := Load(global, ws)
	if got := c.Limits(); got.Files != 5000 {
		t.Fatalf("workspace must not change limits: %+v", got)
	}
	if !strings.Contains(strings.Join(c.Warnings(), "\n"), "limits can only be set in the global config") {
		t.Fatalf("warnings: %v", c.Warnings())
	}

	writeFile(t, global, "limits:\n  files: 0\n  entries: 99999999\n  depth: 3\n  openTimeout: 9999\n")
	c = Load(global, "")
	if got := c.Limits(); got != (Limits{Files: 0, Entries: MaxLimitEntries, OpenTimeout: MaxOpenTimeout}) {
		t.Fatalf("invalid/capped: %+v", got)
	}
	w := strings.Join(c.Warnings(), "\n")
	for _, want := range []string{"limits.files must be a whole number", "limits.entries is capped", "unknown key limits.depth"} {
		if !strings.Contains(w, want) {
			t.Errorf("missing warning %q in %s", want, w)
		}
	}

	writeFile(t, global, "limits: 5\n")
	if got := Load(global, "").Limits(); got != (Limits{}) {
		t.Fatalf("non-mapping: %+v", got)
	}
}
