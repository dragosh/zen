package sitetemplates

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Limits of one template: larger ones are refused before anything is written.
var (
	MaxFiles       = 2000
	MaxBytes int64 = 50 << 20
)

// maxClashes is how many clashing files an error names.
const maxClashes = 10

// ErrClash is wrapped by the error of a Create that found files in the way.
var ErrClash = errors.New("files already exist")

// Result lists what Create wrote: target-relative paths, in creation order.
type Result struct{ Files []string }

// now is the clock behind {{date}}; tests replace it.
var now = time.Now

// placeholderRE is one {{name}}: no spaces, so "{{ title }}" stays as it is.
var placeholderRE = regexp.MustCompile(`\{\{([A-Za-z][A-Za-z0-9_-]*)\}\}`)

// textExts are the files whose contents get placeholders.
var textExts = map[string]bool{".md": true, ".yml": true, ".yaml": true, ".txt": true, ".css": true}

func replace(s string, vals map[string]string) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	return placeholderRE.ReplaceAllStringFunc(s, func(m string) string {
		if v, ok := vals[m[2:len(m)-2]]; ok {
			return v
		}
		return m
	})
}

// values works out every placeholder: the built-ins, then each ask key from
// values (an empty one takes its default, which may use {{folder}}).
func (t Template) values(given map[string]string, folder string) (map[string]string, error) {
	out := map[string]string{"date": now().Format("2006-01-02"), "folder": folder, "title": "", "author": ""}
	asked := map[string]bool{}
	for _, a := range t.Ask {
		asked[a.Key] = true
	}
	if !asked["title"] {
		out["title"] = folder
	}
	for _, a := range t.Ask {
		out[a.Key] = replace(a.Default, out)
	}
	for k, v := range given {
		if k == "title" || k == "author" || asked[k] {
			if strings.ContainsRune(v, 0) {
				return nil, fmt.Errorf("the value of %s contains a NUL byte", k)
			}
			if len(v) > 4096 {
				return nil, fmt.Errorf("the value of %s is too long", k)
			}
			if v != "" {
				out[k] = v
			}
		}
	}
	return out, nil
}

type item struct {
	src  string // slash path in the template
	dest string // slash path in the target
	size int64
}

// plan walks the template and works out what would be written. It refuses
// symlinks, special files, names that leave the target, duplicates and
// templates over the limits, all before anything is written.
func (t Template) plan(vals map[string]string) ([]item, error) {
	if t.Err != "" {
		return nil, fmt.Errorf("template %q is not valid: %s", t.ID, t.Err)
	}
	if t.fsys == nil {
		return nil, fmt.Errorf("template %q has no files", t.ID)
	}
	var items []item
	var total int64
	seen := map[string]string{}
	err := fs.WalkDir(t.fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symbolic link; templates cannot contain links", p)
		}
		if t.ignored(p) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s is not a regular file", p)
		}
		if p == "template.yml" {
			return nil
		}
		dest := replace(p, vals)
		clean := path.Clean(strings.ReplaceAll(dest, "\\", "/"))
		if strings.ContainsRune(dest, 0) || path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") || clean == "." {
			return fmt.Errorf("%s would be written outside the new folder", p)
		}
		if other, dup := seen[strings.ToLower(clean)]; dup {
			return fmt.Errorf("%s and %s would both be written to %s", other, p, clean)
		}
		seen[strings.ToLower(clean)] = p
		info, err := d.Info()
		if err != nil {
			return err
		}
		items = append(items, item{src: p, dest: clean, size: info.Size()})
		total += info.Size()
		if len(items) > MaxFiles {
			return fmt.Errorf("the template has more than %d files", MaxFiles)
		}
		if total > MaxBytes {
			return fmt.Errorf("the template is larger than %d MB", MaxBytes>>20)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ignored reports whether the template leaves p out: by its ignore: list
// (matched against the name and the whole path) and always .DS_Store.
func (t Template) ignored(p string) bool {
	base := path.Base(p)
	if base == ".DS_Store" {
		return true
	}
	for _, pat := range t.ignore {
		if ok, _ := path.Match(pat, base); ok {
			return true
		}
		if ok, _ := path.Match(pat, p); ok {
			return true
		}
	}
	return false
}

// Create copies the template into target, a new or an existing folder.
// Nothing is ever overwritten: when any file the template would write exists,
// nothing is written and the error wraps ErrClash with up to 10 names.
// Placeholders are replaced in text files and in file names. A failure
// half-way removes what was written.
func Create(t Template, target string, values map[string]string) (Result, error) {
	if target == "" {
		return Result{}, errors.New("choose a folder")
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return Result{}, err
	}
	if t.insideTemplates(abs) {
		return Result{}, errors.New("choose a folder outside the templates folder")
	}
	folder := filepath.Base(abs)
	vals, err := t.values(values, folder)
	if err != nil {
		return Result{}, err
	}
	items, err := t.plan(vals)
	if err != nil {
		return Result{}, err
	}

	// The target: new, or an existing folder.
	if fi, err := os.Stat(abs); err == nil && !fi.IsDir() {
		return Result{}, fmt.Errorf("%s is a file, not a folder", folder)
	}
	var clashes []string
	for _, it := range items {
		if c := clashAt(abs, it.dest); c {
			clashes = append(clashes, it.dest)
		}
	}
	if len(clashes) > 0 {
		list := strings.Join(clashes[:min(len(clashes), maxClashes)], ", ")
		if len(clashes) > maxClashes {
			list += fmt.Sprintf(" and %d more", len(clashes)-maxClashes)
		}
		return Result{}, fmt.Errorf("%w: %s", ErrClash, list)
	}

	// Write, remembering what is ours so a failure can undo it.
	var madeDirs, madeFiles []string
	undo := func() {
		for i := len(madeFiles) - 1; i >= 0; i-- {
			os.Remove(madeFiles[i])
		}
		for i := len(madeDirs) - 1; i >= 0; i-- {
			os.Remove(madeDirs[i])
		}
	}
	mkdirs := func(dir string) error {
		var todo []string
		for d := dir; ; d = filepath.Dir(d) {
			if _, err := os.Lstat(d); err == nil {
				break
			}
			todo = append(todo, d)
			if filepath.Dir(d) == d {
				break
			}
		}
		for i := len(todo) - 1; i >= 0; i-- {
			if err := os.Mkdir(todo[i], 0o755); err != nil {
				return err
			}
			_ = os.Chmod(todo[i], 0o755)
			madeDirs = append(madeDirs, todo[i])
		}
		return nil
	}
	if err := mkdirs(abs); err != nil {
		undo()
		return Result{}, err
	}
	var res Result
	for _, it := range items {
		full := filepath.Join(abs, filepath.FromSlash(it.dest))
		if err := mkdirs(filepath.Dir(full)); err != nil {
			undo()
			return Result{}, err
		}
		if err := t.writeFile(it, full, vals); err != nil {
			undo()
			if errors.Is(err, fs.ErrExist) {
				return Result{}, fmt.Errorf("%w: %s", ErrClash, it.dest)
			}
			return Result{}, fmt.Errorf("writing %s: %w", it.dest, err)
		}
		madeFiles = append(madeFiles, full)
		res.Files = append(res.Files, it.dest)
	}
	return res, nil
}

// clashAt reports whether dest (slash path) cannot be created under root
// without touching something: it exists, or a parent is a file or a link.
func clashAt(root, dest string) bool {
	cur := root
	parts := strings.Split(dest, "/")
	for i, part := range parts {
		cur = filepath.Join(cur, part)
		fi, err := os.Lstat(cur)
		switch {
		case err != nil:
			return false // does not exist (or its parent does not): free
		case i == len(parts)-1:
			return true
		case !fi.IsDir(): // a file or a symlink where a folder goes
			return true
		}
	}
	return false
}

// insideTemplates reports whether abs is the templates folder or inside it.
func (t Template) insideTemplates(abs string) bool {
	if t.Dir == "" {
		return false
	}
	tdir := filepath.Dir(t.Dir) // .../sites
	if filepath.Base(tdir) == "sites" {
		tdir = filepath.Dir(tdir)
	}
	return within(real(abs), real(tdir))
}

// real resolves symlinks of the longest existing part of p.
func real(p string) string {
	rest := ""
	for cur := p; ; cur = filepath.Dir(cur) {
		if r, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(r, rest)
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		if filepath.Dir(cur) == cur {
			return p
		}
	}
}

func within(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// writeFile writes one file through a temporary file in its folder; the final
// name is taken with a hard link, which fails when something appeared there
// meanwhile (nothing is ever replaced). Files are 0644.
func (t Template) writeFile(it item, full string, vals map[string]string) error {
	in, err := t.fsys.Open(it.src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(full), ".zen-new-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	var werr error
	if textExts[strings.ToLower(path.Ext(it.src))] {
		var data []byte
		if data, werr = io.ReadAll(io.LimitReader(in, MaxBytes+1)); werr == nil {
			_, werr = tmp.WriteString(replace(string(data), vals))
		}
	} else {
		_, werr = io.Copy(tmp, in)
	}
	if werr == nil {
		werr = tmp.Chmod(0o644)
	}
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return werr
	}
	if err := os.Link(name, full); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return err
		}
		// No hard links on this volume: rename, after one more look.
		if _, serr := os.Lstat(full); serr == nil {
			return fs.ErrExist
		}
		return os.Rename(name, full)
	}
	return nil
}
