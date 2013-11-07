package main

import (
	"bufio"
	//"bytes"
	"fmt"
	"io"
	"log"
	"os"

	//"image"
	//"image/color"
	//"image/png"
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

The first method is to xor each row of one of the halves with itself shifted
by one bit.

The second method is to xor one of the halves with the other.

Note: The way the game does the decompression, it ends up with an image whose
tiles have been transposed. This is unnecessary and, in fact, makes the job
harder. It is easier not to mess around with tiles at all.

*/

// This is the inverse of (i xor i>>1).
var table = [2][16]uint8{
	{0, 1, 3, 2, 7, 6, 4, 5, 15, 14, 12, 13, 8, 9, 11, 10},
	{15, 14, 12, 13, 8, 9, 11, 10, 0, 1, 3, 2, 7, 6, 4, 5},
}

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

// Big-endian bit writer
type bitWriter struct {
	b    []uint8
	n    uint
	bits uint32
}

func (bw *bitWriter) Len() int {
	return len(bw.b)
}

func (bw *bitWriter) WriteBits(n uint, bits0 uint8) {
	bw.bits = bw.bits<<n | uint32(bits0)
	bw.n += n
	for bw.n >= 8 {
		bw.b = append(bw.b, uint8(bw.bits>>(bw.n-8)))
		bw.n -= 8
	}
}

//const TileSize = 8

func Decompress(reader io.ByteReader) (b []uint8, w, h int) {
	r := &bitReader{r: reader}

	width := int(r.ReadBits(4))
	height := int(r.ReadBits(4))

	data := make([]byte, width*height*8*2)
	mid := len(data) / 2

	s0 := data[:mid]
	s1 := data[mid:]
	if r.ReadBits(1) == 1 {
		s0, s1 = s1, s0
	}

	fillRam(r, s0)
	mode := r.ReadBits(1)
	if mode == 1 {
		mode = 1 + r.ReadBits(1)
	}
	fillRam(r, s1)

	if r.Err() != nil {
		//TODO better error handling
		panic(r.Err())
	}

	copy(s0, deinterlace(s0, width, height))
	copy(s1, deinterlace(s1, width, height))

	switch mode {
	case 0:
		thing1(s0, width, height)
		thing1(s1, width, height)
	case 1:
		thing1(s0, width, height)
		thing2(s0, s1)
	case 2:
		thing1(s1, width, height)
		thing1(s0, width, height)
		thing2(s0, s1)
	}

	for i := 0; i < mid; i++ {
		x := mingle(uint16(data[i]), uint16(data[mid+i]))
		for shift := uint(0); shift < 16; shift += 2 {
			b = append(b, uint8(x>>(14-shift))&3)
		}
	}
	return b, width, height
}

func fillRam(r *bitReader, b []uint8) {
	w := bitWriter{b: b[:0]}
	if r.ReadBits(1) == 0 {
		readRle(r, &w)
	}
	for w.Len() < len(b) {
		px := r.ReadBits(2)
		if px != 0 {
			w.WriteBits(2, px)
		} else {
			readRle(r, &w)
		}
	}
	if w.Len() > len(b) {
		log.Panicf("read too much data: %v vs %v (w.n: %v) %v", w.Len(), len(b), w.n)
	}
}

func readRle(r *bitReader, w *bitWriter) {
	c := uint(0)
	for r.ReadBits(1) == 1 {
		c += 1
	}

	n := uint(2<<c - 1)
	n += uint(r.ReadBits16(c + 1))

	for i := uint(0); i < n; i++ {
		w.WriteBits(2, uint8(0))
	}
}

func deinterlace(b []uint8, width, height int) []uint8 {
	// The compressed image is not stored in tiles, and it is stored with
	// its rows interleaved; two bits from the first row, two bits from
	// the second row, and so on, in effect almost transposing the image.
	// This seems pointless.
	w := bitWriter{b: nil}
	for y := 0; y < height*8; y++ {
		shift := 6 - uint(y)%4*2
		for x := 0; x < width*8; x += 2 {
			i := x*height + y/4
			w.WriteBits(2, b[i]>>shift&3)
		}
	}
	return w.b
}

// This function undoes the operation of xoring each row with itself shifted
// to the left.
func thing1(b []uint8, width, height int) {
	stride := width
	for y := 0; y < height*8; y++ {
		bit := uint8(0)
		for x := 0; x < width; x++ {
			i := y*stride + x

			m := b[i] >> 4
			n := b[i] & 0xf

			m = table[bit][m]
			bit = m & 1

			n = table[bit][n]
			bit = n & 1

			b[i] = m<<4 | n
		}
	}
}

// This function xors each byte of b and d, storing the result in d.
func thing2(b, d []uint8) {
	for i := range d {
		// if mirror {}
		d[i] ^= b[i]
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

func main() {
	f, err := os.Open("red.gb")
	if err != nil {
		fmt.Println(err)
		return
	}
	f.Seek(13<<14, 0)
	r := bufio.NewReader(f)
	b, w, h := Decompress(r)
	fmt.Println("P5")
	fmt.Println(w*8, h*8)
	fmt.Println("3")
	//fmt.Printf("%x\n", b);
	for i := range b {
		fmt.Printf("%c", 3-b[i])
	}
}
