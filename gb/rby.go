// +build ignore

package gb

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"image"
	"image/color"
	"image/draw"

	"github.com/magical/png"
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
func Decode(reader io.Reader) (*image.Paletted, error) {
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

var fakeGbcPalettes []color.Palette

type Ripper struct {
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
}

func newRipper(f *os.File) (*Ripper, error) {
	rip := new(Ripper)
	rip.f = f

	rom, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	header := rom[:0x150]
	title := string(bytes.TrimRight(header[0x134:0x143], "\x00"))
	hasCGB := rom[0x143] == 0x80
	hasSGB := rom[0x146] == 3
	isJP := rom[0x14A] == 0

	_ = hasCGB
	_ = hasSGB

	getBank := getBankRBY
	if isJP && (title == "POKEMON RED" || title == "POKEMON GREEN") {
		getBank = getBankRG
	}

	switch title {
	case "POKEMON RED":
		rip.version = "red"
	case "POKEMON GREEN":
		rip.version = "green"
	case "POKEMON BLUE":
		rip.version = "blue"
	case "POKEMON YELLOW":
		rip.version = "yellow"
	}

	if isJP {
		rip.lang = "jp"
	} else {
		rip.lang = "en"
	}

	// Read pokedex order
	pos := bytes.Index(rom, pokedexOrderBytes)
	if pos < 0 {
		return nil, fmt.Errorf("Couldn't find pokedex order")
	}

	internalId := make(map[int]int)
	for i, n := range rom[pos : pos+0xbe] {
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
	if hasCGB {
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

type RGB15 uint16

func (rgb RGB15) RGBA() (r, g, b, a uint32) {
	r = (uint32(rgb>>0&31)*0xFFFF + 15) / 31
	g = (uint32(rgb>>5&31)*0xFFFF + 15) / 31
	b = (uint32(rgb>>10&31)*0xFFFF + 15) / 31
	a = 0xFFFF
	return
}

func (rip *Ripper) Pokemon(n int) (*image.Paletted, error) {
	ptr := rip.spritePos[n-1].front
	rip.f.Seek(ptr, 0)
	return Decode(rip.f)
}

func (rip *Ripper) PokemonPalette(n int, sys string) color.Palette {
	pi := rip.spritePalette[n-1]
	if sys == "sgb" {
		return rip.sgbPalettes[pi]
	}
	if sys == "fakegbc" && fakeGbcPalettes != nil {
		return fakeGbcPalettes[pi]
	}
	return rip.cgbPalettes[pi]
}

func (rip *Ripper) CombinedPalette(sys string) (p color.Palette) {
	var palettes []color.Palette
	if sys == "sgb" {
		palettes = rip.sgbPalettes[16:26]
	} else if sys == "fakegbc" && fakeGbcPalettes != nil {
		palettes = fakeGbcPalettes[16:26]
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

// GetBankRG returns the bank containg the graphics for pokemon n.
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

var grayPalette = color.Palette{
	color.Gray{255},
	color.Gray{170},
	color.Gray{85},
	color.Gray{0},
}

var gameboyPalette = color.Palette{
	color.NRGBA{0x9B, 0xBC, 0x0F, 0xFF},
	color.NRGBA{0x8B, 0xAC, 0x0F, 0xFF},
	color.NRGBA{0x30, 0x62, 0x30, 0xFF},
	color.NRGBA{0x0F, 0x28, 0x0F, 0xFF},
}

var gbBrownPalette = color.Palette{
	color.NRGBA{0xFF, 0xFF, 0xFF, 0xFF},
	color.NRGBA{0xFF, 173, 99, 0xFF},
	color.NRGBA{132, 49, 0, 0xFF},
	color.NRGBA{0, 0, 0, 0xFF},
}

var gbDarkBrownPalette = color.Palette{
	color.NRGBA{R: 0xff, G: 0xe6, B: 0xc5, A: 0xff},
	color.NRGBA{R: 0xce, G: 0x9c, B: 0x84, A: 0xff},
	color.NRGBA{R: 0x84, G: 0x6b, B: 0x29, A: 0xff},
	color.NRGBA{R: 0x5a, G: 0x31, B: 0x8, A: 0xff},
}

var gbPokemonRedPalette = color.Palette{
	color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	color.NRGBA{R: 0xff, G: 0x84, B: 0x84, A: 0xff},
	color.NRGBA{R: 0x94, G: 0x3a, B: 0x3a, A: 0xff},
	color.NRGBA{R: 0x0, G: 0x0, B: 0x0, A: 0xff},
}

//{color.NRGBA{R:0xff, G:0xff, B:0xff, A:0xff}, color.NRGBA{R:0x7b, G:0xff, B:0x31, A:0xff}, color.NRGBA{R:0x0, G:0x84, B:0x0, A:0xff}, color.NRGBA{R:0x0, G:0x0, B:0x0, A:0xff}},
//{color.NRGBA{R:0xff, G:0xff, B:0xff, A:0xff}, color.NRGBA{R:0xff, G:0x84, B:0x84, A:0xff}, color.NRGBA{R:0x94, G:0x3a, B:0x3a, A:0xff}, color.NRGBA{R:0x0, G:0x0, B:0x0, A:0xff}},

var gbPokemonGreenPalette = color.Palette{
	color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	color.NRGBA{R: 0x7b, G: 0xff, B: 0x31, A: 0xff},
	color.NRGBA{R: 0x0, G: 0x63, B: 0xc5, A: 0xff},
	color.NRGBA{R: 0x0, G: 0x0, B: 0x0, A: 0xff},
}

//{color.NRGBA{R:0xff, G:0xff, B:0xff, A:0xff}, color.NRGBA{R:0xff, G:0x84, B:0x84, A:0xff}, color.NRGBA{R:0x94, G:0x3a, B:0x3a, A:0xff}, color.NRGBA{R:0x0, G:0x0, B:0x0, A:0xff}},
//{color.NRGBA{R:0xff, G:0xff, B:0xff, A:0xff}, color.NRGBA{R:0x7b, G:0xff, B:0x31, A:0xff}, color.NRGBA{R:0x0, G:0x63, B:0xc5, A:0xff}, color.NRGBA{R:0x0, G:0x0, B:0x0, A:0xff}},

var gbPokemonBluePalette = color.Palette{
	color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	color.NRGBA{R: 0x63, G: 0xa5, B: 0xff, A: 0xff},
	color.NRGBA{R: 0x0, G: 0x0, B: 0xff, A: 0xff},
	color.NRGBA{R: 0x0, G: 0x0, B: 0x0, A: 0xff},
}

//{color.NRGBA{R:0xff, G:0xff, B:0xff, A:0xff}, color.NRGBA{R:0xff, G:0x84, B:0x84, A:0xff}, color.NRGBA{R:0x94, G:0x3a, B:0x3a, A:0xff}, color.NRGBA{R:0x0, G:0x0, B:0x0, A:0xff}},
//{color.NRGBA{R:0xff, G:0xff, B:0xff, A:0xff}, color.NRGBA{R:0x63, G:0xa5, B:0xff, A:0xff}, color.NRGBA{R:0x0, G:0x0, B:0xff, A:0xff}, color.NRGBA{R:0x0, G:0x0, B:0x0, A:0xff}},

var gbPokemonYellowPalette = color.Palette{
	color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	color.NRGBA{R: 0xff, G: 0xff, B: 0x0, A: 0xff},
	color.NRGBA{R: 0xff, G: 0x0, B: 0x0, A: 0xff},
	color.NRGBA{R: 0x0, G: 0x0, B: 0x0, A: 0xff},
}

func getValue(min, max int, v uint8) int {
	u := float32(v) / 255
	return min + int(float32(max-min)*(2*u-u*u))
}

// http://sourceforge.net/p/vbam/code/1226/tree/trunk/src/gb/GB.cpp#l585
func muteColors(p color.Palette) {
	for i := range p {
		c := color.NRGBAModel.Convert(p[i]).(color.NRGBA)
		r := getValue(
			getValue(33, 115, c.G),
			getValue(198, 239, c.G),
			c.R) - 33
		r41 := getValue(0, 41, c.R)
		r25 := getValue(0, 25, c.R)
		r8 := getValue(0, 8, c.R)
		g := getValue(
			getValue(33+r41, 115+r25, c.B),
			getValue(198+r25, 229+r8, c.B),
			c.G) - 33
		b := getValue(
			getValue(33+r41, 115+r25, c.G),
			getValue(198+r25, 229+r8, c.G),
			c.B) - 33
		p[i] = color.NRGBA{uint8(r), uint8(g), uint8(b), c.A}
	}
}

// https://code.google.com/p/gnuboy/source/browse/trunk/lcd.c?r=199#722
func muteColors2(p color.Palette) {
	for i := range p {
		c := color.NRGBAModel.Convert(p[i]).(color.NRGBA)
		r, g, b := uint16(c.R), uint16(c.G), uint16(c.B)
		rr := (r*195+g*25+b*0)>>8 + 35
		gg := (r*25+g*170+b*25)>>8 + 35
		bb := (r*25+g*60+b*125)>>8 + 40
		p[i] = color.NRGBA{uint8(rr), uint8(gg), uint8(bb), c.A}
	}
}

func muteColors3(p color.Palette) {
	for i := range p {
		c := color.NRGBAModel.Convert(p[i]).(color.NRGBA)
		r, g, b := uint16(c.R)>>3, uint16(c.G)>>3, uint16(c.B)>>3
		rr := (r*13 + g*2 + b) / 2
		gg := (r*0 + g*12 + b*4) / 2
		bb := (r*3 + g*2 + b*11) / 2
		p[i] = color.NRGBA{uint8(rr), uint8(gg), uint8(bb), c.A}
	}
}

func muteColors4(p color.Palette) {
	for i := range p {
		c := color.NRGBAModel.Convert(p[i]).(color.NRGBA)
		rr := c.R/2 + 82
		gg := c.G/2 + 82
		bb := c.B/2 + 82
		p[i] = color.NRGBA{rr, gg, bb, c.A}
	}
}

func main() {
	flag.Parse()
	for _, filename := range flag.Args() {
		f, err := os.Open(filename)
		if err != nil {
			continue
		}
		rip, err := newRipper(f)
		if err != nil {
			continue
		}
		if rip.cgbPalettes != nil {
			fakeGbcPalettes = rip.cgbPalettes
			break
		}
	}

	for _, filename := range flag.Args() {
		f, err := os.Open(filename)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		rip, err := newRipper(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		path := "out"
		os.MkdirAll(path, 0777)
		var gbcPalette color.Palette
		if rip.cgbPalettes == nil {
			switch rip.version {
			case "red":
				gbcPalette = gbPokemonRedPalette
			case "green":
				gbcPalette = gbPokemonGreenPalette
			case "blue":
				gbcPalette = gbPokemonBluePalette
			case "yellow":
				gbcPalette = gbPokemonYellowPalette
			}
		}
		var systems = []struct {
			system  string
			palette color.Palette
		}{
			{"gb", grayPalette},
			{"sgb", nil},
			{"gbc", gbcPalette},
			{"fakegbc", nil},
		}
		for _, sys := range systems {
			dst, err := os.Create(path + "/" + rip.lang + "-" + rip.version + "-" + sys.system + ".png")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			montage(rip, dst, sys.palette, sys.system)
			dst.Close()
		}
		f.Close()
	}
}

func montage(rip *Ripper, w io.Writer, pal color.Palette, sys string) {
	b := image.Rect(0, 0, 56*15, 56*((151+14)/15))
	var m draw.Image
	if pal != nil {
		m = image.NewPaletted(b, pal)
	} else {
		//m = image.NewNRGBA(b)
		//bg := image.NewUniform(rip.PokemonPalette(1, "sgb")[0])
		//draw.Draw(m, m.Bounds(), bg, image.ZP, draw.Src)
		m = image.NewPaletted(b, rip.CombinedPalette(sys))
	}
	tile := image.Rect(0, 0, 56, 56)
	for i := 0; i < 151; i++ {
		p, err := rip.Pokemon(i + 1)
		if err != nil {
			log.Printf("error getting pokemon %d: %v", i+1, err)
		}
		if pal != nil {
			p.Palette = pal
		} else {
			p.Palette = rip.PokemonPalette(i+1, sys)
		}
		padding := tile.Size().Sub(p.Rect.Size()).Div(2)
		p.Rect = p.Rect.Add(padding)
		row := i / 15
		col := i % 15
		draw.Draw(m, tile.Add(image.Pt(col, row).Mul(56)), p, image.ZP, draw.Src)
	}
	/*if p, ok := m; ok {
		muteColors2(p.Palette)
	}*/
	sBIT := 5
	if pal != nil && &pal[0] == &grayPalette[0] {
		sBIT = 2
	}
	png.EncodeWithSBIT(w, m, uint(sBIT))
}
