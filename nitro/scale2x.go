package nitro

import (
	//"fmt"
	"image"
	"image/draw"
	//"image/png"
	//"os"
)

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
	//case *image.Paletted:
	//	dst = image.NewPaletted(r, m.Palette)
	default:
		_ = m
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
// TODO: Specialize for *image.Paletted and *Tiled.
func scale2x(dst draw.Image, dp image.Point, src image.Image, rect image.Rectangle) {
	rect = rect.Intersect(dst.Bounds())
	for dy, y := dp.Y, rect.Min.Y; y < rect.Max.Y; dy, y = dy+2, y+1 {
		for dx, x := dp.X, rect.Min.X; x < rect.Max.X; dx, x = dx+2, x+1 {
			// Source pixels
			c := src.At(x, y)
			t, l, r, b := c, c, c, c
			if y-1 >= rect.Min.Y {
				t = src.At(x, y-1)
			}
			if y+1 < rect.Max.Y {
				b = src.At(x, y+1)
			}
			if x-1 >= rect.Min.X {
				l = src.At(x-1, y)
			}
			if x+1 < rect.Max.X {
				r = src.At(x+1, y)
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
