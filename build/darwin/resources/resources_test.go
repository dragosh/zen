package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// manifest writes a resources.yml with a plain helper and one that belongs to
// the add-on "voice", and the files they need. The add-on's helper has a fetch
// block that cannot succeed (its tarball is missing), so a fetch that touches
// it fails loudly.
func addonManifest(t *testing.T) (manifest, dir string) {
	t.Helper()
	dir = t.TempDir()
	body := map[string]string{"always": "#!/bin/sh\necho always\n", "voicebox": "#!/bin/sh\necho voice\n"}
	var yml strings.Builder
	for _, name := range []string{"always", "voicebox"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body[name]), 0o755); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte(body[name]))
		yml.WriteString("- name: " + name + "\n")
		if name == "voicebox" {
			yml.WriteString("  addon: voice\n")
		}
		yml.WriteString("  path:\n    darwin-arm64: " + name + "\n    darwin-amd64: " + name + "\n")
		yml.WriteString("  sha256: " + hex.EncodeToString(sum[:]) + "\n")
		if name == "voicebox" {
			yml.WriteString("  fetch:\n    darwin-arm64:\n      url: file://" + filepath.Join(dir, "missing.tar.gz") + "\n      sha256: " + strings.Repeat("0", 64) + "\n      member: voicebox\n      license: LICENSE\n")
			// The licence of voicebox exists, as after a fetch.
			if err := os.WriteFile(filepath.Join(dir, "voicebox.LICENSE"), []byte("MIT"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	manifest = filepath.Join(dir, "resources.yml")
	if err := os.WriteFile(manifest, []byte(yml.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return manifest, dir
}

func has(path string) bool { _, err := os.Stat(path); return err == nil }

func TestAddonHelpers(t *testing.T) {
	manifest, _ := addonManifest(t)

	// Without the add-on: only the plain helper, no licence of the other.
	dest, lic := filepath.Join(t.TempDir(), "Helpers"), filepath.Join(t.TempDir(), "licenses")
	if err := run(manifest, "darwin-arm64", dest, lic, nil); err != nil {
		t.Fatal(err)
	}
	if !has(filepath.Join(dest, "always")) || has(filepath.Join(dest, "voicebox")) || has(filepath.Join(lic, "voicebox.txt")) {
		t.Fatalf("without add-ons: helpers %v", names(t, dest))
	}
	// An add-on that is not this helper's does not include it.
	dest2 := filepath.Join(t.TempDir(), "Helpers")
	if err := run(manifest, "darwin-arm64", dest2, lic, []string{"vale"}); err != nil || has(filepath.Join(dest2, "voicebox")) {
		t.Fatalf("other add-on: %v %v", err, names(t, dest2))
	}

	// With it: both, and the licence goes with the helper.
	dest3, lic3 := filepath.Join(t.TempDir(), "Helpers"), filepath.Join(t.TempDir(), "licenses")
	if err := run(manifest, "darwin-arm64", dest3, lic3, []string{"voice"}); err != nil {
		t.Fatal(err)
	}
	if !has(filepath.Join(dest3, "always")) || !has(filepath.Join(dest3, "voicebox")) || !has(filepath.Join(lic3, "voicebox.txt")) {
		t.Fatalf("with the add-on: helpers %v", names(t, dest3))
	}
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	es, _ := os.ReadDir(dir)
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

// Helpers of add-ons that are not in the build are not fetched; the ones that are, are.
func TestAddonHelpersNotFetched(t *testing.T) {
	manifest, _ := addonManifest(t)
	if err := fetchAll(manifest, "darwin-arm64", nil); err != nil {
		t.Fatalf("fetch without add-ons touched the add-on's helper: %v", err)
	}
	if err := fetchAll(manifest, "darwin-arm64", []string{"voice"}); err == nil {
		t.Fatal("fetch with the add-on skipped its helper")
	}
}

func TestAddonField(t *testing.T) {
	dir := t.TempDir()
	write := func(addon string) string {
		p := filepath.Join(dir, "m.yml")
		y := "- name: h\n  addon: " + addon + "\n  path: {darwin-arm64: h}\n  sha256: " + strings.Repeat("a", 64) + "\n"
		if err := os.WriteFile(p, []byte(y), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if _, err := load(write("../x"), "darwin-arm64", nil); err == nil {
		t.Error("an add-on id with a path in it was accepted")
	}
	if _, err := load(write("Vale"), "darwin-arm64", nil); err == nil {
		t.Error("an invalid add-on id was accepted")
	}
	if items, err := load(write("vale"), "darwin-arm64", nil); err != nil || len(items) != 0 {
		t.Errorf("excluded entry: %v %v", items, err)
	}
	if items, err := load(write("vale"), "darwin-arm64", []string{"vale"}); err != nil || len(items) != 1 {
		t.Errorf("included entry: %v %v", items, err)
	}
}

// The repository's manifest names the add-on of Vale.
func TestRepoManifestValeIsAnAddon(t *testing.T) {
	items, err := load("../../../resources.yml", "darwin-arm64", nil)
	if err != nil || len(items) != 0 {
		t.Fatalf("default build bundles %v (%v)", items, err)
	}
	items, err = load("../../../resources.yml", "darwin-arm64", []string{"vale"})
	if err != nil || len(items) != 1 || items[0].name != "vale" {
		t.Fatalf("ADDONS=vale bundles %v (%v)", items, err)
	}
}
