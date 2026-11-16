package scriptplugins_test

import (
	"os"
	"testing"

	"github.com/dragosh/zen/internal/core/scriptplugins"
)

// The test binary is its own script helper: the pool starts it as
// `<test binary> __script-host`.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == scriptplugins.HostArg {
		os.Exit(scriptplugins.HostMain(os.Stdin, os.Stdout, os.Stderr))
	}
	code := m.Run()
	scriptplugins.Shutdown()
	os.Exit(code)
}
