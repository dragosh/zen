package scriptplugins_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zenmark-org/zenmd/engine/scriptplugins"
)

func TestInvalidPluginMaps(t *testing.T) {
	s := newSite(t)
	cases := []struct{ name, src, want string }{
		{"no plugin map", `x := 1`, "plugin"},
		{"plugin not a map", `plugin := "x"`, "map"},
		{"no version", `plugin := {blocks: ["x"]}`, "version"},
		{"version not semver", `plugin := {version: "1", blocks: ["x"]}`, "SemVer"},
		{"version not a string", `plugin := {version: 1, blocks: ["x"]}`, "version"},
		{"unknown key", `plugin := {version: "1.0.0", blocks: ["x"], colour: "red"}`, `"colour"`},
		{"unknown permission", `plugin := {version: "1.0.0", blocks: ["x"], permissions: ["write"]}`, `"write"`},
		{"network permission", `plugin := {version: "1.0.0", blocks: ["x"], permissions: ["net"]}`, `"net"`},
		{"permissions not a list", `plugin := {version: "1.0.0", blocks: ["x"], permissions: "read"}`, "permissions"},
		{"bad block name", `plugin := {version: "1.0.0", blocks: ["Bad Name"]}`, "block name"},
		{"bad inline name", `plugin := {version: "1.0.0", inlines: ["1x"]}`, "inline name"},
		{"layouts are not plugins", `plugin := {version: "1.0.0", blocks: ["x"], layouts: ["deck"]}`, ".zen/layouts/<id>/layout.yml"},
		{"block twice", `plugin := {version: "1.0.0", blocks: ["x", "x"]}`, "twice"},
		{"blocks not strings", `plugin := {version: "1.0.0", blocks: [1]}`, "blocks"},
		{"unknown hook", `plugin := {version: "1.0.0", blocks: ["x"], hooks: ["stream"]}`, "hook"},
		{"block hook without blocks", `plugin := {version: "1.0.0", hooks: ["block"]}`, "block"},
		{"declares nothing", `plugin := {version: "1.0.0"}`, "declares nothing"},
		{"css not a string", `plugin := {version: "1.0.0", blocks: ["x"], css: 5}`, "css"},
		{"css import", `plugin := {version: "1.0.0", blocks: ["x"], css: "@import url(https://evil.example/x.css);"}`, "css"},
		{"css remote url", `plugin := {version: "1.0.0", blocks: ["x"], css: "a { background: url(https://evil.example/p.gif) }"}`, "css"},
		{"css protocol relative", `plugin := {version: "1.0.0", blocks: ["x"], css: "a { background: url(//evil.example/p.gif) }"}`, "css"},
		{"css too big", `text := import("text"); plugin := {version: "1.0.0", blocks: ["x"], css: text.repeat("a", 70000)}`, "css"},
		{"bad option type", `plugin := {version: "1.0.0", blocks: ["x"], options: {a: {type: "float"}}}`, "option"},
		{"bad option default", `plugin := {version: "1.0.0", blocks: ["x"], options: {a: {type: "bool", default: "yes"}}}`, "option"},
		{"description not a string", `plugin := {version: "1.0.0", blocks: ["x"], description: 5}`, "description"},
		{"js file in a single-file plugin", `plugin := {version: "1.0.0", blocks: ["x"], js: ["a.js"]}`, "folder"},
	}
	for i, c := range cases {
		err := s.loadErr(sprintf("p%d", i), c.src)
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want it to mention %q", c.name, err, c.want)
		}
		noPath(t, err, s.dir)
	}
}

func TestSyntaxAndRuntimeErrorsNameTheLine(t *testing.T) {
	s := newSite(t)
	err := s.loadErr("syntax", "plugin := {version: \"1.0.0\", blocks: [\"x\"]}\n\nif hook == \"block\" {\n  out = (\n}\n")
	errContains(t, err, "line 5")
	noPath(t, err, s.dir)
	if strings.Contains(err.Error(), "\n") {
		t.Errorf("multi-line: %q", err)
	}
	err = s.loadErr("unresolved", header+"out = nothere + 1\n")
	errContains(t, err, "nothere")
	errContains(t, err, "line 2")
	err = s.loadErr("topfail", header+"a := 1\nb := a.nope.deeper\n")
	errContains(t, err, "line 3")
	// Not UTF-8, empty, binary.
	errContains(t, s.loadErr("empty", ""), "plugin")
}

func TestIdComesFromTheFileOrFolderName(t *testing.T) {
	s := newSite(t)
	p := s.plugin("my-note", header)
	if p.Manifest().ID != "my-note" {
		t.Errorf("id %q", p.Manifest().ID)
	}
	// plugin.id in the map is not a key: the name decides.
	err := s.loadErr("other", `plugin := {id: "different", version: "1.0.0", blocks: ["x"]}`)
	errContains(t, err, `"id"`)
	for _, bad := range []string{"Bad", "1x", "a_b", "a b", "-a", ".hidden"} {
		path := s.file(".zen/plugins/"+bad+".tengo", header)
		if _, err := scriptplugins.Load(path); err == nil || !strings.Contains(err.Error(), "invalid id") {
			t.Errorf("file name %q: %v", bad, err)
		}
	}
	f := s.folder("my-folder", header, nil)
	if f.Manifest().ID != "my-folder" {
		t.Errorf("folder id %q", f.Manifest().ID)
	}
	if _, err := scriptplugins.Load(filepath.Join(s.dir, ".zen", "plugins", "nothing.tengo")); err == nil {
		t.Error("a missing file loaded")
	}
	s.file(".zen/plugins/empty-folder/readme.txt", "x")
	if _, err := scriptplugins.Load(filepath.Join(s.dir, ".zen", "plugins", "empty-folder")); err == nil || !strings.Contains(err.Error(), "plugin.tengo") {
		t.Errorf("folder without plugin.tengo: %v", err)
	}
	// A single file must be a .tengo file.
	path := s.file(".zen/plugins/plain.txt", header)
	if _, err := scriptplugins.Load(path); err == nil {
		t.Error("a .txt file loaded as a plugin")
	}
}

func TestScan(t *testing.T) {
	s := newSite(t)
	dir := filepath.Join(s.dir, ".zen", "plugins")
	s.file(".zen/plugins/b-single.tengo", withHeader(""))
	s.file(".zen/plugins/a-folder/plugin.tengo", `plugin := {version: "1.0.0", blocks: ["af"]}`)
	s.file(".zen/plugins/a-folder/extra.txt", "x")
	s.file(".zen/plugins/broken.tengo", "plugin := {")
	s.file(".zen/plugins/Bad Name.tengo", header)
	s.file(".zen/plugins/ymlfolder/plugin.yml", "id: ymlfolder\n")
	s.file(".zen/plugins/ymlfolder/plugin.tengo", header) // plugin.yml wins: not a script plugin
	s.file(".zen/plugins/nothing/readme.txt", "x")        // not a plugin
	s.file(".zen/plugins/README.md", "# not a plugin")
	s.file(".zen/plugins/both.tengo", `plugin := {version: "1.0.0", blocks: ["both1"]}`)
	s.file(".zen/plugins/both/plugin.tengo", `plugin := {version: "1.0.0", blocks: ["both2"]}`)
	target := s.file("elsewhere.tengo", header)
	os.Symlink(target, filepath.Join(dir, "linked.tengo"))

	found, invalid := scriptplugins.Scan(dir, filepath.Join(s.dir, "no-such-dir"))
	var ids []string
	for _, p := range found {
		ids = append(ids, p.Manifest().ID)
	}
	if strings.Join(ids, ",") != "a-folder,b-single" {
		t.Errorf("found %v, want a-folder,b-single", ids)
	}
	problems := map[string]string{}
	for _, in := range invalid {
		problems[in.ID] = in.Problem
		if strings.Contains(in.Problem, s.dir) {
			t.Errorf("problem names a path: %q", in.Problem)
		}
		if in.Dir == "" {
			t.Errorf("invalid %s has no Dir", in.ID)
		}
	}
	for id, want := range map[string]string{"broken": "line", "Bad Name": "invalid id", "linked": "symlink", "both": "both"} {
		if !strings.Contains(problems[id], want) {
			t.Errorf("problem of %q = %q, want it to contain %q", id, problems[id], want)
		}
	}
	if len(invalid) != 4 {
		t.Errorf("invalid = %v", problems)
	}
	// Sorted by id.
	for i := 1; i < len(invalid); i++ {
		if invalid[i-1].ID > invalid[i].ID {
			t.Errorf("invalid not sorted: %v", problems)
		}
	}
	// The first folder wins for an id.
	other := filepath.Join(s.dir, "other-plugins")
	writeFile(t, other, "b-single.tengo", `plugin := {version: "2.0.0", blocks: ["x"]}`)
	found, _ = scriptplugins.Scan(dir, other)
	for _, p := range found {
		if p.Manifest().ID == "b-single" && p.Manifest().Version != "1.0.0" {
			t.Errorf("the second folder won: %s", p.Manifest().Version)
		}
	}
}

func TestRunsDescribesWhatThePluginCanDo(t *testing.T) {
	s := newSite(t)
	plain := s.plugin("plain", header)
	if got := strings.Join(plain.Runs(), "|"); got != "renders blocks inside zen; cannot read files, run programs or use the network" {
		t.Errorf("no permissions: %q", got)
	}
	reads := s.plugin("reads", `plugin := {version: "1.0.0", blocks: ["x"], permissions: ["read"]}`)
	if got := strings.Join(reads.Runs(), "|"); got != "renders blocks inside zen and reads files in this folder; cannot run programs or use the network" {
		t.Errorf("read: %q", got)
	}
	withJS := s.folder("withjs", `plugin := {version: "1.0.0", blocks: ["x"], js: ["a.js"]}`, map[string]string{"a.js": "export default {}"})
	if got := withJS.Runs(); len(got) != 2 || got[1] != "scripts in the page" {
		t.Errorf("js: %v", got)
	}
}

func TestHelpInThePluginMap(t *testing.T) {
	s := newSite(t)
	p := s.plugin("hi", `plugin := {version: "1.0.0", blocks: ["hi"], help: {summary: "Says hi.", snippets: [{title: "Basic", markdown: ":::hi\nthere\n:::"}]}}`+"\nout = \"x\"\n")
	h := p.Manifest().Help
	if h.Summary != "Says hi." || len(h.Snippets) != 1 || h.Snippets[0].Title != "Basic" || h.Snippets[0].Markdown != ":::hi\nthere\n:::" {
		t.Fatalf("help: %+v", h)
	}

	cases := []struct{ name, help, want string }{
		{"not a map", `"x"`, "help"},
		{"summary not a string", `{summary: 5}`, "summary"},
		{"summary too long", `{summary: text.repeat("a", 501)}`, "summary"},
		{"unknown key", `{summary: "x", colour: "red"}`, `"colour"`},
		{"snippets not a list", `{snippets: "x"}`, "snippets"},
		{"snippet not a map", `{snippets: ["x"]}`, "snippet"},
		{"snippet unknown key", `{snippets: [{title: "t", markdown: "m", lang: "go"}]}`, `"lang"`},
		{"snippet without markdown", `{snippets: [{title: "t"}]}`, "markdown"},
		{"nine snippets", `{snippets: [{title: "t", markdown: "m"}, {title: "t", markdown: "m"}, {title: "t", markdown: "m"}, {title: "t", markdown: "m"}, {title: "t", markdown: "m"}, {title: "t", markdown: "m"}, {title: "t", markdown: "m"}, {title: "t", markdown: "m"}, {title: "t", markdown: "m"}]}`, "8"},
		{"snippet too big", `{snippets: [{title: "t", markdown: text.repeat("a", 4097)}]}`, "4 KB"},
	}
	for i, c := range cases {
		err := s.loadErr(sprintf("help%d", i), `text := import("text"); plugin := {version: "1.0.0", blocks: ["x"], help: `+c.help+`}`)
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want it to mention %q", c.name, err, c.want)
		}
	}
}
