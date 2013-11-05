package main

import (
	"bufio"
	"fmt"
	//"image"
	//"image/color"
	//"image/png"
	"io"
	"log"
	"os"
)

// table[i] = i ^ ((nextPowerOfTwo(i+1) - 1) >> 1)
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
		b = b << 8 | uint16(br.ReadBits(n-8))
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
	//bits = bits & uint8(1<<n - 1)
	bw.bits = bw.bits<<n | uint32(bits0)
	bw.n += n
	for bw.n >= 8 {
		bw.b = append(bw.b, uint8(bw.bits>>(bw.n-8)))
		bw.n -= 8
	}
}

//const TileSize = 8

func Decompress(reader io.ByteReader) (b []uint8) {
	r := &bitReader{r: reader}

	// TODO make these make sense
	sizex := int(r.ReadBits(4)) * 8
	sizey := int(r.ReadBits(4))

	//fmt.Println("size:", sizex, sizey)

	size := sizex * sizey

	var rams [2][]uint8
	ramorder := r.ReadBits(1)

	r1 := ramorder
	r2 := ramorder ^ 1

	rams[r1] = fillRam(r, rams[r1], size)
	mode := r.ReadBits(1)
	if mode == 1 {
		mode = 1 + r.ReadBits(1)
	}
	rams[r2] = fillRam(r, rams[r2], size)

	rams[r1] = deinterlace(rams[r1], sizex, sizey)
	rams[r2] = deinterlace(rams[r2], sizex, sizey)

	switch mode {
	case 0:
		thing1(rams[0], sizex, sizey)
		thing1(rams[1], sizex, sizey)
	case 1:
		thing1(rams[r1], sizex, sizey)
		thing2(rams[r1], rams[r2])
	case 2:
		thing1(rams[r2], sizex, sizey)
		thing1(rams[r1], sizex, sizey)
		thing2(rams[r1], rams[r2])
	}

	for i := range rams[0] {
		x := mingle(uint16(rams[0][i]), uint16(rams[1][i]))
		b = append(b, uint8(x>>8), uint8(x))
	}
	return b
}

func fillRam(r *bitReader, b []uint8, size int) []uint8{
	w := bitWriter{b: b}
	if r.ReadBits(1) == 0 {
		readRle(r, &w)
	}
	for w.Len() < size {
		px := r.ReadBits(2)
		if px != 0 {
			w.WriteBits(2, px)
		} else {
			readRle(r, &w)
		}
	}
	if w.Len() > size {
		log.Panicf("read too much data: %v vs %v (w.n: %v) %v", w.Len(), size, w.n)
	}
	return w.b
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

func deinterlace(b []uint8, sizex, sizey int) []uint8 {
	w := bitWriter{b: nil}
	for y := 0; y < sizey; y++ {
		for x := 0; x < sizex; x++ {
			i := y*sizex + x/4
			shift := 6 - uint(x) * 2 % 8
			w.WriteBits(2, b[i+sizex*0/4]>>shift & 3)
			w.WriteBits(2, b[i+sizex*1/4]>>shift & 3)
			w.WriteBits(2, b[i+sizex*2/4]>>shift & 3)
			w.WriteBits(2, b[i+sizex*3/4]>>shift & 3)
		}
	}
	return w.b
}

func thing1(b []uint8, sizex, sizey int) {
	for x := 0; x < sizex; x++ {
		bit := uint8(0)
		for y := 0; y < sizey; y++ {
			i := y*sizex + x
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

/*
func untile(b []uint8, sizex, sizey int) []uint8 {
	out := make([]uint8, 0, len(b))
	for x := 0; x < sizex; x++ {
		for y := 0; y < sizey; y++ {
			i := y*sizex + x
			out = append(out, b[i*2], b[i*2+1])
		}
	}
	return out
}
*/

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

func untile(b []uint8) (out []uint8) {
	for x := 0; x < 40; x++ {
		for y := 0; y < 5; y++ {
			i := (y * 40 + x) * 2
			out = append(out,
				b[i]>>6 & 3,
				b[i]>>4 & 3,
				b[i]>>2 & 3,
				b[i]>>0 & 3,
				b[i+1]>>6 & 3,
				b[i+1]>>4 & 3,
				b[i+1]>>2 & 3,
				b[i+1]>>0 & 3,
			)
		}
	}
	return
}

func main() {
	f, err := os.Open("red.gb")
	if err != nil {
		fmt.Println(err)
		return
	}
	f.Seek(13<< 14, 0)
	b := Decompress(bufio.NewReader(f))
	fmt.Println("P5")
	fmt.Println("40 40")
	fmt.Println("3")
	//fmt.Printf("%x\n", b);
	for _, x := range untile(b) {
		fmt.Printf("%c", 3 - x)
	}
}
