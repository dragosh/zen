package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func manyFiles(t *testing.T, n int) string {
	t.Helper()
	dir := t.TempDir()
	for i := 0; i < n; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("p%04d.md", i)), []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestOpenContextReportsAndHandsOutTheListingOnce(t *testing.T) {
	dir := manyFiles(t, 600)
	var seen []int
	w, err := OpenContext(context.Background(), dir, 0, 0, func(n int) { seen = append(seen, n) })
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0] != 256 || seen[1] != 512 {
		t.Errorf("progress = %v, want [256 512]", seen)
	}
	if got := w.Opened(); len(got) != 600 {
		t.Errorf("Opened = %d files", len(got))
	}
	if w.Opened() != nil {
		t.Error("Opened handed the listing out twice")
	}
}

func TestOpenContextStopsWhenCancelled(t *testing.T) {
	dir := manyFiles(t, 600)
	ctx, cancel := context.WithCancel(context.Background())
	_, err := OpenContext(ctx, dir, 0, 0, func(int) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
