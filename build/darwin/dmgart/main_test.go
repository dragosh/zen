package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBackgroundSVG(t *testing.T) {
	a, b := backgroundSVG("Zenmark"), backgroundSVG("Zenmark")
	if a != b {
		t.Fatal("backgroundSVG is not deterministic")
	}
	if !strings.Contains(a, "Drag Zenmark to Applications") {
		t.Error("the DMG background lacks the instruction")
	}
	if strings.Contains(strings.ToLower(a), "wails") {
		t.Error("the DMG background mentions Wails")
	}
	// Black on white only.
	for _, c := range []string{"#000", "#fff"} {
		if !strings.Contains(a, c) {
			t.Errorf("no %s in the art", c)
		}
	}
	// The arrow sits between the app icon (28% of 540) and Applications (72%),
	// at the icon row of the Finder window (y = 380/2-26).
	if !strings.Contains(a, `data-arrow`) {
		t.Error("no arrow")
	}
}

func TestICNS(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	for y := 0; y < 1024; y++ {
		for x := 0; x < 1024; x++ {
			src.SetNRGBA(x, y, color.NRGBA{uint8(x / 4), uint8(y / 4), 128, 255})
		}
	}
	a, err := icns(src)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := icns(src)
	if !bytes.Equal(a, b) {
		t.Error("icns is not deterministic")
	}
	if string(a[:4]) != "icns" || int(binary.BigEndian.Uint32(a[4:8])) != len(a) {
		t.Fatalf("bad icns header (%d bytes)", len(a))
	}
	sizes := map[string]int{"ic07": 128, "ic08": 256, "ic09": 512, "ic10": 1024, "ic11": 32, "ic12": 64, "ic13": 256, "ic14": 512}
	seen := map[string]bool{}
	for off := 8; off < len(a); {
		typ := string(a[off : off+4])
		n := int(binary.BigEndian.Uint32(a[off+4 : off+8]))
		img, err := png.Decode(bytes.NewReader(a[off+8 : off+n]))
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if w := sizes[typ]; w == 0 || img.Bounds().Dx() != w || img.Bounds().Dy() != w {
			t.Errorf("%s is %v, want %d px", typ, img.Bounds(), sizes[typ])
		}
		seen[typ] = true
		off += n
	}
	for typ := range sizes {
		if !seen[typ] {
			t.Errorf("missing %s", typ)
		}
	}
}

// TestArt renders the real art (needs macOS QuickLook, like `task icon`).
func TestArt(t *testing.T) {
	if testing.Short() {
		t.Skip("renders with qlmanage")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only")
	}
	if _, err := exec.LookPath("qlmanage"); err != nil {
		t.Skip("no qlmanage")
	}
	render := func() string {
		out := t.TempDir()
		if err := run(options{svg: "../../appicon.svg", icon: "../../appicon.png", out: out, name: "Zenmark"}); err != nil {
			t.Fatal(err)
		}
		return out
	}
	one, two := render(), render()
	for name, dim := range map[string][2]int{"dmg-background.png": {540, 380}, "dmg-background@2x.png": {1080, 760}} {
		b, err := os.ReadFile(filepath.Join(one, name))
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != dim[0] || img.Bounds().Dy() != dim[1] {
			t.Errorf("%s is %v, want %dx%d", name, img.Bounds(), dim[0], dim[1])
		}
		dark := 0
		for y := 0; y < img.Bounds().Dy(); y += 2 {
			for x := 0; x < img.Bounds().Dx(); x += 2 {
				r, g, bl, _ := img.At(x, y).RGBA()
				if r != g || g != bl {
					t.Fatalf("%s has a coloured pixel at %d,%d: the art is black and white", name, x, y)
				}
				if r < 0x4000 {
					dark++
				}
			}
		}
		if dark == 0 {
			t.Errorf("%s has no black at all", name)
		}
		b2, _ := os.ReadFile(filepath.Join(two, name))
		if !bytes.Equal(b, b2) {
			t.Errorf("%s differs between two runs", name)
		}
	}
	ic, err := os.ReadFile(filepath.Join(one, "dmg-file-icon.icns"))
	if err != nil || string(ic[:4]) != "icns" {
		t.Errorf("dmg-file-icon.icns: %v", err)
	}
	if ic2, _ := os.ReadFile(filepath.Join(two, "dmg-file-icon.icns")); !bytes.Equal(ic, ic2) {
		t.Error("dmg-file-icon.icns differs between two runs")
	}
}

// TestCommitted: the committed art is zen's, at the sizes the DMG window needs.
func TestCommitted(t *testing.T) {
	for name, dim := range map[string][2]int{"../dmg-background.png": {540, 380}, "../dmg-background@2x.png": {1080, 760}} {
		f, err := os.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := png.DecodeConfig(f)
		f.Close()
		if err != nil || cfg.Width != dim[0] || cfg.Height != dim[1] {
			t.Errorf("%s: %v %v, want %dx%d", name, cfg, err, dim[0], dim[1])
		}
	}
	ic, err := os.ReadFile("../dmg-file-icon.icns")
	if err != nil || len(ic) < 8 || string(ic[:4]) != "icns" {
		t.Fatalf("dmg-file-icon.icns: %v", err)
	}
	if !bytes.Contains(ic, []byte("ic10")) {
		t.Error("dmg-file-icon.icns has no 1024 px image")
	}
}
