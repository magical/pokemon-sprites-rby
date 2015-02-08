package nitro

import (
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"io"
)

// An NCER (nitro cell resource) describes a bank of cells.
// Cells define subregions of an NCGR.
type NCER struct {
	header Header
	cebk   _CEBK

	cells []celldata
	objs  []_OBJ
}

// Cell bank
type _CEBK struct {
	CellCount  uint16
	CellType   uint16
	CellOffset uint32
	Flags      uint32
	_          uint32 // "partition data" offset
	_          uint32
	_          uint32
}

type celldata struct {
	OBJCount  uint16
	_         uint16
	OBJOffset uint32
}

type celldata_ex struct {
	C celldata
	_ [4]uint16
}

func ReadNCER(r io.Reader) (*NCER, error) {
	var ncer = new(NCER)
	err := readNitroHeader(r, "RECN", &ncer.header)
	if err != nil {
		return nil, err
	}
	if ncer.header.ChunkCount != 1 && ncer.header.ChunkCount != 3 {
		return nil, errors.New("NCER: too many chunks")
	}
	chunk, err := readChunk(r, "KBEC", &ncer.cebk)
	if err != nil {
		return nil, err
	}
	ncer.cells = make([]celldata, ncer.cebk.CellCount)
	switch ncer.cebk.CellType {
	case 0:
		err = binary.Read(chunk, le, ncer.cells)
		if err != nil {
			return nil, err
		}
	case 1:
		var cells_ex = make([]celldata_ex, ncer.cebk.CellCount)
		err = binary.Read(chunk, le, cells_ex)
		if err != nil {
			return nil, err
		}
		for i, cx := range cells_ex {
			ncer.cells[i] = cx.C
		}
	default:
		return nil, errors.New("NCER: unknown cell type")
	}

	if int(ncer.cebk.CellOffset) != 0x18 {
		return nil, errors.New("NCER: bad cell data offset")
	}

	nobjs := 0
	for _, c := range ncer.cells {
		nobjs += int(c.OBJCount)
		if c.OBJOffset%6 != 0 {
			return nil, errors.New("NCER: unaligned obj")
		}
	}
	ncer.objs = make([]_OBJ, nobjs)
	err = binary.Read(chunk, le, &ncer.objs)
	if err != nil {
		return nil, err
	}

	return ncer, nil
}

// Len returns the number of cells in the cell bank.
func (ncer *NCER) Len() int {
	return int(ncer.cebk.CellCount)
}

// Cell returns the image data for a cell.
// It panics if the cell index is out of bounds.
func (ncer *NCER) Cell(i int, ncgr *NCGR, pal color.Palette) *image.Paletted {
	if i < 0 || i >= int(ncer.cebk.CellCount) {
		panic("NCER: no such cell")
	}
	c := ncer.cells[i]
	objs := ncer.objs[c.OBJOffset/6:][:c.OBJCount]
	r := image.ZR
	for _, obj := range objs {
		r = r.Union(obj.Bounds())
	}
	m := image.NewPaletted(r, pal)
	for _, obj := range objs {
		drawUnder(m, obj.bounds(), ncgr.Tile(obj.Tile(), obj.Dx(), obj.Dy(), pal), image.ZP)
	}
	return m
}

func drawUnder(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point) {
	//draw.DrawMask(dst, r, src, sp, under{dst}, r.Min, draw.Over)
	rotate(dst, r, r.Min, src, sp, 1, 1, 0)
}

type under struct{ m image.Image }

func (u under) Bounds() image.Rectangle { return u.m.Bounds() }
func (u under) ColorModel() color.Model { return color.Alpha16Model }
func (u under) At(x, y int) color.Color {
	_, _, _, a := u.m.At(x, y).RGBA()
	return color.Alpha16{uint16(0xffff - a)}
}

// bounds returns the bounds of an object without doubling.
func (obj _OBJ) bounds() image.Rectangle {
	dx := obj.Dx()
	dy := obj.Dy()
	x := obj.X()
	y := obj.Y()
	if obj.Double() {
		x += dx/2
		y += dy/2
	}
	return image.Rect(x, y, x+dx, y+dy)
}
