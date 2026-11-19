// Command resources verifies the helper binaries listed in resources.yml and
// copies those for one architecture into an app bundle's Contents/Helpers.
//
//	go run ./build/darwin/resources -manifest resources.yml -arch arm64 [-addons vale] -dest bin/Zenmark.app/Contents/Helpers [-licenses bin/Zenmark.app/Contents/Resources/licenses]
//	go run ./build/darwin/resources -manifest resources.yml -arch arm64 [-addons vale] -fetch
//	go run ./build/darwin/resources -manifest resources.yml -arch arm64 -validate
//
// An entry may name its add-on (`addon: vale`, specs/kit/addons). It is
// verified, fetched and bundled (with its licence) only when that add-on is in
// the -addons list (the ADDONS of the build); entries without `addon:` always
// are. -validate checks every entry.
//
// With -arch universal every entry is verified for both architectures
// (arm64 and amd64; each needs a path and sha256) and the two binaries are
// merged with lipo into one universal helper. -fetch and -validate then cover
// both architectures.
//
// Every entry must name an existing file whose sha256 matches; otherwise the
// command fails before copying anything. An empty list is valid.
//
// With -fetch, entries that have a fetch block for the architecture are
// downloaded (https:// or file://), the tarball's sha256 is checked, the
// member is extracted to the entry's path and its sha256 checked too. A
// helper that is already there and verified is not downloaded again.
//
// With -validate, only the manifest itself is checked (no files needed).
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// shaSpec is either one sha256 for every architecture or a per-arch map.
type shaSpec struct {
	all    string
	byArch map[string]string
}

func (s *shaSpec) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		return n.Decode(&s.all)
	case yaml.MappingNode:
		return n.Decode(&s.byArch)
	}
	return fmt.Errorf("line %d: sha256 must be a string or a map of architecture to string", n.Line)
}

func (s shaSpec) forArch(target string) string {
	if s.byArch != nil {
		return s.byArch[target]
	}
	return s.all
}

// fetchSpec says where a helper comes from: a tarball and the member in it.
type fetchSpec struct {
	URL     string `yaml:"url"`
	SHA256  string `yaml:"sha256"`
	Member  string `yaml:"member"`
	License string `yaml:"license"` // optional member copied to Resources/licenses/<name>.txt
}

type entry struct {
	Name   string               `yaml:"name"`
	Path   map[string]string    `yaml:"path"`
	SHA256 shaSpec              `yaml:"sha256"`
	Fetch  map[string]fetchSpec `yaml:"fetch"`
	Addon  string               `yaml:"addon"` // optional: bundled only when this add-on is in the build
}

var (
	nameRE  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	addonRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`) // kit/addons
	shaRE   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	archs   = map[string]bool{"darwin-arm64": true, "darwin-amd64": true}
)

// Download limits.
const (
	maxTarball = 256 << 20
	maxMember  = 256 << 20
)

func main() {
	manifest := flag.String("manifest", "resources.yml", "manifest file")
	arch := flag.String("arch", "arm64", "target architecture (arm64, amd64 or universal)")
	dest := flag.String("dest", "", "destination directory (Contents/Helpers)")
	licenses := flag.String("licenses", "", "destination directory for license notices (Contents/Resources/licenses)")
	fetch := flag.Bool("fetch", false, "download and verify helpers that have a fetch block")
	validate := flag.Bool("validate", false, "only validate the manifest")
	addonList := flag.String("addons", "", "comma-separated add-ons of the build; entries of other add-ons are skipped")
	flag.Parse()
	addons, err := parseAddons(*addonList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resources: %v\n", err)
		os.Exit(1)
	}
	targets := []string{"darwin-" + *arch}
	if *arch == "universal" {
		targets = []string{"darwin-arm64", "darwin-amd64"}
	}
	switch {
	case *validate:
		for _, t := range targets {
			if _, err = loadAll(*manifest, t); err != nil {
				break
			}
		}
	case *fetch:
		for _, t := range targets {
			if err = fetchAll(*manifest, t, addons); err != nil {
				break
			}
		}
	case len(targets) == 2:
		err = runUniversal(*manifest, targets, *dest, *licenses, addons)
	default:
		err = run(*manifest, targets[0], *dest, *licenses, addons)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "resources: %v\n", err)
		os.Exit(1)
	}
}

// parseAddons reads the -addons list: comma-separated valid add-on ids.
func parseAddons(list string) ([]string, error) {
	var out []string
	for _, id := range strings.Split(list, ",") {
		if id = strings.TrimSpace(id); id == "" {
			continue
		}
		if !addonRE.MatchString(id) {
			return nil, fmt.Errorf("invalid add-on id %q", id)
		}
		out = append(out, id)
	}
	return out, nil
}

type file struct{ name, src, license string }

func run(manifest, target, dest, licenses string, addons []string) error {
	if dest == "" {
		return errors.New("-dest is required")
	}
	files, err := verify(manifest, target, addons)
	if err != nil {
		return err
	}
	return install(files, dest, licenses)
}

// install copies verified helpers and their license notices.
func install(files []file, dest, licenses string) error {
	for _, f := range files {
		if f.license == "" {
			continue
		}
		if licenses == "" {
			return fmt.Errorf("%s has a license notice; -licenses is required", f.name)
		}
		if fi, err := os.Stat(f.license); err != nil || !fi.Mode().IsRegular() {
			return fmt.Errorf("%s: license notice %s missing (run `task darwin:fetch:helpers`)", f.name, f.license)
		}
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	for _, f := range files {
		if err := copyFile(f.src, filepath.Join(dest, f.name), 0o755); err != nil {
			return err
		}
		fmt.Printf("helper %s <- %s\n", f.name, f.src)
		if f.license != "" {
			if err := os.MkdirAll(licenses, 0o755); err != nil {
				return err
			}
			if err := copyFile(f.license, filepath.Join(licenses, f.name+".txt"), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// runUniversal verifies every helper for both targets, then merges the two
// binaries of each with lipo into dest. Nothing is written when any check fails.
func runUniversal(manifest string, targets []string, dest, licenses string, addons []string) error {
	if dest == "" {
		return errors.New("-dest is required")
	}
	var sets [][]file
	for _, t := range targets {
		files, err := verify(manifest, t, addons)
		if err != nil {
			return err
		}
		sets = append(sets, files)
	}
	// verify keeps the manifest order for every target.
	stage, err := os.MkdirTemp("", "zen-universal")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	for i, f := range sets[0] {
		args := []string{"-create", "-output", filepath.Join(stage, f.name)}
		for _, set := range sets {
			args = append(args, set[i].src)
		}
		if out, err := exec.Command("lipo", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("lipo %s: %v\n%s", f.name, err, out)
		}
	}
	merged := make([]file, len(sets[0]))
	for i, f := range sets[0] {
		merged[i] = file{name: f.name, src: filepath.Join(stage, f.name), license: f.license}
	}
	return install(merged, dest, licenses)
}

// item is one manifest entry resolved for the target architecture.
type item struct {
	where, name, src, sha string
	fetch                 *fetchSpec
}

// load parses and validates the manifest for target without touching helper
// files, and returns the entries of this build: those without an add-on and
// those of an add-on in addons.
func load(manifest, target string, addons []string) ([]item, error) {
	return loadFor(manifest, target, func(a string) bool { return a == "" || slices.Contains(addons, a) })
}

// loadAll is load for every entry, whatever its add-on (-validate).
func loadAll(manifest, target string) ([]item, error) {
	return loadFor(manifest, target, func(string) bool { return true })
}

func loadFor(manifest, target string, include func(addon string) bool) ([]item, error) {
	if !archs[target] {
		return nil, fmt.Errorf("unsupported target %s", target)
	}
	b, err := os.ReadFile(manifest)
	if err != nil {
		return nil, err
	}
	var entries []entry
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&entries); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: %w", manifest, err)
	}
	base := filepath.Dir(manifest)
	seen := map[string]bool{}
	var items []item
	for i, e := range entries {
		where := fmt.Sprintf("%s entry %d (%s)", manifest, i+1, e.Name)
		if !nameRE.MatchString(e.Name) {
			return nil, fmt.Errorf("%s: invalid name", where)
		}
		if seen[e.Name] {
			return nil, fmt.Errorf("%s: duplicate name", where)
		}
		seen[e.Name] = true
		if e.Addon != "" && !addonRE.MatchString(e.Addon) {
			return nil, fmt.Errorf("%s: invalid add-on id %q", where, e.Addon)
		}
		for k := range e.Path {
			if !archs[k] {
				return nil, fmt.Errorf("%s: unknown path key %s", where, k)
			}
		}
		for k := range e.SHA256.byArch {
			if !archs[k] {
				return nil, fmt.Errorf("%s: unknown sha256 key %s", where, k)
			}
		}
		for k, f := range e.Fetch {
			if !archs[k] {
				return nil, fmt.Errorf("%s: unknown fetch key %s", where, k)
			}
			if err := checkFetch(f); err != nil {
				return nil, fmt.Errorf("%s: fetch %s: %w", where, k, err)
			}
		}
		sha := strings.ToLower(e.SHA256.forArch(target))
		if !shaRE.MatchString(sha) {
			return nil, fmt.Errorf("%s: sha256 for %s must be 64 hex digits", where, target)
		}
		src := e.Path[target]
		if src == "" {
			return nil, fmt.Errorf("%s: no path for %s", where, target)
		}
		if !filepath.IsAbs(src) {
			src = filepath.Join(base, src)
		}
		if !include(e.Addon) {
			continue
		}
		it := item{where: where, name: e.Name, src: src, sha: sha}
		if f, ok := e.Fetch[target]; ok {
			it.fetch = &f
		}
		items = append(items, it)
	}
	return items, nil
}

func checkFetch(f fetchSpec) error {
	u, err := url.Parse(f.URL)
	if err != nil {
		return err
	}
	if u.Scheme != "https" && u.Scheme != "file" {
		return fmt.Errorf("url %q must be https:// or file://", f.URL)
	}
	if !shaRE.MatchString(strings.ToLower(f.SHA256)) {
		return errors.New("sha256 must be 64 hex digits")
	}
	if err := checkMember(f.Member); err != nil {
		return err
	}
	if f.License != "" {
		return checkMember(f.License)
	}
	return nil
}

// checkMember accepts a clean relative path inside the tarball.
func checkMember(m string) error {
	if m == "" || path.IsAbs(m) || path.Clean(m) != m || m == ".." || strings.HasPrefix(m, "../") {
		return fmt.Errorf("invalid member %q", m)
	}
	return nil
}

// licensePath is where fetch stores the license notice of a helper.
func licensePath(src string) string { return src + ".LICENSE" }

// verify parses the manifest and checks every entry of the build for target.
func verify(manifest, target string, addons []string) ([]file, error) {
	items, err := load(manifest, target, addons)
	if err != nil {
		return nil, err
	}
	var files []file
	for _, it := range items {
		got, err := sum(it.src)
		if err != nil {
			hint := ""
			if it.fetch != nil {
				hint = " (run `task darwin:fetch:helpers`)"
			}
			return nil, fmt.Errorf("%s: %w%s", it.where, err, hint)
		}
		if got != it.sha {
			return nil, fmt.Errorf("%s: sha256 mismatch for %s: got %s", it.where, it.src, got)
		}
		f := file{name: it.name, src: it.src}
		if it.fetch != nil && it.fetch.License != "" {
			f.license = licensePath(it.src)
		}
		files = append(files, f)
	}
	return files, nil
}

// fetchAll downloads every helper with a fetch block for target.
func fetchAll(manifest, target string, addons []string) error {
	items, err := load(manifest, target, addons)
	if err != nil {
		return err
	}
	for _, it := range items {
		if it.fetch == nil {
			if got, err := sum(it.src); err != nil || got != it.sha {
				return fmt.Errorf("%s: no fetch block for %s and no verified file at %s", it.where, target, it.src)
			}
			continue
		}
		if err := fetchOne(it); err != nil {
			return fmt.Errorf("%s: %w", it.where, err)
		}
	}
	return nil
}

func fetchOne(it item) error {
	f := it.fetch
	if got, err := sum(it.src); err == nil && got == it.sha {
		fi, err := os.Stat(licensePath(it.src))
		if f.License == "" || (err == nil && fi.Mode().IsRegular()) {
			fmt.Printf("helper %s verified, skipping download\n", it.name)
			return nil
		}
	}
	fmt.Printf("helper %s <- %s\n", it.name, f.URL)
	tgz, err := download(f.URL)
	if err != nil {
		return err
	}
	if got := hex.EncodeToString(sha256Of(tgz)); got != strings.ToLower(f.SHA256) {
		return fmt.Errorf("tarball sha256 mismatch for %s: got %s", f.URL, got)
	}
	want := []string{f.Member}
	if f.License != "" {
		want = append(want, f.License)
	}
	members, err := extract(tgz, want)
	if err != nil {
		return err
	}
	bin := members[f.Member]
	if got := hex.EncodeToString(sha256Of(bin)); got != it.sha {
		return fmt.Errorf("binary %s sha256 mismatch: got %s", f.Member, got)
	}
	if err := os.MkdirAll(filepath.Dir(it.src), 0o755); err != nil {
		return err
	}
	if f.License != "" {
		if err := writeAtomic(licensePath(it.src), members[f.License], 0o644); err != nil {
			return err
		}
	}
	return writeAtomic(it.src, bin, 0o755)
}

func download(raw string) ([]byte, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	var r io.ReadCloser
	switch u.Scheme {
	case "file":
		f, err := os.Open(u.Path)
		if err != nil {
			return nil, err
		}
		r = f
	case "https":
		c := &http.Client{Timeout: 5 * time.Minute}
		resp, err := c.Get(raw)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("GET %s: %s", raw, resp.Status)
		}
		r = resp.Body
	default:
		return nil, fmt.Errorf("unsupported url %s", raw)
	}
	defer r.Close()
	b, err := io.ReadAll(io.LimitReader(r, maxTarball+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxTarball {
		return nil, fmt.Errorf("%s is larger than %d bytes", raw, maxTarball)
	}
	return b, nil
}

// extract returns the named regular-file members of a .tar.gz.
func extract(tgz []byte, names []string) (map[string][]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(tgz))
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	out := map[string][]byte{}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		name := strings.TrimPrefix(h.Name, "./")
		if !want[name] || h.Typeflag != tar.TypeReg {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(tr, maxMember+1))
		if err != nil {
			return nil, err
		}
		if len(b) > maxMember {
			return nil, fmt.Errorf("member %s too large", name)
		}
		out[name] = b
	}
	for _, n := range names {
		if _, ok := out[n]; !ok {
			return nil, fmt.Errorf("member %s not found in tarball", n)
		}
	}
	return out, nil
}

// writeAtomic writes b to dst through a temporary file in the same directory.
func writeAtomic(dst string, b []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

func sha256Of(b []byte) []byte {
	s := sha256.Sum256(b)
	return s[:]
}

func sum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", path)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
