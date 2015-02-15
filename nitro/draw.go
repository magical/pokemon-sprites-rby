package nitro

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

// DrawUnder aligns r.Min in dst with sp in src andsp in r and draws
func drawUnder(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point) {
	//draw.DrawMask(dst, r, src, sp, under{dst}, r.Min, draw.Over)
	rotate(dst, r, r.Min, src, sp, 1, 1, 0)
}

// Under represents an image.Image by the inverse of its alpha channel.
type under struct{ m image.Image }

func (u under) Bounds() image.Rectangle { return u.m.Bounds() }
func (u under) ColorModel() color.Model { return color.Alpha16Model }
func (u under) At(x, y int) color.Color {
	_, _, _, a := u.m.At(x, y).RGBA()
	return color.Alpha16{uint16(0xffff - a)}
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
	sin := -math.Sin(deg * (2*math.Pi))
	cos := math.Cos(deg * (2*math.Pi))
	sr := src.Bounds()
	r = r.Intersect(dst.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
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
	sin := -math.Sin(deg * (2*math.Pi))
	cos := math.Cos(deg * (2*math.Pi))
	sr := src.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if dst.ColorIndexAt(x, y) != 0 {
				continue
			}
			sx := sp.X + int((float64(x-cp.X)*cos-float64(y-cp.Y)*sin)*scaleX)
			sy := sp.Y + int((float64(x-cp.X)*sin+float64(y-cp.Y)*cos)*scaleY)
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
