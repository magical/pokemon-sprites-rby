package nitro

import (
	"image"
	"image/color"
	"image/draw"
	"testing"
)

func checkerboard(pal color.Palette) *image.Paletted {
	p := image.NewPaletted(image.Rect(0, 0, 16, 16), pal)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			p.SetColorIndex(x, y, uint8((x+y)%2))
		}
	}
	return p
}

func uniform(index uint8, pal color.Palette) *image.Paletted {
	p := image.NewPaletted(image.Rect(0, 0, 16, 16), pal)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			p.SetColorIndex(x, y, index)
		}
	}
	return p
}

func TestDrawPalettedUnder(t *testing.T) {
	// Test transparency
	pal := color.Palette{
		color.Transparent,
		color.Black,
		color.White,
	}
	r := image.Rect(0, 0, 16, 16)
	dst := checkerboard(pal)
	src := uniform(2, pal)
	rgb := image.NewRGBA(r)
	draw.Draw(rgb, r, src, r.Min, draw.Src)
	draw.Draw(rgb, r, dst, r.Min, draw.Over)
	drawPalettedUnder(dst, r, src, r.Min)
	for x := 0; x < 100; x++ {
		for y := 0; y < 100; y++ {
			if !equal(dst.At(x, y), rgb.At(x, y)) {
				t.Errorf("color at %+v, %+v: got %v, expected %v", x, y, dst.At(x, y), rgb.At(x, y))
			}
		}
	}
}

func diag() image.Image {
	m := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
	for x := 0; x < 16; x++ {
		m.SetRGBA(x, y, color.RGBA{uint8(((x+y)*255 + 15 )/30), 0, 0, 255})
	}
	}
	return m
}

func TestRotate(t *testing.T) {
	// Rotating by 0 should be a no-op.
	dst := image.NewRGBA(image.Rect(0, 0, 16, 16))
	src := diag()
	rotate(dst, dst.Bounds(), image.ZP, src, image.ZP, 1, 1, 0)
	for y := 0; y < 16; y++ {
	for x := 0; x < 16; x++ {
		if !equal(dst.At(x, y), src.At(x,y)) {
			t.Errorf("color at (%v,%v): got %v, want %v", x, y, dst.At(x, y), src.At(x, y))
		}
	}
	}

	// Rotating by 45°
	dst = image.NewRGBA(image.Rect(0, 0, 16, 16))
	rotate(dst, dst.Bounds(), image.Pt(15, 0), src, image.ZP, 1, 1, .25)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if !equal(dst.At(x, y), src.At(y, 15-x)) {
				t.Errorf("color at (%v,%v): got %v, want %v", x, y ,dst.At(x, y), src.At(y, 15-x))
			}
		}
	}
}

func equal(c0, c1 color.Color) bool {
	r0, g0, b0, a0 := c0.RGBA()
	r1, g1, b1, a1 := c1.RGBA()
	return r0 == r1 && g0 == g1 && b0 == b1 && a0 == a1
}
