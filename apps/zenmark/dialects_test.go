package app

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/zenmark-org/zenmd/engine/session"
)

var mystFolder = map[string]string{"README.md": "# Home\n\n```{note} Hi\nbody\n```\n", "myst.yml": "version: 1\n"}

func TestZenmarkHasDialects(t *testing.T) {
	if !reflect.DeepEqual(assemblyInfo.Dialects, []string{"myst"}) {
		t.Errorf("assembly dialects %v", assemblyInfo.Dialects)
	}
	if got := assemblyDialects().List(); len(got) != 1 || got[0].ID() != "myst" {
		t.Errorf("assemblyDialects %v", got)
	}
	if b := buildInfo(); !reflect.DeepEqual(b.Dialects, []string{"myst"}) {
		t.Errorf("buildInfo %+v", b)
	}
}

func TestDialectsService(t *testing.T) {
	dir := tree(t, mystFolder)
	z, _, _, _ := newTestZen(t, dir, "", "read")
	if got := z.Startup().DialectHint; got != "myst" {
		t.Errorf("Startup hint %q", got)
	}
	if got := z.DialectHint(); got != "myst" {
		t.Errorf("DialectHint %q", got)
	}
	if d := z.Dialects(); len(d) != 1 || d[0].ID != "myst" || d[0].On {
		t.Errorf("Dialects %+v", d)
	}
	if err := z.SetDialect("myst", true); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, ".zen.yml")); !strings.Contains(string(b), "dialects: [myst]") {
		t.Errorf(".zen.yml: %q", b)
	}
	if got := z.Startup().DialectHint; got != "" {
		t.Errorf("hint after turning on: %q", got)
	}
	if d := z.Dialects(); !d[0].On {
		t.Errorf("Dialects %+v", d)
	}
	if err := z.SetDialect("myst", false); err != nil {
		t.Fatal(err)
	}
	if err := z.DismissDialectHint("myst"); err != nil {
		t.Fatal(err)
	}
	if got := z.Startup().DialectHint; got != "" {
		t.Errorf("hint after Not now: %q", got)
	}
}

func TestDialectsReadOnlyApp(t *testing.T) {
	dir := tree(t, mystFolder)
	z, err := newZen(zenOptions{
		Registry: nil, Dir: dir, ReadOnly: true, GlobalPath: filepath.Join(t.TempDir(), "config.yml"),
		Events: &sink{}, Dialogs: &fakeDialogs{}, Open: func(string) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	if err := z.SetDialect("myst", true); !errors.Is(err, session.ErrReadOnly) {
		t.Errorf("SetDialect: %v", err)
	}
	if got := z.Startup().DialectHint; got != "" {
		t.Errorf("read-only hint %q", got)
	}
	if d := z.Dialects(); len(d) != 1 || d[0].On {
		t.Errorf("Dialects %+v", d)
	}
}
