package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The names: .zenmd.yml is the folder's file; .zen.yml still works.
func TestWorkspaceConfigPath(t *testing.T) {
	if WorkspaceFile != ".zenmd.yml" || LegacyWorkspaceFile != ".zen.yml" {
		t.Fatalf("names: %q %q", WorkspaceFile, LegacyWorkspaceFile)
	}
	t.Run("none: the new name, nothing legacy", func(t *testing.T) {
		dir := t.TempDir()
		p, legacy, both := WorkspaceConfigPath(dir)
		if p != filepath.Join(dir, ".zenmd.yml") || legacy || both {
			t.Errorf("got %q %v %v", p, legacy, both)
		}
	})
	t.Run("new only", func(t *testing.T) {
		dir := t.TempDir()
		write(t, filepath.Join(dir, ".zenmd.yml"), "theme: dark\n")
		p, legacy, both := WorkspaceConfigPath(dir)
		if p != filepath.Join(dir, ".zenmd.yml") || legacy || both {
			t.Errorf("got %q %v %v", p, legacy, both)
		}
		if c := Load("", dir); c.ForPage(nil).Theme != "dark" || c.Notice() != "" {
			t.Errorf("theme %q notice %q", c.ForPage(nil).Theme, c.Notice())
		}
	})
	t.Run("legacy only is read, with a notice", func(t *testing.T) {
		dir := t.TempDir()
		write(t, filepath.Join(dir, ".zen.yml"), "theme: light\n")
		p, legacy, both := WorkspaceConfigPath(dir)
		if p != filepath.Join(dir, ".zen.yml") || !legacy || both {
			t.Errorf("got %q %v %v", p, legacy, both)
		}
		c := Load("", dir)
		if c.ForPage(nil).Theme != "light" {
			t.Errorf("the legacy file was not read: %q", c.ForPage(nil).Theme)
		}
		want := "This folder's settings are in .zen.yml; Zenmark now calls the file .zenmd.yml."
		if c.Notice() != want {
			t.Errorf("notice %q", c.Notice())
		}
		if !hasWarning(c.ForPage(nil).Warnings, want) {
			t.Errorf("the notice is not a page warning: %v", c.ForPage(nil).Warnings)
		}
		if hasWarning(c.Warnings(), "settings are in") {
			t.Errorf("the notice is not a load warning: %v", c.Warnings())
		}
	})
	t.Run("both: the new one wins and the notice says so", func(t *testing.T) {
		dir := t.TempDir()
		write(t, filepath.Join(dir, ".zen.yml"), "theme: light\n")
		write(t, filepath.Join(dir, ".zenmd.yml"), "theme: dark\n")
		p, legacy, both := WorkspaceConfigPath(dir)
		if p != filepath.Join(dir, ".zenmd.yml") || legacy || !both {
			t.Errorf("got %q %v %v", p, legacy, both)
		}
		c := Load("", dir)
		if c.ForPage(nil).Theme != "dark" {
			t.Errorf("theme %q", c.ForPage(nil).Theme)
		}
		if c.Notice() != ".zen.yml is ignored because .zenmd.yml exists." {
			t.Errorf("notice %q", c.Notice())
		}
	})
}

func TestMoveLegacy(t *testing.T) {
	body := "# mine\nversion: 1\ntheme: light   # keep\n"
	t.Run("renames in place, bytes kept", func(t *testing.T) {
		dir := t.TempDir()
		write(t, filepath.Join(dir, ".zen.yml"), body)
		if err := MoveLegacy(dir); err != nil {
			t.Fatal(err)
		}
		if b, _ := os.ReadFile(filepath.Join(dir, ".zenmd.yml")); string(b) != body {
			t.Errorf("bytes: %q", b)
		}
		if _, err := os.Lstat(filepath.Join(dir, ".zen.yml")); err == nil {
			t.Error(".zen.yml is still there")
		}
	})
	t.Run("nothing legacy: nothing to do", func(t *testing.T) {
		dir := t.TempDir()
		write(t, filepath.Join(dir, ".zenmd.yml"), body)
		if err := MoveLegacy(dir); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("both: neither is touched", func(t *testing.T) {
		dir := t.TempDir()
		write(t, filepath.Join(dir, ".zen.yml"), "a: 1\n")
		write(t, filepath.Join(dir, ".zenmd.yml"), "b: 2\n")
		if err := MoveLegacy(dir); err != nil {
			t.Fatal(err)
		}
		if b, _ := os.ReadFile(filepath.Join(dir, ".zen.yml")); string(b) != "a: 1\n" {
			t.Errorf(".zen.yml: %q", b)
		}
		if b, _ := os.ReadFile(filepath.Join(dir, ".zenmd.yml")); string(b) != "b: 2\n" {
			t.Errorf(".zenmd.yml: %q", b)
		}
	})
	t.Run("a failed rename leaves .zen.yml and reports it", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can write anywhere")
		}
		dir := t.TempDir()
		write(t, filepath.Join(dir, ".zen.yml"), body)
		if err := os.Chmod(dir, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(dir, 0o755) })
		if err := MoveLegacy(dir); err == nil {
			t.Fatal("want an error")
		}
		os.Chmod(dir, 0o755)
		if b, _ := os.ReadFile(filepath.Join(dir, ".zen.yml")); string(b) != body {
			t.Errorf(".zen.yml changed: %q", b)
		}
		if _, err := os.Lstat(filepath.Join(dir, ".zenmd.yml")); err == nil {
			t.Error(".zenmd.yml was created")
		}
	})
	t.Run("a link pointing out of the folder is refused", func(t *testing.T) {
		dir, outside := t.TempDir(), t.TempDir()
		target := write(t, filepath.Join(outside, "secret.yml"), "theme: light\n")
		if err := os.Symlink(target, filepath.Join(dir, ".zen.yml")); err != nil {
			t.Skip("no symlinks:", err)
		}
		err := MoveLegacy(dir)
		if err == nil || !strings.Contains(err.Error(), ".zen.yml") {
			t.Fatalf("err: %v", err)
		}
		if _, err := os.Lstat(filepath.Join(dir, ".zenmd.yml")); err == nil {
			t.Error(".zenmd.yml was created")
		}
		if fi, err := os.Lstat(filepath.Join(dir, ".zen.yml")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Error("the link is gone")
		}
		if b, _ := os.ReadFile(target); string(b) != "theme: light\n" {
			t.Errorf("the target changed: %q", b)
		}
	})
	t.Run("a folder named .zen.yml is refused", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, ".zen.yml"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := MoveLegacy(dir); err == nil {
			t.Error("want an error")
		}
	})
}
