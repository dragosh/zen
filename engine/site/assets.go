package site

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/zenmark-org/zenmd/engine/config"
)

// assetRef is a plugin or layout file named by a page's asset URL.
type assetRef struct {
	kind string // "plugins" | "layouts"
	id   string
	file string
	url  string // canonical app URL, without ?v=
}

// module is the key of the script of a plugin or layout: plugin:<id>, layout:<id>.
func (a assetRef) module() string {
	if a.kind == "layouts" {
		return "layout:" + a.id
	}
	return "plugin:" + a.id
}

// outFile is where the file goes, relative to the site root.
func (a assetRef) outFile() string { return zenDir + "/" + a.kind + "/" + a.id + "/" + a.file }

// parseAssetURL parses /_plugins/<id>/<file>?v=… and /_layouts/<id>/<file>?v=….
func parseAssetURL(u string) (assetRef, bool) {
	u, _, _ = strings.Cut(u, "#")
	u, _, _ = strings.Cut(u, "?")
	var kind, rest string
	switch {
	case strings.HasPrefix(u, "/_plugins/"):
		kind, rest = "plugins", strings.TrimPrefix(u, "/_plugins/")
	case strings.HasPrefix(u, "/_layouts/"):
		kind, rest = "layouts", strings.TrimPrefix(u, "/_layouts/")
	default:
		return assetRef{}, false
	}
	id, file, ok := strings.Cut(rest, "/")
	if !ok || id == "" || id == "." || id == ".." || strings.ContainsAny(id, "\\\x00") || !fs.ValidPath(file) || file == "." ||
		strings.ContainsAny(file, "\\\x00") {
		return assetRef{}, false
	}
	return assetRef{kind: kind, id: id, file: file, url: "/_" + kind + "/" + id + "/" + file}, true
}

// copyAsset copies a plugin or layout file to the output (once).
func (s *site) copyAsset(a assetRef) error {
	if err, done := s.assets[a.url]; done {
		return err
	}
	err := func() error {
		data, err := s.readAsset(a)
		if err != nil {
			return err
		}
		return s.writeFile(a.outFile(), data)
	}()
	s.assets[a.url] = err
	return err
}

// copyExtras copies every file the app serves for the plugin or layout of a,
// once per plugin or layout; failures are warnings for page.
// In portable mode the scripts pages list are inlined, so they are not copied.
func (s *site) copyExtras(page string, a assetRef) {
	l, ok := s.in.(assetLister)
	key := a.kind + "/" + a.id
	if !ok || s.extras[key] {
		return
	}
	s.extras[key] = true
	for _, f := range l.AssetFiles(a.kind, a.id) {
		ref, ok := parseAssetURL("/_" + a.kind + "/" + a.id + "/" + f)
		if !ok || (s.o.Portable && s.scripts[ref.url]) {
			continue
		}
		if err := s.copyAsset(ref); err != nil {
			s.warn(page, ref.url+": "+err.Error()+"; left out")
		}
	}
}

// readAsset reads the file the way the app serves it; only listed files exist.
func (s *site) readAsset(a assetRef) ([]byte, error) {
	f, err := s.in.Asset(a.url)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, errors.New("not found")
		}
		return nil, err
	}
	return readAll(f)
}

// copyWorkspace copies a file of the folder (an image) to the same path in
// the output, once, and returns that path.
func (s *site) copyWorkspace(p string) (string, error) {
	if err, done := s.wsFiles[p]; done {
		return p, err
	}
	err := func() error {
		low := strings.ToLower(p)
		base := path.Base(low) // the folder's settings file, under either name, is not content
		if low == zenDir || strings.HasPrefix(low, zenDir+"/") || strings.HasSuffix(low, ".html") || path.Base(low) == markerName || base == config.WorkspaceFile || base == config.LegacyWorkspaceFile {
			return errors.New("a name zen keeps for itself")
		}
		if s.outRel != "" && (p == s.outRel || strings.HasPrefix(p, s.outRel+"/")) {
			return errors.New("inside the output folder")
		}
		f, err := s.in.WorkspaceFile(p)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return errors.New("not found")
			}
			return fmt.Errorf("cannot be read (%v)", shortErr(err))
		}
		data, err := readAll(f)
		if err != nil {
			return err
		}
		return s.writeFile(p, data)
	}()
	s.wsFiles[p] = err
	if err != nil {
		return "", err
	}
	return p, nil
}

func shortErr(err error) string {
	msg := err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 {
		msg = msg[i+2:]
	}
	return msg
}
