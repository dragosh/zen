package scriptplugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/dragosh/zenmd/internal/core/extplugins"
	"github.com/dragosh/zenmd/internal/core/plugins"
	"github.com/dragosh/zenmd/internal/core/render"
	"github.com/dragosh/zenmd/internal/core/workspace"
)

const maxReadBytes = 1 << 20 // zen.read: largest file

// Files zen.read opens: markdown and plain text.
var readable = map[string]bool{".md": true, ".markdown": true, ".txt": true, ".text": true, ".csv": true,
	".tsv": true, ".json": true, ".yml": true, ".yaml": true}

// hostCall answers the callbacks of one call, inside zen. The permission
// checks, the path rules, the storage and the renderer are here and nowhere
// else: the helper is only asked, and what it asks is checked.
type hostCall struct {
	p    *Plugin
	ctx  plugins.Context
	rt   Runtime
	root string // resolved folder root, "" when there is none
	read bool   // the "read" permission
}

func oneString(args []json.RawMessage) (string, error) {
	if len(args) != 1 {
		return "", errors.New("bad callback arguments")
	}
	var s string
	if err := json.Unmarshal(args[0], &s); err != nil {
		return "", errors.New("bad callback arguments")
	}
	return s, nil
}

// handle runs the callback fn; the result is marshalled into the reply.
func (h *hostCall) handle(fn string, args []json.RawMessage) (out any, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, errors.New("internal error")
		}
	}()
	switch fn {
	case "markdown":
		src, err := oneString(args)
		if err != nil {
			return nil, err
		}
		if h.ctx.Markdown == nil {
			return nil, errors.New("zen.markdown is not available here")
		}
		return h.ctx.Markdown(src)
	case "storage.get", "storage.set", "storage.keys":
		st := h.rt.Storage
		if st == nil {
			return nil, errors.New("zen.storage is not available here")
		}
		switch fn {
		case "storage.get":
			key, err := oneString(args)
			if err != nil {
				return nil, err
			}
			raw, err := st.Get(key)
			if err != nil {
				return nil, err
			}
			if raw == "" || raw == "null" {
				return nil, nil
			}
			return json.RawMessage(raw), nil
		case "storage.set":
			if len(args) != 2 {
				return nil, errors.New("bad callback arguments")
			}
			var key string
			if err := json.Unmarshal(args[0], &key); err != nil {
				return nil, errors.New("bad callback arguments")
			}
			if err := st.Set(key, string(args[1])); err != nil {
				return nil, err
			}
			return nil, nil
		default:
			prefix, err := oneString(args)
			if err != nil {
				return nil, err
			}
			keys, err := st.Keys(prefix)
			if err != nil {
				return nil, err
			}
			if keys == nil {
				keys = []string{}
			}
			return keys, nil
		}
	case "read.check":
		return nil, h.needRead("zen.read")
	case "read":
		if err := h.needRead("zen.read"); err != nil {
			return nil, err
		}
		rel, err := oneString(args)
		if err != nil {
			return nil, err
		}
		text, exists, err := readText(h.root, rel, true)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, nil
		}
		return text, nil
	case "pages":
		if err := h.needRead("zen.pages"); err != nil {
			return nil, err
		}
		ws, err := workspace.Open(h.root)
		if err != nil {
			return nil, errors.New("zen.pages: the folder cannot be read")
		}
		files, err := ws.Files()
		if err != nil {
			return nil, errors.New("zen.pages: the folder has too many files")
		}
		type page struct {
			Path  string `json:"path"`
			Title string `json:"title"`
		}
		out := make([]page, len(files))
		for i, f := range files {
			out[i] = page{f, h.p.titleOf(h.root, f)}
		}
		return out, nil
	}
	return nil, fmt.Errorf("unknown callback %q", fn)
}

func (h *hostCall) needRead(what string) error {
	if !h.read {
		return fmt.Errorf(`%s needs the "read" permission (plugin := {permissions: ["read"], …})`, what)
	}
	if h.root == "" {
		return errors.New(what + ": no folder is open")
	}
	return nil
}

// readText reads the text file rel of the folder root: a markdown or text
// file (the extension is checked when checkExt is set), a regular file
// reached without leaving the folder (symlinks are resolved first), at most
// 1 MB of valid UTF-8. exists is false for a file that is not there.
func readText(root, rel string, checkExt bool) (text string, exists bool, err error) {
	if rel == "" {
		return "", false, errors.New("zen.read needs a file path inside the folder")
	}
	if strings.ContainsRune(rel, 0) || strings.ContainsAny(rel, "\\") {
		return "", false, errors.New("the path is not valid")
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", false, fmt.Errorf("%q is outside the folder", rel)
	}
	if checkExt && !readable[strings.ToLower(filepath.Ext(clean))] {
		return "", false, errors.New("only markdown and text files can be read (.md, .txt, .csv, .json, .yml)")
	}
	full, err := filepath.EvalSymlinks(filepath.Join(root, clean))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("%q cannot be read", rel)
	}
	if !extplugins.InsideRel(root, full) {
		return "", false, fmt.Errorf("%q is outside the folder", rel)
	}
	if checkExt && !readable[strings.ToLower(filepath.Ext(full))] {
		return "", false, errors.New("only markdown and text files can be read (.md, .txt, .csv, .json, .yml)")
	}
	fi, err := os.Stat(full)
	if err != nil || !fi.Mode().IsRegular() {
		return "", false, fmt.Errorf("%q is not a file", rel)
	}
	data, err := extplugins.ReadRegular(full, maxReadBytes)
	if err != nil {
		if err.Error() == "too large" {
			return "", false, fmt.Errorf("%q is larger than 1 MB", rel)
		}
		return "", false, fmt.Errorf("%q cannot be read", rel)
	}
	if !utf8.Valid(data) || strings.ContainsRune(string(data), 0) {
		return "", false, fmt.Errorf("%q is not UTF-8 text", rel)
	}
	return string(data), true, nil
}

// titles caches page titles by path, validated by size and modification time.
type titleEntry struct {
	size  int64
	mtime int64
	title string
}

var titler = render.New(nil, nil, nil) // no plugins: the title is the frontmatter's, the first heading or the file name

// titleOf is the title of the page rel in the folder (the file name without
// extension when the page cannot be read).
func (p *Plugin) titleOf(root, rel string) string {
	stem := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	if root == "" || rel == "" {
		return stem
	}
	full, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil || !extplugins.InsideRel(root, full) {
		return stem
	}
	fi, err := os.Stat(full)
	if err != nil || !fi.Mode().IsRegular() {
		return stem
	}
	if v, ok := p.titles.Load(rel); ok {
		if e := v.(titleEntry); e.size == fi.Size() && e.mtime == fi.ModTime().UnixNano() {
			return e.title
		}
	}
	text, exists, err := readText(root, rel, false)
	if err != nil || !exists {
		return stem
	}
	title := titler.Title(rel, []byte(text))
	if p.titleCount.Add(1) > 4096 { // a bounded cache
		p.titles.Range(func(k, _ any) bool { p.titles.Delete(k); return true })
		p.titleCount.Store(0)
	}
	p.titles.Store(rel, titleEntry{size: fi.Size(), mtime: fi.ModTime().UnixNano(), title: title})
	return title
}
