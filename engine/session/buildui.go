package session

// File › Build Website… (⇧⌘B) and Settings › Website: the services behind the
// dialog (ui/static-build). They run the same code as `zenmd build` (WriteSite)
// on the open folder; this file adds the options, progress, cancel and the
// "save as folder defaults" edit of .zenmd.yml.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zenmark-org/zenmd/engine/config"
	"github.com/zenmark-org/zenmd/engine/site"
	"gopkg.in/yaml.v3"
)

// EventBuildProgress is emitted after each page the build renders; the payload
// is a BuildProgress.
const EventBuildProgress = "build:progress"

// BuildOptions are what the dialog shows and Build uses.
type BuildOptions struct {
	Out      string `json:"out"` // absolute; the dialog shows it relative when inside the folder
	Portable bool   `json:"portable"`
	Drafts   bool   `json:"drafts"`
	Clean    bool   `json:"clean"`
	Print    bool   `json:"print"` // also write print.html (when ui/print is merged)
}

// BuildResult is what a build did. Cancelled means the build stopped after a
// page: Pages counts the pages written, and the output is usable.
type BuildResult struct {
	Out       string         `json:"out"`
	Pages     int            `json:"pages"`
	Files     int            `json:"files"`
	Warnings  []BuildWarning `json:"warnings"`
	Millis    int64          `json:"millis"`
	Cancelled bool           `json:"cancelled"`
}

// BuildWarning is one warning; Path is "" for a site-wide one.
type BuildWarning struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// BuildProgress is the payload of build:progress.
type BuildProgress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

var (
	errBuildRunning   = errors.New("a build is already running")
	errBuildCancelled = errors.New("build cancelled")
)

// runningBuild is the build in progress of one Session. At most one runs at a
// time.
type runningBuild struct {
	cancelled atomic.Bool
	ctx       context.Context
}

// stopped reports whether the build was cancelled, by CancelBuild, by its
// context or by a switch to another folder.
func (r *runningBuild) stopped() bool { return r.cancelled.Load() || r.ctx.Err() != nil }

// buildFile is <config dir>/data/<folderKey>/build.json: the options used
// last time for a folder, and the output of its last build.
type buildFile struct {
	BuildOptions
	Last string `json:"last,omitempty"`
}

// buildFolder is the open folder.
func (s *Session) buildFolder() (root string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ws == nil {
		return "", errNoWorkspace
	}
	return s.ws.Root(), nil
}

func (s *Session) buildFilePath(root string) string {
	if s.configDir == "" {
		return ""
	}
	return filepath.Join(s.configDir, "data", folderKey(root), "build.json")
}

func (s *Session) readBuildFile(root string) (f buildFile, ok bool) {
	p := s.buildFilePath(root)
	if p == "" {
		return f, false
	}
	b, err := os.ReadFile(p)
	if err != nil || json.Unmarshal(b, &f) != nil {
		return buildFile{}, false
	}
	return f, true
}

func (s *Session) writeBuildFile(root string, f buildFile) {
	p := s.buildFilePath(root)
	if p == "" {
		return
	}
	if b, err := json.MarshalIndent(f, "", "  "); err == nil {
		_ = writeAtomic(p, append(b, '\n'), 0o644) // remembering is a convenience: a failure is not an error
	}
}

// writeAtomic writes data next to path and renames it over path.
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Chmod(perm)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		_ = os.Remove(name)
	}
	return err
}

// BuildOptions returns the form's starting values for the open folder: the
// export: section of the config, then what was used last time.
func (s *Session) BuildOptions() (r0 BuildOptions, err error) {
	defer s.logFail("BuildOptions", &err)
	root, err := s.buildFolder()
	if err != nil {
		return BuildOptions{}, err
	}
	bc, _, err := config.Load(s.globalPath, root).Build()
	if err != nil {
		return BuildOptions{}, err
	}
	o := BuildOptions{Out: absOut(root, bc.Out), Portable: true, Drafts: bc.Drafts, Print: true}
	if bc.Portable != nil {
		o.Portable = *bc.Portable
	}
	if f, ok := s.readBuildFile(root); ok {
		o = f.BuildOptions
		o.Out = absOut(root, o.Out)
	}
	return o, nil
}

// absOut makes the output folder absolute: "" is exports/html, relative is inside root.
func absOut(root, out string) string {
	if out == "" {
		out = config.DefaultExportOut
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(root, out)
	}
	return filepath.Clean(out)
}

// progressInput decorates the build's input: each Render reports progress,
// and after CancelBuild (or a switch to another folder) it fails, so the
// build goes on without rendering more pages and writes what it has.
type progressInput struct {
	site.Input
	s        *Session
	root     string
	run      *runningBuild
	progress func(done, total int)
	mu       sync.Mutex
	pages    []string
	done     int
}

func (p *progressInput) Pages() ([]string, error) {
	names, err := p.Input.Pages()
	p.mu.Lock()
	p.pages = append([]string(nil), names...)
	p.mu.Unlock()
	return names, err
}

func (p *progressInput) Render(path string) (site.Page, error) {
	if cur, err := p.s.buildFolder(); err != nil || cur != p.root {
		p.run.cancelled.Store(true) // the folder was switched
	}
	if p.run.cancelled.Load() {
		return site.Page{}, errBuildCancelled
	}
	pg, err := p.Input.Render(path)
	p.mu.Lock()
	p.done++
	prog := BuildProgress{Done: p.done, Total: len(p.pages)}
	p.mu.Unlock()
	p.s.emit(EventBuildProgress, prog)
	return pg, err
}

// Root and AssetFiles keep the optional parts of the input the build looks for.
func (p *progressInput) Root() string {
	if r, ok := p.Input.(interface{ Root() string }); ok {
		return r.Root()
	}
	return ""
}

func (p *progressInput) AssetFiles(kind, id string) []string {
	if a, ok := p.Input.(interface {
		AssetFiles(kind, id string) []string
	}); ok {
		return a.AssetFiles(kind, id)
	}
	return nil
}

var summaryRE = regexp.MustCompile(`^Built (\d+) pages and (\d+) files into `)

// BuildWebsite is Build for the app: cancelled by CancelBuild, and it reports
// progress as build:progress events.
func (s *Session) BuildWebsite(o BuildOptions) (r0 BuildResult, err error) {
	defer s.logFail("BuildWebsite", &err)
	root, _ := s.buildFolder()
	logApp(slog.LevelInfo, "export started", "out", relPath(root, absOut(root, o.Out)), "portable", o.Portable)
	res, err := s.Build(context.Background(), o, func(done, total int) {
		s.emit(EventBuildProgress, BuildProgress{Done: done, Total: total})
	})
	switch {
	case err != nil:
		// The service logs the error the UI gets.
	case res.Cancelled:
		logApp(slog.LevelInfo, "export cancelled", "pages", res.Pages)
	default:
		logApp(slog.LevelInfo, "export finished", "pages", res.Pages, "files", res.Files, "ms", res.Millis,
			"out", relPath(root, res.Out), "warnings", len(res.Warnings))
	}
	return res, err
}

// Build builds the open folder like `zenmd build` and returns the result.
// Errors are the CLI's. After CancelBuild, when ctx ends (or a switch to
// another folder) the pages not yet rendered are left out, the rest of the
// site is written as for any build (so the marker is there and the folder can
// be built again), and the result has Cancelled set; the "cancelled" warnings
// of the left-out pages are not listed. progress, when set, is called after
// each page. One build runs at a time.
func (s *Session) Build(ctx context.Context, o BuildOptions, progress func(done, total int)) (BuildResult, error) {
	root, err := s.buildFolder()
	if err != nil {
		return BuildResult{}, err
	}
	run := &runningBuild{ctx: ctx}
	s.buildMu.Lock()
	if s.building != nil {
		s.buildMu.Unlock()
		return BuildResult{}, errBuildRunning
	}
	s.building = run
	s.buildMu.Unlock()
	defer func() {
		s.buildMu.Lock()
		s.building = nil
		s.buildMu.Unlock()
	}()

	prev, _ := s.readBuildFile(root)
	if o.Out != "" {
		o.Out = absOut(root, o.Out)
	}
	s.writeBuildFile(root, buildFile{BuildOptions: o, Last: prev.Last})

	var pi *progressInput
	env := SiteEnv{GlobalPath: s.globalPath, Runtime: s.runtime, Registry: s.base, Layouts: s.lbase, Dialects: s.dbase, Version: s.version, Wrap: func(in site.Input) site.Input {
		pi = &progressInput{Input: in, s: s, root: root, run: run, progress: progress}
		return pi
	}}
	var stdout, stderr bytes.Buffer
	start := time.Now()
	err = WriteSite(env, SiteOptions{Dir: root, Out: o.Out, Portable: o.Portable, Hosted: !o.Portable, Drafts: o.Drafts, NoDrafts: !o.Drafts, Clean: o.Clean, NoPrint: !o.Print}, &stdout, &stderr)
	if err != nil {
		return BuildResult{}, err
	}

	res := BuildResult{Out: o.Out, Millis: time.Since(start).Milliseconds(), Cancelled: run.stopped(), Warnings: []BuildWarning{}}
	if res.Out == "" {
		bc, _, _ := config.Load(s.globalPath, root).Build()
		res.Out = absOut(root, bc.Out)
	}
	if m := summaryRE.FindStringSubmatch(stdout.String()); m != nil {
		res.Pages, _ = strconv.Atoi(m[1])
		res.Files, _ = strconv.Atoi(m[2])
	}
	var pages []string
	if pi != nil {
		pages = pi.pages
	}
	for _, line := range strings.Split(stderr.String(), "\n") {
		if line == "" {
			continue
		}
		w := splitWarning(line, pages)
		if w.Message == errBuildCancelled.Error() {
			continue
		}
		res.Warnings = append(res.Warnings, w)
	}
	s.writeBuildFile(root, buildFile{BuildOptions: o, Last: res.Out})
	return res, nil
}

// splitWarning turns a CLI warning line ("page.md: message" or "site: message")
// into a BuildWarning, matching the page against the folder's pages.
func splitWarning(line string, pages []string) BuildWarning {
	if m, ok := strings.CutPrefix(line, "site: "); ok {
		return BuildWarning{Message: m}
	}
	best := ""
	for _, p := range pages {
		if len(p) > len(best) && strings.HasPrefix(line, p+": ") {
			best = p
		}
	}
	if best == "" {
		return BuildWarning{Message: line}
	}
	return BuildWarning{Path: best, Message: strings.TrimPrefix(line, best+": ")}
}

// CancelBuild stops the running build after the current page; idle, it does nothing.
func (s *Session) CancelBuild() {
	s.buildMu.Lock()
	run := s.building
	s.buildMu.Unlock()
	if run != nil {
		run.cancelled.Store(true)
	}
}

// SaveBuildDefaults writes out, portable and drafts into the export: section of
// the folder's .zenmd.yml (clean and print are per run). Without confirm it
// only returns the preview text; with it, it edits the YAML tree, so comments
// and other keys stay, and writes the file atomically. It returns the same
// text either way.
func (s *Session) SaveBuildDefaults(o BuildOptions, confirm bool) (r0 string, err error) {
	defer s.logFail("SaveBuildDefaults", &err)
	return s.editExport(confirm, "Nothing to change: .zenmd.yml already has these export defaults.", func(root string, exp *yaml.Node) []exportChange {
		out := o.Out
		if out == "" {
			out = config.DefaultExportOut
		}
		if abs := absOut(root, out); !strings.HasPrefix(abs, root+string(filepath.Separator)) {
			out = abs
		} else {
			out, _ = filepath.Rel(root, abs)
			out = filepath.ToSlash(out)
		}
		return setExportKeys(exp, "export.", []exportKey{
			{"out", out, "!!str"},
			{"portable", strconv.FormatBool(o.Portable), "!!bool"},
			{"drafts", strconv.FormatBool(o.Drafts), "!!bool"},
		})
	})
}

// exportKey is a key of an export mapping to set, with its YAML tag.
type exportKey struct{ key, val, tag string }

// exportChange is one key setExportKeys changed or added.
type exportChange struct {
	key, was, now string
	existed       bool
}

// setExportKeys sets the keys in the mapping m and returns what changed; a key
// that already has the value (an out path compared cleaned) is left alone.
func setExportKeys(m *yaml.Node, prefix string, want []exportKey) []exportChange {
	var changes []exportChange
	for _, w := range want {
		cur := mapValue(m, w.key)
		if cur != nil {
			same := cur.Value == w.val || w.key == "out" && filepath.ToSlash(filepath.Clean(cur.Value)) == w.val
			if same {
				continue
			}
			changes = append(changes, exportChange{prefix + w.key, cur.Value, w.val, true})
			cur.Value, cur.Tag, cur.Style = w.val, w.tag, 0
			continue
		}
		changes = append(changes, exportChange{prefix + w.key, "", w.val, false})
		setKey(m, w.key, scalar(w.val, w.tag))
	}
	return changes
}

// editExport edits the export: mapping of the folder's .zenmd.yml (made when
// there is none) with edit, and returns the preview text of what changed, or
// nothing when there is none to change. With confirm it also writes the file
// atomically; the YAML tree keeps the comments and the other keys. A first
// write moves an old settings file to the new name.
func (s *Session) editExport(confirm bool, nothing string, edit func(root string, exp *yaml.Node) []exportChange) (string, error) {
	if err := s.writable(); err != nil {
		return "", err
	}
	root, err := s.buildFolder()
	if err != nil {
		return "", err
	}
	path, legacy, _ := config.WorkspaceConfigPath(root)
	var doc yaml.Node
	data, rerr := os.ReadFile(path)
	if rerr != nil && !os.IsNotExist(rerr) {
		return "", rerr
	}
	if rerr == nil {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return "", fmt.Errorf(".zenmd.yml is not valid YAML: %v", err)
		}
	}
	fresh := doc.Kind == 0
	if fresh {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	top := doc.Content[0]
	if doc.Kind != yaml.DocumentNode || top.Kind != yaml.MappingNode {
		return "", errors.New(".zenmd.yml is not a YAML mapping; fix it first")
	}
	if fresh {
		setKey(top, "version", scalar("1", "!!int"))
	}
	exp, renamed, err := exportSection(top)
	if err != nil {
		return "", err
	}
	changes := edit(root, exp)
	if len(changes) == 0 && !renamed {
		return nothing, nil
	}
	added := true
	for _, c := range changes {
		if c.existed {
			added = false
		}
	}
	var sb strings.Builder
	if added && !renamed {
		sb.WriteString("Adds to .zenmd.yml:")
	} else {
		sb.WriteString("Changes .zenmd.yml:")
	}
	if renamed {
		sb.WriteString("\n  build: becomes export: (its values move with it)")
	}
	for _, c := range changes {
		sb.WriteString("\n  " + c.key + ": " + c.now)
		if c.was != "" {
			sb.WriteString("  (was " + c.was + ")")
		}
	}
	preview := sb.String()
	if !confirm {
		return preview, nil
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	perm := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		perm = fi.Mode().Perm()
	}
	if legacy { // the first write moves the old file to the new name
		if err := config.MoveLegacy(root); err != nil {
			return "", err
		}
		path = filepath.Join(root, config.WorkspaceFile)
	}
	if err := writeAtomic(path, buf.Bytes(), perm); err != nil {
		return "", err
	}
	return preview, nil
}

// PDFOptions are what the PDF form shows and the export uses.
type PDFOptions struct {
	Out    string `json:"out"` // absolute from PDFDefaults; the form may send a path relative to the folder
	Drafts bool   `json:"drafts"`
	Cover  bool   `json:"cover"` // the cover sheet and the table of contents
	Open   bool   `json:"open"`  // open the file when done
}

// PDFRun is a PDF export or preview of the form: the options, and whether it
// is only the page that was open and the pages under it.
type PDFRun struct {
	PDFOptions
	Section bool `json:"section"`
}

// PDFDefaults returns the form's starting values for the open folder: the
// export.pdf section of the config files, over the defaults.
func (s *Session) PDFDefaults() (r0 PDFOptions, err error) {
	defer s.logFail("PDFDefaults", &err)
	root, err := s.buildFolder()
	if err != nil {
		return PDFOptions{}, err
	}
	p, err := config.Load(s.globalPath, root).PDF()
	if err != nil {
		return PDFOptions{}, err
	}
	out, err := config.ResolvePDFOut(root, p.Out)
	if err != nil {
		return PDFOptions{}, err
	}
	return PDFOptions{Out: out, Drafts: p.Drafts, Cover: p.CoverOn(), Open: p.OpenOn()}, nil
}

// SavePDFDefaults writes the options into export.pdf of the folder's
// .zenmd.yml, as SaveBuildDefaults does for the website: the rest of export:
// and the file's comments stay. Without confirm it only returns the preview.
func (s *Session) SavePDFDefaults(o PDFOptions, confirm bool) (r0 string, err error) {
	defer s.logFail("SavePDFDefaults", &err)
	if err := s.writable(); err != nil {
		return "", err
	}
	root, err := s.buildFolder()
	if err != nil {
		return "", err
	}
	if _, err := config.ResolvePDFOut(root, o.Out); err != nil {
		return "", err
	}
	return s.editExport(confirm, "Nothing to change: .zenmd.yml already has these PDF defaults.", func(root string, exp *yaml.Node) []exportChange {
		out := ""
		if abs, err := config.ResolvePDFOut(root, o.Out); err == nil {
			if rel, err := filepath.Rel(root, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				abs = rel
			}
			out = filepath.ToSlash(abs)
		}
		pdf := mapValue(exp, "pdf")
		if pdf == nil || pdf.Kind != yaml.MappingNode {
			pdf = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			setKey(exp, "pdf", pdf)
		}
		changes := setExportKeys(pdf, "export.pdf.", []exportKey{
			{"out", out, "!!str"},
			{"drafts", strconv.FormatBool(o.Drafts), "!!bool"},
			{"cover", strconv.FormatBool(o.Cover), "!!bool"},
			{"open", strconv.FormatBool(o.Open), "!!bool"},
		})
		if len(changes) == 0 && len(pdf.Content) == 0 { // nothing was needed: do not leave an empty pdf:
			deleteKey(exp, "pdf")
		}
		return changes
	})
}

// exportSection returns the export: mapping of the top-level mapping, creating
// it when there is none. An old build: mapping moves into it (renamed in place
// when there is no export:, else its keys are added to the ones export: lacks)
// and the build: key goes; renamed says it did. A build: or export: that is
// not a mapping is an error and nothing has changed.
func exportSection(top *yaml.Node) (exp *yaml.Node, renamed bool, err error) {
	isNull := func(n *yaml.Node) bool { return n == nil || n.Kind == yaml.ScalarNode && n.Tag == "!!null" }
	old, cur := mapValue(top, "build"), mapValue(top, "export")
	if !isNull(old) && old.Kind != yaml.MappingNode {
		return nil, false, errors.New("build in .zenmd.yml must be a mapping with keys such as out and portable")
	}
	if !isNull(cur) && cur.Kind != yaml.MappingNode {
		return nil, false, errors.New("export in .zenmd.yml must be a mapping with keys such as out and portable")
	}
	switch {
	case old == nil: // no old key: the plain case
		if isNull(cur) {
			cur = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			setKey(top, "export", cur)
		}
		return cur, false, nil
	case isNull(cur):
		if old.Kind != yaml.MappingNode {
			old = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		}
		if cur == nil && mapKey(top, "export") == nil { // rename in place: position and comments stay
			if k := mapKey(top, "build"); k != nil {
				k.Value = "export"
			}
			if mapValue(top, "export") != old {
				setKey(top, "export", old)
			}
			return old, true, nil
		}
		setKey(top, "export", old)
		deleteKey(top, "build")
		return old, true, nil
	}
	for i := 0; i+1 < len(old.Content); i += 2 {
		if mapValue(cur, old.Content[i].Value) == nil {
			cur.Content = append(cur.Content, old.Content[i], old.Content[i+1])
		}
	}
	deleteKey(top, "build")
	return cur, true, nil
}

// mapKey is the key node of key in a mapping node, or nil.
func mapKey(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i]
		}
	}
	return nil
}

// deleteKey removes key from a mapping node.
func deleteKey(m *yaml.Node, key string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}

func scalar(v, tag string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: v} }

// mapValue is the value node of key in a mapping node, or nil.
func mapValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// setKey sets key to v in a mapping node, replacing an existing value or appending.
func setKey(m *yaml.Node, key string, v *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = v
			return
		}
	}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, v)
}

// BuiltSite checks that out is the output of the last build of the open
// folder, has its marker and an index.html (a regular file), and returns the
// index.html path.
func (s *Session) BuiltSite(out string) (string, error) {
	root, err := s.buildFolder()
	if err != nil {
		return "", err
	}
	f, ok := s.readBuildFile(root)
	if !ok || f.Last == "" || out == "" || filepath.Clean(out) != filepath.Clean(f.Last) {
		return "", errors.New("that is not the output of the last build of this folder")
	}
	for _, name := range []string{".zen-build", "index.html"} {
		fi, err := os.Lstat(filepath.Join(out, name))
		if err != nil || !fi.Mode().IsRegular() {
			return "", fmt.Errorf("%s is not a built website (no %s)", out, name)
		}
	}
	return filepath.Join(out, "index.html"), nil
}
