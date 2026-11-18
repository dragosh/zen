package scriptplugins_test

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zenmark-org/zenmd/engine/extplugins"
	"github.com/zenmark-org/zenmd/engine/plugins"
	"github.com/zenmark-org/zenmd/engine/scriptplugins"
)

// edit rewrites a file and makes sure its modification time differs.
func edit(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(3 * time.Second)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
}

func TestHashAndFingerprintOfASingleFile(t *testing.T) {
	s := newSite(t)
	path := s.file(".zen/plugins/one.tengo", withHeader(`if hook == "block" { out = "v1" }`))
	load := func() *scriptplugins.Plugin {
		p, err := scriptplugins.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := load()
	h1, err := p.Hash()
	if err != nil || len(h1) != 64 {
		t.Fatalf("hash %q %v", h1, err)
	}
	if h2, _ := load().Hash(); h2 != h1 {
		t.Error("hash not stable")
	}
	if !strings.HasSuffix(p.Dir(), "one.tengo") || len(p.AssetVersion()) != 12 {
		t.Errorf("dir %q version %q", p.Dir(), p.AssetVersion())
	}
	if p.Changed() {
		t.Fatal("changed without a change")
	}
	var calls atomic.Int32
	p.OnChange(func() { calls.Add(1) })
	p.Bind(scriptplugins.Runtime{Workspace: s.dir})
	if got := mustBlock(t, p, "x", "", nil); got != "v1" {
		t.Fatalf("before the edit: %q", got)
	}

	edit(t, path, withHeader(`if hook == "block" { out = "EVIL" }`))
	_, err = block(p, "x", "", nil)
	if err != extplugins.ErrChanged {
		t.Fatalf("a call after an edit: %v, want ErrChanged", err)
	}
	if calls.Load() != 1 {
		t.Errorf("OnChange called %d times", calls.Load())
	}
	for i := 0; i < 3; i++ {
		if _, err := block(p, "x", "", nil); err != extplugins.ErrChanged {
			t.Errorf("call %d after the edit: %v", i, err)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("OnChange called %d times", calls.Load())
	}
	// The edited file is a different plugin: a different hash, to be asked about.
	if h3, _ := load().Hash(); h3 == h1 {
		t.Error("hash did not change with the content")
	}
	// Each hook refuses.
	for name, f := range map[string]func() error{
		"inline": func() error { _, err := p.RenderInline(plugins.Context{}, plugins.Inline{}); return err },
		"source": func() error { _, err := p.Source(plugins.Context{}, nil); return err },
		"html":   func() error { _, err := p.HTML(plugins.Context{}, ""); return err },
		"action": func() error { _, err := p.Action(plugins.Context{}, []byte("1")); return err },
	} {
		if err := f(); err == nil {
			t.Errorf("%s ran on a changed plugin", name)
		}
	}
	// Deleting the file counts as a change too.
	q := s.plugin("gone", header)
	os.Remove(q.Dir())
	if !q.Changed() {
		t.Error("a deleted plugin file is not a change")
	}
}

func TestChangeWithTheSameSizeAndTimeIsStillCaughtByTheHashOnReload(t *testing.T) {
	// The fingerprint is a cheap check; a new load hashes the content.
	s := newSite(t)
	path := s.file(".zen/plugins/same.tengo", withHeader(`if hook == "block" { out = "AAAA" }`))
	p1, _ := scriptplugins.Load(path)
	st, _ := os.Stat(path)
	os.WriteFile(path, []byte(withHeader(`if hook == "block" { out = "BBBB" }`)), 0o644)
	os.Chtimes(path, st.ModTime(), st.ModTime())
	p2, _ := scriptplugins.Load(path)
	h1, _ := p1.Hash()
	h2, _ := p2.Hash()
	if h1 == h2 {
		t.Error("hashes of different content are equal")
	}
}

func TestTrustFlowForScriptPlugins(t *testing.T) {
	s := newSite(t)
	path := s.file(".zen/plugins/t.tengo", header)
	p, _ := scriptplugins.Load(path)
	trustFile := filepath.Join(t.TempDir(), "trusted-plugins.yml")
	tr := extplugins.LoadTrust(trustFile)
	if _, decided := tr.Decision(p, s.dir); decided {
		t.Fatal("decided before asking")
	}
	if err := tr.Set(p, s.dir, true); err != nil {
		t.Fatal(err)
	}
	if allowed, decided := extplugins.LoadTrust(trustFile).Decision(p, s.dir); !allowed || !decided {
		t.Error("the decision was not kept")
	}
	if _, decided := tr.Decision(p, "/another/folder"); decided {
		t.Error("a decision for one folder applies to another")
	}
	// The same file after an edit is a different plugin to the trust file.
	edit(t, path, header+"// edited\n")
	p2, _ := scriptplugins.Load(path)
	if _, decided := tr.Decision(p2, s.dir); decided {
		t.Error("an edited file kept its approval")
	}
	// Allowing what changed since it was shown is refused.
	if err := tr.Set(p, s.dir, true); err != extplugins.ErrChanged {
		t.Errorf("allow after change: %v", err)
	}
	if err := tr.Set(p2, s.dir, true); err != nil {
		t.Errorf("allow of the new content: %v", err)
	}
}

func TestFolderFormHashCoversTheWholeFolder(t *testing.T) {
	s := newSite(t)
	s.folder("wf", `plugin := {version: "1.0.0", blocks: ["x"], permissions: ["read"]}`, map[string]string{"data.txt": "one", ".DS_Store": "a"})
	dir := filepath.Join(s.dir, ".zen", "plugins", "wf")
	hash := func() (string, bool) {
		p, err := scriptplugins.Load(dir)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		h, _ := p.Hash()
		return h, p.Changed()
	}
	h0, _ := hash()
	edit(t, filepath.Join(dir, "data.txt"), "two")
	if h1, _ := hash(); h1 == h0 {
		t.Error("an edit of another file of the folder did not change the hash")
	}
	h1, _ := hash()
	edit(t, filepath.Join(dir, "plugin.tengo"), `plugin := {version: "1.0.1", blocks: ["x"], permissions: ["read"]}`)
	if h2, _ := hash(); h2 == h1 {
		t.Error("an edit of plugin.tengo did not change the hash")
	}
	// Finder's file is not part of it.
	h2, _ := hash()
	edit(t, filepath.Join(dir, ".DS_Store"), "rewritten by Finder")
	if h3, _ := hash(); h3 != h2 {
		t.Error(".DS_Store changed the hash")
	}
	// A running plugin notices a file added to the folder.
	p, _ := scriptplugins.Load(dir)
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("n"), 0o644)
	if !p.Changed() {
		t.Error("a new file in the folder was not noticed")
	}
	// A symlink out of the folder is refused at load (same rules as program plugins).
	os.Remove(filepath.Join(dir, "new.txt"))
	outside := filepath.Join(t.TempDir(), "x.txt")
	os.WriteFile(outside, []byte("x"), 0o644)
	os.Symlink(outside, filepath.Join(dir, "evil.txt"))
	if _, err := scriptplugins.Load(dir); err == nil || !strings.Contains(err.Error(), "outside the plugin folder") {
		t.Errorf("symlink out: %v", err)
	}
	// So is plugin.tengo itself as a symlink out of the folder.
	os.Remove(filepath.Join(dir, "evil.txt"))
	os.Remove(filepath.Join(dir, "plugin.tengo"))
	evil := filepath.Join(t.TempDir(), "plugin.tengo")
	os.WriteFile(evil, []byte(header), 0o644)
	os.Symlink(evil, filepath.Join(dir, "plugin.tengo"))
	if _, err := scriptplugins.Load(dir); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Errorf("plugin.tengo outside: %v", err)
	}
}

func TestInlineCSSIsServedAndVersioned(t *testing.T) {
	s := newSite(t)
	css := "div.note { border-left: 3px solid; padding: .5em 1em }"
	p := s.plugin("note", `plugin := {version: "1.0.0", blocks: ["x"], css: "`+css+`"}`)
	if e := p.Entry(); len(e.CSS) != 1 || e.CSS[0] != "inline.css" || len(e.JS) != 0 {
		t.Fatalf("entry: %+v", e)
	}
	var ap plugins.AssetProvider = p
	fsys := ap.Assets()
	if fsys == nil {
		t.Fatal("no asset file system")
	}
	f, err := fsys.Open("inline.css")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(f)
	f.Close()
	if string(b) != css {
		t.Errorf("css: %q", b)
	}
	if _, ok := f.(io.Seeker); !ok {
		t.Error("the file cannot seek (http.ServeContent needs it)")
	}
	for _, name := range []string{"plugin.tengo", "../x", "note.tengo", "inline.css/x", ""} {
		if _, err := fsys.Open(name); err == nil {
			t.Errorf("Open(%q) succeeded", name)
		}
	}
	var _ plugins.AssetVersioner = p
	if len(p.AssetVersion()) != 12 {
		t.Errorf("version %q", p.AssetVersion())
	}
	// Registered, the page offers /_plugins/<id>/inline.css?v=<hash>.
	reg := plugins.NewRegistry()
	if err := reg.Register(p); err != nil {
		t.Fatal(err)
	}
	set, _ := reg.Enabled(nil, nil)
	got := set.Assets()
	if len(got.CSS) != 1 || !strings.HasPrefix(got.CSS[0], "/_plugins/note/inline.css?v="+p.AssetVersion()) {
		t.Errorf("assets: %+v", got)
	}
	if _, ok := reg.AssetFS("note"); !ok {
		t.Error("registry has no asset file system for the plugin")
	}
	// A plugin without css ships nothing.
	none := s.plugin("none", header)
	if none.Assets() != nil || len(none.Entry().CSS) != 0 {
		t.Error("a plugin without css has assets")
	}
	// After an edit nothing is served any more.
	edit(t, p.Dir(), `plugin := {version: "1.0.0", blocks: ["x"], css: "a{color:red}"}`)
	if _, err := fsys.Open("inline.css"); err == nil {
		t.Error("css of a changed plugin was served")
	}
}

func TestFolderFormFrontendFiles(t *testing.T) {
	s := newSite(t)
	p := s.folder("fe", `plugin := {version: "1.0.0", blocks: ["x"], css: "a{color:blue}", js: ["fe.js"], stylesheets: ["fe.css"]}`,
		map[string]string{"fe.js": "export default {}", "fe.css": "p{margin:0}", "secret.txt": "no"})
	e := p.Entry()
	if strings.Join(e.JS, ",") != "fe.js" || strings.Join(e.CSS, ",") != "inline.css,fe.css" {
		t.Fatalf("entry: %+v", e)
	}
	fsys := p.Assets()
	for name, want := range map[string]string{"fe.js": "export default {}", "fe.css": "p{margin:0}", "inline.css": "a{color:blue}"} {
		f, err := fsys.Open(name)
		if err != nil {
			t.Errorf("Open(%s): %v", name, err)
			continue
		}
		b, _ := io.ReadAll(f)
		f.Close()
		if string(b) != want {
			t.Errorf("%s = %q", name, b)
		}
	}
	for _, name := range []string{"secret.txt", "plugin.tengo", "../fe.js"} {
		if _, err := fsys.Open(name); !isNotExist(err) {
			t.Errorf("Open(%q): %v, want not exist", name, err)
		}
	}
	// Listed files must exist, inside the folder.
	s.file(".zen/plugins/bad/plugin.tengo", `plugin := {version: "1.0.0", blocks: ["x"], js: ["missing.js"]}`)
	if _, err := scriptplugins.Load(filepath.Join(s.dir, ".zen", "plugins", "bad")); err == nil {
		t.Error("a missing frontend file loaded")
	}
	s.file(".zen/plugins/bad2/plugin.tengo", `plugin := {version: "1.0.0", blocks: ["x"], js: ["../x.js"]}`)
	if _, err := scriptplugins.Load(filepath.Join(s.dir, ".zen", "plugins", "bad2")); err == nil {
		t.Error("a frontend file outside the folder loaded")
	}
}

func isNotExist(err error) bool {
	return err != nil && (os.IsNotExist(err) || err == fs.ErrNotExist || strings.Contains(err.Error(), "not exist"))
}
