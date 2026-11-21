package cli

import (
	"fmt"
	"slices"
	"strings"
)

// The options of `zenmd export site` (and build) and `zenmd export pdf` are listed once, here, and the parser and the
// help are made from the lists. apps/zenmark/export-options.json is the same table for the Settings forms; its tests
// check that this list, the help and both forms agree with it, so an option cannot exist in one place only.

// option is one flag, with its short form, its negative and its argument.
type option struct {
	Short    string   // "-o"
	Flag     string   // "--out"
	Negative string   // "--no-drafts": the opposite flag, when there is one
	Arg      string   // "<dir>": the argument it takes, when it takes one
	Values   []string // the values of an argument that has a fixed set
	Help     string
}

// perRun options are never saved by --save-defaults.
const (
	flagSaveDefaults = "--save-defaults"
	flagDryRun       = "--dry-run"
)

var saveOptions = []option{
	{Flag: flagDryRun, Help: "with --save-defaults: show the change, write nothing and export nothing"},
}

var siteOptions = []option{
	{Short: "-o", Flag: "--out", Arg: "<dir>", Help: "output folder (default: exports/html in the folder)"},
	{Flag: "--portable", Negative: "--hosted", Help: "portable opens from disk and from any web server (the default); hosted loads scripts as files, for a web server only"},
	{Flag: "--drafts", Negative: "--no-drafts", Help: `include pages with "draft: true"`},
	{Flag: "--clean", Help: "empty the output folder first (needs a .zen-build file)"},
	{Flag: "--print", Negative: "--no-print", Help: "write print.html, the whole site as one page"},
	{Flag: "--scheme", Values: []string{"auto", "light", "dark"}, Help: "the website's colours: auto follows the reader's system, light is black on white, dark is white on black"},
	{Flag: flagSaveDefaults, Help: "write the options given here into the folder's .zenmd.yml under export:, then export"},
	saveOptions[0],
}

var pdfOptions = []option{
	{Short: "-o", Flag: "--out", Arg: "<file>", Help: "output file (default: exports/pdf/<folder name>.pdf)"},
	{Flag: "--drafts", Negative: "--no-drafts", Help: `include pages with "draft: true"`},
	{Flag: "--section", Arg: "<page>", Negative: "--all", Help: "only this page and the pages under it; --all is the whole folder (the default)"},
	{Flag: "--cover", Negative: "--no-cover", Help: "the cover and the contents (on by default)"},
	{Flag: "--open", Negative: "--no-open", Help: "open the PDF when it is written (off here, on in the app)"},
	{Flag: flagSaveDefaults, Help: "write the options given here into the folder's .zenmd.yml under export.pdf, then export"},
	saveOptions[0],
}

// left is the flags of an option as the help shows them: "-o, --out <dir>", "--drafts, --no-drafts".
func (o option) left() string {
	main := o.Flag
	switch {
	case o.Arg != "":
		main += " " + o.Arg
	case len(o.Values) > 0:
		main += " " + strings.Join(o.Values, "|")
	}
	parts := []string{main}
	if o.Short != "" {
		parts = []string{o.Short, main}
	}
	if o.Negative != "" {
		parts = append(parts, o.Negative)
	}
	return strings.Join(parts, ", ")
}

// optionHelp is the lines of the options under a heading: the flags in a column, their help wrapped beside it,
// no line longer than 80 columns.
func optionHelp(heading string, opts []option) string {
	col := 0
	for _, o := range opts {
		col = max(col, len(o.left()))
	}
	indent := "  "
	var sb strings.Builder
	sb.WriteString(heading + ":\n")
	for _, o := range opts {
		lines := wrap(o.Help, 80-len(indent)-col-2)
		for i, l := range lines {
			left := ""
			if i == 0 {
				left = o.left()
			}
			sb.WriteString(strings.TrimRight(fmt.Sprintf("%s%-*s  %s", indent, col, left, l), " ") + "\n")
		}
	}
	return sb.String()
}

// wrap breaks text into lines of at most width columns, at spaces.
func wrap(text string, width int) []string {
	var lines []string
	line := ""
	for _, w := range strings.Fields(text) {
		if line != "" && len(line)+1+len(w) > width {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += w
	}
	return append(lines, line)
}

// flags is what parseOptions found on a command line.
type flags struct {
	on, off map[string]bool   // by the positive flag: given as it, or as its negative
	val     map[string]string // the argument of an option that takes one
	paths   []string
	help    bool
}

// given reports whether the option, or its negative, was on the command line.
func (f flags) given(flag string) bool { return f.on[flag] || f.off[flag] }

// parseOptions reads args against the options of one export. Anything after "--" is a path; a flag and its negative
// together, a missing or wrong argument and an unknown option are usage errors.
func parseOptions(args []string, opts []option) (flags, error) {
	f := flags{on: map[string]bool{}, off: map[string]bool{}, val: map[string]string{}}
	byName := map[string]struct {
		o   option
		neg bool
	}{}
	for _, o := range opts {
		byName[o.Flag] = struct {
			o   option
			neg bool
		}{o, false}
		if o.Short != "" {
			byName[o.Short] = byName[o.Flag]
		}
		if o.Negative != "" {
			byName[o.Negative] = struct {
				o   option
				neg bool
			}{o, true}
		}
	}
	noun := func(o option) string {
		switch {
		case o.Arg == "<dir>":
			return "a folder"
		case o.Arg == "<file>":
			return "a file name"
		case o.Arg != "":
			return "a " + strings.Trim(o.Arg, "<>")
		}
		return "a value: " + strings.Join(o.Values, ", ")
	}
	flagsOn := true
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !flagsOn || !strings.HasPrefix(arg, "-") || arg == "-" {
			f.paths = append(f.paths, arg)
			continue
		}
		if arg == "--" {
			flagsOn = false
			continue
		}
		if arg == "-h" || arg == "--help" {
			f.help = true
			return f, nil
		}
		name, value, hasValue := arg, "", false
		if k := strings.Index(arg, "="); k > 0 {
			name, value, hasValue = arg[:k], arg[k+1:], true
		}
		e, ok := byName[name]
		if !ok {
			return f, usageError("unknown option " + arg)
		}
		o := e.o
		if e.neg || o.Arg == "" && len(o.Values) == 0 {
			if hasValue {
				return f, usageError(name + " takes no value")
			}
			if e.neg {
				f.off[o.Flag] = true
			} else {
				f.on[o.Flag] = true
			}
			if f.on[o.Flag] && f.off[o.Flag] {
				return f, usageError(o.Flag + " and " + o.Negative + " cannot be used together")
			}
			continue
		}
		if !hasValue {
			if i+1 >= len(args) {
				return f, usageError(name + " needs " + noun(o))
			}
			i++
			value = args[i]
		}
		if value == "" {
			return f, usageError(name + " needs " + noun(o))
		}
		if len(o.Values) > 0 && !slices.Contains(o.Values, value) {
			return f, usageError(o.Flag + " must be " + strings.Join(o.Values[:len(o.Values)-1], ", ") + " or " + o.Values[len(o.Values)-1])
		}
		f.on[o.Flag] = true
		f.val[o.Flag] = value
		if o.Negative != "" && f.off[o.Flag] {
			return f, usageError(o.Flag + " and " + o.Negative + " cannot be used together")
		}
	}
	return f, nil
}
