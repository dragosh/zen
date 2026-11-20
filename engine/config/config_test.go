package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func write(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func hasWarning(ws []string, sub string) bool {
	for _, w := range ws {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}

func TestGlobalPath(t *testing.T) {
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Skip("no user config dir:", err)
	}
	got, err := GlobalPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "zen", "config.yml"); got != want {
		t.Fatalf("GlobalPath() = %q, want %q", got, want)
	}
}

func TestLoadTolerant(t *testing.T) {
	t.Run("missing files", func(t *testing.T) {
		dir := t.TempDir()
		c := Load(filepath.Join(dir, "nope", "config.yml"), dir)
		if c == nil {
			t.Fatal("Load returned nil")
		}
		if len(c.Warnings()) != 0 {
			t.Fatalf("unexpected warnings: %v", c.Warnings())
		}
		s := c.ForPage(nil)
		if s.Theme != "system" {
			t.Fatalf("Theme = %q, want system", s.Theme)
		}
		if len(s.Warnings) != 0 {
			t.Fatalf("unexpected warnings: %v", s.Warnings)
		}
	})

	t.Run("empty paths", func(t *testing.T) {
		c := Load("", "")
		if c == nil || len(c.Warnings()) != 0 {
			t.Fatalf("Load(\"\", \"\") = %v", c)
		}
	})

	t.Run("default", func(t *testing.T) {
		c := Default()
		if c == nil {
			t.Fatal("Default returned nil")
		}
		if s := c.ForPage(nil); s.Theme != "system" {
			t.Fatalf("Theme = %q", s.Theme)
		}
		if _, _, mentioned := c.ForPage(nil).Plugin("highlight"); mentioned {
			t.Fatal("default config mentions a plugin")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		dir := t.TempDir()
		g := write(t, filepath.Join(dir, "g", "config.yml"), "")
		c := Load(g, "")
		if len(c.Warnings()) != 0 {
			t.Fatalf("unexpected warnings: %v", c.Warnings())
		}
	})

	t.Run("invalid global keeps workspace", func(t *testing.T) {
		dir := t.TempDir()
		g := write(t, filepath.Join(dir, "g", "config.yml"), "theme: [dark\nplugins: {{{")
		write(t, filepath.Join(dir, ".zen.yml"), "version: 1\ntheme: light\n")
		c := Load(g, dir)
		if len(c.Warnings()) != 1 || !hasWarning(c.Warnings(), g) {
			t.Fatalf("warnings = %v, want one naming %s", c.Warnings(), g)
		}
		if s := c.ForPage(nil); s.Theme != "light" {
			t.Fatalf("Theme = %q, want light", s.Theme)
		}
	})

	t.Run("non-map document", func(t *testing.T) {
		dir := t.TempDir()
		write(t, filepath.Join(dir, ".zen.yml"), "- a\n- b\n")
		c := Load("", dir)
		if len(c.Warnings()) != 1 {
			t.Fatalf("warnings = %v", c.Warnings())
		}
		if s := c.ForPage(nil); s.Theme != "system" {
			t.Fatalf("Theme = %q", s.Theme)
		}
	})

	t.Run("unreadable file", func(t *testing.T) {
		dir := t.TempDir()
		// A directory where the file should be: reading fails, not "missing".
		if err := os.MkdirAll(filepath.Join(dir, ".zen.yml"), 0o755); err != nil {
			t.Fatal(err)
		}
		c := Load("", dir)
		if len(c.Warnings()) != 1 {
			t.Fatalf("warnings = %v", c.Warnings())
		}
	})

	t.Run("future version ignored", func(t *testing.T) {
		dir := t.TempDir()
		g := write(t, filepath.Join(dir, "g", "config.yml"), "version: 1\ntheme: dark\nplugins:\n  diagram: true\n")
		write(t, filepath.Join(dir, ".zen.yml"), "version: 2\ntheme: light\nplugins:\n  diagram: false\n  slides: true\n")
		c := Load(g, dir)
		if len(c.Warnings()) != 1 || !hasWarning(c.Warnings(), "version") {
			t.Fatalf("warnings = %v", c.Warnings())
		}
		s := c.ForPage(nil)
		if s.Theme != "dark" {
			t.Fatalf("Theme = %q, want dark (workspace layer ignored)", s.Theme)
		}
		if on, _, m := s.Plugin("diagram"); !on || !m {
			t.Fatalf("diagram = %v,%v", on, m)
		}
		if _, _, m := s.Plugin("slides"); m {
			t.Fatal("slides from ignored layer is mentioned")
		}
		if !hasWarning(s.Warnings, "version") {
			t.Fatalf("Settings.Warnings = %v, want config warnings carried over", s.Warnings)
		}
	})
}

func TestSchema(t *testing.T) {
	dir := t.TempDir()
	g := write(t, filepath.Join(dir, "g", "config.yml"), `version: 1
theme: dark
plugins:
  highlight: true
  diagram: { theme: neutral }
  slides: false
editor:
  font: mono
`)
	c := Load(g, "")
	if len(c.Warnings()) != 1 || !hasWarning(c.Warnings(), "editor") {
		t.Fatalf("warnings = %v, want one about editor", c.Warnings())
	}
	if want := map[string]any{"font": "mono"}; !reflect.DeepEqual(c.Extra["editor"], want) {
		t.Fatalf("Extra[editor] = %#v", c.Extra["editor"])
	}
	s := c.ForPage(nil)
	if s.Theme != "dark" {
		t.Fatalf("Theme = %q", s.Theme)
	}
	if on, opts, m := s.Plugin("highlight"); !on || !m || len(opts) != 0 {
		t.Fatalf("highlight = %v %v %v", on, opts, m)
	}
	if on, opts, m := s.Plugin("diagram"); !on || !m || opts["theme"] != "neutral" {
		t.Fatalf("diagram = %v %v %v", on, opts, m)
	}
	if on, _, m := s.Plugin("slides"); on || !m {
		t.Fatalf("slides = %v %v", on, m)
	}

	t.Run("invalid values warn and are skipped", func(t *testing.T) {
		dir := t.TempDir()
		write(t, filepath.Join(dir, ".zen.yml"), "theme: purple\nplugins:\n  highlight: 3\n  diagram: true\n")
		c := Load("", dir)
		if !hasWarning(c.Warnings(), "theme") || !hasWarning(c.Warnings(), "highlight") {
			t.Fatalf("warnings = %v", c.Warnings())
		}
		s := c.ForPage(nil)
		if s.Theme != "system" {
			t.Fatalf("Theme = %q", s.Theme)
		}
		if _, _, m := s.Plugin("highlight"); m {
			t.Fatal("invalid plugin value counted as mentioned")
		}
		if on, _, _ := s.Plugin("diagram"); !on {
			t.Fatal("valid plugin entry lost")
		}
	})

	t.Run("plugins not a map", func(t *testing.T) {
		dir := t.TempDir()
		write(t, filepath.Join(dir, ".zen.yml"), "plugins: [highlight]\ntheme: light\n")
		c := Load("", dir)
		if !hasWarning(c.Warnings(), "plugins") {
			t.Fatalf("warnings = %v", c.Warnings())
		}
		if s := c.ForPage(nil); s.Theme != "light" {
			t.Fatalf("Theme = %q", s.Theme)
		}
	})
}

func TestSplitFrontmatter(t *testing.T) {
	big := strings.Repeat("k: v\n", 64*1024/5+10)
	tests := []struct {
		name     string
		src      string
		wantMeta map[string]any
		wantBody string
		warn     bool
	}{
		{"none", "# Title\n\ntext\n", map[string]any{}, "# Title\n\ntext\n", false},
		{"empty source", "", map[string]any{}, "", false},
		{"basic", "---\ntitle: A\n---\n# Body\n", map[string]any{"title": "A"}, "# Body\n", false},
		{"dots close", "---\ntitle: A\n...\nbody", map[string]any{"title": "A"}, "body", false},
		{"nested", "---\nplugins:\n  diagram: false\n  highlight: { style: github }\n---\nx",
			map[string]any{"plugins": map[string]any{"diagram": false, "highlight": map[string]any{"style": "github"}}}, "x", false},
		{"crlf", "---\r\ntitle: A\r\n---\r\nbody\r\n", map[string]any{"title": "A"}, "body\r\n", false},
		{"bom", "\ufeff---\ntitle: A\n---\nbody", map[string]any{"title": "A"}, "body", false},
		{"bom without frontmatter", "\ufeff# T\n", map[string]any{}, "\ufeff# T\n", false},
		{"later rule in body", "---\na: 1\n---\nbody\n\n---\n\nmore\n", map[string]any{"a": 1}, "body\n\n---\n\nmore\n", false},
		{"rule not on first line", "text\n---\na: 1\n---\n", map[string]any{}, "text\n---\na: 1\n---\n", false},
		{"empty block", "---\n---\nbody", map[string]any{}, "body", false},
		{"block of comments", "---\n# nothing\n---\nbody", map[string]any{}, "body", false},
		{"closed at eof", "---\na: 1\n---", map[string]any{"a": 1}, "", false},
		{"unclosed", "---\na: 1\nbody\n", map[string]any{}, "---\na: 1\nbody\n", false},
		{"four dashes", "----\na: 1\n---\n", map[string]any{}, "----\na: 1\n---\n", false},
		{"list", "---\n- a\n- b\n---\nbody", map[string]any{}, "body", true},
		{"scalar", "---\njust text\n---\nbody", map[string]any{}, "body", true},
		{"invalid yaml", "---\na: [1\nb: {{\n---\nbody", map[string]any{}, "body", true},
		{"oversize", "---\n" + big + "---\nbody", map[string]any{}, "---\n" + big + "---\nbody", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := []byte(tt.src)
			meta, body, warning := SplitFrontmatter(src)
			if meta == nil {
				t.Fatal("meta is nil")
			}
			if !reflect.DeepEqual(meta, tt.wantMeta) {
				t.Errorf("meta = %#v, want %#v", meta, tt.wantMeta)
			}
			if !bytes.Equal(body, []byte(tt.wantBody)) {
				if len(body) > 200 {
					t.Errorf("body differs (len %d, want %d)", len(body), len(tt.wantBody))
				} else {
					t.Errorf("body = %q, want %q", body, tt.wantBody)
				}
			}
			if (warning != "") != tt.warn {
				t.Errorf("warning = %q, want warning: %v", warning, tt.warn)
			}
		})
	}

	t.Run("alias bomb", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("---\na0: &a0 [x, x, x, x, x, x, x, x, x]\n")
		for i := 1; i < 12; i++ {
			prev := "*a" + string(rune('0'+(i-1)%10)) + strings.Repeat("_", (i-1)/10)
			name := "a" + string(rune('0'+i%10)) + strings.Repeat("_", i/10)
			b.WriteString(name + ": &" + name + " [" + strings.Repeat(prev+", ", 8) + prev + "]\n")
		}
		b.WriteString("---\nbody")
		meta, body, warning := SplitFrontmatter([]byte(b.String()))
		if len(meta) != 0 || warning == "" || string(body) != "body" {
			t.Fatalf("alias bomb: meta has %d keys, warning %q, body %q", len(meta), warning, body)
		}
	})
}

func TestPrecedence(t *testing.T) {
	dir := t.TempDir()
	g := write(t, filepath.Join(dir, "g", "config.yml"), `version: 1
theme: dark
plugins:
  highlight: { style: monokai, lines: true }
  diagram: true
  slides: true
  polls: false
`)
	write(t, filepath.Join(dir, ".zen.yml"), `version: 1
plugins:
  highlight: { style: github }
  diagram: { theme: neutral }
  slides: false
`)
	c := Load(g, dir)
	if len(c.Warnings()) != 0 {
		t.Fatalf("warnings: %v", c.Warnings())
	}

	// defaults < global < workspace
	s := c.ForPage(nil)
	if s.Theme != "dark" {
		t.Fatalf("Theme = %q", s.Theme)
	}
	on, opts, _ := s.Plugin("highlight")
	if want := map[string]any{"style": "github", "lines": true}; !on || !reflect.DeepEqual(opts, want) {
		t.Fatalf("highlight = %v %#v", on, opts)
	}
	if on, opts, _ := s.Plugin("diagram"); !on || opts["theme"] != "neutral" {
		t.Fatalf("diagram = %v %#v", on, opts)
	}
	if on, _, m := s.Plugin("slides"); on || !m {
		t.Fatalf("slides = %v %v", on, m)
	}
	if on, _, m := s.Plugin("polls"); on || !m {
		t.Fatalf("polls = %v %v", on, m)
	}

	// ... < frontmatter
	meta, _, _ := SplitFrontmatter([]byte(`---
title: Architecture
theme: light
plugins:
  diagram: false
  highlight: { lines: false }
  slides: true
  polls: { anonymous: true }
---
`))
	s = c.ForPage(meta)
	if s.Theme != "light" {
		t.Fatalf("Theme = %q, want light", s.Theme)
	}
	if len(s.Warnings) != 0 {
		t.Fatalf("warnings: %v", s.Warnings)
	}
	if on, opts, _ := s.Plugin("diagram"); on || opts["theme"] != "neutral" {
		t.Fatalf("diagram = %v %#v", on, opts)
	}
	on, opts, _ = s.Plugin("highlight")
	if want := map[string]any{"style": "github", "lines": false}; !on || !reflect.DeepEqual(opts, want) {
		t.Fatalf("highlight = %v %#v", on, opts)
	}
	if on, _, _ := s.Plugin("slides"); !on {
		t.Fatal("slides not re-enabled by frontmatter")
	}
	if on, opts, _ := s.Plugin("polls"); !on || opts["anonymous"] != true {
		t.Fatalf("polls = %v %#v", on, opts)
	}

	// A page must not change what the next page sees.
	s = c.ForPage(nil)
	if on, opts, _ := s.Plugin("highlight"); !on || opts["lines"] != true {
		t.Fatalf("config mutated by ForPage: %#v", opts)
	}
	if on, _, _ := s.Plugin("diagram"); !on {
		t.Fatal("config mutated by ForPage: diagram off")
	}

	// Bad frontmatter values warn and fall through to the lower layers.
	s = c.ForPage(map[string]any{"theme": "purple", "plugins": map[string]any{"diagram": "yes"}})
	if s.Theme != "dark" {
		t.Fatalf("Theme = %q", s.Theme)
	}
	if on, _, _ := s.Plugin("diagram"); !on {
		t.Fatal("diagram should stay on")
	}
	if !hasWarning(s.Warnings, "diagram") {
		t.Fatalf("warnings = %v", s.Warnings)
	}
	s = c.ForPage(map[string]any{"plugins": []any{"diagram"}})
	if !hasWarning(s.Warnings, "plugins") {
		t.Fatalf("warnings = %v", s.Warnings)
	}
}

func TestPluginLookup(t *testing.T) {
	c := Default()
	s := c.ForPage(map[string]any{"plugins": map[string]any{
		"on":   true,
		"off":  false,
		"opts": map[string]any{"a": 1, "nested": map[string]any{"b": 2}},
	}})

	if on, opts, m := s.Plugin("on"); !on || !m || len(opts) != 0 {
		t.Fatalf("on = %v %v %v", on, opts, m)
	}
	if on, opts, m := s.Plugin("off"); on || !m || len(opts) != 0 {
		t.Fatalf("off = %v %v %v", on, opts, m)
	}
	on, opts, m := s.Plugin("opts")
	if !on || !m || opts["a"] != 1 {
		t.Fatalf("opts = %v %v %v", on, opts, m)
	}
	if on, opts, m := s.Plugin("absent"); on || m || len(opts) != 0 {
		t.Fatalf("absent = %v %v %v", on, opts, m)
	}

	// Returned options are a copy.
	opts["a"] = 99
	opts["nested"].(map[string]any)["b"] = 99
	_, again, _ := s.Plugin("opts")
	if again["a"] != 1 || again["nested"].(map[string]any)["b"] != 2 {
		t.Fatalf("options aliased: %#v", again)
	}

	// Zero value is usable.
	var zero Settings
	if on, _, m := zero.Plugin("x"); on || m {
		t.Fatal("zero Settings mentions a plugin")
	}
}

func TestLayoutDefaults(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "config.yml")
	ws := filepath.Join(dir, "ws")
	os.MkdirAll(ws, 0o755)
	os.WriteFile(global, []byte("version: 1\nlayout: wide\nlayouts:\n  \"talks/**\": slides\n  \"blog/**\": wide\n"), 0o644)
	os.WriteFile(filepath.Join(ws, ".zen.yml"), []byte("version: 1\nlayout: docs\nlayouts:\n  \"blog/**\": post\n  \"talks/*.md\": slides\n  \"**/*.md\": article\n"), 0o644)
	c := Load(global, ws)
	if w := c.Warnings(); len(w) != 0 {
		t.Fatalf("layout and layouts are known keys: %v", w)
	}
	d := c.ForPage(nil).Layouts()
	if d.Default != "docs" {
		t.Errorf("workspace default wins over global: %q", d.Default)
	}
	var got []string
	for _, r := range d.Rules {
		got = append(got, r.Pattern+"="+r.Name)
	}
	// Workspace rules first, in file order, then the global ones.
	want := `blog/**=post talks/*.md=slides **/*.md=article talks/**=slides blog/**=wide`
	if strings.Join(got, " ") != want {
		t.Errorf("rules %v", got)
	}
	if d := Default().ForPage(nil).Layouts(); d.Default != "" || len(d.Rules) != 0 {
		t.Errorf("defaults %+v", d)
	}
	// The global default applies when the folder has none.
	os.WriteFile(filepath.Join(ws, ".zen.yml"), []byte("version: 1\n"), 0o644)
	if d := Load(global, ws).ForPage(nil).Layouts(); d.Default != "wide" || len(d.Rules) != 2 {
		t.Errorf("global only: %+v", d)
	}
}

func TestLayoutDefaultsInvalid(t *testing.T) {
	dir := t.TempDir()
	ws := filepath.Join(dir, "ws")
	os.MkdirAll(ws, 0o755)
	os.WriteFile(filepath.Join(ws, ".zen.yml"), []byte("version: 1\nlayout: Not A Name\nlayouts:\n  \"a/**\": 3\n  \"b/**\": \"Bad Name\"\n  \"c/**\": ok\n"), 0o644)
	c := Load("", ws)
	if len(c.Warnings()) != 3 {
		t.Errorf("warnings %v", c.Warnings())
	}
	d := c.ForPage(nil).Layouts()
	if d.Default != "" || len(d.Rules) != 1 || d.Rules[0].Name != "ok" {
		t.Errorf("only valid entries kept: %+v", d)
	}
	os.WriteFile(filepath.Join(ws, ".zen.yml"), []byte("version: 1\nlayouts: [a, b]\n"), 0o644)
	if w := Load("", ws).Warnings(); len(w) != 1 || !strings.Contains(w[0], "layouts") {
		t.Errorf("layouts must be a mapping: %v", w)
	}
}

func TestMarkdownDialects(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(t.TempDir(), "config.yml")
	write := func(p, s string) {
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// none by default
	if got := Default().Dialects(); len(got) != 0 {
		t.Errorf("default: %v", got)
	}
	// the folder's .zen.yml chooses them
	write(filepath.Join(dir, WorkspaceFile), "markdown:\n  dialects: [myst, mkdocs]\n")
	c := Load("", dir)
	if got := c.Dialects(); !reflect.DeepEqual(got, []string{"myst", "mkdocs"}) {
		t.Errorf("folder: %v", got)
	}
	if len(c.Warnings()) != 0 {
		t.Errorf("warnings: %v", c.Warnings())
	}
	// flow mapping
	write(filepath.Join(dir, WorkspaceFile), "markdown: { dialects: [myst] }\n")
	if got := Load("", dir).Dialects(); !reflect.DeepEqual(got, []string{"myst"}) {
		t.Errorf("flow: %v", got)
	}
	// the global config cannot
	write(global, "markdown:\n  dialects: [myst]\n")
	write(filepath.Join(dir, WorkspaceFile), "version: 1\n")
	c = Load(global, dir)
	if got := c.Dialects(); len(got) != 0 {
		t.Errorf("global: %v", got)
	}
	if w := strings.Join(c.Warnings(), "\n"); !strings.Contains(w, "markdown") || !strings.Contains(w, ".zen.yml") {
		t.Errorf("global warning: %q", w)
	}
	// front matter cannot, and says so
	write(filepath.Join(dir, WorkspaceFile), "markdown:\n  dialects: [myst]\n")
	s := Load("", dir).ForPage(map[string]any{"markdown": map[string]any{"dialects": []any{"mkdocs"}}})
	if w := strings.Join(s.Warnings, "\n"); !strings.Contains(w, "frontmatter: markdown") {
		t.Errorf("frontmatter warning: %q", w)
	}
	if got := Load("", dir).Dialects(); !reflect.DeepEqual(got, []string{"myst"}) {
		t.Errorf("frontmatter changed the folder's: %v", got)
	}
	// wrong shapes warn and contribute nothing
	for yml, want := range map[string]string{
		"markdown: yes\n":                      "markdown must be a mapping",
		"markdown:\n  dialects: myst\n":        "dialects must be a list",
		"markdown:\n  dialects: [Bad Id, 3]\n": "dialects",
		"markdown:\n  other: 1\n":              "markdown.other",
	} {
		write(filepath.Join(dir, WorkspaceFile), yml)
		c := Load("", dir)
		if len(c.Dialects()) != 0 || !strings.Contains(strings.Join(c.Warnings(), "\n"), want) {
			t.Errorf("%q: %v, warnings %v", yml, c.Dialects(), c.Warnings())
		}
	}
	// a repeated id counts once
	write(filepath.Join(dir, WorkspaceFile), "markdown:\n  dialects: [myst, myst]\n")
	if got := Load("", dir).Dialects(); !reflect.DeepEqual(got, []string{"myst"}) {
		t.Errorf("repeat: %v", got)
	}
}
