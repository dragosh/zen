// Command iconmask makes everything outside the macOS icon tile (the rounded
// rectangle x,y 100..924, radius 185 of build/appicon.svg on a 1024 canvas)
// transparent. QuickLook renders SVG onto an opaque background; this restores
// the transparent margin the .icns fallback icon needs.
//
//	go run ./build/darwin/iconmask in.png out.png
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

const (
	lo, hi, radius = 100.0, 924.0, 185.0
	canvas         = 1024.0
)

// coverage returns the fraction (0..1) of the pixel centred at x,y inside the tile.
func coverage(x, y float64) float64 {
	cx := math.Max(lo+radius, math.Min(x, hi-radius))
	cy := math.Max(lo+radius, math.Min(y, hi-radius))
	d := math.Hypot(x-cx, y-cy) - radius // signed distance to the rounded rect
	return math.Max(0, math.Min(1, 0.5-d))
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: iconmask in.png out.png")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintf(os.Stderr, "iconmask: %v\n", err)
		os.Exit(1)
	}
}

func run(in, out string) error {
	f, err := os.Open(in)
	if err != nil {
		return err
	}
	src, err := png.Decode(f)
	f.Close()
	if err != nil {
		return err
	}
	b := src.Bounds()
	scale := canvas / float64(b.Dx())
	dst := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			a := coverage((float64(x-b.Min.X)+0.5)*scale, (float64(y-b.Min.Y)+0.5)*scale)
			c.A = uint8(math.Round(float64(c.A) * a))
			dst.SetNRGBA(x, y, c)
		}
	}
	w, err := os.Create(out)
	if err != nil {
		return err
	}
	if err := png.Encode(w, dst); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}
