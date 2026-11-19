package build_test

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The release workflow builds with the repository variable ADDONS (empty when
// unset), so the official DMG has the add-ons the project chose, and the
// release notes list them.
func TestReleaseUsesAddons(t *testing.T) {
	b, err := os.ReadFile("../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	var wf struct {
		Jobs map[string]struct {
			Env   map[string]string `yaml:"env"`
			Steps []struct {
				Run string            `yaml:"run"`
				Env map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(b, &wf); err != nil {
		t.Fatal(err)
	}
	job := wf.Jobs["dmg"]
	if got := strings.ReplaceAll(job.Env["ADDONS"], " ", ""); got != "${{vars.ADDONS}}" {
		t.Fatalf("job env ADDONS = %q, want ${{ vars.ADDONS }}", job.Env["ADDONS"])
	}
	// The steps that build and publish inherit it; none overrides it.
	release := false
	for _, s := range job.Steps {
		if v, ok := s.Env["ADDONS"]; ok {
			t.Errorf("step %q overrides ADDONS = %q", s.Run, v)
		}
		if strings.Contains(s.Run, "task release") {
			release = true
		}
	}
	if !release {
		t.Fatal("no step runs `task release`")
	}

	// The notes task of the Taskfile writes the add-ons of the build.
	tf, err := os.ReadFile("../Taskfile.yml")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Tasks map[string]struct {
			Cmds []any `yaml:"cmds"`
		} `yaml:"tasks"`
	}
	if err := yaml.Unmarshal(tf, &f); err != nil {
		t.Fatal(err)
	}
	notes := false
	for _, c := range f.Tasks["release:notes"].Cmds {
		if s, ok := c.(string); ok && strings.Contains(s, "ADDONS") && strings.Contains(s, "release-notes.md") {
			notes = true
		}
	}
	if !notes {
		t.Error("release:notes does not add the add-ons of the build to the notes")
	}
}
