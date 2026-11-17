package extplugins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writePlugin creates <base>/<id>/ with plugin.yml and files; names ending
// in .sh are executable.
func writePlugin(t *testing.T, base, id, yml string, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(base, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(name, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(full, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const fullYML = `id: note
version: 0.1.0
apiVersion: 1
description: Callout boxes
default: true

blocks: [note, warning]
inlines: [badge]
layouts: [slides]

options:
  icon: { type: bool, default: true, description: Show an icon }

allow:
  - { element: div, attrs: [class] }

backend:
  command: ["./note.sh"]
  hooks: [block, inline]
  output: html
  timeout: 5s

frontend:
  js: [note.js]
  css: [note.css]
`

func TestLoadValid(t *testing.T) {
	base := t.TempDir()
	dir := writePlugin(t, base, "note", fullYML, map[string]string{"note.sh": "#!/bin/sh\ncat\n", "note.js": "//", "note.css": "a{}"})
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := p.Manifest()
	if m.ID != "note" || m.Version != "0.1.0" || m.APIVersion != 1 || m.Description != "Callout boxes" || !m.Default {
		t.Fatalf("manifest: %+v", m)
	}
	if strings.Join(m.Blocks, ",") != "note,warning" || strings.Join(m.Inlines, ",") != "badge" || strings.Join(m.Layouts, ",") != "slides" {
		t.Fatalf("names: %+v", m)
	}
	if o := m.Config["icon"]; o.Type != "bool" || o.Default != true || o.Description != "Show an icon" {
		t.Fatalf("option: %+v", m.Config)
	}
	if len(m.Allow) != 1 || m.Allow[0].Element != "div" || strings.Join(m.Allow[0].Attrs, ",") != "class" {
		t.Fatalf("allow: %+v", m.Allow)
	}
	real, _ := filepath.EvalSymlinks(dir)
	if p.Dir() != real {
		t.Fatalf("dir %q, want %q", p.Dir(), real)
	}
	if !p.HasHook("block") || !p.HasHook("inline") || p.HasHook("source") || p.HasHook("action") {
		t.Fatal("hooks")
	}
	runs := strings.Join(p.Runs(), "\n")
	if !strings.Contains(runs, "./note.sh") || !strings.Contains(runs, "scripts in the page") {
		t.Fatalf("runs: %q", runs)
	}
	e := p.Entry()
	if strings.Join(e.JS, ",") != "note.js" || strings.Join(e.CSS, ",") != "note.css" {
		t.Fatalf("entry: %+v", e)
	}
}

func TestLoadTool(t *testing.T) {
	base := t.TempDir()
	dir := writePlugin(t, base, "calc", "id: calc\nversion: 0.1.0\napiVersion: 1\nblocks: [calc]\nbackend: { command: [\"bc\", \"-l\"], hooks: [block], output: text }\n", nil)
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Runs(); len(got) != 1 || got[0] != "bc -l" {
		t.Fatalf("runs: %q", got)
	}
	if p.Assets() != nil {
		t.Fatal("a plugin without frontend files has no assets")
	}
}

func TestRejections(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "evil.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	head := "id: p\nversion: 1.0.0\napiVersion: 1\n"
	sh := map[string]string{"run.sh": "#!/bin/sh\n"}
	cases := []struct {
		name  string
		id    string // folder name, default "p"
		yml   string
		files map[string]string
		setup func(dir string)
		want  string
	}{
		{name: "invalid yaml", yml: "id: [", want: "YAML"},
		{name: "id differs from folder", id: "q", yml: head, want: "folder"},
		{name: "bad id", id: "Bad_Id", yml: "id: Bad_Id\nversion: 1.0.0\napiVersion: 1\n", want: "id"},
		{name: "api version", yml: "id: p\nversion: 1.0.0\napiVersion: 2\n", want: "apiVersion"},
		{name: "no api version", yml: "id: p\nversion: 1.0.0\n", want: "apiVersion"},
		{name: "semver", yml: "id: p\nversion: \"1.0\"\napiVersion: 1\n", want: "SemVer"},
		{name: "unknown key", yml: head + "colour: red\n", want: "colour"},
		{name: "unknown backend key", yml: head + "backend: { command: [\"./run.sh\"], hooks: [block], shell: true }\n", files: sh, want: "shell"},
		{name: "bad block name", yml: head + "blocks: [Bad]\n", want: "Bad"},
		{name: "same name twice", yml: head + "blocks: [a, a]\n", want: "twice"},
		{name: "empty command", yml: head + "backend: { command: [], hooks: [block] }\n", want: "command"},
		{name: "blank command", yml: head + "backend: { command: [\"\"], hooks: [block] }\n", want: "command"},
		{name: "missing command file", yml: head + "backend: { command: [\"./nope.sh\"], hooks: [block] }\n", want: "nope.sh"},
		{name: "command outside", yml: head + "backend: { command: [\"./../evil.sh\"], hooks: [block] }\n", want: "outside"},
		{name: "absolute command", yml: head + "backend: { command: [\"/bin/sh\"], hooks: [block] }\n", want: "command"},
		{name: "command symlink outside", yml: head + "backend: { command: [\"./link.sh\"], hooks: [block] }\n", want: "outside",
			setup: func(dir string) { os.Symlink(filepath.Join(outside, "evil.sh"), filepath.Join(dir, "link.sh")) }},
		{name: "command not executable", yml: head + "backend: { command: [\"./plain.txt\"], hooks: [block] }\n", files: map[string]string{"plain.txt": "x"}, want: "executable"},
		{name: "unknown hook", yml: head + "backend: { command: [\"./run.sh\"], hooks: [render] }\n", files: sh, want: "render"},
		{name: "no hooks", yml: head + "backend: { command: [\"./run.sh\"] }\n", files: sh, want: "hooks"},
		{name: "bad output", yml: head + "backend: { command: [\"./run.sh\"], hooks: [block], output: json }\n", files: sh, want: "output"},
		{name: "timeout too long", yml: head + "backend: { command: [\"./run.sh\"], hooks: [block], timeout: 61s }\n", files: sh, want: "timeout"},
		{name: "timeout invalid", yml: head + "backend: { command: [\"./run.sh\"], hooks: [block], timeout: soon }\n", files: sh, want: "timeout"},
		{name: "frontend missing", yml: head + "frontend: { js: [gone.js] }\n", want: "gone.js"},
		{name: "frontend traversal", yml: head + "frontend: { css: [../x.css] }\n", want: "x.css"},
		{name: "frontend symlink outside", yml: head + "frontend: { js: [link.js] }\n", want: "outside",
			setup: func(dir string) { os.Symlink(filepath.Join(outside, "evil.sh"), filepath.Join(dir, "link.js")) }},
		{name: "bad option type", yml: head + "options: { n: { type: float } }\n", want: "option"},
		{name: "option default of the wrong type", yml: head + "options: { n: { type: int, default: yes } }\n", want: "option"},
		{name: "too many files", yml: head, want: "500",
			setup: func(dir string) {
				for i := 0; i < 501; i++ {
					os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%03d", i)), nil, 0o644)
				}
			}},
		{name: "too large", yml: head, want: "20 MB",
			setup: func(dir string) {
				f, _ := os.Create(filepath.Join(dir, "big.bin"))
				f.Truncate(21 << 20)
				f.Close()
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base := t.TempDir()
			id := c.id
			if id == "" {
				id = "p"
			}
			dir := writePlugin(t, base, id, c.yml, c.files)
			if strings.Contains(c.yml, "../evil.sh") {
				os.WriteFile(filepath.Join(base, "evil.sh"), []byte("#!/bin/sh\n"), 0o755)
			}
			if c.setup != nil {
				c.setup(dir)
			}
			p, err := Load(dir)
			if err == nil {
				t.Fatalf("accepted: %+v", p.Manifest())
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q does not mention %q", err, c.want)
			}
			if strings.Contains(err.Error(), base) {
				t.Fatalf("error leaks the absolute path: %q", err)
			}
		})
	}
}

func TestDiscover(t *testing.T) {
	folder, user := t.TempDir(), t.TempDir()
	yml := func(id, desc string) string {
		return "id: " + id + "\nversion: 1.0.0\napiVersion: 1\ndescription: " + desc + "\n"
	}
	writePlugin(t, folder, "beta", yml("beta", "from folder"), nil)
	writePlugin(t, folder, "alpha", yml("alpha", "a"), nil)
	writePlugin(t, folder, "broken", "id: other\nversion: 1.0.0\napiVersion: 1\n", nil)
	writePlugin(t, user, "beta", yml("beta", "from user"), nil)
	writePlugin(t, user, "gamma", yml("gamma", "g"), nil)
	os.WriteFile(filepath.Join(folder, "stray.txt"), []byte("x"), 0o644) // not a plugin folder
	os.MkdirAll(filepath.Join(folder, "empty"), 0o755)                   // no plugin.yml: ignored

	found, warnings := Discover(folder, user, filepath.Join(user, "does-not-exist"))
	var ids []string
	for _, p := range found {
		ids = append(ids, p.Manifest().ID+":"+p.Manifest().Description)
	}
	if got := strings.Join(ids, " "); got != "alpha:a beta:from folder gamma:g" {
		t.Fatalf("found %q", got)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], `plugin "broken"`) {
		t.Fatalf("warnings: %q", warnings)
	}

	_, invalid := Scan(folder, user)
	if len(invalid) != 1 || invalid[0].ID != "broken" || invalid[0].Problem == "" || invalid[0].Dir == "" {
		t.Fatalf("invalid: %+v", invalid)
	}
}

func TestOptionsMapListAndScope(t *testing.T) {
	yml := "id: box\nversion: 1.0.0\napiVersion: 1\n" +
		"options:\n" +
		"  images: { type: map, scope: user, default: { py: \"python:3\" } }\n" +
		"  tags: { type: list, default: [a, b] }\n" +
		"  none: { type: map }\n" +
		"  nolist: { type: list }\n" +
		"  plain: { type: string }\n" +
		"backend: { command: [\"cat\"], hooks: [block] }\n"
	p, err := Load(writePlugin(t, t.TempDir(), "box", yml, nil))
	if err != nil {
		t.Fatal(err)
	}
	c := p.Manifest().Config
	if c["images"].Type != "map" || c["images"].Scope != "user" || fmt.Sprint(c["images"].Default) != "map[py:python:3]" {
		t.Errorf("images: %+v", c["images"])
	}
	if _, ok := c["images"].Default.(map[string]string); !ok {
		t.Errorf("map default is %T", c["images"].Default)
	}
	if v, ok := c["tags"].Default.([]string); !ok || strings.Join(v, ",") != "a,b" || c["tags"].Scope != "" {
		t.Errorf("tags: %+v", c["tags"])
	}
	if v, ok := c["none"].Default.(map[string]string); !ok || len(v) != 0 {
		t.Errorf("none: %#v", c["none"].Default)
	}
	if v, ok := c["nolist"].Default.([]string); !ok || len(v) != 0 {
		t.Errorf("nolist: %#v", c["nolist"].Default)
	}

	bad := map[string]string{
		"scope: folder":                        "options:\n  o: { type: string, scope: folder }\n",
		"map default that is not a mapping":    "options:\n  o: { type: map, default: [a] }\n",
		"map default with a non-string value":  "options:\n  o: { type: map, default: { a: 1 } }\n",
		"list default that is not a list":      "options:\n  o: { type: list, default: x }\n",
		"list default with a non-string entry": "options:\n  o: { type: list, default: [1] }\n",
	}
	for name, opts := range bad {
		y := "id: box\nversion: 1.0.0\napiVersion: 1\n" + opts + "backend: { command: [\"cat\"], hooks: [block] }\n"
		if _, err := Load(writePlugin(t, t.TempDir(), "box", y, nil)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestStreamHookAndTimeouts(t *testing.T) {
	load := func(backend string) error {
		y := "id: s\nversion: 1.0.0\napiVersion: 1\nbackend: " + backend + "\n"
		_, err := Load(writePlugin(t, t.TempDir(), "s", y, nil))
		return err
	}
	if err := load(`{ command: ["cat"], hooks: [stream], timeout: 10m }`); err != nil {
		t.Errorf("10m stream timeout refused: %v", err)
	}
	if err := load(`{ command: ["cat"], hooks: [stream], timeout: 11m }`); err == nil {
		t.Error("11m accepted")
	}
	if err := load(`{ command: ["cat"], hooks: [block], timeout: 2m }`); err == nil {
		t.Error("2m accepted without the stream hook")
	}
	if err := load(`{ command: ["cat"], hooks: [stream, block], timeout: 61s }`); err != nil {
		t.Errorf("stream plugin with 61s refused: %v", err)
	}
	p, err := Load(writePlugin(t, t.TempDir(), "s", "id: s\nversion: 1.0.0\napiVersion: 1\nbackend: { command: [\"cat\"], hooks: [stream] }\n", nil))
	if err != nil || !p.HasHook("stream") {
		t.Fatalf("stream hook: %v", err)
	}
	if p.streamTimeout() != 60*time.Second {
		t.Errorf("default stream timeout %v", p.streamTimeout())
	}
}

func TestHelp(t *testing.T) {
	base := t.TempDir()
	head := "id: h\nversion: 1.0.0\napiVersion: 1\nblocks: [h]\n"
	dir := writePlugin(t, base, "h", head+`help:
  summary: Says hello.
  snippets:
    - title: Basic
      markdown: |
        :::h
        hi
        :::
    - { title: Second, markdown: "second" }
`, nil)
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	h := p.Manifest().Help
	if h.Summary != "Says hello." || len(h.Snippets) != 2 || h.Snippets[0].Title != "Basic" ||
		h.Snippets[0].Markdown != ":::h\nhi\n:::\n" || h.Snippets[1].Markdown != "second" {
		t.Fatalf("help: %+v", h)
	}
	// Help is optional.
	p, err = Load(writePlugin(t, t.TempDir(), "h", head, nil))
	if err != nil || p.Manifest().Help.Summary != "" || len(p.Manifest().Help.Snippets) != 0 {
		t.Fatalf("no help: %v %+v", err, p)
	}

	snip := func(n int) string {
		var b strings.Builder
		b.WriteString("help:\n  snippets:\n")
		for i := 0; i < n; i++ {
			b.WriteString("    - { title: t, markdown: m }\n")
		}
		return b.String()
	}
	cases := []struct{ name, yml, want string }{
		{"summary too long", head + "help: { summary: " + strings.Repeat("a", 501) + " }\n", "summary"},
		{"nine snippets", head + snip(9), "8"},
		{"snippet too big", head + "help:\n  snippets:\n    - { title: t, markdown: " + strings.Repeat("a", 4097) + " }\n", "4 KB"},
		{"unknown key", head + "help: { summary: x, colour: red }\n", "colour"},
		{"unknown snippet key", head + "help:\n  snippets:\n    - { title: t, markdown: m, lang: go }\n", "lang"},
		{"snippet without markdown", head + "help:\n  snippets:\n    - { title: t }\n", "markdown"},
		{"help not a map", head + "help: hello\n", "help"},
	}
	for i, c := range cases {
		_, err := Load(writePlugin(t, t.TempDir(), "h", c.yml, nil))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%d %s: %v, want it to mention %q", i, c.name, err, c.want)
		}
	}
}
