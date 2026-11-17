package app

// File › Build Website… (⇧⌘B) and Settings › Website: the services behind the
// dialog (ui/static-build). They run the same code as `zen build` (buildSite)
// on the open folder; this file adds the options, progress, cancel, the
// "save as folder defaults" edit of .zen.yml, and opening the result.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dragosh/zen/internal/cli"
	"github.com/dragosh/zen/internal/core/config"
	"github.com/dragosh/zen/internal/core/site"
	"gopkg.in/yaml.v3"
)

// EventBuildProgress is emitted after each page the build renders; the payload
// is a BuildProgress.
const EventBuildProgress = "build:progress"

// EventBuildWebsite asks the main window to show the Build Website dialog
// (File > Build Website…, Shift+Cmd+B). No payload.
const EventBuildWebsite = "menu:build-website"

// BuildOptions are what the dialog shows and BuildWebsite uses.
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

// revealPath shows a folder in Finder; tests replace it.
var revealPath = func(p string) error { return exec.Command("open", p).Run() }

// runningBuild is the build in progress of one Zen. At most one runs at a time.
type runningBuild struct{ cancelled atomic.Bool }

var builds = struct {
	sync.Mutex
	m map[*Zen]*runningBuild
}{m: map[*Zen]*runningBuild{}}

// buildFile is <config dir>/data/<folderKey>/build.json: the options used
// last time for a folder, and the output of its last build.
type buildFile struct {
	BuildOptions
	Last string `json:"last,omitempty"`
}

// buildFolder is the open folder.
func (z *Zen) buildFolder() (root string, err error) {
	z.state.mu.Lock()
	defer z.state.mu.Unlock()
	if z.state.ws == nil {
		return "", errNoWorkspace
	}
	return z.state.ws.Root(), nil
}

func (z *Zen) buildFilePath(root string) string {
	if z.state.configDir == "" {
		return ""
	}
	return filepath.Join(z.state.configDir, "data", folderKey(root), "build.json")
}

func (z *Zen) readBuildFile(root string) (f buildFile, ok bool) {
	p := z.buildFilePath(root)
	if p == "" {
		return f, false
	}
	b, err := os.ReadFile(p)
	if err != nil || json.Unmarshal(b, &f) != nil {
		return buildFile{}, false
	}
	return f, true
}

func (z *Zen) writeBuildFile(root string, f buildFile) {
	p := z.buildFilePath(root)
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
// build: section of the config, then what was used last time.
func (z *Zen) BuildOptions() (BuildOptions, error) {
	root, err := z.buildFolder()
	if err != nil {
		return BuildOptions{}, err
	}
	bc, _, err := config.Load(z.state.globalPath, root).Build()
	if err != nil {
		return BuildOptions{}, err
	}
	o := BuildOptions{Out: absOut(root, bc.Out), Portable: true, Drafts: bc.Drafts, Print: true}
	if bc.Portable != nil {
		o.Portable = *bc.Portable
	}
	if f, ok := z.readBuildFile(root); ok {
		o = f.BuildOptions
		o.Out = absOut(root, o.Out)
	}
	return o, nil
}

// absOut makes the output folder absolute: "" is _site, relative is inside root.
func absOut(root, out string) string {
	if out == "" {
		out = "_site"
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
	z     *Zen
	root  string
	run   *runningBuild
	mu    sync.Mutex
	pages []string
	done  int
}

func (p *progressInput) Pages() ([]string, error) {
	names, err := p.Input.Pages()
	p.mu.Lock()
	p.pages = append([]string(nil), names...)
	p.mu.Unlock()
	return names, err
}

func (p *progressInput) Render(path string) (site.Page, error) {
	if cur, err := p.z.buildFolder(); err != nil || cur != p.root {
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
	p.z.state.emit(EventBuildProgress, prog)
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

// BuildWebsite builds the open folder like `zen build` and returns the result.
// Errors are the CLI's. After CancelBuild (or a switch to another folder) the
// pages not yet rendered are left out, the rest of the site is written as for
// any build (so the marker is there and the folder can be built again), and
// the result has Cancelled set; the "cancelled" warnings of the left-out
// pages are not listed. One build runs at a time.
func (z *Zen) BuildWebsite(o BuildOptions) (BuildResult, error) {
	root, err := z.buildFolder()
	if err != nil {
		return BuildResult{}, err
	}
	run := &runningBuild{}
	builds.Lock()
	if _, busy := builds.m[z]; busy {
		builds.Unlock()
		return BuildResult{}, errBuildRunning
	}
	builds.m[z] = run
	builds.Unlock()
	defer func() {
		builds.Lock()
		delete(builds.m, z)
		builds.Unlock()
	}()

	prev, _ := z.readBuildFile(root)
	if o.Out != "" {
		o.Out = absOut(root, o.Out)
	}
	z.writeBuildFile(root, buildFile{BuildOptions: o, Last: prev.Last})

	var pi *progressInput
	env := buildEnv{GlobalPath: z.state.globalPath, Assets: StaticAssets, Wrap: func(in site.Input) site.Input {
		pi = &progressInput{Input: in, z: z, root: root, run: run}
		return pi
	}}
	var stdout, stderr bytes.Buffer
	start := time.Now()
	err = buildSite(env, cli.Build{Dir: root, Out: o.Out, Portable: o.Portable, Hosted: !o.Portable, Drafts: o.Drafts, NoDrafts: !o.Drafts, Clean: o.Clean, NoPrint: !o.Print}, &stdout, &stderr)
	if err != nil {
		return BuildResult{}, err
	}

	res := BuildResult{Out: o.Out, Millis: time.Since(start).Milliseconds(), Cancelled: run.cancelled.Load(), Warnings: []BuildWarning{}}
	if res.Out == "" {
		bc, _, _ := config.Load(z.state.globalPath, root).Build()
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
	z.writeBuildFile(root, buildFile{BuildOptions: o, Last: res.Out})
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
func (z *Zen) CancelBuild() {
	builds.Lock()
	run := builds.m[z]
	builds.Unlock()
	if run != nil {
		run.cancelled.Store(true)
	}
}

// SaveBuildDefaults writes out, portable and drafts into the build: section of
// the folder's .zen.yml (clean and print are per run). Without confirm it
// only returns the preview text; with it, it edits the YAML tree, so comments
// and other keys stay, and writes the file atomically. It returns the same
// text either way.
func (z *Zen) SaveBuildDefaults(o BuildOptions, confirm bool) (string, error) {
	root, err := z.buildFolder()
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, ".zen.yml")
	var doc yaml.Node
	data, rerr := os.ReadFile(path)
	if rerr != nil && !os.IsNotExist(rerr) {
		return "", rerr
	}
	if rerr == nil {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return "", fmt.Errorf(".zen.yml is not valid YAML: %v", err)
		}
	}
	fresh := doc.Kind == 0
	if fresh {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	top := doc.Content[0]
	if doc.Kind != yaml.DocumentNode || top.Kind != yaml.MappingNode {
		return "", errors.New(".zen.yml is not a YAML mapping; fix it first")
	}
	if fresh {
		setKey(top, "version", scalar("1", "!!int"))
	}
	build := mapValue(top, "build")
	switch {
	case build == nil || build.Kind == yaml.ScalarNode && build.Tag == "!!null":
		build = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		setKey(top, "build", build)
	case build.Kind != yaml.MappingNode:
		return "", errors.New("build in .zen.yml must be a mapping with keys such as out and portable")
	}

	out := o.Out
	if out == "" {
		out = "_site"
	}
	if abs := absOut(root, out); !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		out = abs
	} else {
		out, _ = filepath.Rel(root, abs)
		out = filepath.ToSlash(out)
	}
	type change struct{ key, was, now string }
	var changes []change
	want := []struct{ key, val, tag string }{
		{"out", out, "!!str"},
		{"portable", strconv.FormatBool(o.Portable), "!!bool"},
		{"drafts", strconv.FormatBool(o.Drafts), "!!bool"},
	}
	added := true
	for _, w := range want {
		cur := mapValue(build, w.key)
		if cur != nil {
			same := cur.Value == w.val || w.key == "out" && filepath.ToSlash(filepath.Clean(cur.Value)) == w.val
			if same {
				continue
			}
			added = false
			changes = append(changes, change{w.key, cur.Value, w.val})
			cur.Value, cur.Tag, cur.Style = w.val, w.tag, 0
			continue
		}
		changes = append(changes, change{w.key, "", w.val})
		setKey(build, w.key, scalar(w.val, w.tag))
	}
	if len(changes) == 0 {
		return "Nothing to change: .zen.yml already has these build defaults.", nil
	}
	var sb strings.Builder
	if added {
		sb.WriteString("Adds to .zen.yml:")
	} else {
		sb.WriteString("Changes .zen.yml:")
	}
	for _, c := range changes {
		sb.WriteString("\n  build." + c.key + ": " + c.now)
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
	if err := writeAtomic(path, buf.Bytes(), perm); err != nil {
		return "", err
	}
	return preview, nil
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

// PickBuildFolder shows the folder panel (New Folder allowed) and returns the
// chosen folder, "" on cancel. It does not open the folder; the build still
// checks it like any other output.
func (z *Zen) PickBuildFolder() (string, error) {
	if d, ok := z.dialogs.(*wailsDialogs); ok && d.app != nil && d.win != nil {
		dir, err := d.app.Dialog.OpenFile().
			SetTitle("Choose Output Folder").
			CanChooseDirectories(true).
			CanChooseFiles(false).
			CanCreateDirectories(true).
			AttachToWindow(d.win).
			PromptForSingleSelection()
		return dir, err
	}
	if z.dialogs == nil {
		return "", errors.New("no dialogs available")
	}
	return z.dialogs.PickFolder()
}

// builtSite checks that out is the output of the last build of the open
// folder, has its marker and an index.html (a regular file), and returns the
// index.html path.
func (z *Zen) builtSite(out string) (string, error) {
	root, err := z.buildFolder()
	if err != nil {
		return "", err
	}
	f, ok := z.readBuildFile(root)
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

// OpenBuiltSite opens the index.html of the last build of the open folder in
// the default browser. It opens nothing else.
func (z *Zen) OpenBuiltSite(out string) error {
	index, err := z.builtSite(out)
	if err != nil {
		return err
	}
	if z.open == nil {
		return errors.New("cannot open the browser")
	}
	return z.open(fileURL(index))
}

// RevealBuiltSite shows the output folder of the last build in Finder.
func (z *Zen) RevealBuiltSite(out string) error {
	if _, err := z.builtSite(out); err != nil {
		return err
	}
	return revealPath(out)
}

func fileURL(p string) string { return (&url.URL{Scheme: "file", Path: filepath.ToSlash(p)}).String() }
