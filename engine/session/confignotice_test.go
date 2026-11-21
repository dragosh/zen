package session

import (
	"github.com/zenmark-org/zenmd/builtins"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	legacyNotice = "This folder's settings are in .zen.yml; Zenmark now calls the file .zenmd.yml."
	bothNotice   = ".zen.yml is ignored because .zenmd.yml exists."
)

func TestConfigNotice(t *testing.T) {
	// Legacy only: read as before, with the notice.
	dir := tree(t, map[string]string{"README.md": mystPage, ".zen.yml": "# mine\nversion: 1\nmarkdown:\n  dialects: [myst]\n"})
	s := dialectSession(t, dir, t.TempDir(), false, false)
	if got := s.ConfigNotice(); got != legacyNotice {
		t.Errorf("legacy: %q", got)
	}
	if !strings.Contains(pageHTML(t, s, "README.md"), `data-zen-plugin="note"`) {
		t.Error("the legacy file was not read")
	}
	// The notice is a page warning too: it reaches the Problems list the way any warning does.
	p, err := s.Page("README.md")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range p.Warnings {
		found = found || w == legacyNotice
	}
	if !found {
		t.Errorf("page warnings: %v", p.Warnings)
	}
	if path, _, err := s.ConfigFile("workspace"); err != nil || filepath.Base(path) != ".zen.yml" {
		t.Errorf("ConfigFile while legacy: %q %v", path, err)
	}
	if ws := s.Workspace(); ws.ConfigFile != ".zen.yml" {
		t.Errorf("Workspace.ConfigFile: %q", ws.ConfigFile)
	}

	// Both: the new one is read, the old ignored, and the notice says so.
	both := tree(t, map[string]string{"README.md": mystPage, ".zen.yml": "markdown:\n  dialects: [myst]\n", ".zenmd.yml": "version: 1\n"})
	sb := dialectSession(t, both, t.TempDir(), false, false)
	if got := sb.ConfigNotice(); got != bothNotice {
		t.Errorf("both: %q", got)
	}
	if strings.Contains(pageHTML(t, sb, "README.md"), `data-zen-plugin="note"`) {
		t.Error(".zen.yml was applied although .zenmd.yml exists")
	}
	if path, _, _ := sb.ConfigFile("workspace"); filepath.Base(path) != ".zenmd.yml" {
		t.Errorf("ConfigFile with both: %q", path)
	}

	// New name only, and no file: no notice.
	for name, files := range map[string]map[string]string{
		"new":  {"README.md": "# a\n", ".zenmd.yml": "version: 1\n"},
		"none": {"README.md": "# a\n"},
	} {
		sn := dialectSession(t, tree(t, files), t.TempDir(), false, false)
		if got := sn.ConfigNotice(); got != "" {
			t.Errorf("%s: %q", name, got)
		}
		if ws := sn.Workspace(); ws.ConfigFile != ".zenmd.yml" {
			t.Errorf("%s: Workspace.ConfigFile %q", name, ws.ConfigFile)
		}
	}
	none := New(Options{Registry: noteRegistry(t), Events: &sink{}})
	t.Cleanup(none.Close)
	if none.ConfigNotice() != "" {
		t.Error("a notice without a folder")
	}
}

func TestConfigNoticeLoggedOnce(t *testing.T) {
	h := &recHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })

	dir := tree(t, map[string]string{"README.md": "# a\n", ".zen.yml": "version: 1\n"})
	s := dialectSession(t, dir, t.TempDir(), false, false)
	for i := 0; i < 3; i++ { // reloads do not repeat it
		s.reloadConfig()
	}
	want := "WARN " + legacyNotice + " source=workspace"
	if n := strings.Count(h.all(), want); n != 1 {
		t.Errorf("logged %d times, want once:\n%s", n, h.all())
	}
}

// Behaviour 4: the first write renames .zen.yml, keeping its bytes and comments.
func TestFirstWriteMovesLegacyFile(t *testing.T) {
	dir := tree(t, map[string]string{"README.md": mystPage, ".zen.yml": "# mine\nversion: 1\n"})
	s := dialectSession(t, dir, t.TempDir(), false, false)
	if err := s.SetDialect("myst", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, ".zen.yml")); err == nil {
		t.Error(".zen.yml is still there")
	}
	b, _ := os.ReadFile(filepath.Join(dir, ".zenmd.yml"))
	if !strings.HasPrefix(string(b), "# mine\nversion: 1\n") || !strings.Contains(string(b), "dialects: [myst]") {
		t.Errorf(".zenmd.yml:\n%s", b)
	}
	if got := s.ConfigNotice(); got != "" {
		t.Errorf("notice after the move: %q", got)
	}
	if ws := s.Workspace(); ws.ConfigFile != ".zenmd.yml" {
		t.Errorf("Workspace.ConfigFile: %q", ws.ConfigFile)
	}
}

func TestWriteWithBothFilesGoesToTheNewOne(t *testing.T) {
	dir := tree(t, map[string]string{"README.md": mystPage, ".zen.yml": "version: 1\n# old\n", ".zenmd.yml": "version: 1\n"})
	s := dialectSession(t, dir, t.TempDir(), false, false)
	if err := s.SetDialect("myst", true); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, ".zen.yml")); string(b) != "version: 1\n# old\n" {
		t.Errorf(".zen.yml was touched: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, ".zenmd.yml")); !strings.Contains(string(b), "dialects: [myst]") {
		t.Errorf(".zenmd.yml: %q", b)
	}
}

func TestSaveBuildDefaultsMovesLegacyFile(t *testing.T) {
	dir := tree(t, map[string]string{"README.md": "# a\n", ".zen.yml": "# keep me\nversion: 1\n"})
	s := dialectSession(t, dir, t.TempDir(), false, false)
	if _, err := s.SaveBuildDefaults(BuildOptions{Drafts: true}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, ".zen.yml")); err == nil {
		t.Error(".zen.yml is still there")
	}
	b, _ := os.ReadFile(filepath.Join(dir, ".zenmd.yml"))
	if !strings.Contains(string(b), "# keep me") || !strings.Contains(string(b), "drafts: true") {
		t.Errorf(".zenmd.yml:\n%s", b)
	}
	// The preview (no confirm) writes nothing.
	dir2 := tree(t, map[string]string{"README.md": "# a\n", ".zen.yml": "version: 1\n"})
	s2 := dialectSession(t, dir2, t.TempDir(), false, false)
	if _, err := s2.SaveBuildDefaults(BuildOptions{Drafts: true}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir2, ".zen.yml")); err != nil {
		t.Error("the preview moved the file")
	}
}

func TestFailedMoveLeavesLegacyFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	dir := tree(t, map[string]string{"README.md": mystPage, ".zen.yml": "version: 1\n"})
	s := dialectSession(t, dir, t.TempDir(), false, false)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	if err := s.SetDialect("myst", true); err == nil {
		t.Error("want the rename error")
	}
	os.Chmod(dir, 0o755)
	if b, _ := os.ReadFile(filepath.Join(dir, ".zen.yml")); string(b) != "version: 1\n" {
		t.Errorf(".zen.yml changed: %q", b)
	}
}

// A read-only app never renames.
func TestReadOnlyNeverMoves(t *testing.T) {
	dir := tree(t, map[string]string{"README.md": mystPage, ".zen.yml": "version: 1\n"})
	s := dialectSession(t, dir, t.TempDir(), true, false)
	before := snapshot(t, dir)
	if err := s.SetDialect("myst", true); err == nil {
		t.Error("want read-only")
	}
	if p, err := s.EnsureConfig("workspace"); err != nil || filepath.Base(p) != ".zen.yml" {
		t.Errorf("EnsureConfig: %q %v", p, err) // it only names the file in use; nothing is written
	}
	if after := snapshot(t, dir); len(after) != len(before) || after[".zen.yml"] != before[".zen.yml"] {
		t.Errorf("the folder changed: %v", after)
	}
	if _, err := os.Stat(filepath.Join(dir, ".zenmd.yml")); err == nil {
		t.Error(".zenmd.yml was created")
	}
}

// Creating (Settings › Configuration, the editor) makes .zenmd.yml, and does not add one beside a legacy file.
func TestEnsureConfigNames(t *testing.T) {
	dir := tree(t, map[string]string{"README.md": "# a\n"})
	s := dialectSession(t, dir, t.TempDir(), false, false)
	p, err := s.EnsureConfig("workspace")
	if err != nil || filepath.Base(p) != ".zenmd.yml" {
		t.Fatalf("EnsureConfig: %q %v", p, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".zenmd.yml")); err != nil {
		t.Error(err)
	}
	old := tree(t, map[string]string{"README.md": "# a\n", ".zen.yml": "version: 1\n"})
	so := dialectSession(t, old, t.TempDir(), false, false)
	if p, err := so.EnsureConfig("workspace"); err != nil || filepath.Base(p) != ".zen.yml" {
		t.Fatalf("EnsureConfig on legacy: %q %v", p, err)
	}
	if _, err := os.Stat(filepath.Join(old, ".zenmd.yml")); err == nil {
		t.Error("a .zenmd.yml was created beside the legacy file")
	}
}

// Editing in the editor saves to .zenmd.yml: the legacy file is moved first.
func TestEditorSaveMovesLegacyFile(t *testing.T) {
	dir := tree(t, map[string]string{"README.md": "# a\n", ".zen.yml": "version: 1\n"})
	s := dialectSession(t, dir, t.TempDir(), false, false)
	if err := s.saveConfig("workspace", "version: 1\ntheme: dark\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, ".zen.yml")); err == nil {
		t.Error(".zen.yml is still there")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, ".zenmd.yml")); string(b) != "version: 1\ntheme: dark\n" {
		t.Errorf(".zenmd.yml: %q", b)
	}
}

// Behaviour 6: the watcher reloads on either file, and the sidebar hides both.
func TestEitherConfigFileReloads(t *testing.T) {
	for _, name := range []string{".zenmd.yml", ".zen.yml"} {
		dir := tree(t, map[string]string{"README.md": mystPage})
		s := New(Options{
			GlobalPath: filepath.Join(t.TempDir(), "config.yml"), Registry: noteRegistry(t), Dialects: builtins.Dialects(),
			Events: &sink{}, Runtime: stubAssets(), Watch: true,
		})
		t.Cleanup(s.Close)
		if err := s.Open(dir); err != nil {
			t.Fatal(err)
		}
		time.Sleep(200 * time.Millisecond) // the watcher is up
		if strings.Contains(pageHTML(t, s, "README.md"), `data-zen-plugin="note"`) {
			t.Fatal("on too early")
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte("markdown:\n  dialects: [myst]\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for !strings.Contains(pageHTML(t, s, "README.md"), `data-zen-plugin="note"`) {
			if time.Now().After(deadline) {
				t.Fatalf("%s did not reload the folder", name)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestIndexHidesBothConfigFiles(t *testing.T) {
	dir := tree(t, map[string]string{"README.md": "# a\n", ".zen.yml": "version: 1\n", ".zenmd.yml": "version: 1\n"})
	s := dialectSession(t, dir, t.TempDir(), false, false)
	var walk func(ns []NavNode)
	walk = func(ns []NavNode) {
		for _, n := range ns {
			if strings.Contains(n.Path, "zen") && strings.HasSuffix(n.Path, ".yml") {
				t.Errorf("the tree lists %s", n.Path)
			}
			walk(n.Children)
		}
	}
	walk(s.Workspace().Tree)
}

// The old build: key is a Problems note too, the same way.
func TestConfigNoticeBuildKey(t *testing.T) {
	dir := tree(t, map[string]string{"README.md": mystPage, ".zenmd.yml": "version: 1\nbuild:\n  out: x\n"})
	s := dialectSession(t, dir, t.TempDir(), false, false)
	const want = "build: in .zenmd.yml is now export:; rename the key"
	if got := s.ConfigNotice(); got != want {
		t.Errorf("notice: %q", got)
	}
	p, err := s.Page("README.md")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range p.Warnings {
		found = found || w == want
	}
	if !found {
		t.Errorf("page warnings: %v", p.Warnings)
	}
}
