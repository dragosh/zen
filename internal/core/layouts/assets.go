package layouts

import (
	"io/fs"
	"net/url"
	"os"
	"path"

	"github.com/dragosh/zenmd/internal/core/extplugins"
	"github.com/dragosh/zenmd/internal/core/plugins"
)

func readDir(dir string) ([]string, error) {
	es, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(es))
	for i, e := range es {
		names[i] = e.Name()
	}
	return names, nil
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// Assets serves the css and js files the manifest lists, nothing else (not
// layout.yml, not the template). For a folder layout every open checks that
// the folder is unchanged and that the file resolves inside it.
func (l *Layout) Assets() fs.FS {
	if len(l.files) == 0 {
		return nil
	}
	return assetFS{l: l}
}

type assetFS struct{ l *Layout }

func (a assetFS) Open(name string) (fs.File, error) {
	notExist := &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	if !fs.ValidPath(name) || !a.l.files[name] || a.l.Changed() {
		return nil, notExist
	}
	if a.l.fsys != nil {
		f, err := a.l.fsys.Open(name)
		if err != nil {
			return nil, notExist
		}
		return f, nil
	}
	full, err := extplugins.ResolveInside(a.l.dir, name, "file")
	if err != nil {
		return nil, notExist
	}
	f, err := os.Open(full)
	if err != nil {
		return nil, notExist
	}
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() {
		f.Close()
		return nil, notExist
	}
	return f, nil
}

// URLs returns the layout's asset URLs (/_layouts/<id>/<file>?v=<version>);
// version is the app version unless the layout has a hash of its own.
func (l *Layout) URLs(appVersion string) plugins.Assets {
	ver := l.AssetVersion()
	if ver == "" {
		ver = appVersion
	}
	mk := func(files []string) []string {
		var out []string
		for _, f := range files {
			u := "/_layouts/" + l.man.ID + "/" + path.Clean(f)
			if ver != "" {
				u += "?v=" + url.QueryEscape(ver)
			}
			out = append(out, u)
		}
		return out
	}
	return plugins.Assets{CSS: mk(l.man.CSS), JS: mk(l.man.JS)}
}
