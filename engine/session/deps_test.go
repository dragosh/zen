package session

import (
	"os/exec"
	"strings"
	"testing"
)

// The session is headless: no Wails, no app, no command line.
func TestNoGUIImports(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	for _, pkg := range strings.Fields(string(out)) {
		for _, banned := range []string{
			"github.com/wailsapp/",
			"github.com/zenmark-org/zenmd/apps/zenmark",
		} {
			if strings.HasPrefix(pkg, banned) {
				t.Errorf("engine/session depends on %s", pkg)
			}
		}
	}
}
