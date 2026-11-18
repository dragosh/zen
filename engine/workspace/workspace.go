// Package workspace represents the opened folder and is the only code that
// touches its files. Every path it accepts or returns is relative to the
// workspace root and uses "/" separators.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var (
	// ErrOutside is returned for paths that are not under the workspace root.
	ErrOutside = errors.New("workspace: path is outside the workspace")
	// ErrTooLarge is returned by Read for files larger than 10 MB.
	ErrTooLarge = errors.New("workspace: file is too large")
	// ErrNotMarkdown is returned by Write for anything but .md/.markdown files.
	ErrNotMarkdown = errors.New("workspace: not a markdown file")
	// ErrNotAsset is returned by Asset for extensions that are not servable images.
	ErrNotAsset = errors.New("workspace: not an asset file")
	// ErrTooManyFiles is returned by Open and Files when a folder is too big
	// to index (see MaxFiles and MaxEntries), e.g. a home folder.
	ErrTooManyFiles = errors.New("workspace: folder is too large")
)

// Limits on the folder scan. Variables so tests can lower them.
var (
	// MaxFiles is the most markdown files a workspace may contain.
	MaxFiles = 2000
	// MaxEntries is the most files and folders the scan visits (skipped
	// folders are not entered). It bounds the time spent on huge trees.
	MaxEntries = 50000
)

// TooManyFilesError says which limit a folder hit; it matches ErrTooManyFiles.
type TooManyFilesError struct {
	Markdown bool // true: MaxFiles hit; false: MaxEntries hit
	Limit    int
}

// Error says which limit was hit.
func (e *TooManyFilesError) Error() string {
	if e.Markdown {
		return fmt.Sprintf("this folder has more than %d markdown files; open a smaller folder", e.Limit)
	}
	return fmt.Sprintf("this folder is too large to open (more than %d files and folders); open a smaller folder", e.Limit)
}

// Is reports whether target is ErrTooManyFiles.
func (e *TooManyFilesError) Is(target error) bool { return target == ErrTooManyFiles }

// maxFileSize is the largest file Read returns.
const maxFileSize = 10 << 20

// LinkKind classifies a link found in a page.
type LinkKind string

const (
	kindInternal LinkKind = "internal"
	kindAsset    LinkKind = "asset"
	kindExternal LinkKind = "external"
	kindBroken   LinkKind = "broken"
)

// Link is the result of ResolveLink. For internal, asset and broken links
// Path is workspace-relative; for external links it is the original href.
type Link struct {
	Path, Anchor string
	Kind         LinkKind
}

// Workspace is an opened folder.
type Workspace struct {
	root                 string
	maxFiles, maxEntries int

	mu      sync.Mutex
	first   []string // the listing made by OpenContext, handed out once by Opened
	entries int      // files and folders the last walk visited
}

// Open returns the workspace for an existing directory, with the default
// limits MaxFiles and MaxEntries.
func Open(dir string) (*Workspace, error) { return OpenWithLimits(dir, 0, 0) }

// OpenWithLimits is Open with custom limits; 0 means the default.
func OpenWithLimits(dir string, maxFiles, maxEntries int) (*Workspace, error) {
	return OpenContext(context.Background(), dir, maxFiles, maxEntries, nil)
}

// OpenContext is OpenWithLimits that stops with ctx's error when ctx ends and
// calls onEntry (if not nil) with the number of files and folders visited so
// far, every 256 entries. Opened hands out the listing it made once.
func OpenContext(ctx context.Context, dir string, maxFiles, maxEntries int, onEntry func(int)) (*Workspace, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	root, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("workspace: %s is not a directory", dir)
	}
	w := &Workspace{root: root, maxFiles: maxFiles, maxEntries: maxEntries}
	// Refuse folders that are too big before anything indexes or watches them.
	files, err := w.FilesContext(ctx, onEntry)
	if err != nil {
		return nil, err
	}
	w.first = files
	return w, nil
}

// Root is the absolute, symlink-resolved path of the folder.
func (w *Workspace) Root() string { return w.root }

// Name is the folder's base name.
func (w *Workspace) Name() string { return filepath.Base(w.root) }

// Files lists every markdown file as sorted, "/"-separated relative paths.
func (w *Workspace) Files() ([]string, error) {
	return w.FilesContext(context.Background(), nil)
}

// Opened returns, once, the listing OpenContext made, so the first index
// does not walk the folder again; nil afterwards (call Files).
func (w *Workspace) Opened() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	f := w.first
	w.first = nil
	return f
}

// FilesContext is Files that stops with ctx's error when ctx ends and reports
// the entries visited through onEntry (every 256, may be nil).
func (w *Workspace) FilesContext(ctx context.Context, onEntry func(int)) ([]string, error) {
	maxFiles, maxEntries := MaxFiles, MaxEntries
	if w.maxFiles > 0 {
		maxFiles = w.maxFiles
	}
	if w.maxEntries > 0 {
		maxEntries = w.maxEntries
	}
	files := []string{}
	entries := 0
	err := filepath.WalkDir(w.root, func(p string, d fs.DirEntry, err error) error {
		if p == w.root {
			return err
		}
		if entries++; entries > maxEntries {
			return &TooManyFilesError{Limit: maxEntries}
		}
		if entries&255 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
			if onEntry != nil {
				onEntry(entries)
			}
		}
		if err != nil {
			// Unreadable entry below the root: leave it out.
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return fs.SkipDir
			}
			return nil
		}
		if !isMarkdown(name) {
			return nil
		}
		rel, err := filepath.Rel(w.root, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.Type().IsRegular() || (d.Type()&fs.ModeSymlink != 0 && w.isFile(rel)) {
			if len(files) == maxFiles {
				return &TooManyFilesError{Markdown: true, Limit: maxFiles}
			}
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	w.mu.Lock()
	w.entries = entries
	w.mu.Unlock()
	return files, nil
}

// Entries is the number of files and folders the last walk visited.
func (w *Workspace) Entries() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.entries
}

// Home returns the root README.md (any case), or "".
func (w *Workspace) Home() string { return w.readme("") }

// readme returns the relative path of the README.md directly in the
// workspace-relative directory dir, or "".
func (w *Workspace) readme(dir string) string {
	abs, err := w.Resolve(dir)
	if err != nil {
		return ""
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return ""
	}
	found := ""
	for _, e := range entries {
		name := e.Name()
		if !strings.EqualFold(name, "README.md") || !w.isFile(path.Join(dir, name)) {
			continue
		}
		if name == "README.md" {
			return path.Join(dir, name)
		}
		if found == "" {
			found = path.Join(dir, name)
		}
	}
	return found
}

// isFile reports whether rel is a regular file inside the workspace.
func (w *Workspace) isFile(rel string) bool {
	p, err := w.Resolve(rel)
	if err != nil {
		return false
	}
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// Resolve returns the absolute, symlink-resolved path for a relative path.
// The target does not have to exist. It returns ErrOutside for absolute
// inputs, NUL bytes and anything that ends up outside the root.
func (w *Workspace) Resolve(rel string) (string, error) {
	if strings.ContainsRune(rel, 0) || filepath.IsAbs(rel) ||
		strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, `\`) {
		return "", ErrOutside
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrOutside
	}
	resolved, err := evalExisting(filepath.Join(w.root, clean))
	if err != nil {
		return "", err
	}
	if !w.contains(resolved) {
		return "", ErrOutside
	}
	return resolved, nil
}

// contains reports whether the absolute path p is the root or below it.
func (w *Workspace) contains(p string) bool {
	if p == w.root {
		return true
	}
	prefix := w.root
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(p, prefix)
}

// evalExisting resolves symlinks in the longest existing prefix of p and
// appends the missing remainder, so paths of files yet to be created can be
// checked too. A dangling symlink is ErrOutside: its target cannot be checked.
func evalExisting(p string) (string, error) {
	rest := ""
	for {
		resolved, err := filepath.EvalSymlinks(p)
		if err == nil {
			return filepath.Join(resolved, rest), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		if _, lerr := os.Lstat(p); lerr == nil {
			return "", ErrOutside
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", err
		}
		rest = filepath.Join(filepath.Base(p), rest)
		p = parent
	}
}

// Read returns the bytes of a file; ErrTooLarge above 10 MB.
func (w *Workspace) Read(rel string) ([]byte, error) {
	p, err := w.Resolve(rel)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("workspace: %s is a directory", rel)
	}
	if fi.Size() > maxFileSize {
		return nil, ErrTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(f, maxFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFileSize {
		return nil, ErrTooLarge
	}
	return data, nil
}

// Write atomically replaces or creates a markdown file, keeping its mode.
func (w *Workspace) Write(rel string, data []byte) error {
	if !isMarkdown(rel) {
		return ErrNotMarkdown
	}
	p, err := w.Resolve(rel)
	if err != nil {
		return err
	}
	// rel may be a symlink: the real target has to be markdown as well.
	if !isMarkdown(p) {
		return ErrNotMarkdown
	}
	mode := fs.FileMode(0o644)
	switch fi, err := os.Stat(p); {
	case err == nil && !fi.Mode().IsRegular():
		return fmt.Errorf("workspace: %s is not a regular file", rel)
	case err == nil:
		mode = fi.Mode().Perm()
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(p), "."+filepath.Base(p)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func(err error) error {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return cleanup(err)
	}
	if err := tmp.Chmod(mode); err != nil {
		return cleanup(err)
	}
	if err := tmp.Sync(); err != nil {
		return cleanup(err)
	}
	if err := tmp.Close(); err != nil {
		return cleanup(err)
	}
	if err := os.Rename(tmpName, p); err != nil {
		return cleanup(err)
	}
	return nil
}

// Asset opens an image file for serving.
func (w *Workspace) Asset(rel string) (*os.File, error) {
	if !isAsset(rel) {
		return nil, ErrNotAsset
	}
	p, err := w.Resolve(rel)
	if err != nil {
		return nil, err
	}
	// rel may be a symlink: the real target has to be an asset as well.
	if !isAsset(p) {
		return nil, ErrNotAsset
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("workspace: %s is not a regular file", rel)
	}
	return f, nil
}

// ResolveLink resolves a link found in page from (a workspace-relative path).
// Links starting with "/" are relative to the workspace root.
func (w *Workspace) ResolveLink(from, href string) Link {
	if strings.HasPrefix(href, "//") || hasScheme(href) {
		return Link{Path: href, Kind: kindExternal}
	}
	if href == "" {
		return Link{Kind: kindBroken}
	}

	target, anchor, _ := strings.Cut(href, "#")
	target, _, _ = strings.Cut(target, "?")
	if a, err := url.PathUnescape(anchor); err == nil {
		anchor = a
	}
	if t, err := url.PathUnescape(target); err == nil {
		target = t
	}
	broken := Link{Anchor: anchor, Kind: kindBroken}
	if strings.ContainsRune(target, 0) || strings.ContainsRune(from, 0) {
		return broken
	}

	var rel string
	switch {
	case target == "":
		rel = path.Clean(from) // same-page anchor
	case strings.HasPrefix(target, "/"):
		rel = path.Clean(strings.TrimLeft(target, "/"))
	default:
		rel = path.Join(path.Dir(from), target)
	}
	if rel == ".." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
		return broken
	}
	if rel == "." {
		rel = ""
	}
	broken.Path = rel

	abs, err := w.Resolve(rel)
	if err != nil {
		return broken
	}
	fi, err := os.Stat(abs)
	switch {
	case err == nil && fi.IsDir():
		if readme := w.readme(rel); readme != "" {
			return Link{Path: readme, Anchor: anchor, Kind: kindInternal}
		}
	case err == nil && fi.Mode().IsRegular():
		// Classify by the real file, so a symlink cannot change the kind.
		if isMarkdown(rel) && isMarkdown(abs) {
			return Link{Path: rel, Anchor: anchor, Kind: kindInternal}
		}
		if isAsset(rel) && isAsset(abs) {
			return Link{Path: rel, Anchor: anchor, Kind: kindAsset}
		}
	case err != nil && rel != "" && !isMarkdown(rel):
		for _, ext := range []string{".md", ".markdown"} {
			if w.isFile(rel + ext) {
				return Link{Path: rel + ext, Anchor: anchor, Kind: kindInternal}
			}
		}
	}
	return broken
}

// hasScheme reports whether href starts with a URL scheme ("https:", "mailto:").
func hasScheme(href string) bool {
	for i, c := range href {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		case i > 0 && (c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'):
		case c == ':':
			return i > 0
		default:
			return false
		}
	}
	return false
}

func isMarkdown(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".markdown":
		return true
	}
	return false
}

func isAsset(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".avif":
		return true
	}
	return false
}
