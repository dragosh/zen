// Command resources verifies the helper binaries listed in resources.yml and
// copies those for one architecture into an app bundle's Contents/Helpers.
//
//	go run ./build/darwin/resources -manifest resources.yml -arch arm64 -dest bin/Zen.app/Contents/Helpers
//
// Every entry must name an existing file whose sha256 matches; otherwise the
// command fails before copying anything. An empty list is valid.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type entry struct {
	Name   string            `yaml:"name"`
	Path   map[string]string `yaml:"path"`
	SHA256 string            `yaml:"sha256"`
}

var (
	nameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	shaRE  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	archs  = map[string]bool{"darwin-arm64": true, "darwin-amd64": true}
)

func main() {
	manifest := flag.String("manifest", "resources.yml", "manifest file")
	arch := flag.String("arch", "arm64", "target architecture (arm64 or amd64)")
	dest := flag.String("dest", "", "destination directory (Contents/Helpers)")
	flag.Parse()
	if err := run(*manifest, "darwin-"+*arch, *dest); err != nil {
		fmt.Fprintf(os.Stderr, "resources: %v\n", err)
		os.Exit(1)
	}
}

type file struct{ name, src string }

func run(manifest, target, dest string) error {
	if !archs[target] {
		return fmt.Errorf("unsupported target %s", target)
	}
	if dest == "" {
		return errors.New("-dest is required")
	}
	files, err := verify(manifest, target)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	for _, f := range files {
		if err := copyFile(f.src, filepath.Join(dest, f.name)); err != nil {
			return err
		}
		fmt.Printf("helper %s <- %s\n", f.name, f.src)
	}
	return nil
}

// verify parses the manifest and checks every entry for target.
func verify(manifest, target string) ([]file, error) {
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
	var files []file
	for i, e := range entries {
		where := fmt.Sprintf("%s entry %d (%s)", manifest, i+1, e.Name)
		if !nameRE.MatchString(e.Name) {
			return nil, fmt.Errorf("%s: invalid name", where)
		}
		if seen[e.Name] {
			return nil, fmt.Errorf("%s: duplicate name", where)
		}
		seen[e.Name] = true
		for k := range e.Path {
			if !archs[k] {
				return nil, fmt.Errorf("%s: unknown path key %s", where, k)
			}
		}
		if !shaRE.MatchString(strings.ToLower(e.SHA256)) {
			return nil, fmt.Errorf("%s: sha256 must be 64 hex digits", where)
		}
		src := e.Path[target]
		if src == "" {
			return nil, fmt.Errorf("%s: no path for %s", where, target)
		}
		if !filepath.IsAbs(src) {
			src = filepath.Join(base, src)
		}
		got, err := sum(src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
		if got != strings.ToLower(e.SHA256) {
			return nil, fmt.Errorf("%s: sha256 mismatch for %s: got %s", where, src, got)
		}
		files = append(files, file{e.Name, src})
	}
	return files, nil
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

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
