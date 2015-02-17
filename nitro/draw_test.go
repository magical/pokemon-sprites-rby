package nitro

import (
	"image"
	"image/color"
	"image/draw"
	"math/rand"
	"testing"
)

func TestDrawPalettedUnder(t *testing.T) {
	// Test transparency
	pal := color.Palette{
		color.Transparent,
		color.White,
		color.Black,
	}
	r := image.Rect(0, 0, 100, 100)
	rgb := image.NewRGBA(r)
	dst := image.NewPaletted(r, pal)
	src := image.NewPaletted(r, pal)
	for i := range dst.Pix {
		dst.Pix[i] = uint8(rand.Intn(len(pal)))
	}
	for i := range src.Pix {
		src.Pix[i] = uint8(rand.Intn(len(pal)))
	}
	r1 := r
	//r1 := image.Rect(45, 45, 55, 55)
	draw.Draw(rgb, r1, src, r1.Min, draw.Src)
	draw.Draw(rgb, r, dst, r.Min, draw.Over)
	drawPalettedUnder(dst, r1, src, r1.Min)
	for x := 0; x < 100; x++ {
		for y := 0; y < 100; y++ {
			if !equal(dst.At(x, y), rgb.At(x, y)) {
				t.Errorf("color at %+v, %+v: got %v, expected %v", x, y, dst.At(x, y), rgb.At(x, y))
			}
		}
	}
}

func equal(c0, c1 color.Color) bool {
	r0, g0, b0, a0 := c0.RGBA()
	r1, g1, b1, a1 := c1.RGBA()
	return r0 == r1 && g0 == g1 && b0 == b1 && a0 == a1
}
