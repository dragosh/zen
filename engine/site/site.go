// Package site turns the pages of a folder into a static website: one HTML
// page per markdown page inside a static shell (sidebar, breadcrumb), with the
// files the pages use copied next to it. It is pure: pages and files come
// through Input, and it does not import apps/zenmark.
package site

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zenmark-org/zenmd/engine/layouts"
)

// Page is one rendered page, as the app shows it.
type Page struct {
	Path, Title, HTML string
	Breadcrumb        []Crumb
	Layout            string
	CSS, JS           []string // app URLs: /_plugins/<id>/<f>?v=, /_layouts/<id>/<f>?v=
	Draft             bool
	Warnings          []string
	// Lang is the page's frontmatter lang; "" means en.
	Lang string
}

// Crumb is one breadcrumb entry.
type Crumb struct{ Title, Path string }

// NavNode is a sidebar node: Kind is "page" or "group".
type NavNode struct {
	Title, Path, Kind string
	Children          []NavNode
}

// Input is where a build gets its pages and files.
type Input interface {
	Pages() ([]string, error)                   // sorted workspace-relative paths
	Home() string                               // the home page, "" for none
	Tree() []NavNode                            // the sidebar
	Render(path string) (Page, error)           // as the app renders it
	Slug(slug string) (string, bool)            // frontmatter slug -> page
	WorkspaceFile(path string) (fs.File, error) // inside the root only, no symlinks out
	Asset(url string) (fs.File, error)          // /_plugins/… and /_layouts/… as the app serves them
	Runtime() (js, css []byte)                  // zen.js and zen.css (package runtime)
}

// rooter is implemented by an Input that knows the folder it reads; a build
// then keeps its output folder out of the input.
type rooter interface{ Root() string }

// assetLister is implemented by an Input that can list every file the app
// serves for a plugin or layout (kind "plugins" | "layouts"). A build copies
// all of them for each plugin or layout a page uses, so scripts can load
// files they do not list (the diagram plugin's mermaid bundle) through
// ctx.assetUrl.
type assetLister interface {
	AssetFiles(kind, id string) []string
}

// Options are what a build does.
type Options struct {
	Out              string
	SiteID, EditURL  string
	Drafts, Portable bool
	Clean            bool
	Exclude          []string
	// NoPrint leaves print.html out (zenmd build --no-print).
	NoPrint bool
}

// Report is what a build produced.
type Report struct {
	Pages, Files int
	Warnings     []string // "path: message", sorted
	// Print is the output path of the single printable document ("print.html"),
	// "" when none was written. It is not counted in Files.
	Print string
}

const (
	zenDir     = "_zen"
	markerName = ".zen-build"
	maxFile    = 64 << 20 // bytes read for one copied file
)

var errUnsafe = errors.New("path outside the folder")

// outPath is the output file of a page: a/b.md becomes a/b.html.
func outPath(p string) string {
	if e := path.Ext(p); strings.EqualFold(e, ".md") || strings.EqualFold(e, ".markdown") {
		return p[:len(p)-len(e)] + ".html"
	}
	return p + ".html"
}

// escapePath escapes each segment of a relative URL path.
func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

// relFile is the relative path from the file `from` to the file `to`
// (both slash-separated, relative to the site root).
func relFile(from, to string) string {
	fromDir := path.Dir(from)
	if fromDir == "." {
		fromDir = ""
	}
	a := splitPath(fromDir)
	b := splitPath(to)
	i := 0
	for i < len(a) && i < len(b)-1 && a[i] == b[i] {
		i++
	}
	var out []string
	for range a[i:] {
		out = append(out, "..")
	}
	out = append(out, b[i:]...)
	return strings.Join(out, "/")
}

// rootPrefix is the relative path from the file's folder to the site root:
// "" or "../" or "../../".
func rootPrefix(file string) string {
	return strings.Repeat("../", len(splitPath(path.Dir(file))))
}

func splitPath(p string) []string {
	if p == "" || p == "." {
		return nil
	}
	return strings.Split(p, "/")
}

// cleanRel cleans a workspace-relative path from a URL: no "..", no absolute
// path, no backslashes, no empty or NUL segments.
func cleanRel(p string) (string, bool) {
	if p == "" || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\\x00") {
		return "", false
	}
	for _, s := range strings.Split(p, "/") {
		if s == "" || s == "." || s == ".." {
			return "", false
		}
	}
	return p, true
}

func readAll(f fs.File) ([]byte, error) {
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxFile+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxFile {
		return nil, errors.New("file too large")
	}
	return b, nil
}

// site is the state of one build.
type site struct {
	in   Input
	o    Options
	w    *writer
	root string // the input folder ("" if unknown)
	home string // the home page, if it is written

	outRel   string // the output folder inside the input, "" if outside
	hasIndex bool   // index.html was written with a page

	pages    map[string]*built // by workspace path, the pages that are written
	order    []string          // sorted paths of pages
	warns    map[string]bool
	files    map[string]bool // output files written (not pages)
	wsFiles  map[string]error
	assetURL map[string]assetRef
	assets   map[string]error
	extras   map[string]bool // kind/id whose other files were copied
	scripts  map[string]bool // every page script URL (canonical), known before pages are written
}

type built struct {
	page Page
	out  string
}

func (s *site) warn(p, msg string) {
	if p != "" {
		msg = p + ": " + msg
	} else {
		msg = "site: " + msg
	}
	s.warns[msg] = true
}

// Build writes the site for in into o.Out.
func Build(in Input, o Options) (Report, error) {
	if o.Out == "" {
		return Report{}, errors.New("no output folder")
	}
	out, err := filepath.Abs(o.Out)
	if err != nil {
		return Report{}, err
	}
	o.Out = out
	if o.EditURL != "" {
		if err := checkEditURL(o.EditURL); err != nil {
			return Report{}, err
		}
	}
	s := &site{in: in, o: o, pages: map[string]*built{}, warns: map[string]bool{}, files: map[string]bool{},
		wsFiles: map[string]error{}, assetURL: map[string]assetRef{}, assets: map[string]error{}, extras: map[string]bool{}}
	if r, ok := in.(rooter); ok {
		s.root = r.Root()
	}
	if err := CheckOut(s.root, o.Out); err != nil {
		return Report{}, err
	}
	if s.root != "" {
		if rel, ok := insideRel(s.root, o.Out); ok {
			s.outRel = rel
		}
	}

	names, err := in.Pages()
	if err != nil {
		return Report{}, err
	}
	sort.Strings(names)

	// Render everything that may be built.
	var rendered []Page
	skipped := 0
	for _, p := range names {
		if s.outRel != "" && (p == s.outRel || strings.HasPrefix(p, s.outRel+"/")) {
			skipped++
			continue
		}
		if excluded(o.Exclude, p) {
			continue
		}
		pg, err := in.Render(p)
		if err != nil {
			s.warn(p, err.Error())
			continue
		}
		pg.Path = p
		for _, w := range pg.Warnings {
			s.warn(p, w)
		}
		if pg.Draft && !o.Drafts {
			continue
		}
		rendered = append(rendered, pg)
	}

	s.scripts = map[string]bool{}
	for _, pg := range rendered {
		for _, u := range pg.JS {
			if a, ok := parseAssetURL(u); ok {
				s.scripts[a.url] = true
			}
		}
	}

	if skipped > 0 {
		s.warn("", fmt.Sprintf("the output folder %s is inside the folder; its %d pages are left out of the input", s.outRel, skipped))
	}
	home := in.Home()
	s.w = newWriter(o.Out)
	if err := s.w.prepare(o.Clean); err != nil {
		return Report{}, err
	}

	// Output names: the first in sort order wins a (case-insensitive) clash.
	taken := map[string]string{}
	for _, pg := range rendered {
		if pg.Path == home && !strings.Contains(outPath(home), "/") {
			taken["index.html"] = home // the home is also index.html at the root
		}
	}
	for _, pg := range rendered {
		o := outPath(pg.Path)
		low := strings.ToLower(o)
		if low == zenDir || strings.HasPrefix(low, zenDir+"/") {
			s.warn(pg.Path, "the folder _zen is reserved for zen's own files; page left out")
			continue
		}
		if first, clash := taken[low]; clash && first != pg.Path {
			s.warn(pg.Path, fmt.Sprintf("output name %s collides with %s; page left out", o, first))
			continue
		}
		taken[low] = pg.Path
		pgc := pg
		s.pages[pg.Path] = &built{page: pgc, out: o}
		s.order = append(s.order, pg.Path)
	}
	sort.Strings(s.order)

	if b, ok := s.pages[home]; ok {
		s.home = b.page.Path
	}
	for _, p := range s.order {
		if err := s.writePage(s.pages[p]); err != nil {
			return Report{}, err
		}
	}
	if err := s.writeIndex(home); err != nil {
		return Report{}, err
	}
	js, css := in.Runtime()
	for name, data := range map[string][]byte{zenDir + "/zen.js": js, zenDir + "/zen.css": css} {
		if err := s.writeFile(name, data); err != nil {
			return Report{}, err
		}
	}
	printed, err := s.writePrint()
	if err != nil {
		return Report{}, err
	}
	rep := Report{Pages: len(s.order), Files: len(s.files), Print: printed}
	for w := range s.warns {
		rep.Warnings = append(rep.Warnings, w)
	}
	sort.Strings(rep.Warnings)
	return rep, nil
}

// writeFile writes a non-page output file and counts it.
func (s *site) writeFile(rel string, data []byte) error {
	if err := s.w.write(rel, data); err != nil {
		return err
	}
	s.files[rel] = true
	return nil
}

func excluded(globs []string, p string) bool {
	for _, g := range globs {
		if layouts.Match(g, p) {
			return true
		}
	}
	return false
}

func checkEditURL(u string) error {
	pu, err := url.Parse(strings.ReplaceAll(u, "{path}", "x"))
	if err != nil || (pu.Scheme != "http" && pu.Scheme != "https") || pu.Host == "" {
		return errors.New("build.editURL must be an http or https address, with {path} for the page")
	}
	return nil
}
