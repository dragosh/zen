// Package darwin_test inspects the macOS bundle produced by `task package`
// (bin/Zen.app) and checks the helper-binary manifest (resources.yml).
//
// The bundle tests are skipped with -short, off macOS, or when bin/Zen.app
// does not exist, unless ZEN_REQUIRE_BUNDLE=1 (set by `task darwin:package:test`).
package darwin_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

const (
	bundle   = "../../bin/Zen.app"
	bundleID = "io.github.dragosh.zen"
)

func requireBundle(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("bundle tests skipped with -short")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("bundle tests run on macOS only")
	}
	if _, err := os.Stat(bundle); err != nil {
		if os.Getenv("ZEN_REQUIRE_BUNDLE") == "1" {
			t.Fatalf("bundle missing: %v (run `task package`)", err)
		}
		t.Skip("bin/Zen.app not built; run `task package` (or `task darwin:package:test`)")
	}
}

func readVersion(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

func plist(t *testing.T, key string) string {
	t.Helper()
	out, err := exec.Command("plutil", "-extract", key, "raw", "-o", "-",
		filepath.Join(bundle, "Contents", "Info.plist")).Output()
	if err != nil {
		t.Fatalf("Info.plist key %s: %v", key, err)
	}
	return strings.TrimSpace(string(out))
}

func TestBundleLayout(t *testing.T) {
	requireBundle(t)

	for _, p := range []string{"Contents/MacOS/zen", "Contents/Info.plist", "Contents/Resources/icons.icns"} {
		fi, err := os.Stat(filepath.Join(bundle, p))
		if err != nil || fi.IsDir() {
			t.Errorf("missing file %s", p)
		}
	}
	if fi, err := os.Stat(filepath.Join(bundle, "Contents/MacOS/zen")); err == nil && fi.Mode()&0o111 == 0 {
		t.Error("Contents/MacOS/zen is not executable")
	}
	if fi, err := os.Stat(filepath.Join(bundle, "Contents/Helpers")); err != nil || !fi.IsDir() {
		t.Error("missing directory Contents/Helpers")
	}

	v := readVersion(t)
	short := regexp.MustCompile(`^\d+\.\d+\.\d+`).FindString(v)
	want := map[string]string{
		"CFBundleName":               "Zen",
		"CFBundleDisplayName":        "Zen",
		"CFBundleExecutable":         "zen",
		"CFBundleIdentifier":         bundleID,
		"CFBundlePackageType":        "APPL",
		"CFBundleIconFile":           "icons",
		"CFBundleShortVersionString": short,
		"LSMinimumSystemVersion":     "12.0",
	}
	for k, w := range want {
		if got := plist(t, k); got != w {
			t.Errorf("%s = %q, want %q", k, got, w)
		}
	}
	if b := plist(t, "CFBundleVersion"); !regexp.MustCompile(`^[1-9]\d*$`).MatchString(b) {
		t.Errorf("CFBundleVersion = %q, want the commit count", b)
	}

	// Document types: markdown files and folders.
	raw, err := exec.Command("plutil", "-extract", "CFBundleDocumentTypes", "json", "-o", "-",
		filepath.Join(bundle, "Contents", "Info.plist")).Output()
	if err != nil {
		t.Fatalf("CFBundleDocumentTypes: %v", err)
	}
	for _, s := range []string{"net.daringfireball.markdown", "public.folder", `"md"`, `"markdown"`} {
		if !strings.Contains(string(raw), s) {
			t.Errorf("CFBundleDocumentTypes lacks %s: %s", s, raw)
		}
	}
	if strings.Contains(string(raw), "{{") {
		t.Error("Info.plist still has template placeholders")
	}
}

func TestBundledCLI(t *testing.T) {
	requireBundle(t)
	out, err := exec.Command(filepath.Join(bundle, "Contents/MacOS/zen"), "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("zen --version: %v\n%s", err, out)
	}
	if got, want := strings.TrimSpace(string(out)), "zen "+readVersion(t); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// wantEntitlements is the complete set of entitlements: sandbox plus access
// to user-selected files and app-scoped bookmarks, nothing broader.
var wantEntitlements = []string{
	"com.apple.security.app-sandbox",
	"com.apple.security.files.bookmarks.app-scope",
	"com.apple.security.files.user-selected.read-write",
}

func entitlementKeys(t *testing.T, xml string) []string {
	t.Helper()
	var keys []string
	for _, m := range regexp.MustCompile(`<key>([^<]+)</key>\s*<true\s*/>`).FindAllStringSubmatch(xml, -1) {
		keys = append(keys, m[1])
	}
	if n := strings.Count(xml, "<key>"); n != len(keys) {
		t.Errorf("entitlements must all be boolean true; found %d keys, %d true: %s", n, len(keys), xml)
	}
	return keys
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]bool{}
	for _, s := range a {
		m[s] = true
	}
	for _, s := range b {
		if !m[s] {
			return false
		}
	}
	return true
}

func TestEntitlementsFile(t *testing.T) {
	b, err := os.ReadFile("entitlements.plist")
	if err != nil {
		t.Fatal(err)
	}
	if got := entitlementKeys(t, string(b)); !sameSet(got, wantEntitlements) {
		t.Errorf("entitlements.plist keys = %v, want %v", got, wantEntitlements)
	}
}

func TestSignature(t *testing.T) {
	requireBundle(t)
	if out, err := exec.Command("codesign", "--verify", "--deep", "--strict", bundle).CombinedOutput(); err != nil {
		t.Fatalf("codesign --verify: %v\n%s", err, out)
	}
	out, err := exec.Command("codesign", "-d", "--entitlements", "-", "--xml", bundle).Output()
	if err != nil {
		t.Fatalf("codesign -d --entitlements: %v", err)
	}
	if got := entitlementKeys(t, string(out)); !sameSet(got, wantEntitlements) {
		t.Errorf("signed entitlements = %v, want %v", got, wantEntitlements)
	}
	info, _ := exec.Command("codesign", "-dv", bundle).CombinedOutput()
	if !strings.Contains(string(info), "Identifier="+bundleID) {
		t.Errorf("signature identifier: %s", info)
	}
}

// resources runs the manifest tool: verify every entry for arch and copy it to dest.
func resources(t *testing.T, manifest, arch, dest string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command("go", "run", "./resources", "-manifest", manifest, "-arch", arch, "-dest", dest)
	return cmd.CombinedOutput()
}

func TestResourcesManifest(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go run; skipped with -short")
	}
	dir := t.TempDir()
	helper := filepath.Join(dir, "helper-arm64")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("#!/bin/sh\necho hi\n"))
	good := hex.EncodeToString(sum[:])

	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	entry := func(path, sha string) string {
		return "- name: helper\n  path:\n    darwin-arm64: " + path + "\n    darwin-amd64: " + path + "\n  sha256: " + sha + "\n"
	}

	t.Run("repository manifest is valid", func(t *testing.T) {
		if out, err := resources(t, "../../resources.yml", "arm64", t.TempDir()); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
	})
	t.Run("empty list", func(t *testing.T) {
		dest := t.TempDir()
		if out, err := resources(t, write("empty.yml", "[]\n"), "arm64", dest); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
	})
	t.Run("valid entry is copied", func(t *testing.T) {
		dest := t.TempDir()
		if out, err := resources(t, write("ok.yml", entry(helper, good)), "arm64", dest); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		fi, err := os.Stat(filepath.Join(dest, "helper"))
		if err != nil || fi.Mode()&0o111 == 0 {
			t.Fatalf("helper not copied as executable: %v", err)
		}
	})
	t.Run("relative path resolves against manifest", func(t *testing.T) {
		if out, err := resources(t, write("rel.yml", entry("helper-arm64", good)), "arm64", t.TempDir()); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
	})
	fails := map[string]string{
		"missing file":   entry(filepath.Join(dir, "nope"), good),
		"checksum":       entry(helper, strings.Repeat("0", 64)),
		"no sha256":      "- name: helper\n  path:\n    darwin-arm64: " + helper + "\n",
		"no arch path":   "- name: helper\n  path:\n    darwin-amd64: " + helper + "\n  sha256: " + good + "\n",
		"bad name":       strings.Replace(entry(helper, good), "name: helper", "name: ../evil", 1),
		"unknown field":  entry(helper, good) + "  extra: 1\n",
		"duplicate name": entry(helper, good) + entry(helper, good),
	}
	for name, body := range fails {
		t.Run("rejects "+name, func(t *testing.T) {
			dest := t.TempDir()
			out, err := resources(t, write(strings.ReplaceAll(name, " ", "_")+".yml", body), "arm64", dest)
			if err == nil {
				t.Fatalf("accepted bad manifest:\n%s", out)
			}
			if _, err := os.Stat(filepath.Join(dest, "helper")); err == nil {
				t.Error("helper copied despite failure")
			}
		})
	}
}
