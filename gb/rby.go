package gb

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"image"
	"image/color"
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

// BitReader is a big-endian bit reader.
type bitReader struct {
	r     io.ByteReader
	bits  uint32
	count uint
	err   error
}

func (br *bitReader) ReadBits(n uint) uint32 {
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
		br.bits |= uint32(b)
		br.count += 8
	}

	shift := br.count - n
	mask := uint32(1<<n - 1)
	b := (br.bits >> shift) & mask
	br.count -= n
	return b
}

func (br *bitReader) Err() error {
	return br.err
}

// Decode reads a compressed pokemon image and returns it as an
// image.Paletted.
func DecodeRBY(reader io.Reader) (*image.Paletted, error) {
	r := &bitReader{r: bufio.NewReader(reader)}

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

	readPixels(r, s0, width, height)
	mode := r.ReadBits(1)
	if mode == 1 {
		mode = 1 + r.ReadBits(1)
	}
	readPixels(r, s1, width, height)

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

// ReadPixels reads, expands, and deinterleaves compressed pixel data.
func readPixels(r *bitReader, b []uint8, width, height int) {
	var z uint16
	if r.ReadBits(1) == 0 {
		z = decode16(r)
	}
	for x := 0; x < width; x++ {
		for shift := 6; shift >= 0; shift -= 2 {
			for y := 0; y < height*8; y++ {
			loop:
				var bits uint8
				if z > 0 {
					bits = 0
					z--
				} else {
					bits = uint8(r.ReadBits(2))
					if bits == 0 {
						z = decode16(r)
						goto loop
					}
				}
				i := y*width + x
				b[i] |= bits << uint(shift)
			}
		}
	}
}

// Decode16 reads a compressed 16-bit integer.
func decode16(r *bitReader) uint16 {
	var n uint = 1
	for r.ReadBits(1) == 1 {
		n += 1
	}
	return uint16(1<<n + r.ReadBits(n) - 1)
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

//

var (
	bulbasaurStats    = []byte{1, 0x2D, 0x31, 0x31, 0x2D, 0x41}
	mewStats          = []byte{151, 100, 100, 100, 100, 100}
	pokedexOrderBytes = []byte{0x70, 0x73, 0x20, 0x23, 0x15, 0x64, 0x22, 0x50}
	paletteMapBytes   = []byte{16, 22, 22, 22, 18, 18, 18, 19, 19, 19}
)

type RBYRipper struct {
	f         *os.File
	lang      string
	version   string
	spritePos [151]struct {
		front int64
		back  int64
	}
	spritePalette [151]byte
	sgbPalettes   []color.Palette
	cgbPalettes   []color.Palette
	header        Header
}

func NewRBYRipper(f *os.File) (*RBYRipper, error) {
	rip := new(RBYRipper)
	rip.f = f

	h, err := readHeader(f)
	if err != nil {
		return nil, err
	}
	rip.header = h

	rom, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	getBank := getBankRBY
	if h.Lang == "jp" && (h.Title == "POKEMON RED" || h.Title == "POKEMON GREEN") {
		getBank = getBankRG
	}

	// Read pokedex order
	pos := bytes.Index(rom, pokedexOrderBytes)
	if pos < 0 {
		return nil, fmt.Errorf("Couldn't find pokedex order")
	}

	internalId := make(map[int]int)
	const pokedexOrderLength = 190
	for i, n := range rom[pos : pos+190] {
		if n != 0 {
			internalId[int(n)] = i + 1
		}
	}

	// Read sprite pointers
	pos = bytes.Index(rom, bulbasaurStats)
	if pos < 0 {
		return nil, fmt.Errorf("Couldn't find Bulbasaur's stats")
	}

	f.Seek(int64(pos), 0)
	var stats [151]struct {
		N         uint8
		Stats     [5]uint8
		Types     [2]uint8
		CatchRate uint8
		ExpYield  uint8

		SpriteSize         uint8
		FrontSpritePointer uint16
		BackSpritePointer  uint16

		Attacks    [4]uint8
		GrowthRate uint8
		TMs        [8]uint8
	}
	err = binary.Read(f, binary.LittleEndian, stats[:])
	if err != nil {
		return nil, err
	}
	for i, s := range stats {
		bank := getBank(internalId[int(s.N)])
		base := int64(bank-1) << 14
		rip.spritePos[i].front = base + int64(s.FrontSpritePointer)
		rip.spritePos[i].back = base + int64(s.BackSpritePointer)
	}

	// Find Mew if missing
	if stats[150].N != 151 {
		pos = bytes.Index(rom, mewStats)
		if pos < 0 {
			return nil, fmt.Errorf("Couldn't find Mew's stats")
		}
		f.Seek(int64(pos), 0)
		s := &stats[150]
		err = binary.Read(f, binary.LittleEndian, s)
		if err != nil {
			return nil, err
		}
		rip.spritePos[150].front = int64(s.FrontSpritePointer)
		rip.spritePos[150].back = int64(s.BackSpritePointer)
	}

	// Read palettes
	pos = bytes.Index(rom, paletteMapBytes)
	if pos < 0 {
		return nil, fmt.Errorf("Couldn't find palettes")
	}
	paletteMap := rom[pos+1 : pos+152]
	copy(rip.spritePalette[:], paletteMap)

	var palettes [40][4]RGB15
	r := bytes.NewReader(rom[pos+152:])
	err = binary.Read(r, binary.LittleEndian, &palettes)
	if err != nil {
		return nil, err
	}
	for _, p := range palettes {
		var cp [4]color.Color
		for i, c := range p {
			cp[i] = c
		}
		rip.sgbPalettes = append(rip.sgbPalettes, cp[:])
	}
	if h.HasCGB {
		err = binary.Read(r, binary.LittleEndian, &palettes)
		if err != nil {
			return nil, err
		}
		for _, p := range palettes {
			var cp [4]color.Color
			for i, c := range p {
				cp[i] = c
			}
			rip.cgbPalettes = append(rip.cgbPalettes, cp[:])
		}
	}

	return rip, nil
}

type Header struct {
	Title   string // Game title from ROM: POKEMON_RED, etc
	Version string // Version identifier: red, blue, etc
	Lang    string // Game language: en, jp
	HasSGB  bool   // Game has Super Gameboy support
	HasCGB  bool   // Game has Gameboy Color support
}

func readHeader(r io.ReaderAt) (Header, error) {
	var header [0x150]byte

	if _, err := r.ReadAt(header[:], 0); err != nil {
		return Header{}, err
	}

	hasCGB := header[0x143] >= 0x80
	hasSGB := header[0x146] == 3
	isJP := header[0x14A] == 0

	var title string
	// In *some* GBC games (like GSC), the last four bytes are a
	// manufacturer code. I don't know of a reliable way to detect this,
	// so let's just say that if the last byte of the title is not \x00
	// then it must be a manufacturer code.
	if hasCGB && header[0x142] != '\x00' {
		title = string(bytes.TrimRight(header[0x134:0x143-4], "\x00"))
	} else {
		title = string(bytes.TrimRight(header[0x134:0x143], "\x00"))
	}

	version := "unknown"
	switch title {
	case "POKEMON RED":
		version = "red"
	case "POKEMON GREEN":
		version = "green"
	case "POKEMON BLUE":
		version = "blue"
	case "POKEMON YELLOW":
		version = "yellow"
	}

	lang := "en"
	if isJP {
		lang = "jp"
	}

	h := Header{
		Title:   title,
		Version: version,
		Lang:    lang,
		HasCGB:  hasCGB,
		HasSGB:  hasSGB,
	}

	return h, nil
}

type RGB15 uint16

func (rgb RGB15) RGBA() (r, g, b, a uint32) {
	r = (uint32(rgb>>0&31)*0xFFFF + 15) / 31
	g = (uint32(rgb>>5&31)*0xFFFF + 15) / 31
	b = (uint32(rgb>>10&31)*0xFFFF + 15) / 31
	a = 0xFFFF
	return
}

func (rip *RBYRipper) Pokemon(n int) (*image.Paletted, error) {
	ptr := rip.spritePos[n-1].front
	rip.f.Seek(ptr, 0)
	return DecodeRBY(rip.f)
}

func (rip *RBYRipper) PokemonPalette(n int, sys string) color.Palette {
	pi := rip.spritePalette[n-1]
	if sys == "sgb" {
		return rip.sgbPalettes[pi]
	}
	return rip.cgbPalettes[pi]
}

func (rip *RBYRipper) CombinedPalette(sys string) (p color.Palette) {
	var palettes []color.Palette
	if sys == "sgb" {
		palettes = rip.sgbPalettes[16:26]
	} else {
		palettes = rip.cgbPalettes[16:26]
	}
	p = append(p, palettes[0][0])
	for _, sp := range palettes {
		p = append(p, sp[1], sp[2])
	}
	p = append(p, palettes[0][3])
	return
}

// GetBankRG returns the bank containing the graphics for pokemon n
// in Japanese R/G.
func getBankRG(n int) int {
	switch {
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

// GetBankRBY returns the bank containing the graphics for pokemon n
// in the R/B/Y.
func getBankRBY(n int) int {
	switch {
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

// XXX delete these?
func (rip *RBYRipper) Header() Header                     { return rip.header }
func (rip *RBYRipper) CGBPalettes() []color.Palette       { return rip.cgbPalettes }
func (rip *RBYRipper) SetCGBPalettes(pal []color.Palette) { rip.cgbPalettes = pal }
