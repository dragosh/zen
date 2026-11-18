package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/zenmark-org/zenmd/engine/scriptplugins"
)

// TestMain lets the test binary act as the program: with ZENMD_TEST_MAIN set
// it runs main() instead of the tests, under whatever argv[0] the test gives.
func TestMain(m *testing.M) {
	if os.Getenv("ZENMD_TEST_MAIN") == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}

// The script helper runs under the zen alias too (the pool starts the same
// executable) and must print nothing of its own: stdout is a protocol.
func TestScriptHostIsQuiet(t *testing.T) {
	for _, name := range []string{"zen", "zenmd"} {
		cmd := exec.Command(os.Args[0])
		cmd.Args = []string{name, scriptplugins.HostArg}
		cmd.Env = append(os.Environ(), "ZENMD_TEST_MAIN=1")
		cmd.Stdin = strings.NewReader("") // not a terminal: the helper reads a pipe
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		_ = cmd.Run()
		if strings.Contains(out.String()+errOut.String(), "stops working") {
			t.Errorf("%s: the script host printed the alias notice: %q %q", name, out.String(), errOut.String())
		}
		if out.Len() != 0 {
			t.Errorf("%s: the script host wrote to stdout: %q", name, out.String())
		}
	}
}

// The alias notice goes to stderr once, and stdout stays the same.
func TestAliasNoticeFromMain(t *testing.T) {
	run := func(name string) (string, string) {
		cmd := exec.Command(os.Args[0])
		cmd.Args = []string{name, "--version"}
		cmd.Env = append(os.Environ(), "ZENMD_TEST_MAIN=1")
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if err := cmd.Run(); err != nil {
			t.Fatalf("%s --version: %v", name, err)
		}
		return out.String(), errOut.String()
	}
	zenOut, zenErr := run("zen")
	mdOut, mdErr := run("zenmd")
	if zenOut != mdOut || !strings.HasPrefix(zenOut, "zenmd ") {
		t.Errorf("stdout differs: %q vs %q", zenOut, mdOut)
	}
	if zenErr != "zen is now zenmd; the zen name stops working in 0.4.0\n" || mdErr != "" {
		t.Errorf("stderr: zen %q, zenmd %q", zenErr, mdErr)
	}
}
