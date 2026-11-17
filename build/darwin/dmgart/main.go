// Command dmgart generates the DMG art from build/appicon.svg and
// build/appicon.png, so it can be regenerated whenever the icon changes
// (`task icon` runs it):
//
//	go run ./build/darwin/dmgart [-name zen.md] [-svg build/appicon.svg] [-icon build/appicon.png] [-out build/darwin]
//
// It writes dmg-background.png (540x380), dmg-background@2x.png (1080x760) and
// dmg-file-icon.icns. The background is black type and an arrow on white; the
// SVG is rasterised with macOS QuickLook (qlmanage), like `task icon` does.
// The icon is composed and scaled with the standard library only.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	winW, winH = 540, 380
	// Where `wails3 tool package --format dmg` puts the two icons: x at 28% and
	// 72% of the width, y at height/2-26 (icon centres, 96 px icons).
	iconY       = winH/2 - 26
	appX, appsX = winW * 28 / 100, winW * 72 / 100
)

type options struct{ svg, icon, out, name string }

func main() {
	var o options
	flag.StringVar(&o.name, "name", "", "product name in the title (default: info.productName of build/config.yml)")
	flag.StringVar(&o.svg, "svg", "build/appicon.svg", "app icon (SVG), for the check that it exists")
	flag.StringVar(&o.icon, "icon", "build/appicon.png", "app icon (1024 px PNG with transparent margin)")
	flag.StringVar(&o.out, "out", "build/darwin", "output directory")
	flag.Parse()
	if o.name == "" {
		o.name = productName("build/config.yml")
	}
	if err := run(o); err != nil {
		fmt.Fprintf(os.Stderr, "dmgart: %v\n", err)
		os.Exit(1)
	}
}

// productName reads info.productName from the build config (one variable
// names the product everywhere).
func productName(cfg string) string {
	b, err := os.ReadFile(cfg)
	if err != nil {
		return "zen.md"
	}
	for _, l := range bytes.Split(b, []byte("\n")) {
		t := bytes.TrimSpace(l)
		if bytes.HasPrefix(t, []byte("productName:")) {
			v := bytes.TrimSpace(bytes.TrimPrefix(t, []byte("productName:")))
			if i := bytes.Index(v, []byte(" #")); i >= 0 {
				v = bytes.TrimSpace(v[:i])
			}
			return string(bytes.Trim(v, `"`))
		}
	}
	return "zen.md"
}

// backgroundSVG is the art on a square canvas (QuickLook renders SVG onto a
// square): the DMG window is its top 540x380 part.
func backgroundSVG(name string) string {
	const f = `font-family="Georgia, 'Times New Roman', serif"`
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="540" height="540" viewBox="0 0 540 540">
  <rect width="540" height="540" fill="#fff"/>
  <text x="270" y="72" %[1]s font-size="26" text-anchor="middle" fill="#000">Drag %[2]s to Applications</text>
  <g data-arrow="1" fill="none" stroke="#000" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
    <path d="M%[3]d %[5]d H%[4]d"/>
    <path d="M%[6]d %[7]d L%[4]d %[5]d L%[6]d %[8]d"/>
  </g>
  <text x="270" y="330" %[1]s font-size="13" font-style="italic" text-anchor="middle" fill="#000">Markdown reader and editor</text>
</svg>
`, f, html.EscapeString(name), appX+70, appsX-70, iconY, appsX-84, iconY-12, iconY+12)
}

func run(o options) error {
	if _, err := os.Stat(o.svg); err != nil {
		return err
	}
	if err := os.MkdirAll(o.out, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "dmgart")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	svg := filepath.Join(tmp, "background.svg")
	if err := os.WriteFile(svg, []byte(backgroundSVG(o.name)), 0o644); err != nil {
		return err
	}
	for _, s := range []struct {
		name string
		k    int
	}{{"dmg-background.png", 1}, {"dmg-background@2x.png", 2}} {
		img, err := rasterise(svg, 540*s.k, tmp)
		if err != nil {
			return err
		}
		if err := writePNG(filepath.Join(o.out, s.name), crop(img, winW*s.k, winH*s.k)); err != nil {
			return err
		}
	}
	f, err := os.Open(o.icon)
	if err != nil {
		return err
	}
	src, err := png.Decode(f)
	f.Close()
	if err != nil {
		return err
	}
	data, err := icns(fileIcon(src))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(o.out, "dmg-file-icon.icns"), data, 0o644)
}

// rasterise renders svg with QuickLook at size x size.
func rasterise(svg string, size int, tmp string) (image.Image, error) {
	dir := filepath.Join(tmp, fmt.Sprint(size))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if out, err := exec.Command("qlmanage", "-t", "-s", fmt.Sprint(size), "-o", dir, svg).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("qlmanage: %v\n%s", err, out)
	}
	f, err := os.Open(filepath.Join(dir, filepath.Base(svg)+".png"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	if b := img.Bounds(); b.Dx() != size || b.Dy() != size {
		return nil, fmt.Errorf("QuickLook rendered %v, want %dx%d", b, size, size)
	}
	return img, nil
}

// crop returns the top-left w x h part as an opaque grayscale-safe NRGBA.
func crop(img image.Image, w, h int) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(out, out.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	draw.Draw(out, out.Bounds(), img, img.Bounds().Min, draw.Over)
	return out
}

func writePNG(path string, img image.Image) error {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

// fileIcon is the icon of the .dmg file: the app tile standing on a disk
// slab, on a 1024 px transparent canvas.
func fileIcon(tile image.Image) *image.NRGBA {
	const n = 1024
	out := image.NewNRGBA(image.Rect(0, 0, n, n))
	// The slab: a black rounded bar.
	slab := image.Rect(150, 780, 874, 900)
	fillRound(out, slab, 40, color.NRGBA{0, 0, 0, 255})
	fillRound(out, image.Rect(190, 830, 520, 850), 10, color.NRGBA{255, 255, 255, 255})
	// The tile, scaled to 80% (the source has a 100 px transparent margin).
	small := scale(tile, 820)
	off := (n - 820) / 2
	draw.Draw(out, image.Rect(off, off-110, off+820, off-110+820), small, image.Point{}, draw.Over)
	return out
}

func fillRound(dst *image.NRGBA, r image.Rectangle, rad int, c color.NRGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cx := min(max(x, r.Min.X+rad), r.Max.X-1-rad)
			cy := min(max(y, r.Min.Y+rad), r.Max.Y-1-rad)
			if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= rad*rad {
				dst.SetNRGBA(x, y, c)
			}
		}
	}
}

// scale resamples img to size x size by averaging (premultiplied), which is
// exact for the downscales used here and deterministic.
func scale(img image.Image, size int) *image.NRGBA {
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	sx, sy := float64(b.Dx())/float64(size), float64(b.Dy())/float64(size)
	for y := 0; y < size; y++ {
		y0, y1 := float64(y)*sy, float64(y+1)*sy
		for x := 0; x < size; x++ {
			x0, x1 := float64(x)*sx, float64(x+1)*sx
			var r, g, bl, a, wsum float64
			for yy := int(y0); float64(yy) < y1 && yy < b.Dy(); yy++ {
				wy := min(float64(yy+1), y1) - max(float64(yy), y0)
				for xx := int(x0); float64(xx) < x1 && xx < b.Dx(); xx++ {
					w := wy * (min(float64(xx+1), x1) - max(float64(xx), x0))
					pr, pg, pb, pa := img.At(b.Min.X+xx, b.Min.Y+yy).RGBA()
					r += float64(pr) * w
					g += float64(pg) * w
					bl += float64(pb) * w
					a += float64(pa) * w
					wsum += w
				}
			}
			if wsum == 0 || a == 0 {
				continue
			}
			// r,g,bl,a are premultiplied 16-bit sums.
			out.SetNRGBA(x, y, color.NRGBA{
				uint8(min(r/a*255+0.5, 255)), uint8(min(g/a*255+0.5, 255)), uint8(min(bl/a*255+0.5, 255)),
				uint8(min(a/wsum/257+0.5, 255)),
			})
		}
	}
	return out
}

// icns builds a macOS icon file with PNG images at the standard sizes from a
// 1024 px source.
func icns(src image.Image) ([]byte, error) {
	types := []struct {
		typ  string
		size int
	}{{"ic11", 32}, {"ic12", 64}, {"ic07", 128}, {"ic08", 256}, {"ic13", 256}, {"ic09", 512}, {"ic14", 512}, {"ic10", 1024}}
	var body bytes.Buffer
	for _, t := range types {
		var img image.Image = src
		if src.Bounds().Dx() != t.size {
			img = scale(src, t.size)
		}
		var p bytes.Buffer
		if err := png.Encode(&p, img); err != nil {
			return nil, err
		}
		body.WriteString(t.typ)
		_ = binary.Write(&body, binary.BigEndian, uint32(p.Len()+8))
		body.Write(p.Bytes())
	}
	var out bytes.Buffer
	out.WriteString("icns")
	_ = binary.Write(&out, binary.BigEndian, uint32(body.Len()+8))
	out.Write(body.Bytes())
	return out.Bytes(), nil
}
