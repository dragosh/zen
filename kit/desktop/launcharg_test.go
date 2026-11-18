package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLaunchArg(t *testing.T) {
	wd, _ := os.Getwd()
	args := []string{"edit", "guides/install.md"}
	for _, c := range []struct {
		path string
		want bool
	}{
		{"edit", true},                                 // relative: never from Finder
		{"guides/install.md", true},                    // relative
		{filepath.Join(wd, "guides/install.md"), true}, // our own argument, made absolute
		{"/Users/someone/docs", false},                 // a real Open With
		{filepath.Join(wd, "other.md"), false},
	} {
		if got := launchArg(c.path, args); got != c.want {
			t.Errorf("launchArg(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
