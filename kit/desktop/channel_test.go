package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// fakeAccess is a folderAccess that records what zen asks of it.
type fakeAccess struct {
	remembered []string
	resolved   map[string]error // dir -> result of Resolve (missing: ErrNoBookmark)
	stops      int
	rememberFn func(dir string) error
}

func newFake() *fakeAccess { return &fakeAccess{resolved: map[string]error{}} }

func (f *fakeAccess) Remember(dir string) error {
	if f.rememberFn != nil {
		if err := f.rememberFn(dir); err != nil {
			return err
		}
	}
	f.remembered = append(f.remembered, dir)
	f.resolved[dir] = nil
	return nil
}

func (f *fakeAccess) Resolve(dir string) (func(), error) {
	err, ok := f.resolved[dir]
	if !ok {
		return nil, ErrNoBookmark
	}
	if err != nil {
		return nil, err
	}
	return func() { f.stops++ }, nil
}

func TestFolderAccess(t *testing.T) {
	const msg = "Allow zen to show the pages next to this file"
	picked := func(dir string, calls *[]string) folderPicker {
		return func(preselect, message string) (string, error) {
			*calls = append(*calls, preselect+"|"+message)
			return dir, nil
		}
	}
	decline := func(preselect, message string) (string, error) { return "", nil }

	t.Run("first open asks, remembers and resolves later without asking", func(t *testing.T) {
		fa := newFake()
		var calls []string
		root, stop, err := openWithAccess(fa, "/Users/me/notes", picked("/Users/me/notes", &calls))
		if err != nil || root != "/Users/me/notes" {
			t.Fatalf("root %q err %v", root, err)
		}
		stop()
		if len(calls) != 1 || !strings.HasPrefix(calls[0], "/Users/me/notes|") {
			t.Errorf("panel calls = %v, want one, preselected on the folder", calls)
		}
		if len(fa.remembered) != 1 || fa.remembered[0] != "/Users/me/notes" {
			t.Errorf("remembered = %v", fa.remembered)
		}
		calls = nil
		root, stop, err = openWithAccess(fa, "/Users/me/notes", picked("x", &calls))
		if err != nil || root != "/Users/me/notes" || len(calls) != 0 {
			t.Fatalf("reopen: root %q err %v calls %v", root, err, calls)
		}
		stop()
		if fa.stops == 0 {
			t.Error("the access granted by Resolve was never ended")
		}
	})

	t.Run("a stale bookmark asks again, preselected, and replaces it", func(t *testing.T) {
		fa := newFake()
		fa.resolved["/Users/me/notes"] = ErrStale
		var calls []string
		root, _, err := openWithAccess(fa, "/Users/me/notes", picked("/Users/me/moved", &calls))
		if err != nil || root != "/Users/me/moved" {
			t.Fatalf("root %q err %v", root, err)
		}
		if len(calls) != 1 || !strings.HasPrefix(calls[0], "/Users/me/notes|") {
			t.Errorf("panel calls = %v", calls)
		}
		if len(fa.remembered) != 1 || fa.remembered[0] != "/Users/me/moved" {
			t.Errorf("remembered = %v, want the new folder", fa.remembered)
		}
	})

	t.Run("declining the panel opens nothing", func(t *testing.T) {
		fa := newFake()
		root, _, err := openWithAccess(fa, "/Users/me/notes", decline)
		if root != "" || !errors.Is(err, ErrDeclined) {
			t.Fatalf("root %q err %v, want ErrDeclined", root, err)
		}
		if len(fa.remembered) != 0 {
			t.Errorf("remembered %v after declining", fa.remembered)
		}
	})

	t.Run("a folder on a disk that is not mounted is not asked about", func(t *testing.T) {
		fa := newFake()
		var calls []string
		_, _, err := openWithAccess(fa, "/Volumes/ZenNoSuchDisk/notes", picked("x", &calls))
		if !errors.Is(err, ErrGone) {
			t.Fatalf("err = %v, want ErrGone", err)
		}
		if len(calls) != 0 {
			t.Errorf("asked the user %v for a folder that cannot exist", calls)
		}
	})

	t.Run("other errors from Resolve are returned, not hidden by a panel", func(t *testing.T) {
		fa := newFake()
		boom := errors.New("boom")
		fa.resolved["/Users/me/notes"] = boom
		var calls []string
		_, _, err := openWithAccess(fa, "/Users/me/notes", picked("x", &calls))
		if !errors.Is(err, boom) || len(calls) != 0 {
			t.Fatalf("err %v calls %v", err, calls)
		}
	})

	t.Run("a single file asks once for its folder", func(t *testing.T) {
		fa := newFake()
		var calls []string
		root, stop, err := openFileWithAccess(fa, "/Users/me/notes/a.md", picked("/Users/me/notes", &calls))
		if err != nil || root != "/Users/me/notes" {
			t.Fatalf("root %q err %v", root, err)
		}
		stop()
		if len(calls) != 1 || calls[0] != "/Users/me/notes|"+msg {
			t.Errorf("panel calls = %v, want preselect and message %q", calls, msg)
		}
		calls = nil
		root, _, err = openFileWithAccess(fa, "/Users/me/notes/b.md", picked("x", &calls))
		if err != nil || root != "/Users/me/notes" || len(calls) != 0 {
			t.Errorf("second file in the folder: root %q err %v calls %v", root, err, calls)
		}
	})

	t.Run("declining shows the file alone", func(t *testing.T) {
		fa := newFake()
		root, stop, err := openFileWithAccess(fa, "/Users/me/notes/a.md", decline)
		if err != nil || root != "" {
			t.Fatalf("root %q err %v, want the file alone (root \"\", no error)", root, err)
		}
		if stop == nil {
			t.Error("stop is nil")
		} else {
			stop()
		}
		if len(fa.remembered) != 0 {
			t.Errorf("remembered %v", fa.remembered)
		}
	})
}

// fakeMaker stands in for NSURL bookmarks.
type fakeMaker struct {
	stale  map[string]bool
	moved  map[string]string
	closed int
}

func (m *fakeMaker) Create(path string) ([]byte, error) { return []byte("bm:" + path), nil }
func (m *fakeMaker) Resolve(data []byte) (string, bool, func(), error) {
	p := strings.TrimPrefix(string(data), "bm:")
	if to, ok := m.moved[p]; ok {
		p = to
	}
	return p, m.stale[p], func() { m.closed++ }, nil
}

func TestBookmarkStore(t *testing.T) {
	file := filepath.Join(t.TempDir(), "sub", "bookmarks.json")
	mk := &fakeMaker{stale: map[string]bool{}}
	fa := newStoreAccess(file, mk)

	if _, err := fa.Resolve("/a"); !errors.Is(err, ErrNoBookmark) {
		t.Fatalf("unknown folder: %v", err)
	}
	if err := fa.Remember("/a"); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(file)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("store file: %v %v", fi, err)
	}
	// A new process reads what the first one wrote.
	fa = newStoreAccess(file, mk)
	stop, err := fa.Resolve("/a")
	if err != nil {
		t.Fatal(err)
	}
	stop()
	if mk.closed != 1 {
		t.Errorf("stop did not end access (%d)", mk.closed)
	}
	mk.stale["/a"] = true
	if _, err := fa.Resolve("/a"); !errors.Is(err, ErrStale) {
		t.Errorf("stale bookmark: %v", err)
	}
	// A bookmark that now points somewhere else is stale too.
	mk.stale = map[string]bool{}
	mk.moved = map[string]string{"/a": "/elsewhere"}
	if _, err := fa.Resolve("/a"); !errors.Is(err, ErrStale) {
		t.Errorf("moved bookmark: %v", err)
	}
	// A corrupt store is ignored, not fatal.
	if err := os.WriteFile(file, []byte("{nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newStoreAccess(file, mk).Resolve("/a"); !errors.Is(err, ErrNoBookmark) {
		t.Errorf("corrupt store: %v", err)
	}
}

func TestConfigImportOnce(t *testing.T) {
	src := filepath.Join(t.TempDir(), "old.yml")
	if err := os.WriteFile(src, []byte("version: 1\nplugins: {lint: false}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	offers := 0
	offer := func() (string, bool) { offers++; return src, true }

	dir := t.TempDir()
	n, err := importConfigOnce(dir, offer)
	if err != nil || !n || offers != 1 {
		t.Fatalf("first run: imported %v err %v offers %d", n, err, offers)
	}
	b, err := os.ReadFile(filepath.Join(dir, "config.yml"))
	if err != nil || !strings.Contains(string(b), "lint: false") {
		t.Fatalf("config not imported: %q %v", b, err)
	}
	if n, err := importConfigOnce(dir, offer); err != nil || n || offers != 1 {
		t.Errorf("second run: imported %v err %v offers %d (must not offer again)", n, err, offers)
	}

	// Declining is remembered too.
	dir = t.TempDir()
	declined := 0
	no := func() (string, bool) { declined++; return "", false }
	for i := 0; i < 3; i++ {
		if n, err := importConfigOnce(dir, no); err != nil || n {
			t.Fatalf("declined run %d: %v %v", i, n, err)
		}
	}
	if declined != 1 {
		t.Errorf("offered %d times after declining, want 1", declined)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yml")); err == nil {
		t.Error("config.yml written although the user declined")
	}

	// An existing config is never overwritten.
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte("mine: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if n, err := importConfigOnce(dir, offer); err != nil || n {
		t.Errorf("existing config: imported %v err %v", n, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "config.yml")); string(b) != "mine: true\n" {
		t.Errorf("existing config overwritten: %q", b)
	}
}

// execAllowed lists every non-test file of internal/, apps/, kit/, engine/ and builtins/ that starts a process,
// and why that is fine in the App Store build.
var execAllowed = map[string]string{
	"engine/lint/lint.go":                 "Vale, the helper inside Contents/Helpers",
	"engine/scriptplugins/pool.go":        "zen's own executable as the Tengo helper (Contents/MacOS/zenmd)",
	"engine/extplugins/exec.go":           "program backends: refused by extplugins.Available(\"program\") in the App Store build",
	"builtins/plugins/sandbox/sandbox.go": "the container plugin: refused by extplugins.Available(\"sandbox\") in the App Store build",
	"kit/desktop/finder.go":               "/usr/bin/open for Show in Finder and the settings fallback, only on chosen or container paths; to become NSWorkspace before submission",
}

// TestNoExecOutsideBundle scans the sources for process starts. A new caller
// must be listed in execAllowed, with the reason it stays inside the bundle
// (or is switched off) in the App Store build.
func TestNoExecOutsideBundle(t *testing.T) {
	call := regexp.MustCompile(`\bexec\.Command(Context)?\(`)
	found := map[string]bool{}
	for _, root := range []string{"internal", "apps", "kit", "engine", "builtins"} {
		err := filepath.WalkDir(filepath.Join("..", "..", root), func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if call.Match(b) {
				found[filepath.ToSlash(strings.TrimPrefix(p, "../../"))] = true
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	var names []string
	for n := range found {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if _, ok := execAllowed[n]; !ok {
			t.Errorf("%s starts a process but is not in execAllowed: in the App Store build it may only run what is inside Contents/", n)
		}
	}
	for n := range execAllowed {
		if !found[n] {
			t.Errorf("execAllowed lists %s, which no longer starts a process; remove it", n)
		}
	}
}

func TestChannelConst(t *testing.T) {
	if Channel() != "devid" && Channel() != "appstore" {
		t.Fatalf("Channel() = %q", Channel())
	}
	if (Channel() == "appstore") != appStore {
		t.Errorf("Channel() = %q but appStore = %v", Channel(), appStore)
	}
}
