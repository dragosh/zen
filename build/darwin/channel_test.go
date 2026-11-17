package darwin_test

// Tests for specs/packaging/appstore: branding, Info.plist, privacy manifest,
// universal binaries, and the two release channels (Developer ID, App Store).
// Nothing here needs a signing identity: signature checks use ad-hoc
// signatures and the identity-only parts skip with a message.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// channel is the channel the bundle in bin/ was built for (task package:test sets it).
func channel() string {
	if c := os.Getenv("ZEN_CHANNEL"); c != "" {
		return c
	}
	return "devid"
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// configValue reads a top-level key of info: in build/config.yml.
func configValue(t *testing.T, key string) string {
	t.Helper()
	m := regexp.MustCompile(`(?m)^  ` + key + `:\s*"?([^"\n#]*?)"?\s*(#.*)?$`).FindStringSubmatch(read(t, "../config.yml"))
	if m == nil {
		t.Fatalf("build/config.yml has no info.%s", key)
	}
	return m[1]
}

// helperFiles lists the files in Contents/Helpers of the bundle.
func helperFiles(t *testing.T) []string {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join(bundle, "Contents", "Helpers"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() {
			out = append(out, filepath.Join(bundle, "Contents", "Helpers", e.Name()))
		}
	}
	return out
}

// machOs lists every Mach-O file of the bundle.
func machOs(t *testing.T) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(bundle, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer f.Close()
		var m [4]byte
		if _, err := f.Read(m[:]); err != nil {
			return nil
		}
		switch string(m[:]) {
		case "\xcf\xfa\xed\xfe", "\xce\xfa\xed\xfe", "\xca\xfe\xba\xbe", "\xbe\xba\xfe\xca":
			out = append(out, p)
		}
		return nil
	})
	return out
}

// plistJSON converts a plist file to a JSON map.
func plistJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	out, err := exec.Command("plutil", "-convert", "json", "-o", "-", path).Output()
	if err != nil {
		t.Fatalf("plutil %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func needDarwin(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only")
	}
}

// wailsArt are the sha256 sums of the Wails project template files that used
// to be the DMG art; none of them may come back.
var wailsArt = map[string]string{
	"5f07c1d4eafd111b76316b7d25ab50906848cf468d6862b1b101c20a89a920df": "dmg-background.png (Wails template)",
	"cc1162fd1d662437e5aa50b99fc5b2aea164cf5b9f8390f492e2508e7bd2bf60": "dmg-file-icon.icns (Wails template)",
	"4e04ec7f61f6ffabd86a581b43a6c0768c28a81c84c7704d054d010e3af0731a": "dmg-file-icon.png (Wails template)",
}

func TestNoWailsBranding(t *testing.T) {
	// 1. No file under build/ is a Wails template image.
	_ = filepath.WalkDir("..", func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return nil
		}
		b, _ := os.ReadFile(p)
		sum := sha256.Sum256(b)
		if what, ok := wailsArt[hex.EncodeToString(sum[:])]; ok {
			t.Errorf("%s is the %s", p, what)
		}
		return nil
	})
	if _, err := os.Stat("dmg-file-icon.png"); err == nil {
		t.Error("dmg-file-icon.png is a Wails template file; the art is generated into dmg-file-icon.icns")
	}

	// 2. The config has no template comments or example file types.
	cfg := read(t, "../config.yml")
	for _, s := range []string{"wailsFileIcon", "jpegFileIcon", "v3.wails.io", "ext: wails", "common:update:build-assets"} {
		if strings.Contains(cfg, s) {
			t.Errorf("build/config.yml still has the Wails template text %q", s)
		}
	}
	for i, line := range strings.Split(cfg, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") && strings.Contains(strings.ToLower(line), "wails") {
			t.Errorf("build/config.yml:%d is a comment that names Wails: %s", i+1, line)
		}
	}

	// 3. What a user can see: the plist templates, the privacy manifest, the
	// generated art, and (when built) the bundle's Resources and Info.plist.
	var files []string
	for _, g := range []string{"Info.plist", "Info.dev.plist", "PrivacyInfo.xcprivacy", "dmg-*", "icons.icns", "*.entitlements*.plist", "entitlements*.plist"} {
		m, _ := filepath.Glob(g)
		files = append(files, m...)
	}
	if _, err := os.Stat(bundle); err == nil {
		files = append(files, filepath.Join(bundle, "Contents", "Info.plist"))
		_ = filepath.WalkDir(filepath.Join(bundle, "Contents", "Resources"), func(p string, d os.DirEntry, err error) error {
			if err == nil && d.Type().IsRegular() {
				files = append(files, p)
			}
			return nil
		})
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		if strings.Contains(strings.ToLower(string(b)), "wails") {
			t.Errorf("%s contains the string wails", f)
		}
	}
	if len(files) < 4 {
		t.Errorf("scanned only %v", files)
	}
}

func TestIconAssets(t *testing.T) {
	// The fallback .icns carries the 1024 px image (ic10) the App Store needs.
	icns, err := os.ReadFile("icons.icns")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(icns), "ic10") {
		t.Error("icons.icns has no 1024 px (ic10) image")
	}
	// The sources of the asset catalog are in the tree.
	for _, p := range []string{"../appicon.icon/icon.json", "../appicon.svg", "../appicon.png"} {
		if _, err := os.Stat(p); err != nil {
			t.Error(err)
		}
	}
	tf := read(t, "Taskfile.yml")
	for _, s := range []string{"actool", "Assets.car"} {
		if !strings.Contains(tf, s) {
			t.Errorf("Taskfile.yml does not mention %s", s)
		}
	}
	if i, j := strings.Index(tf, "  icon:assets:"), strings.Index(tf, "  fetch:helpers:"); i < 0 || j < i ||
		!strings.Contains(tf[i:j], "actool") || !strings.Contains(tf[i:j], "CHANNEL") {
		t.Error("the actool step (icon:assets) must depend on the channel (fail for appstore, warn for devid)")
	}

	// The bundle: icns always; Assets.car compiled from appicon.icon.
	requireBundle(t)
	if _, err := os.Stat(filepath.Join(bundle, "Contents/Resources/icons.icns")); err != nil {
		t.Error(err)
	}
	car := filepath.Join(bundle, "Contents/Resources/Assets.car")
	if _, err := os.Stat(car); err != nil {
		if out, e := exec.Command("xcrun", "--find", "actool").CombinedOutput(); e != nil {
			if channel() == "appstore" {
				t.Errorf("no Assets.car and no actool (full Xcode needed): the App Store build must fail: %s", out)
			}
			t.Skip("no Assets.car: actool is not installed (full Xcode); a Developer ID build only warns")
		}
		t.Fatalf("Assets.car missing from the bundle although actool is available")
	}
	out, err := exec.Command("assetutil", "--info", car).Output()
	if err != nil {
		t.Fatalf("assetutil: %v", err)
	}
	if !strings.Contains(string(out), "appicon") {
		t.Errorf("Assets.car has no appicon:\n%.500s", out)
	}
}

func renderPlist(t *testing.T, extra ...string) (string, []byte, error) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "Info.plist")
	args := append([]string{"run", "./plist", "-config", "../config.yml", "-template", "Info.plist",
		"-version", "9.8.7", "-build", "42", "-out", out}, extra...)
	b, err := exec.Command("go", args...).CombinedOutput()
	return out, b, err
}

func TestInfoPlist(t *testing.T) {
	needDarwin(t)
	if testing.Short() {
		t.Skip("runs go run")
	}
	out, log, err := renderPlist(t)
	if err != nil {
		t.Fatalf("%v\n%s", err, log)
	}
	got := plistJSON(t, out)
	want := map[string]any{
		"CFBundleName":                  configValue(t, "productName"),
		"CFBundleDisplayName":           configValue(t, "productName"),
		"CFBundleIdentifier":            configValue(t, "bundleId"),
		"CFBundleShortVersionString":    "9.8.7",
		"CFBundleVersion":               "42",
		"CFBundleIconFile":              "icons",
		"CFBundleIconName":              "appicon",
		"LSMinimumSystemVersion":        "13.0",
		"LSApplicationCategoryType":     "public.app-category.productivity",
		"ITSAppUsesNonExemptEncryption": false,
		"NSHumanReadableCopyright":      configValue(t, "copyright"),
		"CFBundleExecutable":            "zen",
		"CFBundlePackageType":           "APPL",
		"NSHighResolutionCapable":       true,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %v, want %v", k, got[k], w)
		}
	}
	if configValue(t, "bundleId") != bundleID {
		t.Errorf("bundle id %q changed; the owner has not decided yet", configValue(t, "bundleId"))
	}
	if c := configValue(t, "copyright"); c != "© 2026 zen.md authors" {
		t.Errorf("copyright placeholder = %q", c)
	}
	if configValue(t, "category") != "public.app-category.productivity" {
		t.Errorf("category = %q", configValue(t, "category"))
	}
	if strings.Contains(read(t, out), "{{") {
		t.Error("placeholders left in the rendered plist")
	}

	t.Run("the config drives the values", func(t *testing.T) {
		dir := t.TempDir()
		cfg := filepath.Join(dir, "config.yml")
		body := "info:\n  productName: Zen Test\n  bundleId: app.example.zen\n  copyright: \"© 2030 <Me & Co>\"\n  category: public.app-category.education\n"
		if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(dir, "Info.plist")
		if b, err := exec.Command("go", "run", "./plist", "-config", cfg, "-template", "Info.plist", "-version", "1.2.3", "-build", "7", "-out", out).CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, b)
		}
		m := plistJSON(t, out) // also proves the XML is well formed after escaping
		if m["CFBundleIdentifier"] != "app.example.zen" || m["CFBundleName"] != "Zen Test" ||
			m["NSHumanReadableCopyright"] != "© 2030 <Me & Co>" || m["LSApplicationCategoryType"] != "public.app-category.education" {
			t.Errorf("config values not used: %v", m)
		}
	})
	t.Run("missing config value is an error", func(t *testing.T) {
		cfg := filepath.Join(t.TempDir(), "config.yml")
		if err := os.WriteFile(cfg, []byte("info:\n  productName: Zen\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		b, err := exec.Command("go", "run", "./plist", "-config", cfg, "-template", "Info.plist", "-version", "1.2.3", "-build", "7", "-out", filepath.Join(t.TempDir(), "x")).CombinedOutput()
		if err == nil {
			t.Fatalf("accepted a config without bundleId:\n%s", b)
		}
	})

	// The built bundle carries the same keys.
	requireBundle(t)
	for _, k := range []string{"LSApplicationCategoryType", "ITSAppUsesNonExemptEncryption", "NSHumanReadableCopyright", "CFBundleIconName"} {
		if v := plist(t, k); v == "" {
			t.Errorf("bundle Info.plist lacks %s", k)
		}
	}
	if v := plist(t, "ITSAppUsesNonExemptEncryption"); v != "false" {
		t.Errorf("ITSAppUsesNonExemptEncryption = %s", v)
	}
	if v := plist(t, "NSHumanReadableCopyright"); v != configValue(t, "copyright") {
		t.Errorf("NSHumanReadableCopyright = %q", v)
	}
}

// requiredReason maps symbols a binary imports to the privacy category that
// must declare a reason for them.
var requiredReason = []struct {
	category string
	symbols  []string
}{
	{"NSPrivacyAccessedAPICategoryFileTimestamp", []string{"_stat", "_fstat", "_lstat", "_fstatat", "_getattrlist", "_getattrlistbulk", "_fgetattrlist"}},
	{"NSPrivacyAccessedAPICategorySystemBootTime", []string{"_mach_absolute_time", "_systemUptime"}},
	{"NSPrivacyAccessedAPICategoryDiskSpace", []string{"_statfs", "_fstatfs", "_statvfs", "_fstatvfs", "_getattrlist_volume"}},
	{"NSPrivacyAccessedAPICategoryUserDefaults", []string{"_OBJC_CLASS_$_NSUserDefaults"}},
}

func TestPrivacyManifest(t *testing.T) {
	needDarwin(t)
	m := plistJSON(t, "PrivacyInfo.xcprivacy")
	if m["NSPrivacyTracking"] != false {
		t.Errorf("NSPrivacyTracking = %v, want false", m["NSPrivacyTracking"])
	}
	for _, k := range []string{"NSPrivacyTrackingDomains", "NSPrivacyCollectedDataTypes"} {
		if a, ok := m[k].([]any); !ok || len(a) != 0 {
			t.Errorf("%s = %v, want an empty list", k, m[k])
		}
	}
	declared := map[string][]string{}
	apis, _ := m["NSPrivacyAccessedAPITypes"].([]any)
	for _, a := range apis {
		e := a.(map[string]any)
		cat, _ := e["NSPrivacyAccessedAPIType"].(string)
		rs, _ := e["NSPrivacyAccessedAPITypeReasons"].([]any)
		for _, r := range rs {
			declared[cat] = append(declared[cat], r.(string))
		}
	}
	has := func(cat string, reasons ...string) {
		for _, w := range reasons {
			found := false
			for _, g := range declared[cat] {
				found = found || g == w
			}
			if !found {
				t.Errorf("%s lacks reason %s (has %v)", cat, w, declared[cat])
			}
		}
	}
	has("NSPrivacyAccessedAPICategoryFileTimestamp", "C617.1", "3B52.1")

	// Symbol scan: a category is declared exactly when the binary imports its symbols.
	bin := ""
	for _, p := range []string{filepath.Join(bundle, "Contents/MacOS/zen"), "../../bin/zen"} {
		if _, err := os.Stat(p); err == nil {
			bin = p
			break
		}
	}
	if bin == "" {
		t.Log("no built binary: the symbol scan is skipped (run task package)")
		return
	}
	out, err := exec.Command("nm", "-u", bin).Output()
	if err != nil {
		t.Fatalf("nm: %v", err)
	}
	imported := map[string]bool{}
	for _, l := range strings.Fields(string(out)) {
		imported[l] = true
	}
	for _, c := range requiredReason {
		used := false
		var which []string
		for _, s := range c.symbols {
			if imported[s] {
				used = true
				which = append(which, s)
			}
		}
		_, isDeclared := declared[c.category]
		if used && !isDeclared {
			t.Errorf("the binary imports %v but the manifest does not declare %s", which, c.category)
		}
		if !used && isDeclared && c.category != "NSPrivacyAccessedAPICategoryFileTimestamp" {
			t.Errorf("the manifest declares %s but the binary imports none of its symbols", c.category)
		}
	}
	if _, err := os.Stat(bundle); err == nil {
		if _, err := os.Stat(filepath.Join(bundle, "Contents/Resources/PrivacyInfo.xcprivacy")); err != nil {
			t.Errorf("the bundle lacks Resources/PrivacyInfo.xcprivacy: %v", err)
		}
	}
}

func archs(t *testing.T, p string) []string {
	t.Helper()
	out, err := exec.Command("lipo", "-archs", p).Output()
	if err != nil {
		t.Fatalf("lipo -archs %s: %v", p, err)
	}
	a := strings.Fields(string(out))
	sort.Strings(a)
	return a
}

func TestUniversal(t *testing.T) {
	requireBundle(t)
	all := machOs(t)
	if len(all) < 2 {
		t.Fatalf("expected zen and a helper, found %v", all)
	}
	want := []string{"arm64"}
	if runtime.GOARCH == "amd64" {
		want = []string{"x86_64"}
	}
	if os.Getenv("ZEN_UNIVERSAL") == "1" {
		want = []string{"arm64", "x86_64"}
	}
	for _, p := range all {
		got := archs(t, p)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s is %v, want %v", strings.TrimPrefix(p, bundle+"/"), got, want)
		}
	}
}

// goBinary builds a tiny Mach-O for goarch.
func goBinary(t *testing.T, dir, goarch string) string {
	t.Helper()
	src := filepath.Join(dir, "src-"+goarch)
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "main.go"), []byte("package main\nfunc main() { println(\""+goarch+"\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "go.mod"), []byte("module h\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "h-"+goarch)
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = src
	cmd.Env = append(os.Environ(), "GOOS=darwin", "GOARCH="+goarch, "CGO_ENABLED=0", "GOFLAGS=-mod=mod")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build %s: %v\n%s", goarch, err, b)
	}
	return out
}

func fileSHA(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// TestResourcesUniversal: -arch universal verifies both architectures of every
// helper and merges them with lipo.
func TestResourcesUniversal(t *testing.T) {
	needDarwin(t)
	if testing.Short() {
		t.Skip("runs go build")
	}
	if _, err := exec.LookPath("lipo"); err != nil {
		t.Skip("no lipo")
	}
	dir := t.TempDir()
	arm, amd := goBinary(t, dir, "arm64"), goBinary(t, dir, "amd64")
	body := "- name: helper\n  path:\n    darwin-arm64: " + arm + "\n    darwin-amd64: " + amd + "\n" +
		"  sha256:\n    darwin-arm64: " + fileSHA(t, arm) + "\n    darwin-amd64: " + fileSHA(t, amd) + "\n"
	manifest := filepath.Join(dir, "resources.yml")
	if err := os.WriteFile(manifest, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	if out, err := resources(t, manifest, "universal", dest); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got := archs(t, filepath.Join(dest, "helper")); strings.Join(got, ",") != "arm64,x86_64" {
		t.Errorf("universal helper archs = %v", got)
	}
	if out, err := tool(t, "-manifest", manifest, "-arch", "universal", "-validate"); err != nil {
		t.Errorf("-validate universal: %v\n%s", err, out)
	}

	t.Run("a bad checksum for either arch fails and copies nothing", func(t *testing.T) {
		bad := strings.Replace(body, fileSHA(t, amd), strings.Repeat("0", 64), 1)
		dest := t.TempDir()
		if out, err := resources(t, filepath.Join(dir, "bad.yml"), "universal", dest); err == nil {
			t.Fatalf("missing manifest accepted:\n%s", out)
		}
		if err := os.WriteFile(filepath.Join(dir, "bad.yml"), []byte(bad), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := resources(t, filepath.Join(dir, "bad.yml"), "universal", dest); err == nil {
			t.Fatalf("accepted a bad amd64 checksum:\n%s", out)
		}
		if _, err := os.Stat(filepath.Join(dest, "helper")); err == nil {
			t.Error("helper copied despite failure")
		}
	})
	t.Run("an entry without an amd64 path is an error", func(t *testing.T) {
		only := "- name: helper\n  path:\n    darwin-arm64: " + arm + "\n  sha256: " + fileSHA(t, arm) + "\n"
		p := filepath.Join(dir, "armonly.yml")
		if err := os.WriteFile(p, []byte(only), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := resources(t, p, "universal", t.TempDir()); err == nil {
			t.Fatalf("universal accepted a helper without amd64:\n%s", out)
		}
	})
}

// scriptEnv runs build/darwin/check-channel.sh.
func checkChannel(t *testing.T, step string, env ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", "check-channel.sh", step)
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir()}, env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestChannelPreconditions(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "zen.provisionprofile")
	if err := os.WriteFile(profile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	dist := "SIGN_IDENTITY=Apple Distribution: Example (ABCDE12345)"
	ok := func(name, step string, env ...string) {
		t.Run(name, func(t *testing.T) {
			if out, err := checkChannel(t, step, env...); err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
		})
	}
	bad := func(name, step, mention string, env ...string) {
		t.Run(name, func(t *testing.T) {
			out, err := checkChannel(t, step, env...)
			if err == nil {
				t.Fatalf("accepted:\n%s", out)
			}
			if !strings.Contains(out, mention) {
				t.Errorf("message does not mention %s:\n%s", mention, out)
			}
		})
	}
	ok("devid defaults", "sign", "CHANNEL=devid")
	ok("channel defaults to devid", "sign")
	bad("unknown channel", "sign", "devid", "CHANNEL=beta")
	bad("appstore without PROFILE fails early", "sign", "PROFILE", "CHANNEL=appstore", dist, "TEAM_ID=ABCDE12345")
	bad("appstore with a missing profile file", "sign", "PROFILE", "CHANNEL=appstore", dist, "TEAM_ID=ABCDE12345", "PROFILE=/nonexistent.provisionprofile")
	bad("appstore with the ad-hoc identity", "sign", "Apple Distribution", "CHANNEL=appstore", "PROFILE="+profile, "TEAM_ID=ABCDE12345")
	bad("appstore without TEAM_ID", "sign", "TEAM_ID", "CHANNEL=appstore", dist, "PROFILE="+profile)
	ok("appstore signing inputs", "sign", "CHANNEL=appstore", dist, "TEAM_ID=ABCDE12345", "PROFILE="+profile)
	bad("appstore pkg without INSTALLER_IDENTITY", "pkg", "INSTALLER_IDENTITY", "CHANNEL=appstore", dist, "TEAM_ID=ABCDE12345", "PROFILE="+profile)
	ok("appstore pkg", "pkg", "CHANNEL=appstore", dist, "TEAM_ID=ABCDE12345", "PROFILE="+profile, "INSTALLER_IDENTITY=3rd Party Mac Developer Installer: Example (ABCDE12345)")
	ok("ADHOC=1 is a local dry run", "sign", "CHANNEL=appstore", "ADHOC=1")
	ok("ADHOC=1 pkg is skipped", "pkg", "CHANNEL=appstore", "ADHOC=1")
}

func TestSignatureAppStore(t *testing.T) {
	needDarwin(t)
	// The files, always.
	for f, want := range map[string][]string{
		"entitlements.sandbox.plist":        wantSandboxEntitlements,
		"helper.entitlements.sandbox.plist": {"com.apple.security.app-sandbox", "com.apple.security.inherit"},
	} {
		if got := entitlementKeys(t, read(t, f)); !sameSet(got, want) {
			t.Errorf("%s keys = %v, want %v", f, got, want)
		}
	}
	for _, bad := range []string{"files.all", "temporary-exception", "network.server", "files.downloads", "device."} {
		if strings.Contains(read(t, "entitlements.sandbox.plist"), bad) {
			t.Errorf("entitlements.sandbox.plist has %s", bad)
		}
	}
	if _, err := os.Stat("sign.sh"); err != nil {
		t.Fatal(err)
	}
	requireBundle(t)

	// Re-sign a copy of the built bundle the App Store way, ad hoc.
	cp := filepath.Join(t.TempDir(), "Zen.app")
	if out, err := exec.Command("cp", "-R", bundle, cp).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	cmd := exec.Command("sh", "sign.sh", cp)
	cmd.Env = append(os.Environ(), "CHANNEL=appstore", "ADHOC=1", "SIGN_IDENTITY=-", "TEAM_ID=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sign.sh: %v\n%s", err, out)
	}
	if out, err := exec.Command("codesign", "--verify", "--deep", "--strict", cp).CombinedOutput(); err != nil {
		t.Fatalf("verify: %v\n%s", err, out)
	}
	ents := func(p string) []string {
		out, err := exec.Command("codesign", "-d", "--entitlements", "-", "--xml", p).Output()
		if err != nil {
			t.Fatalf("codesign -d %s: %v", p, err)
		}
		return entitlementKeys(t, string(out))
	}
	if got := ents(cp); !sameSet(got, wantSandboxEntitlements) {
		t.Errorf("app entitlements = %v, want exactly %v", got, wantSandboxEntitlements)
	}
	n := 0
	ents2 := []string{"com.apple.security.app-sandbox", "com.apple.security.inherit"}
	_ = filepath.WalkDir(filepath.Join(cp, "Contents", "Helpers"), func(p string, d os.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() && !strings.HasSuffix(p, ".LICENSE") {
			n++
			if got := ents(p); !sameSet(got, ents2) {
				t.Errorf("helper %s entitlements = %v, want %v", filepath.Base(p), got, ents2)
			}
		}
		return nil
	})
	if n == 0 {
		t.Error("no helper found to check")
	}

	// Built for the App Store with a real profile and identity: the artefacts.
	if channel() == "appstore" && os.Getenv("ZEN_ADHOC") != "1" {
		if _, err := os.Stat(filepath.Join(bundle, "Contents/embedded.provisionprofile")); err != nil {
			t.Errorf("no embedded provisioning profile: %v", err)
		}
		if _, err := os.Stat("../../bin/Zen.pkg"); err != nil {
			t.Errorf("no bin/Zen.pkg: %v", err)
		}
	} else if channel() == "appstore" {
		t.Log("ad-hoc App Store build: profile embedding and Zen.pkg need real identities")
	}
}

func TestTaskfileChannels(t *testing.T) {
	tf := read(t, "Taskfile.yml")
	for _, s := range []string{
		"CHANNEL", "appstore", "package:appstore", "sign:appstore", "art:", "dmgart",
		"embedded.provisionprofile", "productbuild", "INSTALLER_IDENTITY", "PROFILE", "TEAM_ID",
		"check-channel.sh", "sign.sh", "darwin/plist", "PrivacyInfo.xcprivacy",
		// notarisation of the DMG
		"notarytool", "stapler", "spctl -a -t open --context context:primary-signature",
	} {
		if !strings.Contains(tf, s) {
			t.Errorf("Taskfile.yml lacks %q", s)
		}
	}
	if !regexp.MustCompile(`-tags[^\n]*appstore|EXTRA_TAGS[^\n]*appstore`).MatchString(tf) {
		t.Error("the appstore channel does not build with -tags appstore")
	}
}

// TestNoSecrets: no team id, identity, password or key in the committed build files.
func TestNoSecrets(t *testing.T) {
	teamID := regexp.MustCompile(`\([A-Z0-9]{10}\)`)
	pw := regexp.MustCompile(`(?i)(--password|app-specific|apple-id\s*[:=]|@[a-z0-9-]+\.(com|org|net))`)
	var scanned int
	_ = filepath.WalkDir("..", func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		base := d.Name()
		for _, ext := range []string{".p12", ".cer", ".provisionprofile", ".pem", ".key", ".mobileprovision"} {
			if strings.HasSuffix(base, ext) {
				t.Errorf("%s: a credential file in the tree", p)
			}
		}
		if strings.HasSuffix(base, "_test.go") {
			return nil
		}
		if !(strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".plist") || strings.HasSuffix(base, ".sh") || strings.HasSuffix(base, ".xcprivacy")) {
			return nil
		}
		scanned++
		b, _ := os.ReadFile(p)
		if m := teamID.Find(b); m != nil {
			t.Errorf("%s has what looks like a team id %s", p, m)
		}
		if m := pw.Find(b); m != nil {
			t.Errorf("%s has %q", p, m)
		}
		return nil
	})
	if scanned < 5 {
		t.Errorf("scanned only %d files", scanned)
	}
}
