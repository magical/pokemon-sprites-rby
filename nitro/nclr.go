package nitro

import (
	"encoding/binary"
	"image/color"
	"io"
)

// An NCLR (nitro color resource) defines a color palette.
type NCLR struct {
	header Header
	pltt   _PLTT

	Colors []RGB15
}

type _PLTT struct {
	Magic      [4]byte
	Size       uint32
	BitDepth   uint32
	_          uint32
	DataSize   uint32
	DataOffset uint32
}

func ReadNCLR(r io.Reader) (*NCLR, error) {
	nclr := new(NCLR)

	err := binary.Read(r, binary.LittleEndian, &nclr.header)
	if err != nil {
		return nil, err
	}
	if string(nclr.header.Magic[:]) != "RLCN" {
		return nil, errBadMagic
	}

	err = binary.Read(r, binary.LittleEndian, &nclr.pltt)
	if err != nil {
		return nil, err
	}
	if string(nclr.pltt.Magic[:]) != "TTLP" {
		return nil, errInvalidChunk
	}

	n := int(nclr.pltt.Size) - int(binary.Size(&nclr.pltt))
	n /= 2
	nclr.Colors = make([]RGB15, n)
	err = binary.Read(r, binary.LittleEndian, &nclr.Colors)
	if err != nil {
		return nil, err
	}

	return nclr, nil
}

func (nclr *NCLR) Palette(n int) color.Palette {
	pal := make(color.Palette, 16)
	for i, c := range nclr.Colors[n*16 : n*16+16] {
		pal[i] = c
	}
	return pal
}

type RGB15 uint16

func (rgb RGB15) RGBA() (r, g, b, a uint32) {
	r = (uint32(rgb>>0&31)*0xFFFF + 15) / 31
	g = (uint32(rgb>>5&31)*0xFFFF + 15) / 31
	b = (uint32(rgb>>10&31)*0xFFFF + 15) / 31
	a = 0xFFFF
	return
}

func (rgb RGB15) String() string {
	const hex = "0123456789ABCDEF"
	var s [7]byte
	s[0] = '#'
	s[1] = hex[rgb>>12&0xF]
	s[2] = hex[rgb>>8&0xF]
	s[3] = hex[rgb>>4&0xF]
	s[4] = hex[rgb&0xF]
	return string(s[:])
}
