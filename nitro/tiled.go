package nitro

import (
	"image"
	"image/color"
)

// Tiled is a paletted image stored as a sequence of 8x8 tiles.
// Since it is conceptually one-dimensional, its bounds may be undefined.
type Tiled struct {
	Pix     []uint8
	Stride  int // number of tiles per row
	Rect    image.Rectangle
	Palette color.Palette
}

func (t *Tiled) ColorModel() color.Model { return t.Palette }
func (t *Tiled) Bounds() image.Rectangle { return t.Rect }

// Get a subimage with the specified width and height starting at the given tile offset.
func (t *Tiled) Tile(offset, width, height int) *Tiled {
	if offset*64 >= len(t.Pix) {
		return &Tiled{
			Palette: t.Palette,
		}
	}
	r := image.Rect(0, 0, width, height)
	stride := (width + 7) / 8
	return &Tiled{
		Pix:     t.Pix[offset*64:],
		Rect:    r,
		Stride:  stride,
		Palette: t.Palette,
	}
}

// PixOffset returns the index Pix that corresponds to the pixel at (x, y).
func (t *Tiled) PixOffset(x, y int) int {
	x, y = x-t.Rect.Min.X, y-t.Rect.Min.Y
	tx, ty := x%8, y%8
	i := ((y-ty)*t.Stride+(x-tx))*8 + ty*8 + tx
	return i
}

func (t *Tiled) ColorIndexAt(x, y int) uint8 {
	if !image.Pt(x, y).In(t.Rect) {
		return 0
	}
	i := t.PixOffset(x, y)
	if i < 0 {
		panic("out of bounds")
	}
	if i >= len(t.Pix) {
		return 0
	}
	return t.Pix[i]
}

func (t *Tiled) SetColorIndexAt(x, y int, index uint8) {
	if !image.Pt(x, y).In(t.Rect) {
		return
	}
	i := t.PixOffset(x, y)
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
