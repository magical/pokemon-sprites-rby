package nitro

import (
	//"fmt"
	"image"
	"image/draw"
	//"image/png"
	"math"
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
*/

func centerOf(r image.Rectangle) (p image.Point) {
	r = r.Canon()
	p.X = r.Min.X + (r.Max.X-r.Min.X)/2
	p.Y = r.Min.Y + (r.Max.Y-r.Min.Y)/2
	return
}

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
func scale2x(dst draw.Image, dp image.Point, src image.Image, r image.Rectangle) {
	xlo, xhi := r.Min.X, r.Max.X
	ylo, yhi := r.Min.Y, r.Max.Y
	for dy, y := dp.Y, ylo; y < yhi; dy, y = dy+2, y+1 {
		for dx, x := dp.X, xlo; x < xhi; dx, x = dx+2, x+1 {
			// Source pixels
			c := src.At(x, y)
			t, l, r, b := c, c, c, c
			if y-1 >= ylo {
				t = src.At(x, y-1)
			}
			if y+1 < yhi {
				b = src.At(x, y+1)
			}
			if x-1 >= xlo {
				l = src.At(x-1, y)
			}
			if x+1 < xhi {
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

// Rotate draws a image rotated clockwise around the point cp by deg degrees
// and scaled by 1/scale. The point sp gives the corresponding center point in
// the source image.
func rotate(dst draw.Image, r image.Rectangle, cp image.Point, src image.Image, sp image.Point, scaleX, scaleY, deg float64) {
	if dstp, ok := dst.(*image.Paletted); ok {
		if srcp, ok := src.(*image.Paletted); ok {
			rotatePaletted(dstp, r, cp, srcp, sp, scaleX, scaleY, deg)
			return
		}
	}
	//panic("wrong one")
	sin := -math.Sin(deg*(math.Pi/180))
	cos := math.Cos(deg*(math.Pi/180))
	xlo, xhi := r.Min.X, r.Max.X
	ylo, yhi := r.Min.Y, r.Max.Y
	sr := src.Bounds()
	for y := ylo; y < yhi; y++ {
		for x := xlo; x < xhi; x++ {
			if _, _, _, a := dst.At(x, y).RGBA(); a != 0 {
				continue
			}
			sx := sp.X + int((float64(x-cp.X)*cos-float64(y-cp.Y)*sin)*scaleX)
			sy := sp.Y + int((float64(x-cp.X)*sin+float64(y-cp.Y)*cos)*scaleY)
			if !image.Pt(sx, sy).In(sr) {
				continue
			}
			dst.Set(x, y, src.At(sx, sy))
		}
	}
}

func rotatePaletted(dst *image.Paletted, r image.Rectangle, cp image.Point, src *image.Paletted, sp image.Point, scaleX, scaleY, deg float64) {
	sin := -math.Sin(deg*(math.Pi/180))
	cos := math.Cos(deg*(math.Pi/180))
	sr := src.Bounds()
	xoff := 0
	yoff := 0
	if scaleX != 1 && scaleY != 1 {
		xoff = 4
		yoff = 4
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if dst.ColorIndexAt(x, y) != 0 {
				continue
			}
			sx := sp.X + int((float64(x-cp.X)*cos-float64(y-cp.Y)*sin)*scaleX) + xoff
			sy := sp.Y + int((float64(x-cp.X)*sin+float64(y-cp.Y)*cos)*scaleY) + yoff
			if !image.Pt(sx, sy).In(sr) {
				continue
			}
			si := src.ColorIndexAt(sx, sy)
			if si == 0 {
				continue
			}
			//fmt.Fprintln(os.Stderr, x, y, sx, sy)
			dst.SetColorIndex(x, y, si)
		}
	}
}
