// +build ignore

package main

import (
	"bufio"
	"errors"
	//"fmt"
	"flag"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"strconv"
)

var errMalformed = errors.New("malformed data")

// decode lzss10
func decode10(r io.ByteReader) ([]byte, error) {
	var err error
	var nextbyte = func() (b byte) {
		if err == nil {
			b, err = r.ReadByte()
		}
		return
	}
	magic := nextbyte()
	if magic != 0x10 {
		return nil, errMalformed
	}
	size := int(nextbyte()) +
		int(nextbyte())<<8 +
		int(nextbyte())<<16
	if err != nil {
		return nil, err
	}
	data := make([]byte, 0, size)
	for len(data) < size && err == nil {
		bits := nextbyte()
		for i := 0; i < 8 && len(data) < size; i, bits = i+1, bits<<1 {
			if bits&0x80 == 0 {
				data = append(data, nextbyte())
				continue
			}
			n := int(nextbyte())<<8 + int(nextbyte())
			count := n>>12 + 3
			disp := n&0xFFF + 1
			if disp > len(data) {
				return nil, errMalformed
			}
			for j := 0; j < count; j++ {
				data = append(data, data[len(data)-disp])
			}
		}
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Sprite pointers
//   Pointer  uint32
//   Size     uint16
//   Number   uint16
//
// Palette pointers
//   Pointer uint32
//   Number  uint16
//   _       uint16
//

// 0x1FC1E0 Internal ID => Hoenn dex number
// 0x1FC516 Internal ID => National dex number
// 0x1FC84C Hoenn dex number => National dex number

var info = struct {
	Code               string
	SpriteOffset       int64
	BackSpriteOffset   int64
	PaletteOffset      int64
	ShinyPaletteOffset int64
	NationalDexOffset int64
}{
	Code:               "AXVE",
	SpriteOffset:       0x1E8354,
	BackSpriteOffset:   0x1E97F4,
	PaletteOffset:      0x1EA5B4,
	ShinyPaletteOffset: 0x1EB374,
	NationalDexOffset: 0x1FC516,
}

func readPointerAt(r io.ReaderAt, base int64, n int) int64 {
	var b [4]byte
	_, err := r.ReadAt(b[:], base+int64(n)*4)
	if err != nil {
		panic(err)
	}
	return int64(b[0]) + int64(b[1])<<8 + int64(b[2])<<16 + int64(b[3]&^0x08)<<24
}

type RGBA16 uint16

func (rgb RGBA16) RGBA() (r, g, b, a uint32) {
	r = (uint32(rgb>>0&31)*0xFFFF + 15) / 31
	g = (uint32(rgb>>5&31)*0xFFFF + 15) / 31
	b = (uint32(rgb>>10&31)*0xFFFF + 15) / 31
	a = uint32(^rgb>>15) * 0xFFFF
	return
}

func untile(m *image.Paletted, data []byte, w, h int) {
	for i, y := 0, 0; y < h; y += 8 {
		for x := 0; x < w; x += 8 {
			for ty := 0; ty < 8; ty++ {
				for tx := 0; tx < 8; tx, i = tx+2, i+1 {
					di := m.PixOffset(x+tx, y+ty)
					m.Pix[di+0] = data[i] >> 0 & 0xF
					m.Pix[di+1] = data[i] >> 4 & 0xF
				}
			}
		}
	}
}

func Sprite(r io.ByteReader, pal color.Palette, w, h int) (*image.Paletted, error) {
	data, err := decode10(r)
	if err != nil {
		return nil, err
		panic(err)
	}
	if len(data) < w*h/2 {
		return nil, errors.New("data too short")
	}
	m := image.NewPaletted(image.Rect(0, 0, w, h), pal)
	untile(m, data, w, h)
	return m, nil
}

func Palette(r io.ByteReader) (color.Palette, error) {
	data, err := decode10(r)
	if err != nil {
		return nil, err
	}
	if len(data) < 16*2 {
		return nil, errors.New("palette data too short")
	}
	if len(data) > 16*2 {
		return nil, errors.New("palette data too long")
	}
	var pal = make(color.Palette, 16)
	for i := range pal {
		c := RGBA16(uint16(data[i*2]) + uint16(data[i*2+1])<<8)
		if i == 0 {
			c |= 0x8000
		}
		pal[i] = c
	}
	return pal, nil
}

func main() {
	flag.Parse()
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		panic(err)
	}
	number, err := strconv.Atoi(flag.Arg(1))
	if err != nil {
		number = 1
	}
	var idmap = make(map[int]int)
	f.Seek(info.NationalDexOffset, 0)
	for i := 1; i <= 440; i++ {
		var b [2]byte
		f.Read(b[:])
		n := int(b[0]) + int(b[1])<<8
		idmap[n] = i
	}

	var ok bool
	number, ok = idmap[number]
	if !ok {
		panic("bad number")
	}

	f.Seek(readPointerAt(f, info.PaletteOffset, number*2), 0)
	pal, err := Palette(bufio.NewReader(f))
	if err != nil {
		panic(err)
	}
	f.Seek(readPointerAt(f, info.SpriteOffset, number*2), 0)
	m, err := Sprite(bufio.NewReader(f), pal, 64, 64)
	if err != nil {
		panic(err)
	}
	png.Encode(os.Stdout, m)
}
