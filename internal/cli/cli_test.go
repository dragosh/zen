package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type result struct {
	code     int
	stdout   string
	stderr   string
	launched bool
	files    []string
}

func run(t *testing.T, launchErr error, args ...string) result {
	t.Helper()
	var out, errOut bytes.Buffer
	r := result{}
	r.code = Run(args, &out, &errOut, func(files []string) error {
		r.launched = true
		r.files = files
		return launchErr
	})
	r.stdout, r.stderr = out.String(), errOut.String()
	return r
}

func TestRunNoArgsLaunches(t *testing.T) {
	r := run(t, nil)
	if r.code != 0 || !r.launched || len(r.files) != 0 {
		t.Fatalf("got %+v", r)
	}
}

func TestRunFilesAreAbsolute(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(t.TempDir(), "b.md")

	r := run(t, nil, "a.md", abs)

	want := []string{filepath.Join(wd, "a.md"), abs}
	if r.code != 0 || !reflect.DeepEqual(r.files, want) {
		t.Fatalf("files = %v, want %v (code %d)", r.files, want, r.code)
	}
}

func TestRunVersion(t *testing.T) {
	for _, flag := range []string{"--version", "-v"} {
		r := run(t, nil, flag)
		if r.code != 0 || r.launched || r.stdout != "zen "+Version+"\n" {
			t.Fatalf("%s: got %+v", flag, r)
		}
	}
}

func TestRunHelp(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		r := run(t, nil, flag)
		if r.code != 0 || r.launched || !strings.Contains(r.stdout, "Usage:") {
			t.Fatalf("%s: got %+v", flag, r)
		}
	}
}

func TestRunUnknownFlag(t *testing.T) {
	r := run(t, nil, "--nope", "a.md")
	if r.code != 2 || r.launched || r.stdout != "" {
		t.Fatalf("got %+v", r)
	}
	if !strings.Contains(r.stderr, "--nope") || !strings.Contains(r.stderr, "Usage:") {
		t.Fatalf("stderr = %q", r.stderr)
	}
}

func TestRunDoubleDash(t *testing.T) {
	r := run(t, nil, "--", "-v")
	if r.code != 0 || !r.launched || len(r.files) != 1 || filepath.Base(r.files[0]) != "-v" {
		t.Fatalf("got %+v", r)
	}
}

func TestRunIgnoresProcessSerialNumber(t *testing.T) {
	r := run(t, nil, "-psn_0_12345")
	if r.code != 0 || !r.launched || len(r.files) != 0 {
		t.Fatalf("got %+v", r)
	}
}

func TestRunLaunchError(t *testing.T) {
	r := run(t, errors.New("boom"))
	if r.code != 1 || !strings.Contains(r.stderr, "boom") {
		t.Fatalf("got %+v", r)
	}
}
