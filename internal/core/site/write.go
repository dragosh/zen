package site

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CheckOut refuses output folders that would damage the input: the folder
// itself, a parent of it, anything inside .zen, or a symlink. An output
// inside the folder is fine (the build leaves it out of its input). root may
// be "" when unknown: only the symlink check applies then.
func CheckOut(root, out string) error {
	abs, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	if fi, err := os.Lstat(abs); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("output folder %s is a symlink", abs)
	}
	real, err := resolveExisting(abs)
	if err != nil {
		return err
	}
	if root == "" {
		return nil
	}
	r, err := resolveExisting(root)
	if err != nil {
		return err
	}
	switch {
	case real == r:
		return fmt.Errorf("output folder %s is the folder being built", abs)
	case within(real, r) == false && within(r, real):
		return fmt.Errorf("output folder %s contains the folder being built", abs)
	case within(real, filepath.Join(r, ".zen")):
		return fmt.Errorf("output folder %s is inside .zen", abs)
	}
	return nil
}

// resolveExisting resolves symlinks in the longest existing part of p and
// keeps the rest as it is.
func resolveExisting(p string) (string, error) {
	p = filepath.Clean(p)
	var rest []string
	for cur := p; ; cur = filepath.Dir(cur) {
		if real, err := filepath.EvalSymlinks(cur); err == nil {
			for i := len(rest) - 1; i >= 0; i-- {
				real = filepath.Join(real, rest[i])
			}
			return real, nil
		}
		if cur == filepath.Dir(cur) {
			return p, nil
		}
		rest = append(rest, filepath.Base(cur))
	}
}

// within reports whether p is a, or inside a.
func within(p, a string) bool {
	rel, err := filepath.Rel(a, p)
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
}

// insideRel returns out relative to root (slash-separated) when out is inside root.
func insideRel(root, out string) (string, bool) {
	r, err := resolveExisting(root)
	if err != nil {
		return "", false
	}
	o, err := resolveExisting(out)
	if err != nil || o == r || !within(o, r) {
		return "", false
	}
	rel, err := filepath.Rel(r, o)
	if err != nil {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// writer writes files below one output folder: atomically, never through a
// symlink.
type writer struct {
	out    string
	marker bool // the folder is zen's (empty before, or has a marker)
}

func newWriter(out string) *writer { return &writer{out: out} }

// prepare checks the folder, cleans it when asked and writes the marker.
func (w *writer) prepare(clean bool) error {
	fi, err := os.Lstat(w.out)
	switch {
	case errors.Is(err, os.ErrNotExist):
		w.marker = true
	case err != nil:
		return err
	case fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir():
		return fmt.Errorf("output %s is not a folder", w.out)
	default:
		entries, err := os.ReadDir(w.out)
		if err != nil {
			return err
		}
		_, merr := os.Lstat(filepath.Join(w.out, markerName))
		hasMarker := merr == nil
		w.marker = len(entries) == 0 || hasMarker
		if clean {
			if !w.marker {
				return fmt.Errorf("will not clean %s: it is not empty and has no %s file from an earlier build", w.out, markerName)
			}
			for _, e := range entries {
				if err := os.RemoveAll(filepath.Join(w.out, e.Name())); err != nil {
					return err
				}
			}
		}
	}
	if err := os.MkdirAll(w.out, 0o755); err != nil {
		return err
	}
	if w.marker {
		return w.write(markerName, []byte("zen build output: this folder may be emptied by zen build --clean\n"))
	}
	return nil
}

// write writes rel (slash-separated, below the output folder).
func (w *writer) write(rel string, data []byte) error {
	if _, ok := cleanRel(rel); !ok {
		return fmt.Errorf("refusing to write %q", rel)
	}
	full := filepath.Join(w.out, filepath.FromSlash(rel))
	if err := w.ensureDir(filepath.Dir(full)); err != nil {
		return err
	}
	if fi, err := os.Lstat(full); err == nil && fi.IsDir() {
		return fmt.Errorf("%s is a folder", rel)
	}
	tmp, err := os.CreateTemp(filepath.Dir(full), ".zen-tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr == nil && cerr == nil {
		werr = os.Chmod(name, 0o644)
	}
	if err := firstErr(werr, cerr); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, full); err != nil { // replaces a symlink, does not follow it
		os.Remove(name)
		return err
	}
	return nil
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// ensureDir creates dir (inside the output folder), replacing any symlink on
// the way with a real folder.
func (w *writer) ensureDir(dir string) error {
	rel, err := filepath.Rel(w.out, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("path outside the output folder")
	}
	cur := w.out
	if rel == "." {
		return nil
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		cur = filepath.Join(cur, part)
		fi, err := os.Lstat(cur)
		switch {
		case errors.Is(err, os.ErrNotExist):
			if err := os.Mkdir(cur, 0o755); err != nil {
				return err
			}
		case err != nil:
			return err
		case fi.Mode()&os.ModeSymlink != 0:
			if err := os.Remove(cur); err != nil {
				return err
			}
			if err := os.Mkdir(cur, 0o755); err != nil {
				return err
			}
		case !fi.IsDir():
			return fmt.Errorf("%s is a file, not a folder", filepath.ToSlash(strings.TrimPrefix(cur, w.out+string(filepath.Separator))))
		}
	}
	return nil
}
