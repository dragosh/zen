// Package darwin_test inspects the macOS bundle produced by `task package`
// (bin/Zenmark.app) and checks the helper-binary manifest (resources.yml).
//
// The bundle tests are skipped with -short, off macOS, or when bin/Zenmark.app
// does not exist, unless ZEN_REQUIRE_BUNDLE=1 (set by `task darwin:package:test`).
package darwin_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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
	bundle   = "../../bin/Zenmark.app"
	bundleID = "org.zenmark.app"
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
		t.Skip("bin/Zenmark.app not built; run `task package` (or `task darwin:package:test`)")
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

	for _, p := range []string{"Contents/MacOS/zenmd", "Contents/Info.plist", "Contents/Resources/icons.icns"} {
		fi, err := os.Stat(filepath.Join(bundle, p))
		if err != nil || fi.IsDir() {
			t.Errorf("missing file %s", p)
		}
	}
	if fi, err := os.Stat(filepath.Join(bundle, "Contents/MacOS/zenmd")); err == nil && fi.Mode()&0o111 == 0 {
		t.Error("Contents/MacOS/zenmd is not executable")
	}
	if fi, err := os.Stat(filepath.Join(bundle, "Contents/Helpers")); err != nil || !fi.IsDir() {
		t.Error("missing directory Contents/Helpers")
	}

	v := readVersion(t)
	short := regexp.MustCompile(`^\d+\.\d+\.\d+`).FindString(v)
	want := map[string]string{
		"CFBundleName":               "Zenmark",
		"CFBundleDisplayName":        "Zenmark",
		"CFBundleExecutable":         "zenmd",
		"CFBundleIdentifier":         bundleID,
		"CFBundlePackageType":        "APPL",
		"CFBundleIconFile":           "icons",
		"CFBundleShortVersionString": short,
		"LSMinimumSystemVersion":     "13.0",
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
	out, err := exec.Command(filepath.Join(bundle, "Contents/MacOS/zenmd"), "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("zenmd --version: %v\n%s", err, out)
	}
	if got, want := strings.TrimSpace(string(out)), "zenmd "+readVersion(t); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// wantEntitlements is the complete set of entitlements the alpha signs with:
// none. The app is not sandboxed so the zen command can open any folder.
var wantEntitlements = []string{}

// wantSandboxEntitlements is the App Store set kept for beta in
// entitlements.sandbox.plist: sandbox plus user-selected files and bookmarks.
var wantSandboxEntitlements = []string{
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
	b, err = os.ReadFile("entitlements.sandbox.plist")
	if err != nil {
		t.Fatal(err)
	}
	if got := entitlementKeys(t, string(b)); !sameSet(got, wantSandboxEntitlements) {
		t.Errorf("entitlements.sandbox.plist keys = %v, want %v", got, wantSandboxEntitlements)
	}
}

func TestSignatureDevID(t *testing.T) {
	requireBundle(t)
	if channel() != "devid" {
		t.Skip("the bundle was built for the App Store channel; see TestSignatureAppStore")
	}
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
	info, _ := exec.Command("codesign", "-dvv", bundle).CombinedOutput()
	if !strings.Contains(string(info), "Identifier="+bundleID) {
		t.Errorf("signature identifier: %s", info)
	}
	if strings.Contains(string(out), "app-sandbox") {
		t.Error("the Developer ID build must not be sandboxed")
	}
	if strings.Contains(string(info), "Authority=Developer ID Application") {
		// A real identity: hardened runtime and a secure timestamp.
		if !strings.Contains(string(info), "runtime") {
			t.Errorf("Developer ID signature lacks the hardened runtime:\n%s", info)
		}
		if !strings.Contains(string(info), "Timestamp=") {
			t.Errorf("Developer ID signature lacks a secure timestamp:\n%s", info)
		}
	} else {
		t.Log("ad-hoc signature: hardened runtime and timestamp are checked with a Developer ID identity only")
	}
	for _, h := range helperFiles(t) {
		if out, err := exec.Command("codesign", "--verify", "--strict", h).CombinedOutput(); err != nil {
			t.Errorf("helper %s: %v\n%s", h, err, out)
		}
	}
}

// resources runs the manifest tool: verify every entry for arch and copy it to dest.
func resources(t *testing.T, manifest, arch, dest string) ([]byte, error) {
	t.Helper()
	return tool(t, "-manifest", manifest, "-arch", arch, "-dest", dest)
}

// tool runs the manifest tool with args.
func tool(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run", "./resources"}, args...)...)
	return cmd.CombinedOutput()
}

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
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
		for _, arch := range []string{"arm64", "amd64"} {
			if out, err := tool(t, "-manifest", "../../resources.yml", "-arch", arch, "-validate"); err != nil {
				t.Fatalf("%s: %v\n%s", arch, err, out)
			}
		}
	})
	t.Run("repository manifest pins vale 3.25.1", func(t *testing.T) {
		b, err := os.ReadFile("../../resources.yml")
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range []string{
			"name: vale",
			"https://github.com/vale-cli/vale/releases/download/v3.25.1/vale_3.25.1_macOS_arm64.tar.gz",
			"https://github.com/vale-cli/vale/releases/download/v3.25.1/vale_3.25.1_macOS_64-bit.tar.gz",
			"fbcaa68d9b9e229b9c8dc918f209e93572ce943039700b7e04a423ff12f684cd",
			"e27097e84a812d45546de8d6a4ffa75538cab39c96f79b153b601240f8cefaf9",
			"ac853f996e8cce7390d4df35f7c9890a0b7a1728e41336bf98a9ca2e24b5fef5",
			"6f44a266dae578e37bc0c46df54b6f137f4fedc3385219c67b889db846843bc4",
		} {
			if !strings.Contains(string(b), s) {
				t.Errorf("resources.yml lacks %s", s)
			}
		}
	})
	t.Run("per-arch sha256 map", func(t *testing.T) {
		other := filepath.Join(dir, "helper-amd64")
		if err := os.WriteFile(other, []byte("amd64"), 0o755); err != nil {
			t.Fatal(err)
		}
		body := "- name: helper\n  path:\n    darwin-arm64: " + helper + "\n    darwin-amd64: " + other +
			"\n  sha256:\n    darwin-arm64: " + good + "\n    darwin-amd64: " + sha([]byte("amd64")) + "\n"
		m := write("perarch.yml", body)
		for _, arch := range []string{"arm64", "amd64"} {
			if out, err := resources(t, m, arch, t.TempDir()); err != nil {
				t.Fatalf("%s: %v\n%s", arch, err, out)
			}
		}
		swapped := "- name: helper\n  path:\n    darwin-arm64: " + helper + "\n    darwin-amd64: " + other +
			"\n  sha256:\n    darwin-arm64: " + sha([]byte("amd64")) + "\n    darwin-amd64: " + good + "\n"
		if out, err := resources(t, write("swapped.yml", swapped), "arm64", t.TempDir()); err == nil {
			t.Fatalf("accepted swapped per-arch sha256:\n%s", out)
		}
	})
	t.Run("license is copied", func(t *testing.T) {
		if err := os.WriteFile(helper+".LICENSE", []byte("MIT License\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(helper + ".LICENSE")
		body := entry(helper, good) + "  fetch:\n    darwin-arm64:\n      url: https://example.com/h.tar.gz\n      sha256: " + good +
			"\n      member: helper\n      license: LICENSE\n"
		dest, lic := t.TempDir(), t.TempDir()
		if out, err := tool(t, "-manifest", write("lic.yml", body), "-arch", "arm64", "-dest", dest, "-licenses", lic); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		if b, err := os.ReadFile(filepath.Join(lic, "helper.txt")); err != nil || string(b) != "MIT License\n" {
			t.Errorf("license not copied: %q %v", b, err)
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
		"missing file":                   entry(filepath.Join(dir, "nope"), good),
		"checksum":                       entry(helper, strings.Repeat("0", 64)),
		"no sha256":                      "- name: helper\n  path:\n    darwin-arm64: " + helper + "\n",
		"no arch path":                   "- name: helper\n  path:\n    darwin-amd64: " + helper + "\n  sha256: " + good + "\n",
		"bad name":                       strings.Replace(entry(helper, good), "name: helper", "name: ../evil", 1),
		"unknown field":                  entry(helper, good) + "  extra: 1\n",
		"duplicate name":                 entry(helper, good) + entry(helper, good),
		"sha256 map without target arch": "- name: helper\n  path:\n    darwin-arm64: " + helper + "\n  sha256:\n    darwin-amd64: " + good + "\n",
		"sha256 map unknown arch":        "- name: helper\n  path:\n    darwin-arm64: " + helper + "\n  sha256:\n    darwin-arm64: " + good + "\n    linux-arm64: " + good + "\n",
		"fetch bad scheme":               entry(helper, good) + "  fetch:\n    darwin-arm64:\n      url: ftp://x/y.tar.gz\n      sha256: " + good + "\n      member: helper\n",
		"fetch bad sha256":               entry(helper, good) + "  fetch:\n    darwin-arm64:\n      url: https://x/y.tar.gz\n      sha256: abc\n      member: helper\n",
		"fetch member traversal":         entry(helper, good) + "  fetch:\n    darwin-arm64:\n      url: https://x/y.tar.gz\n      sha256: " + good + "\n      member: ../helper\n",
		"fetch unknown field":            entry(helper, good) + "  fetch:\n    darwin-arm64:\n      url: https://x/y.tar.gz\n      sha256: " + good + "\n      member: helper\n      extra: 1\n",
		"fetch unknown arch":             entry(helper, good) + "  fetch:\n    linux-arm64:\n      url: https://x/y.tar.gz\n      sha256: " + good + "\n      member: helper\n",
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

// tarball builds a .tar.gz holding the given members.
func tarball(t *testing.T, members map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range members {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestFetchHelper(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go run; skipped with -short")
	}
	bin := []byte("#!/bin/sh\necho vale version 9.9.9\n")
	license := []byte("The MIT License (MIT)\n")
	tgz := tarball(t, map[string][]byte{"README.md": []byte("readme"), "LICENSE": license, "vale": bin})

	// setup writes the tarball and a manifest fetching it into dir/helpers/arm64/vale.
	setup := func(t *testing.T, tarSHA, binSHA, member string) (manifest, target, tgzPath string) {
		t.Helper()
		dir := t.TempDir()
		tgzPath = filepath.Join(dir, "vale.tar.gz")
		if err := os.WriteFile(tgzPath, tgz, 0o644); err != nil {
			t.Fatal(err)
		}
		manifest = filepath.Join(dir, "resources.yml")
		body := "- name: vale\n  path:\n    darwin-arm64: helpers/arm64/vale\n    darwin-amd64: helpers/amd64/vale\n" +
			"  sha256:\n    darwin-arm64: " + binSHA + "\n    darwin-amd64: " + strings.Repeat("1", 64) + "\n" +
			"  fetch:\n    darwin-arm64:\n      url: file://" + tgzPath + "\n      sha256: " + tarSHA +
			"\n      member: " + member + "\n      license: LICENSE\n"
		if err := os.WriteFile(manifest, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return manifest, filepath.Join(dir, "helpers", "arm64", "vale"), tgzPath
	}
	fetch := func(t *testing.T, manifest string) ([]byte, error) {
		t.Helper()
		return tool(t, "-manifest", manifest, "-arch", "arm64", "-fetch")
	}

	t.Run("downloads, verifies and extracts", func(t *testing.T) {
		manifest, target, tgzPath := setup(t, sha(tgz), sha(bin), "vale")
		if out, err := fetch(t, manifest); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		b, err := os.ReadFile(target)
		if err != nil || !bytes.Equal(b, bin) {
			t.Fatalf("helper not extracted: %v", err)
		}
		if fi, _ := os.Stat(target); fi.Mode()&0o111 == 0 {
			t.Error("helper not executable")
		}
		if b, err := os.ReadFile(target + ".LICENSE"); err != nil || !bytes.Equal(b, license) {
			t.Errorf("license not extracted: %v", err)
		}
		// Second run is skipped: the verified binary is there, the tarball need not be.
		if err := os.Remove(tgzPath); err != nil {
			t.Fatal(err)
		}
		if out, err := fetch(t, manifest); err != nil {
			t.Fatalf("re-fetched a verified helper: %v\n%s", err, out)
		}
		// Then package mode copies it.
		dest, lic := t.TempDir(), t.TempDir()
		if out, err := tool(t, "-manifest", manifest, "-arch", "arm64", "-dest", dest, "-licenses", lic); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		if _, err := os.Stat(filepath.Join(dest, "vale")); err != nil {
			t.Error(err)
		}
		if _, err := os.Stat(filepath.Join(lic, "vale.txt")); err != nil {
			t.Error(err)
		}
	})
	t.Run("replaces a tampered helper", func(t *testing.T) {
		manifest, target, _ := setup(t, sha(tgz), sha(bin), "vale")
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("evil"), 0o755); err != nil {
			t.Fatal(err)
		}
		if out, err := fetch(t, manifest); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		if b, _ := os.ReadFile(target); !bytes.Equal(b, bin) {
			t.Error("tampered helper kept")
		}
	})
	fails := map[string][3]string{
		"tarball checksum": {strings.Repeat("0", 64), sha(bin), "vale"},
		"binary checksum":  {sha(tgz), strings.Repeat("0", 64), "vale"},
		"missing member":   {sha(tgz), sha(bin), "bin/vale"},
	}
	for name, f := range fails {
		t.Run("rejects "+name, func(t *testing.T) {
			manifest, target, _ := setup(t, f[0], f[1], f[2])
			out, err := fetch(t, manifest)
			if err == nil {
				t.Fatalf("accepted:\n%s", out)
			}
			if _, err := os.Stat(target); err == nil {
				t.Error("helper written despite failure")
			}
		})
	}
	t.Run("no fetch block for arch is an error", func(t *testing.T) {
		manifest, _, _ := setup(t, sha(tgz), sha(bin), "vale")
		if out, err := tool(t, "-manifest", manifest, "-arch", "amd64", "-fetch"); err == nil {
			t.Fatalf("accepted:\n%s", out)
		}
	})
}

func TestBundledVale(t *testing.T) {
	requireBundle(t)
	vale := filepath.Join(bundle, "Contents/Helpers/vale")
	out, err := exec.Command(vale, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("vale --version: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "vale version 3.25.1" {
		t.Errorf("vale --version = %q, want vale version 3.25.1", got)
	}
	lic, err := os.ReadFile(filepath.Join(bundle, "Contents/Resources/licenses/vale.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(lic), "MIT") {
		t.Errorf("licenses/vale.txt is not the MIT notice:\n%s", lic)
	}
	if out, err := exec.Command("codesign", "--verify", "--strict", vale).CombinedOutput(); err != nil {
		t.Errorf("vale helper not signed: %v\n%s", err, out)
	}
}
