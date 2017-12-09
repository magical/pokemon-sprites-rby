package nitro

import (
	"image"
	"image/color"
)

// Tiled is an image.Image whose pixels are stored as a sequence of 8x8 tiles.
// Since it is conceptually one-dimensional, its bounds may be undefined.
type Tiled struct {
	Pix     []uint8
	Stride  int // number of tiles per row
	Rect    image.Rectangle
	Palette color.Palette
}

func (t *Tiled) ColorModel() color.Model { return t.Palette }
func (t *Tiled) Bounds() image.Rectangle { return t.Rect }

// Tile returns an image representing a portion of t.
// The upper left tile of the returned image will be the nth tile in t,
// and the tiles following the nth tile will fill the remaining width and
// height of the returned image from left to right, top to bottom.
// The returned value shares pixels with the original image.
func (t *Tiled) Tile(n, width, height int) *Tiled {
	if n*64 >= len(t.Pix) {
		return &Tiled{
			Palette: t.Palette,
		}
	}
	r := image.Rect(0, 0, width, height)
	stride := (width + 7) / 8
	return &Tiled{
		Pix:     t.Pix[n*64:],
		Rect:    r,
		Stride:  stride,
		Palette: t.Palette,
	}
}

// PixOffset returns the index Pix that corresponds to the pixel at (x, y).
func (t *Tiled) PixOffset(x, y int) int {
	// TODO: try to get this under the inlining limit
	x, y = x-t.Rect.Min.X, y-t.Rect.Min.Y
	return (y/8*t.Stride+x/8)*64 + y%8*8 + x%8
}

func (t *Tiled) ColorIndexAt(x, y int) uint8 {
	if !image.Pt(x, y).In(t.Rect) {
		return 0
	}
	i := t.PixOffset(x, y)
	if i >= len(t.Pix) {
		return 0
	}
	return t.Pix[i]
}

func (t *Tiled) SetColorIndex(x, y int, index uint8) {
	if !image.Pt(x, y).In(t.Rect) {
		return
	}
	i := t.PixOffset(x, y)
	if i >= len(t.Pix) {
		return
	}
	t.Pix[i] = index
}

func (t *Tiled) At(x, y int) color.Color {
	if len(t.Palette) == 0 {
		return nil
	}
	if !image.Pt(x, y).In(t.Rect) {
		return t.Palette[0]
	}
	i := t.PixOffset(x, y)
	if i >= len(t.Pix) {
		return t.Palette[0]
	}
	return t.Palette[t.Pix[i]]
}

func (t *Tiled) Set(x, y int, c color.Color) {
	if !image.Pt(x, y).In(t.Rect) {
		return
	}
	i := t.PixOffset(x, y)
	if i >= len(t.Pix) {
		return
	}
	t.Pix[i] = uint8(t.Palette.Index(c))
}

func NewTiled(r image.Rectangle, pal color.Palette) *Tiled {
	return &Tiled{
		Pix:     make([]uint8, r.Dx()*r.Dy()),
		Rect:    r,
		Stride:  r.Dx() / 8,
		Palette: pal,
	}
}
