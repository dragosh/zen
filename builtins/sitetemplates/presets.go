// Package sitetemplates holds zen's built-in site templates (core/init): a
// folder for each preset, each with a template.yml and the files it copies.
// core/sitetemplates installs them into the user's templates folder.
package sitetemplates

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strings"
	"time"
)

// FS has one folder per preset: techdocs, learning and planner. The "all:"
// prefix keeps dotfiles such as .zen.yml.
//
//go:embed all:techdocs all:learning all:planner
var FS embed.FS

// IDs lists the built-in templates in the order New Site shows them (the
// order of their template.yml files).
var IDs = []string{"techdocs", "learning", "planner"}

// Sub returns a file system with only the named templates, in the order
// given; it reports an unknown id with the known ones. Without ids it is
// empty.
func Sub(ids ...string) (fs.FS, error) {
	seen := map[string]bool{}
	for _, id := range ids {
		if !slices.Contains(IDs, id) {
			return nil, fmt.Errorf("unknown site template %q; known templates: %s", id, strings.Join(IDs, ", "))
		}
		if seen[id] {
			return nil, fmt.Errorf("site template %q is listed twice", id)
		}
		seen[id] = true
	}
	return subFS{ids: append([]string(nil), ids...)}, nil
}

// subFS exposes some of the embedded templates, listed in the order given.
type subFS struct{ ids []string }

func (s subFS) allowed(name string) bool {
	if name == "." {
		return true
	}
	top, _, _ := strings.Cut(name, "/")
	return slices.Contains(s.ids, top)
}

func (s subFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) || !s.allowed(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if name != "." {
		return FS.Open(name)
	}
	entries, _ := s.ReadDir(".")
	return &rootDir{entries: entries}, nil
}

func (s subFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) || !s.allowed(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	if name != "." {
		return FS.ReadDir(name)
	}
	out := make([]fs.DirEntry, 0, len(s.ids))
	for _, id := range s.ids {
		info, err := fs.Stat(FS, id)
		if err != nil {
			return nil, err
		}
		out = append(out, fs.FileInfoToDirEntry(info))
	}
	return out, nil
}

// rootDir is the open "." of a subFS.
type rootDir struct {
	entries []fs.DirEntry
	pos     int
}

func (d *rootDir) Stat() (fs.FileInfo, error) { return rootInfo{}, nil }
func (d *rootDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: ".", Err: errors.New("is a directory")}
}
func (d *rootDir) Close() error { return nil }
func (d *rootDir) ReadDir(n int) ([]fs.DirEntry, error) {
	rest := d.entries[d.pos:]
	if n <= 0 {
		d.pos = len(d.entries)
		return rest, nil
	}
	if len(rest) == 0 {
		return nil, io.EOF
	}
	if n > len(rest) {
		n = len(rest)
	}
	d.pos += n
	return rest[:n], nil
}

type rootInfo struct{}

func (rootInfo) Name() string       { return "." }
func (rootInfo) Size() int64        { return 0 }
func (rootInfo) Mode() fs.FileMode  { return fs.ModeDir | 0o555 }
func (rootInfo) ModTime() time.Time { return time.Time{} }
func (rootInfo) IsDir() bool        { return true }
func (rootInfo) Sys() any           { return nil }
