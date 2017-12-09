package nitro

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

// BUG: these drawing routines may be incorrect when src and dst overlap

// DrawUnder aligns r.Min in dst with sp in src and replaces r with the result of (src over dst).
func drawUnder(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point) {
	if dst, ok := dst.(*image.Paletted); ok {
		if src, ok := src.(image.PalettedImage); ok {
			if samePalette(dst.Palette, src.ColorModel().(color.Palette)) {
				drawPalettedUnder(dst, r, src, sp)
			}
		}
	}

	// Slow, and broken for paletted images
	//draw.DrawMask(dst, r, src, sp, under{dst}, r.Min, draw.Over)

	// XXX Is drawGenericUnder actually faster? Does it matter?
	//rotate(dst, r, r.Min, src, sp, 1, 1, 0)
	drawGenericUnder(dst, r, src, sp)
}

func drawGenericUnder(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point) {
	dp := r.Min
	r = clip(r, dst.Bounds(), dp, src.Bounds(), sp)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			dr, dg, db, da := dst.At(x, y).RGBA()
			if da == 0xFFFF {
				continue
			}
			sx := x + sp.X - dp.X
			sy := y + sp.Y - dp.Y
			if da == 0 {
				dst.Set(x, y, src.At(sx, sy))
				continue
			}
			a := 0xFFFF - da
			sr, sg, sb, sa := src.At(sx, sy).RGBA()
			dst.Set(x, y, color.RGBA64{
				uint16(dr + sr*a/0xFFFF),
				uint16(dg + sg*a/0xFFFF),
				uint16(db + sb*a/0xFFFF),
				uint16(da + sa*a/0xFFFF),
			})
		}
	}
}

// n.b. assumes that index 0 is fully transparent and all other colors are fully opaque
func drawPalettedUnder(dst *image.Paletted, r image.Rectangle, src image.PalettedImage, sp image.Point) {
	dp := r.Min
	r = clip(r, dst.Bounds(), dp, src.Bounds(), sp)
	sr := r.Add(sp.Sub(dp))
	d0 := dst.PixOffset(r.Min.X, r.Min.Y)
	for y := sr.Min.Y; y < sr.Max.Y; y++ {
		for i, x := 0, sr.Min.X; x < sr.Max.X; i, x = i+1, x+1 {
			if dst.Pix[d0+i] != 0 {
				continue
			}
			dst.Pix[d0+i] = src.ColorIndexAt(x, y)
		}
		d0 += dst.Stride
	}
}

// Clip clips the rectangle r to the src and dst rectangles.
func clip(r, dst image.Rectangle, dp image.Point, src image.Rectangle, sp image.Point) image.Rectangle {
	r = r.Intersect(dst)
	r = r.Intersect(src.Add(dp.Sub(sp)))
	return r
}

// SamePalette reports whether p and q have the same length and share the same backing array.
func samePalette(p, q color.Palette) bool {
	return len(p) == len(q) && &p[0] == &q[0]
}

// Under represents an image.Image by the inverse of its alpha channel.
type under struct{ m image.Image }

func (u under) Bounds() image.Rectangle { return u.m.Bounds() }
func (u under) ColorModel() color.Model { return color.Alpha16Model }
func (u under) At(x, y int) color.Color {
	_, _, _, a := u.m.At(x, y).RGBA()
	return color.Alpha16{uint16(0xffff - a)}
}

// Rotate draws a image rotated clockwise around the point dp by deg degrees
// and scaled by 1/scale. The point sp gives the corresponding center point in
// the source image.
func rotate(dst draw.Image, r image.Rectangle, dp image.Point, src image.Image, sp image.Point, scaleX, scaleY, deg float64) {
	if dstp, ok := dst.(*image.Paletted); ok {
		if srcp, ok := src.(*image.Paletted); ok {
			rotatePaletted(dstp, r, dp, srcp, sp, scaleX, scaleY, deg)
			return
		}
	}
	sin := -round(math.Sin(deg * (2 * math.Pi)))
	cos := round(math.Cos(deg * (2 * math.Pi)))
	sr := src.Bounds()
	r = r.Intersect(dst.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if _, _, _, a := dst.At(x, y).RGBA(); a != 0 {
				continue
			}
			sx := sp.X + int((float64(x-dp.X)*cos-float64(y-dp.Y)*sin)*scaleX)
			sy := sp.Y + int((float64(x-dp.X)*sin+float64(y-dp.Y)*cos)*scaleY)
			if !image.Pt(sx, sy).In(sr) {
				continue
			}
			dst.Set(x, y, src.At(sx, sy))
		}
	}
}

func rotatePaletted(dst *image.Paletted, r image.Rectangle, dp image.Point, src *image.Paletted, sp image.Point, scaleX, scaleY, deg float64) {
	sin := -round(math.Sin(deg * (2 * math.Pi)))
	cos := round(math.Cos(deg * (2 * math.Pi)))
	sr := src.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if dst.ColorIndexAt(x, y) != 0 {
				continue
			}
			sx := sp.X + int((float64(x-dp.X)*cos-float64(y-dp.Y)*sin)*scaleX)
			sy := sp.Y + int((float64(x-dp.X)*sin+float64(y-dp.Y)*cos)*scaleY)
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

func round(x float64) float64 {
	return math.Trunc(x*4096 + 0.5)/4096
}
