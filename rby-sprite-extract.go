package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"

	"image"
	"image/color"
	"image/png"
)

/*

Okay, so. Let's start at the beginning. The gameboy, like its successors,
works with 8x8 pixel tiles. Tiles are stored in rows of pixels, 2 bits per
pixel, 2 bytes per row. Strangely, the low and high bits of each row are
divided between the two bytes: the first byte stores the low bits and the
second the high bits. The high-endian bit is the first pixel.

The compression scheme used for pokemon images starts by further splitting the
low and high bits into two completely separate images. These halves are
eventually stored with zeros run-length encoded, so the compression methods
are aimed at getting many consecutive zeros.

The first option is to xor one of the halves with the other. Since the high
bits and low bits are likely to be correlated, this can wipe out a lot of
redundant bits.

The second option is to exploit row-level redundancy in either or both of the
halves by xoring each pixel with the previous one. (Remember that at this
point, each pixel is a single bit).

The halves are stored separately, and they are stored they are stored with
rows interleaved; two bits from the first row, two bits from the second row,
and so on, in effect almost transposing the image. This seems pointless.

Note: The way the game does the decompression, it ends up with an image whose
tiles have been transposed. This is unnecessary and, in fact, makes the job
harder. It is easier not to mess around with tiles at all.

*/

type bitReader struct {
	r     io.ByteReader
	bits  uint16
	count uint
	err   error
}

func (br *bitReader) ReadBits(n uint) uint8 {
	for br.count < n {
		b, err := br.r.ReadByte()
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		if err != nil {
			br.err = err
			return 0
		}
		br.bits <<= 8
		br.bits |= uint16(b)
		br.count += 8
	}

	shift := br.count - n
	mask := uint16(1<<n - 1)
	b := uint8((br.bits >> shift) & mask)
	br.count -= n
	return b
}

func (br *bitReader) ReadBits16(n uint) uint16 {
	b := uint16(br.ReadBits(n))
	if n > 8 {
		b = b<<8 | uint16(br.ReadBits(n-8))
	}
	return b
}

func (br *bitReader) Err() error {
	return br.err
}

// Decode reads a compressed pokemon image and returns it as an
// image.Paletted.
func Decode(reader io.ByteReader) (image.Image, error) {
	r := &bitReader{r: reader}

	width := int(r.ReadBits(4))
	height := int(r.ReadBits(4))

	m := image.NewPaletted(image.Rect(0, 0, width*8, height*8), nil)

	data := make([]byte, width*height*8*2)
	mid := len(data) / 2

	s0 := data[:mid]
	s1 := data[mid:]
	if r.ReadBits(1) == 1 {
		s0, s1 = s1, s0
	}

	readData(r, s0, width, height)
	mode := r.ReadBits(1)
	if mode == 1 {
		mode = 1 + r.ReadBits(1)
	}
	readData(r, s1, width, height)

	if r.Err() != nil {
		return nil, r.Err()
	}

	switch mode {
	case 0:
		unxor(s0, width, height)
		unxor(s1, width, height)
	case 1:
		unxor(s0, width, height)
		for i := range s1 {
			s1[i] ^= s0[i]
		}
	case 2:
		unxor(s1, width, height)
		unxor(s0, width, height)
		for i := range s1 {
			s1[i] ^= s0[i]
		}
	}

	b := m.Pix[:0]
	for i := 0; i < mid; i++ {
		x := mingle(uint16(data[i]), uint16(data[mid+i]))
		for shift := uint(0); shift < 16; shift += 2 {
			b = append(b, uint8(x>>(14-shift))&3)
		}
	}
	return m, nil
}

// ReadData reads, expands, and deinterleaves compressed pixel data.
func readData(r *bitReader, b []uint8, width, height int) {
	c := make(chan uint8)
	go func() {
		n := 0
		z := uint16(0)
		if r.ReadBits(1) == 0 {
			z = readUint16(r)
		}
		for n < len(b)*4 {
			if z > 0 {
				c <- uint8(0)
				z--
				n++
				continue
			}

			px := r.ReadBits(2)
			if px != 0 {
				c <- px
				n++
			} else {
				z = readUint16(r)
			}
		}
		if n > len(b)*4 {
			// TODO better error handling
			log.Panicf("read too much data: %v vs %v", n, len(b)*4)
		}
	}()

	for x := 0; x < width; x++ {
		for shift := uint(8); shift > 0; {
			shift -= 2
			for y := 0; y < height*8; y++ {
				i := y*width + x
				b[i] |= <-c << shift
			}
		}
	}
}

// ReadUint16 reads a compressed 16-bit integer.
func readUint16(r *bitReader) (n uint16) {
	e := uint(1)
	for r.ReadBits(1) == 1 {
		e += 1
	}

	n = uint16(uint(1)<<e - 1) + r.ReadBits16(e)
	return n
}

var invXorShift [256]uint8

func init() {
	for i := uint(0); i < 256; i++ {
		invXorShift[i^(i>>1)] = uint8(i)
	}
}

// Unxor performs the inverse of (row ^ row>>1) on each row of b.
func unxor(b []uint8, width, height int) {
	stride := width
	for y := 0; y < height*8; y++ {
		bit := uint8(0)
		for x := 0; x < width; x++ {
			i := y*stride + x
			b[i] = invXorShift[b[i]]
			if bit != 0 {
				b[i] = ^b[i]
			}
			bit = b[i] & 1
		}
	}
}

func mingle(x, y uint16) (z uint16) {
	x = (x | x<<4) & 0x0F0F
	x = (x | x<<2) & 0x3333
	x = (x | x<<1) & 0x5555

	y = (y | y<<4) & 0x0F0F
	y = (y | y<<2) & 0x3333
	y = (y | y<<1) & 0x5555

	z = x | y<<1
	return
}

type romInfo struct {
	StatsPos      uint32
	MewPos        uint32
	OrderPos      uint32
	PalettePos    uint32
	PaletteMapPos uint32
}

// GetBankJP returns the bank containg the graphics for pokemon n.
func getBankJP(n int) int {
	switch {
	case n == 0xb6:
		return 0xb
	case n < 0x1f:
		return 0x9
	case n < 0x4a:
		return 0xa
	case n < 0x75:
		return 0xb
	case n < 0x9a:
		return 0xc
	default:
		return 0xd
	}
}

func getBank(n int) int {
	switch {
	case n == 0xb6:
		// Mew
		return 0xb
	case n < 0x1f:
		return 0x9
	case n < 0x4a:
		return 0xa
	case n < 0x74:
		return 0xb
	case n < 0x99:
		return 0xc
	default:
		return 0xd
	}
}

var grayPalette = color.Palette{
	color.Gray{255},
	color.Gray{170},
	color.Gray{85},
	color.Gray{0},
}

func main() {
	f, err := os.Open("red.gb")
	if err != nil {
		fmt.Println(err)
		return
	}
	f.Seek(13<<14, 0)
	r := bufio.NewReader(f)
	m, err := Decode(r)
	if err != nil {
		fmt.Println(err)
		return
	}
	p := m.(*image.Paletted)
	p.Palette = grayPalette
	png.Encode(os.Stdout, m)
}
