package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// oldCommand finds the retired command name used as a command: `zen build`,
// `zen init …`, `zen --version`, `zen .`, a path argument, or a code line that
// starts with it. Running text such as "zen reads a folder" does not match.
var oldCommand = regexp.MustCompile("(^|[\\s`$(&|])zen( (build|init|edit|help|version)\\b| -{1,2}[a-z]| \\.(\\s|$|`)| \\[| [\\w.~-]+/)|&& zen\\s*$|`zen` \\|")

// docFiles lists the files the cli-rename spec owns for command examples.
func docFiles(t *testing.T) []string {
	t.Helper()
	root := filepath.Join("..", "..")
	var files []string
	add := func(p string) { files = append(files, p) }
	add(filepath.Join(root, "README.md"))
	ms, _ := filepath.Glob(filepath.Join(root, "docs", "*.md"))
	for _, m := range ms {
		switch filepath.Base(m) {
		case "ENGINE.md", "PLAN.md":
		default:
			add(m)
		}
	}
	for _, dir := range []string{"examples/demo", "examples/showcase", "builtins/sitetemplates"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			ext := filepath.Ext(p)
			if !d.IsDir() && (ext == ".md" || (dir == "builtins/sitetemplates" && (ext == ".yml" || ext == ".txt"))) {
				add(p)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return files
}

func TestDocsUseZenmd(t *testing.T) {
	files := docFiles(t)
	if len(files) < 10 {
		t.Fatalf("only %d files found", len(files))
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if oldCommand.MatchString(line) {
				t.Errorf("%s:%d still uses the zen command: %s", f, i+1, strings.TrimSpace(line))
			}
		}
	}
}
