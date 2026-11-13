package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withLimits(t *testing.T, files, entries int) {
	t.Helper()
	f, e := MaxFiles, MaxEntries
	MaxFiles, MaxEntries = files, entries
	t.Cleanup(func() { MaxFiles, MaxEntries = f, e })
}

func fill(t *testing.T, dir string, n int, ext string) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%03d%s", i, ext)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLimits(t *testing.T) {
	t.Run("at the markdown limit opens", func(t *testing.T) {
		withLimits(t, 5, 100)
		dir := t.TempDir()
		fill(t, dir, 5, ".md")
		if _, err := Open(dir); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("over the markdown limit is refused", func(t *testing.T) {
		withLimits(t, 5, 100)
		dir := t.TempDir()
		fill(t, dir, 6, ".md")
		_, err := Open(dir)
		if !errors.Is(err, ErrTooManyFiles) || !strings.Contains(err.Error(), "more than 5 markdown files") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("too many entries is refused even without markdown", func(t *testing.T) {
		withLimits(t, 5, 10)
		dir := t.TempDir()
		fill(t, dir, 11, ".txt")
		_, err := Open(dir)
		if !errors.Is(err, ErrTooManyFiles) || !strings.Contains(err.Error(), "too large to open") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("skipped folders do not count", func(t *testing.T) {
		withLimits(t, 5, 10)
		dir := t.TempDir()
		nm := filepath.Join(dir, "node_modules")
		if err := os.Mkdir(nm, 0o755); err != nil {
			t.Fatal(err)
		}
		fill(t, nm, 50, ".md")
		fill(t, dir, 3, ".md")
		if _, err := Open(dir); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("Files reports growth past the limit", func(t *testing.T) {
		withLimits(t, 5, 100)
		dir := t.TempDir()
		fill(t, dir, 2, ".md")
		w, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		fill(t, dir, 8, ".markdown")
		if _, err := w.Files(); !errors.Is(err, ErrTooManyFiles) {
			t.Fatalf("got %v", err)
		}
	})
}

func TestOpenWithLimits(t *testing.T) {
	withLimits(t, 5, 100)
	dir := t.TempDir()
	fill(t, dir, 8, ".md")
	if _, err := Open(dir); !errors.Is(err, ErrTooManyFiles) {
		t.Fatalf("default limit: %v", err)
	}
	if _, err := OpenWithLimits(dir, 10, 0); err != nil {
		t.Fatalf("raised limit: %v", err)
	}
	if _, err := OpenWithLimits(dir, 0, 3); err == nil || !strings.Contains(err.Error(), "more than 3 files") {
		t.Fatalf("entries limit: %v", err)
	}
}
