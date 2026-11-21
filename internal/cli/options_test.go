package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// tableOption is one entry of apps/zenmark/export-options.json, the one table of export options: the CLI, its help and
// both Settings forms are checked against it.
type tableOption struct {
	Flag     string   `json:"flag"`
	Short    string   `json:"short"`
	Negative string   `json:"negative"`
	Arg      string   `json:"arg"`
	Values   []string `json:"values"`
	Key      *string  `json:"key"`
	Field    *string  `json:"field"`
	Help     string   `json:"help"`
}

func loadTable(t *testing.T) map[string][]tableOption {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "apps", "zenmark", "export-options.json"))
	if err != nil {
		t.Fatal(err)
	}
	var table map[string][]tableOption
	if err := json.Unmarshal(b, &table); err != nil {
		t.Fatal(err)
	}
	if len(table["site"]) < 6 || len(table["pdf"]) < 6 {
		t.Fatalf("the table is too short: %d site, %d pdf options", len(table["site"]), len(table["pdf"]))
	}
	return table
}

var spaces = regexp.MustCompile(`\s+`)

func squash(s string) string { return spaces.ReplaceAllString(s, " ") }

// argFor is a value for an option that takes one.
func argFor(o tableOption) []string {
	switch {
	case len(o.Values) > 0:
		return []string{o.Values[len(o.Values)-1]}
	case o.Arg == "<dir>":
		return []string{"outdir"}
	case o.Arg == "<file>":
		return []string{"x.pdf"}
	case o.Arg != "":
		return []string{"a.md"}
	}
	return nil
}

// Every option of the table is parsed (with its negative and its short form), for the website and the PDF; the
// aliases `build` and `export site` agree.
func TestEveryTableOptionIsParsed(t *testing.T) {
	table := loadTable(t)
	h, _ := pdfSetup(t)
	for target, opts := range table {
		for _, o := range opts {
			forms := [][]string{append([]string{o.Flag}, argFor(o)...)}
			if o.Negative != "" {
				forms = append(forms, []string{o.Negative})
			}
			if o.Short != "" {
				forms = append(forms, append([]string{o.Short}, argFor(o)...))
			}
			for _, f := range forms {
				args := []string{"export", target, "My Docs"}
				args = append(args, f...)
				if o.Flag == "--dry-run" {
					args = append(args, "--save-defaults")
				}
				r := run(t, h, nil, args...)
				if r.code != 0 || !(r.built || r.pdfd) {
					t.Errorf("%s %v: %+v", target, f, r)
				}
				if target == "site" {
					alias := run(t, h, nil, append([]string{"build"}, args[2:]...)...)
					if alias.code != 0 || alias.b != r.b {
						t.Errorf("build %v: %+v", f, alias)
					}
				}
			}
			// A value-taking option without its value is a usage error.
			if len(o.Values) > 0 || o.Arg != "" {
				if r := run(t, h, nil, "export", target, "My Docs", o.Flag); r.code != 2 {
					t.Errorf("%s %s without a value: %+v", target, o.Flag, r)
				}
			}
		}
		// An option that is not in the table is refused.
		if r := run(t, h, nil, "export", target, "My Docs", "--nope"); r.code != 2 {
			t.Errorf("%s --nope: %+v", target, r)
		}
	}
	// A wrong value of --scheme is refused with the choices.
	if r := run(t, h, nil, "export", "site", "My Docs", "--scheme", "blue"); r.code != 2 || !strings.Contains(r.stderr, "auto") || r.built {
		t.Errorf("--scheme blue: %+v", r)
	}
	// An option and its negative together are refused.
	for _, args := range [][]string{
		{"export", "site", "--drafts", "--no-drafts"}, {"export", "site", "--print", "--no-print"},
		{"export", "pdf", "--cover", "--no-cover"}, {"export", "pdf", "--open", "--no-open"},
		{"export", "pdf", "--drafts", "--no-drafts"}, {"export", "pdf", "--section", "a.md", "--all"},
	} {
		if r := run(t, h, nil, append(args, "My Docs")...); r.code != 2 || r.built || r.pdfd || !strings.Contains(r.stderr, "cannot be used together") {
			t.Errorf("%v: %+v", args, r)
		}
	}
}

// The help lists every option of the table with its negative, short form and help line, under "Export site options"
// and "Export pdf options", and nothing that is not in the table; no line is longer than 80 columns.
func TestHelpFollowsTheTable(t *testing.T) {
	table := loadTable(t)
	out := run(t, "/", nil, "--help").stdout
	if strings.Contains(out, "Build options") {
		t.Error(`the help still says "Build options"`)
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) > 80 {
			t.Errorf("line too long: %q", line)
		}
	}
	for target, heading := range map[string]string{"site": "Export site options", "pdf": "Export pdf options"} {
		i := strings.Index(out, heading)
		if i < 0 {
			t.Fatalf("no %q in the help:\n%s", heading, out)
		}
		section := out[i:]
		if j := strings.Index(section[len(heading):], "\n\n"); j >= 0 {
			section = section[:len(heading)+j]
		}
		flat := squash(section)
		listed := map[string]bool{}
		for _, m := range regexp.MustCompile(`(?m)^\s+(-[^ ,]+(?:, -[^ ,]+)*)`).FindAllStringSubmatch(section, -1) {
			for _, f := range strings.Split(m[1], ", ") {
				listed[f] = true
			}
		}
		want := map[string]bool{}
		for _, o := range table[target] {
			for _, f := range []string{o.Flag, o.Negative, o.Short} {
				if f != "" {
					want[f] = true
					if !listed[f] {
						t.Errorf("%s help lacks %s:\n%s", target, f, section)
					}
				}
			}
			if !strings.Contains(flat, squash(o.Help)) {
				t.Errorf("%s help lacks the line of %s: %q", target, o.Flag, o.Help)
			}
			for _, v := range o.Values {
				if !strings.Contains(flat, v) {
					t.Errorf("%s help lacks the value %s of %s", target, v, o.Flag)
				}
			}
			if o.Arg != "" && !strings.Contains(flat, o.Flag+" "+o.Arg) && !strings.Contains(flat, o.Short+", "+o.Flag+" "+o.Arg) {
				t.Errorf("%s help lacks %s %s", target, o.Flag, o.Arg)
			}
		}
		for f := range listed {
			if f != "-h" && f != "--help" && !want[f] {
				t.Errorf("%s help lists %s, which is not in the table", target, f)
			}
		}
	}
}
