// +build ignore

package main

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
)

func main() {
	m, err := png.Decode(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	//d := m.(draw.Image)
	m = scale8x(m)
	//d := image.NewNRGBA(m.Bounds())
	//rotate(d, d.Bounds(), centerOf(d.Bounds()), m, centerOf(m.Bounds()), 8, 45)
	err = png.Encode(os.Stdout, m)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
}

func centerOf(r image.Rectangle) (p image.Point) {
	r = r.Canon()
	p.X = r.Min.X + (r.Max.X - r.Min.X)/2
	p.Y = r.Min.Y + (r.Max.Y - r.Min.Y)/2
	return
}

// Scale8x applies scale2x thrice.
func scale8x(m image.Image) image.Image {
	w, h := m.Bounds().Dx(), m.Bounds().Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w*8, h*8))
	scale2x(dst, image.Pt(w*6, h*6), m, m.Bounds())
	scale2x(dst, image.Pt(w*4, h*4), dst, image.Rect(w*6, h*6, w*8, h*8))
	scale2x(dst, image.Pt(w*0, h*0), dst, image.Rect(w*4, h*4, w*8, h*8))
	return dst

	// Would like to do this instead, but scale2x needs to be smart
	// enough to render backwards.
	//scale2x(dst, image.ZP, m, m.Bounds())
	//scale2x(dst, image.ZP, dst, image.Rect(0, 0, w*2, h*2))
	//scale2x(dst, image.ZP, dst, image.Rect(0, 0, w*4, h*4))
}

// Scale2x scales up an image with the scale2x algorithm.
func scale2x(dst draw.Image, dp image.Point, src image.Image, r image.Rectangle) {
	xlo, xhi := r.Min.X, r.Max.X
	ylo, yhi := r.Min.Y, r.Max.Y
	for dy, y := dp.Y, ylo; y < yhi; dy, y = dy+2, y+1 {
		for dx, x := dp.X, xlo; x < xhi; dx, x = dx+2, x+1 {
			// Source pixels
			c := src.At(x, y)
			t, l, r, b := c, c, c, c
			if y - 1 >= ylo {
				t = src.At(x, y-1)
			}
			if y + 1 < yhi {
				b = src.At(x, y+1)
			}
			if x - 1 >= xlo {
				l = src.At(x-1, y)
			}
			if x + 1 < xhi {
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
func rotate(dst draw.Image, r image.Rectangle, cp image.Point, src image.Image, sp image.Point, scale, deg float64) {
	sin := -math.Sin(deg * (math.Pi/180)) * scale
	cos := math.Cos(deg * (math.Pi/180)) * scale
	xlo, xhi := r.Min.X, r.Max.X
	ylo, yhi := r.Min.Y, r.Max.Y
	for y := ylo; y < yhi; y++ {
		for x := xlo; x < xhi; x++ {
			sx := sp.X + int(float64(x-cp.X)*cos - float64(y-cp.Y)*sin)
			sy := sp.Y + int(float64(x-cp.X)*sin + float64(y-cp.Y)*cos)
			dst.Set(x, y, src.At(sx, sy))
		}
	}
}