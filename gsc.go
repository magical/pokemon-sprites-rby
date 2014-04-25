/* Pokemon Gold/Silver/Crystal sprite ripper. */
package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)

/*

; Pic animations are assembled in 3 parts:

; Top-level animations:
;   frame #, duration: Frame 0 is the original pic (no change)
;   setrepeat #:       Sets the number of times to repeat
;   dorepeat #:        Repeats from command # (starting from 0)
;   end

; Bitmasks:
;   Layered over the pic to designate affected tiles

; Frame definitions:
;   first byte is the bitmask used for this frame
;   following bytes are tile ids mapped to each bit in the mask

(above description from https://github.com/kanzure/pokecrystal)

*/

var ErrMalformed = errors.New("malformed data")
var ErrTooLarge = errors.New("decompressed data is suspeciously large")

// Decode a GSC image of dimensions w*8 x h*8.
func Decode(reader io.Reader, w, h int) (*image.Paletted, error) {
	data, err := decodeTiles(newByteReader(reader), w*h*16)
	if err != nil {
		return nil, err
	}
	var m = image.NewPaletted(image.Rect(0, 0, w*8, h*8), nil)
	untile(m, data)
	return m, nil
}

func newByteReader(r io.Reader) io.ByteReader {
	if r, ok := r.(io.ByteReader); ok {
		return r
	}
	return bufio.NewReader(r)
}

func decodeTiles(r io.ByteReader, sizehint int) ([]byte, error) {
	var data = make([]byte, 0, sizehint)
	var readErr error
	readByte := func() (b byte) {
		if readErr == nil {
			b, readErr = r.ReadByte()
		}
		return b
	}
	for {
		var control, num, seek int
		num = int(readByte())
		if num == 0xFF {
			break
		}
		if len(data) > 0xFFFF {
			return nil, ErrTooLarge
		}
		if num>>5 == 7 {
			num = num<<8 + int(readByte())
			// 3-bit control, 10-bit num
			control = num >> 10 & 7
			num = num&0x3FF + 1
		} else {
			// 3-bit control, 5-bit num
			control = num >> 5
			num = num&0x1F + 1
		}
		if control >= 4 {
			seek = int(readByte())
			if seek&0x80 == 0 {
				seek = seek<<8 + int(readByte())
			} else {
				seek = len(data) - seek&^0x80 - 1
			}
			if !(0 <= seek && seek < len(data)) {
				return nil, ErrMalformed
			}
		}
		//fmt.Fprintln(os.Stderr, control, num, seek, len(data))
		switch control {
		case 0:
			for i := 0; i < num; i++ {
				data = append(data, readByte())
			}
		case 1:
			b := readByte()
			for i := 0; i < num; i++ {
				data = append(data, b)
			}
		case 2:
			b := [2]byte{readByte(), readByte()}
			for i := 0; i < num; i++ {
				data = append(data, b[i%2])
			}
		case 3:
			for i := 0; i < num; i++ {
				data = append(data, 0)
			}
		case 4:
			for i := 0; i < num; i++ {
				data = append(data, data[seek+i])
			}
		case 5:
			for i := 0; i < num; i++ {
				data = append(data, reverseBits(data[seek+i]))
			}
		case 6:
			if num-1 > seek {
				return nil, ErrMalformed
			}
			for i := 0; i < num; i++ {
				data = append(data, data[seek-i])
			}
		}
		//fmt.Fprintf(os.Stderr, "%x\n", data)
	}
	if readErr == io.EOF {
		readErr = io.ErrUnexpectedEOF
	}
	if readErr != nil {
		return nil, readErr
	}
	return data, readErr
}

func untile(m *image.Paletted, data []byte) {
	w, h := m.Rect.Dx(), m.Rect.Dy()
	for i, x := 0, 0; x < w; x += 8 {
		for y := 0; y < h; y += 8 {
			for ty := 0; ty < 8; ty++ {
				pix := mingle(uint16(data[i]), uint16(data[i+1]))
				for tx := 7; tx >= 0; tx-- {
					m.SetColorIndex(x+tx, y+ty, uint8(pix&3))
					pix >>= 2
				}
				i += 2
			}
		}
	}
}

type colorRGB15 uint16

func (rgb colorRGB15) NRGBA() color.NRGBA {
	return color.NRGBA{
		uint8(((rgb>>0&31)*255 + 15) / 31),
		uint8(((rgb>>5&31)*255 + 15) / 31),
		uint8(((rgb>>10&31)*255 + 15) / 31),
		255,
	}
}

func (rgb colorRGB15) RGBA() (r, g, b, a uint32) {
	return rgb.NRGBA().RGBA()
}

func reverseBits(b byte) byte {
	return byte(uint64(b) * 0x0202020202 & 0x010884422010 % 1023)
}

func mingle(x, y uint16) uint16 {
	x = (x | x<<4) & 0x0F0F
	x = (x | x<<2) & 0x3333
	x = (x | x<<1) & 0x5555

	y = (y | y<<4) & 0x0F0F
	y = (y | y<<2) & 0x3333
	y = (y | y<<1) & 0x5555

	return x | y<<1
}

type romInfo struct {
	Title          string
	StatsPos       int64
	PalettePos     int64
	SpritePos      int64
	UnownSpritePos int64

	AnimPos         int64
	ExtraPos        int64
	FramesPos       int64
	BitmapsPos      int64
	UnownAnimPos    int64
	UnownExtraPos   int64
	UnownFramesPos  int64
	UnownBitmapsPos int64
}

var romtab = map[string]romInfo{
	"POKEMON_GLD": {
		Title:          "POKEMON_GLD",
		StatsPos:       0x51B0B,
		PalettePos:     0xAD3D,
		SpritePos:      0x48000,
		UnownSpritePos: 0x7C000,
	},
	"POKEMON_SLV": {
		// Same as Gold
		Title:          "POKEMON_SLV",
		StatsPos:       0x51B0B,
		PalettePos:     0xAD3D,
		SpritePos:      0x48000,
		UnownSpritePos: 0x7C000,
	},
	"PM_CRYSTAL": {
		Title:           "PM_CRYSTAL",
		StatsPos:        0x51424,
		PalettePos:      0xA8CE,
		SpritePos:       0x120000,
		UnownSpritePos:  0x124000,
		AnimPos:         0xD0695,
		ExtraPos:        0xD16A3,
		UnownAnimPos:    0xD2229,
		UnownExtraPos:   0xD23D1,
		BitmapsPos:      0xD24EF,
		UnownBitmapsPos: 0xD3ADE,
		FramesPos:       0xD4000,
		UnownFramesPos:  0xD99A9,
	},
}

var defaultPalette = color.Palette{
	color.Gray{0xFF},
	color.Gray{0x7F},
	color.Gray{0x3F},
	color.Gray{0},
}

type Ripper struct {
	r    io.ReaderAt
	info romInfo
}

func NewRipper(r io.ReaderAt) (_ *Ripper, err error) {
	rip := new(Ripper)
	rip.r = r

	var header [0x150]byte
	r.ReadAt(header[:], 0)
	title := string(header[0x134:0x13F])
	title = strings.TrimRight(title, "\x00")

	info, ok := romtab[title]
	if !ok {
		return nil, errors.New("Couldn't recognize ROM")
	}
	rip.info = info

	return rip, nil
}

func (rip *Ripper) pokemonSize(number int) (width, height int) {
	// Read the base stats structure. We only care about SpriteSize, but
	// what the heck.
	var stats struct {
		N          uint8
		Stats      [6]uint8
		Types      [2]uint8
		CatchRate  uint8
		ExpYield   uint8
		Items      [2]uint8
		Gender     uint8
		_          uint8
		HatchTime  uint8
		_          uint8
		SpriteSize uint8
		_          [4]uint8
		GrowthRate uint8
		EggGroups  uint8
		TMs        [8]uint8
	}
	size := int64(binary.Size(&stats))
	off := rip.info.StatsPos + size*int64(number-1)
	err := binary.Read(
		io.NewSectionReader(rip.r, off, size),
		binary.LittleEndian,
		&stats,
	)
	if err != nil {
		// BUG: shouldn't panic
		panic(err)
	}

	// The high and low nibbles of SpriteSize give the width and height of
	// the sprite in 8x8 tiles. Not sure which is which, but it doesn't
	// matter because they always match.
	width = int(stats.SpriteSize >> 4 & 0xF)
	height = int(stats.SpriteSize >> 0 & 0xF)

	return
}

func (rip *Ripper) pokemonPalette(number int) color.Palette {
	var palettes [252][4]colorRGB15
	r := io.NewSectionReader(rip.r, rip.info.PalettePos, int64(binary.Size(&palettes)))
	err := binary.Read(r, binary.LittleEndian, &palettes)
	if err != nil {
		// BUG: shouldn't panic
		panic(err)
	}
	pal := color.Palette{
		color.White,
		palettes[number][0],
		palettes[number][1],
		color.Black,
	}
	return pal
}

// NewSectionReader returns an io.SectionReader that stops at the next bank boundary.
func newSectionReader(r io.ReaderAt, off int64) *io.SectionReader {
	return io.NewSectionReader(r, off, off&^0x3FFF+0x4000)
}

func (rip *Ripper) Pokemon(number int) (m *image.Paletted, err error) {
	if 1 > number || number > 251 {
		return nil, errors.New("Pokémon number out of range")
	}
	w, h := rip.pokemonSize(number)
	off := rip.pokemonOffset(number)
	pal := rip.pokemonPalette(number)
	log.Printf("Ripping sprite %d, size %dx%d, offset %x", number, w, h, off)
	r := newSectionReader(rip.r, off)
	m, err = Decode(r, w, h)
	if m != nil {
		m.Palette = pal
	}
	return
}

func (rip *Ripper) PokemonAnimation(number int) (g *gif.GIF, err error) {
	if 1 > number || number > 251 {
		return nil, errors.New("Pokémon number out of range")
	}

	frames, err := rip.pokemonFrames(number)
	if err != nil {
		return nil, err
	}

	off := readNearPointerAt(rip.r, rip.info.AnimPos, number-1)
	animdata, err := bufio.NewReader(newSectionReader(rip.r, off)).ReadBytes('\xFF')
	if err != nil {
		return nil, err
	}

	log.Printf("% x", animdata)

	g = new(gif.GIF)

	var loop, clock int
loop:
	for pc := 0; ; pc += 2 {
		//log.Println("PC", pc)
		switch animdata[pc] {
		case 0xFF:
			break loop
		case 0xFE:
			loop = int(animdata[pc+1])
		case 0xFD:
			if loop > 0 {
				pc = int(animdata[pc+1]) * 2
				pc -= 2
				loop--
			}
		default:
			delay := int(animdata[pc+1])
			g.Image = append(g.Image, frames[animdata[pc]])
			//g.Delay = append(g.Delay, delay*100/60)
			g.Delay = append(g.Delay, (clock+delay)*100/60 - clock*100/60)
			clock += delay
		}
	}
	g.Image = append(g.Image, frames[0])
	g.Delay = append(g.Delay, clock*2*100/60 - clock)

	return g, nil
}

func (rip *Ripper) pokemonFrames(number int) ([]*image.Paletted, error) {
	// TODO: Kinda want to just slurp in all the animation data for
	// every sprite at once. It's all in just a couple banks.

	w, h := rip.pokemonSize(number)
	palette := rip.pokemonPalette(number)

	s := rip.r.(io.Seeker)
	size, err := s.Seek(0, 2)
	if err != nil {
		return nil, err
	}

	r := io.NewSectionReader(rip.r, 0, size)

	r.Seek(rip.pokemonOffset(number), 0)
	tiledata, err := decodeTiles(bufio.NewReader(r), w*h*16)
	if err != nil {
		return nil, err
	}

	r.Seek(readNearPointerAt(r, rip.info.AnimPos, number-1), 0)
	animdata, err := bufio.NewReader(r).ReadBytes('\xFF')
	if err != nil {
		return nil, err
	}
	//fmt.Fprintf(os.Stderr, "%x\n", animdata)

	r.Seek(readNearPointerAt(r, rip.info.ExtraPos, number-1), 0)
	extradata, err := bufio.NewReader(r).ReadBytes('\xFF')
	if err != nil {
		return nil, err
	}

	// Find the number of frames by picking the highest frame in the anim data.
	var nframes int
	for i := 0; i < len(animdata); i += 2 {
		if animdata[i] < 0x80 && nframes < int(animdata[i]) {
			nframes = int(animdata[i])
		}
	}
	for i := 0; i < len(extradata); i += 2 {
		if extradata[i] < 0x80 && nframes < int(extradata[i]) {
			nframes = int(extradata[i])
		}
	}
	//fmt.Fprintf(os.Stderr, "%d frames\n", nframes)

	bitmaplen := (w*h + 7) / 8 // 1 pixel per tile
	bitmapdata := make([]byte, bitmaplen*nframes)
	off := readNearPointerAt(r, rip.info.BitmapsPos, number-1)
	_, err = r.ReadAt(bitmapdata, off)
	if err != nil {
		return nil, err
	}

	var frames = make([]*image.Paletted, nframes+1)
	var data = make([]uint8, w*h*16)
	var m = image.NewPaletted(image.Rect(0, 0, w*8, h*8), palette)
	untile(m, tiledata)
	frames[0] = m

	off = readNearPointerAt(r, rip.info.FramesPos, number-1)
	if number > 151 {
		off += 0x4000
	}

	fr := bufio.NewReader(nil)
	for i := 0; i < nframes; i++ {
		r.Seek(readNearPointerAt(r, off, i), 0)
		fr.Reset(r)
		bn, _ := fr.ReadByte()
		//fmt.Fprintf(os.Stderr, "bitmap %d\n", bn)
		if int(bn) > nframes {
			return nil, ErrMalformed
		}
		bitindex := uint(bn) * uint(bitmaplen) * 8
		for di := 0; di < len(data); di += 16 {
			bit := bitmapdata[bitindex/8] >> (bitindex % 8) & 1
			bitindex++
			si := di
			if bit != 0 {
				b, _ := fr.ReadByte()
				si = int(b) * 16
			}
			if si+16 > len(tiledata) {
				return nil, ErrMalformed
			}
			copy(data[di:di+16], tiledata[si:si+16])
		}
		m = image.NewPaletted(m.Rect, m.Palette)
		untile(m, data)
		frames[i+1] = m
	}
	return frames, nil
}

func (rip *Ripper) pokemonOffset(number int) (off int64) {
	var n int
	if number == 201 {
		off = rip.info.UnownSpritePos
		/*if form != "" && 'a' <= form[0] && form[0] <= 'z' {
			n = 2 * (form[0] - 'a')
		}*/
	} else {
		off = rip.info.SpritePos
		n = 2 * (number - 1)
	}
	off = readFarPointerAt(rip.r, off, n)
	if rip.info.Title == "PM_CRYSTAL" {
		off += 0x36 << 14
	} else {
		switch off >> 14 {
		case 0x13, 0x14:
			off += 0xC << 14
		case 0x1F:
			off += (0x2E - 0x1F) << 14
		}
	}

	return off
}

func readFarPointerAt(r io.ReaderAt, off int64, n int) int64 {
	var b [3]byte
	off += int64(len(b)) * int64(n)
	_, err := r.ReadAt(b[:], off)
	if err != nil {
		// BUG: shouldn't panic
		panic(err)
	}
	bank := int64(b[0])
	return bank<<14 + int64(b[2])&0x3F<<8 + int64(b[1])
}

func readNearPointerAt(r io.ReaderAt, off int64, n int) int64 {
	var b [2]byte
	off += int64(len(b)) * int64(n)
	_, err := r.ReadAt(b[:], off)
	if err != nil {
		// BUG: shouldn't panic
		panic(err)
	}
	p := off&^0x3FFF + int64(b[1])&0x3F<<8 + int64(b[0])
	return p
}

func main() {
	flag.Parse()

	game, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer game.Close()
	number, _ := strconv.Atoi(flag.Arg(1))
	//form := flag.Arg(2)

	rip, err := NewRipper(game)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	var (
		//r = rip.r
		info    = rip.info
		w, h    = rip.pokemonSize(number)
		palette = rip.pokemonPalette(number)
	)

	// Check that the file is seekable. After this we will assume that all
	// seeks succeed.
	if _, err := game.Seek(0, 0); err != nil {
		fmt.Println(err)
	}

	if info.AnimPos != 0 {
		for n := 1; n <= 251; n++ {
			g, err := rip.PokemonAnimation(n)
			if err != nil {
				fmt.Println(err)
				continue
			}

			f, err := os.Create(fmt.Sprintf("gscanim/%d.gif", n))
			if err != nil {
				fmt.Println(err)
				continue
			}
			gif.EncodeAll(f, g)
			f.Close()
		}
		return

		frames, err := rip.pokemonFrames(number)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}

		m := image.NewPaletted(image.Rect(0, 0, w*8*len(frames), h*8), palette)
		for i := 0; i < len(frames); i++ {
			r := frames[i].Rect.Add(image.Pt(w*8*i, 0))
			draw.Draw(m, r, frames[i], image.ZP, draw.Src)
		}
		png.Encode(os.Stdout, m)
	} else {
		m, err := rip.Pokemon(number)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		png.Encode(os.Stdout, m)
	}
}
