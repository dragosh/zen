package sitetemplates

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/zenmark-org/zenmd/internal/atomicfile"
	"gopkg.in/yaml.v3"
)

// RecordFile records which built-in presets were installed, and in which
// version; it sits in <templates dir>/sites.
const RecordFile = ".zen-templates.yml"

type record struct {
	Installed map[string]string `yaml:"installed"`
}

func readRecord(sites string) record {
	var r record
	if data, err := os.ReadFile(filepath.Join(sites, RecordFile)); err == nil {
		_ = yaml.Unmarshal(data, &r)
	}
	if r.Installed == nil {
		r.Installed = map[string]string{}
	}
	return r
}

func writeRecord(sites string, r record) error {
	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	head := "# Written by zen: which built-in templates were installed here, and in which version.\n# A template that is listed here and was deleted is not installed again.\n"
	return atomicfile.Write(filepath.Join(sites, RecordFile), append([]byte(head), data...))
}

// builtinIDs lists the valid presets of a bundle, sorted.
func builtinIDs(builtins fs.FS) ([]Template, error) {
	list, _ := Builtins(builtins)
	var out []Template
	for _, t := range list {
		if t.Err == "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no built-in templates")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Install copies the built-in presets that were never installed into
// <dir>/sites. An installed template is never overwritten and one the user
// deleted is not installed again (the record remembers). A folder the user
// made with the same name is theirs and is left alone. An error means the
// folder cannot be written; the presets can then be used from the bundle.
func Install(dir string, builtins fs.FS) error {
	sites := SitesDir(dir)
	if err := os.MkdirAll(sites, 0o755); err != nil {
		return err
	}
	list, err := builtinIDs(builtins)
	if err != nil {
		return err
	}
	rec := readRecord(sites)
	changed := false
	for _, t := range list {
		if _, done := rec.Installed[t.ID]; done {
			continue
		}
		dest := filepath.Join(sites, t.ID)
		if _, err := os.Lstat(dest); err == nil {
			continue // the user's own
		}
		if err := copyTree(t.fsys, dest); err != nil {
			return fmt.Errorf("installing %s: %w", t.ID, err)
		}
		rec.Installed[t.ID] = t.Version
		changed = true
	}
	if changed || !fileExists(filepath.Join(sites, RecordFile)) {
		return writeRecord(sites, rec)
	}
	return nil
}

func fileExists(p string) bool { _, err := os.Lstat(p); return err == nil }

// Update writes the built-in template id, when it is newer than the one that
// was installed, to <dir>/sites/<id>.new beside the user's copy, and returns
// that folder. The user's copy is never touched.
func Update(dir string, builtins fs.FS, id string) (string, error) {
	sites := SitesDir(dir)
	rec := readRecord(sites)
	have, ok := rec.Installed[id]
	if !ok {
		return "", fmt.Errorf("%q is not a built-in template that was installed here", id)
	}
	var t *Template
	list, _ := Builtins(builtins)
	for i := range list {
		if list[i].ID == id && list[i].Err == "" {
			t = &list[i]
		}
	}
	if t == nil {
		return "", fmt.Errorf("this zen has no built-in template %q", id)
	}
	if !newer(t.Version, have) {
		return "", fmt.Errorf("%s is up to date (version %s)", id, have)
	}
	dest := filepath.Join(sites, id+".new")
	if fileExists(dest) {
		return "", fmt.Errorf("%s already exists; compare it with your copy and delete it first", path.Base(dest))
	}
	if err := copyTree(t.fsys, dest); err != nil {
		return "", err
	}
	rec.Installed[id] = t.Version
	if err := writeRecord(sites, rec); err != nil {
		return dest, err
	}
	return dest, nil
}

// copyTree copies a template folder to dest through a temporary folder, so a
// failure leaves nothing half done. Files are 0644, folders 0755.
func copyTree(src fs.FS, dest string) error {
	parent := filepath.Dir(dest)
	tmp, err := os.MkdirTemp(parent, ".tmp-install-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	err = fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(tmp, filepath.FromSlash(p))
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type().IsRegular():
			data, err := fs.ReadFile(src, p)
			if err != nil {
				return err
			}
			return os.WriteFile(target, data, 0o644)
		}
		return nil // links and special files are not copied
	})
	if err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}
