package nitro

import (
	//"fmt"
	"image"
	"image/draw"
	//"image/png"
	//"os"
)

// This file implements the scale2x/EPX algorithm for upscaling an image,
// as specified on http://scale2x.sourceforge.net/algorithm.html
// Unlike other sprite scaling algorithms (like hq2x), scale2x does no interpolation;
// every color comes from the original image.
// It works by replacing each pixel with a 2x2 grid of pixels,
// with colors selected from the source pixel and its immediate neighbors.

/*
func main() {
	m, err := png.Decode(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	//d := m.(draw.Image)
	m = scale8x(m)
	//d := image.NewNRGBA(m.Bounds())
	//rotate(d, d.Bounds(), centerOf(d.Bounds()), m, centerOf(m.Bounds()), 8, 8, 45)
	err = png.Encode(os.Stdout, m)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
}

func centerOf(r image.Rectangle) (p image.Point) {
	r = r.Canon()
	p.X = r.Min.X + (r.Max.X-r.Min.X)/2
	p.Y = r.Min.Y + (r.Max.Y-r.Min.Y)/2
	return
}
*/

// Scale8x applies scale2x thrice.
// If m is a *image.Paletted, so will be the return value;
// otherwise it will be a *image.NRGBA.
func scale8x(m image.Image) image.Image {
	r := m.Bounds()
	w, h := r.Dx(), r.Dy()
	r.Min = r.Min.Mul(8)
	r.Max = r.Max.Mul(8)
	var dst draw.Image
	switch m := m.(type) {
	case *image.Paletted:
		dst = image.NewPaletted(r, m.Palette)
	default:
		dst = image.NewNRGBA(r)
	}
	// Perform scale2x thrice in-place
	// Start in the lower right corner to avoid problems with overwriting
	scale2x(dst, r.Min.Add(image.Pt(w*6, h*6)), m, m.Bounds())
	scale2x(dst, r.Min.Add(image.Pt(w*4, h*4)), dst, image.Rect(r.Min.X+w*6, r.Min.Y+h*6, r.Max.X, r.Max.Y))
	scale2x(dst, r.Min, dst, image.Rect(r.Min.X+w*4, r.Min.Y+h*4, r.Max.X, r.Max.Y))
	return dst

	// Would like to do this instead, but scale2x needs to be smart
	// enough to render backwards.
	//scale2x(dst, image.ZP, m, m.Bounds())
	//scale2x(dst, image.ZP, dst, image.Rect(0, 0, w*2, h*2))
	//scale2x(dst, image.ZP, dst, image.Rect(0, 0, w*4, h*4))
}

// Scale2x scales up an image with the scale2x algorithm.
func scale2x(dst draw.Image, dp image.Point, src image.Image, rect image.Rectangle) {
	if dst, ok := dst.(*image.Paletted); ok {
		if src, ok := src.(*image.Paletted); ok {
			if samePalette(dst.Palette, src.Palette) {
				scale2xPaletted(dst, dp, src, rect)
				return
			}
		}
	}
	sp := rect.Min
	rect = rect.Intersect(src.Bounds())
	dp = dp.Add(rect.Min.Sub(sp).Mul(2)) // adjust dp if rect changed
	for dy, sy := dp.Y, rect.Min.Y; sy < rect.Max.Y; dy, sy = dy+2, sy+1 {
		for dx, sx := dp.X, rect.Min.X; sx < rect.Max.X; dx, sx = dx+2, sx+1 {
			// Source pixels
			c := src.At(sx, sy)
			t, l, r, b := c, c, c, c
			if sy-1 >= rect.Min.Y {
				t = src.At(sx, sy-1)
			}
			if sy+1 < rect.Max.Y {
				b = src.At(sx, sy+1)
			}
			if sx-1 >= rect.Min.X {
				l = src.At(sx-1, sy)
			}
			if sx+1 < rect.Max.X {
				r = src.At(sx+1, sy)
			}
			// Destination pixels
			tl, tr, bl, br := c, c, c, c
			if t != b && l != r {
				if t == l {
					tl = t
				}
				if t == r {
					tr = t
				}
				if b == l {
					bl = b
				}
				if b == r {
					br = b
				}
			}
			dst.Set(dx+0, dy+0, tl)
			dst.Set(dx+1, dy+0, tr)
			dst.Set(dx+0, dy+1, bl)
			dst.Set(dx+1, dy+1, br)
		}
	}
}

func scale2xPaletted(dst *image.Paletted, dp image.Point, src *image.Paletted, rect image.Rectangle) {
	sp := rect.Min
	rect = rect.Intersect(src.Bounds())
	dp = dp.Add(rect.Min.Sub(sp).Mul(2)) // adjust dp if rect changed
	for dy, sy := dp.Y, rect.Min.Y; sy < rect.Max.Y; dy, sy = dy+2, sy+1 {
		for dx, sx := dp.X, rect.Min.X; sx < rect.Max.X; dx, sx = dx+2, sx+1 {
			// Source pixels
			c := src.ColorIndexAt(sx, sy)
			t, l, r, b := c, c, c, c
			if sy-1 >= rect.Min.Y {
				t = src.ColorIndexAt(sx, sy-1)
			}
			if sy+1 < rect.Max.Y {
				b = src.ColorIndexAt(sx, sy+1)
			}
			if sx-1 >= rect.Min.X {
				l = src.ColorIndexAt(sx-1, sy)
			}
			if sx+1 < rect.Max.X {
				r = src.ColorIndexAt(sx+1, sy)
			}
			// Destination pixels
			tl, tr, bl, br := c, c, c, c
			if t != b && l != r {
				if t == l {
					tl = t
				}
				if t == r {
					tr = t
				}
				if b == l {
					bl = b
				}
				if b == r {
					br = b
				}
			}
			dst.SetColorIndex(dx+0, dy+0, tl)
			dst.SetColorIndex(dx+1, dy+0, tr)
			dst.SetColorIndex(dx+0, dy+1, bl)
			dst.SetColorIndex(dx+1, dy+1, br)
		}
	}
}
