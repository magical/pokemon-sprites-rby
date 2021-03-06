package nitro

import "image"

// http://www.problemkaputt.de/gbatek.htm#lcdobjoamattributes

type _OBJ [3]uint16

func (obj *_OBJ) Y() int { return int(int8(obj[0])) }
func (obj *_OBJ) X() int { return int(int16(obj[1]) << 7 >> 7) }

func (obj *_OBJ) Shape() uint { return uint(obj[0] >> 14) }
func (obj *_OBJ) Size() uint  { return uint(obj[1] >> 14) }

// Shapes: square, long, tall
// Sizes: small, medium-small, medium-large, large

var sizes = [][]image.Point {
	{{8, 8}, {16, 16}, {32, 32}, {64, 64}},
	{{16, 8}, {32, 8}, {32, 16}, {64, 32}},
	{{8, 16}, {8, 32}, {16, 32}, {32, 64}},
}

// Dx returns the width of the source cell.
func (obj *_OBJ) Dx() int {
	if obj.Shape() == 3 {
		return 0
	}
	return sizes[obj.Shape()][obj.Size()].X
}

// Dy retuns the height of the source cell.
func (obj *_OBJ) Dy() int {
	if obj.Shape() ==3 {
		return 0
	}
	return sizes[obj.Shape()][obj.Size()].Y
}

// Bounds returns the destination rectangle.
func (obj *_OBJ) Bounds() image.Rectangle {
	shape := obj.Shape()
	if shape == 3 {
		return image.ZR
	}
	size := obj.Size()
	d := sizes[shape][size]
	if obj.Double() {
		d.X *= 2
		d.Y *= 2
	}
	x := obj.X()
	y := obj.Y()
	return image.Rect(x, y, x+d.X, y+d.Y)
}

// 0 - none or flip
// 1 - rotate/scale
// 2 - disable
// 3 - double size
func (obj *_OBJ) TransformMode() uint {
	return uint(obj[0] >> 8 & 3)
}

func (obj *_OBJ) Double() bool { return obj[0]>>8&3 == 3 }
func (obj *_OBJ) FlipX() bool  { return obj[0]>>8&1 == 0 && obj[1]>>12&1 == 1 }
func (obj *_OBJ) FlipY() bool  { return obj[0]>>8&1 == 0 && obj[1]>>13&1 == 1 }

func (obj *_OBJ) Tile() int      { return int(obj[2] << 6 >> 6) }
func (obj *_OBJ) Priority() uint { return uint(obj[2] << 4 >> 14) }
func (obj *_OBJ) Palette() uint  { return uint(obj[2] >> 12) }
