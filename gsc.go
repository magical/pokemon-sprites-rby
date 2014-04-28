/* Pokemon Gold/Silver/Crystal sprite ripper. */
package sprites

import (
	"bufio"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"io"
	//"log"
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

const MaxPokemon = 251

var (
	ErrMalformed     = errors.New("malformed data")
	ErrTooLarge      = errors.New("decompressed data is suspeciously large")
	ErrTooSmall      = errors.New("decompressed data is too short")
	ErrNoSuchPokemon = errors.New("Pokémon number out of range")
)

var UnownForms = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"}

// Decode a GSC image of dimensions w*8 x h*8.
func Decode(reader io.Reader, w, h int) (*image.Paletted, error) {
	data, err := decodeTiles(newByteReader(reader), w*h*16*2)
	if err != nil {
		return nil, err
	}
	if len(data) < w*h*16 {
		return nil, ErrTooSmall
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
					i := m.PixOffset(x+tx, y+ty)
					m.Pix[i] = uint8(pix & 3)
					pix >>= 2
				}
				i += 2
			}
		}
	}
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
	Title             string
	Version           string
	StatsOffset       int64
	PaletteOffset     int64
	SpriteOffset      int64
	UnownSpriteOffset int64

	AnimOffset         int64
	ExtraOffset        int64
	FramesOffset       int64
	BitmapsOffset      int64
	UnownAnimOffset    int64
	UnownExtraOffset   int64
	UnownFramesOffset  int64
	UnownBitmapsOffset int64
}

var romtab = map[string]romInfo{
	"POKEMON_GLD": {
		Title:             "POKEMON_GLD",
		Version:           "gold",
		StatsOffset:       0x51B0B,
		PaletteOffset:     0xAD3D,
		SpriteOffset:      0x48000,
		UnownSpriteOffset: 0x7C000,
	},
	"POKEMON_SLV": {
		// Same as Gold
		Title:             "POKEMON_SLV",
		Version:           "silver",
		StatsOffset:       0x51B0B,
		PaletteOffset:     0xAD3D,
		SpriteOffset:      0x48000,
		UnownSpriteOffset: 0x7C000,
	},
	"PM_CRYSTAL": {
		Title:             "PM_CRYSTAL",
		Version:           "crystal",
		StatsOffset:       0x51424,
		PaletteOffset:     0xA8CE,
		SpriteOffset:      0x120000,
		UnownSpriteOffset: 0x124000,

		AnimOffset:         0xD0695,
		ExtraOffset:        0xD16A3,
		UnownAnimOffset:    0xD2229,
		UnownExtraOffset:   0xD23D1,
		BitmapsOffset:      0xD24EF,
		UnownBitmapsOffset: 0xD3AD3,
		FramesOffset:       0xD4000,
		UnownFramesOffset:  0xD99A9,
	},
}

var defaultPalette = color.Palette{
	color.Gray{0xFF},
	color.Gray{0x7F},
	color.Gray{0x3F},
	color.Gray{0},
}

type Reader interface {
	io.Reader
	io.ReaderAt
	io.Seeker
}

type Ripper struct {
	r    Reader
	buf  *bufferedReader
	info romInfo
}

func NewRipper(r Reader) (_ *Ripper, err error) {
	rip := new(Ripper)
	rip.r = r
	rip.buf = newBufferedReader(r)

	// Check that the reader is seekable. After this we will assume that all
	// seeks succeed.
	if _, err := r.Seek(0, 0); err != nil {
		return nil, err
	}

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

// BufferedReader is a *bufio.Reader that can Seek.
type bufferedReader struct {
	r io.ReadSeeker
	*bufio.Reader
}

func newBufferedReader(r io.ReadSeeker) *bufferedReader {
	return &bufferedReader{r, bufio.NewReader(r)}
}

// Seek sets the position of the underlying reader and resets the buffer.
func (b *bufferedReader) Seek(off int64, whence int) {
	b.r.Seek(off, whence)
	b.Reset(b.r)
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
	off := rip.info.StatsOffset + size*int64(number-1)
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

// PokemonPalette returns the color palette for a Pokémon,
// or nil if there is an error.
func (rip *Ripper) PokemonPalette(number int) color.Palette {
	if 1 > number || number > MaxPokemon {
		return nil
	}
	return rip.pokemonPalette(number, normal)
}

func (rip *Ripper) ShinyPalette(number int) color.Palette {
	if 1 > number || number > MaxPokemon {
		return nil
	}
	return rip.pokemonPalette(number, shiny)
}

const (
	normal = false
	shiny = true
)

func (rip *Ripper) pokemonPalette(number int, shiny bool) color.Palette {
	var palette [4]RGB15
	off := rip.info.PaletteOffset + int64(binary.Size(&palette))*int64(number)
	r := io.NewSectionReader(rip.r, off, int64(binary.Size(&palette)))
	err := binary.Read(r, binary.LittleEndian, &palette)
	if err != nil {
		return nil
	}
	if shiny {
		return color.Palette{
			color.White,
			palette[2],
			palette[3],
			color.Black,
		}
	} else {
		return color.Palette{
			color.White,
			palette[0],
			palette[1],
			color.Black,
		}
	}
}

type RGB15 uint16

func (rgb RGB15) RGBA() (r, g, b, a uint32) {
	r = (uint32(rgb>>0&31)*0xFFFF + 15) / 31
	g = (uint32(rgb>>5&31)*0xFFFF + 15) / 31
	b = (uint32(rgb>>10&31)*0xFFFF + 15) / 31
	a = 0xFFFF
	return
}

func (rip *Ripper) Pokemon(number int) (m *image.Paletted, err error) {
	if 1 > number || number > MaxPokemon {
		return nil, ErrNoSuchPokemon
	}
	w, h := rip.pokemonSize(number)
	off := rip.pokemonOffset(number, 0, front)
	pal := rip.pokemonPalette(number, normal)
	//log.Printf("Ripping sprite %d, size %dx%d, offset %x", number, w, h, off)
	rip.buf.Seek(off, 0)
	m, err = Decode(rip.buf, w, h)
	if m != nil {
		m.Palette = pal
	}
	return
}

func (rip *Ripper) PokemonBack(number int) (m *image.Paletted, err error) {
	if 1 > number || number > MaxPokemon {
		return nil, ErrNoSuchPokemon
	}
	w, h := 6, 6
	off := rip.pokemonOffset(number, 0, back)
	pal := rip.pokemonPalette(number, normal)
	rip.buf.Seek(off, 0)
	m, err = Decode(rip.buf, w, h)
	if m != nil {
		m.Palette = pal
	}
	return
}

func (rip *Ripper) Unown(form string) (m *image.Paletted, err error) {
	if len(form) != 1 || 'a' > form[0] || form[0] > 'z' {
		return nil, ErrNoSuchPokemon
	}
	formi := int(form[0] - 'a')
	w, h := rip.pokemonSize(201)
	off := rip.pokemonOffset(201, formi, front)
	pal := rip.pokemonPalette(201, normal)
	rip.buf.Seek(off, 0)
	m, err = Decode(rip.buf, w, h)
	if m != nil {
		m.Palette = pal
	}
	return
}

func (rip *Ripper) UnownBack(form string) (m *image.Paletted, err error) {
	if len(form) != 1 || 'a' > form[0] || form[0] > 'z' {
		return nil, ErrNoSuchPokemon
	}
	formi := int(form[0] - 'a')
	w, h := 6, 6
	off := rip.pokemonOffset(201, formi, back)
	pal := rip.pokemonPalette(201, normal)
	rip.buf.Seek(off, 0)
	m, err = Decode(rip.buf, w, h)
	if m != nil {
		m.Palette = pal
	}
	return
}

func (rip *Ripper) HasAnimations() bool {
	return rip.info.AnimOffset != 0
}

func (rip *Ripper) Version() string {
	return rip.info.Version
}

func (rip *Ripper) PokemonAnimation(number int) (g *gif.GIF, err error) {
	if 1 > number || number > MaxPokemon {
		return nil, ErrNoSuchPokemon
	}

	frames, err := rip.pokemonFrames(number)
	if err != nil {
		return nil, err
	}

	rip.buf.Seek(readNearPointerAt(rip.r, rip.info.AnimOffset, number-1), 0)
	animdata, err := rip.buf.ReadBytes('\xFF')
	if err != nil {
		return nil, err
	}

	g = animate(animdata, frames)
	return g, err
}

func (rip *Ripper) UnownAnimation(form string) (g *gif.GIF, err error) {
	if len(form) != 1 || 'a' > form[0] || form[0] > 'z' {
		return nil, ErrNoSuchPokemon
	}
	formi := int(form[0] - 'a')

	frames, err := rip.unownFrames(formi)
	if err != nil {
		return nil, err
	}

	rip.buf.Seek(readNearPointerAt(rip.r, rip.info.UnownAnimOffset, formi), 0)
	animdata, err := rip.buf.ReadBytes('\xFF')
	if err != nil {
		return nil, err
	}

	g = animate(animdata, frames)
	return g, nil
}

func animate(animdata []byte, frames []*image.Paletted) *gif.GIF {
	var g gif.GIF
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
			g.Delay = append(g.Delay, (clock+delay)*100/60-clock*100/60)
			clock += delay
		}
	}
	g.Image = append(g.Image, frames[0])
	g.Delay = append(g.Delay, clock*2*100/60-clock)
	return &g
}

func (rip *Ripper) PokemonFrames(number int) ([]*image.Paletted, error) {
	if 1 > number || number > MaxPokemon {
		return nil, ErrNoSuchPokemon
	}
	return rip.pokemonFrames(number)
}
func (rip *Ripper) pokemonFrames(number int) ([]*image.Paletted, error) {
	// TODO: Kinda want to just slurp in all the animation data for
	// every sprite at once. It's all in just a couple banks.
	// OTOH, profiling shows that this isn't a bottleneck.
	offsets := animOffsets{
		Sprite:  rip.pokemonOffset(number, 0, front),
		Anim:    readNearPointerAt(rip.r, rip.info.AnimOffset, number-1),
		Extra:   readNearPointerAt(rip.r, rip.info.ExtraOffset, number-1),
		Frames:  readNearPointerAt(rip.r, rip.info.FramesOffset, number-1),
		Bitmaps: readNearPointerAt(rip.r, rip.info.BitmapsOffset, number-1),
	}
	if number > 151 {
		offsets.Frames += 0x4000
	}
	w, h := rip.pokemonSize(number)
	palette := rip.pokemonPalette(number, normal)
	return rip.frames(offsets, palette, w, h)
}

func (rip *Ripper) unownFrames(form int) ([]*image.Paletted, error) {
	offsets := animOffsets{
		Sprite:  rip.pokemonOffset(201, form, front),
		Anim:    readNearPointerAt(rip.r, rip.info.UnownAnimOffset, form),
		Extra:   readNearPointerAt(rip.r, rip.info.UnownExtraOffset, form),
		Frames:  readNearPointerAt(rip.r, rip.info.UnownFramesOffset, form),
		Bitmaps: readNearPointerAt(rip.r, rip.info.UnownBitmapsOffset, form),
	}
	w, h := rip.pokemonSize(201)
	palette := rip.pokemonPalette(201, normal)
	return rip.frames(offsets, palette, w, h)
}

type animOffsets struct {
	Sprite  int64
	Anim    int64
	Extra   int64
	Frames  int64
	Bitmaps int64
}

func (rip *Ripper) frames(offsets animOffsets, palette color.Palette, w, h int) ([]*image.Paletted, error) {
	buf := rip.buf

	buf.Seek(offsets.Sprite, 0)
	tiledata, err := decodeTiles(buf, w*h*16*2)
	if err != nil {
		return nil, err
	}

	buf.Seek(offsets.Anim, 0)
	animdata, err := buf.ReadBytes('\xFF')
	if err != nil {
		return nil, err
	}
	//fmt.Fprintf(os.Stderr, "%x\n", animdata)

	buf.Seek(offsets.Extra, 0)
	extradata, err := buf.ReadBytes('\xFF')
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
	_, err = rip.r.ReadAt(bitmapdata, offsets.Bitmaps)
	if err != nil {
		return nil, err
	}

	var frames = make([]*image.Paletted, nframes+1)
	var data = make([]uint8, w*h*16)
	var m = image.NewPaletted(image.Rect(0, 0, w*8, h*8), palette)
	untile(m, tiledata)
	frames[0] = m
	for i := 0; i < nframes; i++ {
		buf.Seek(readNearPointerAt(rip.r, offsets.Frames, i), 0)
		bn, _ := buf.ReadByte()
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
				b, _ := buf.ReadByte()
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

const (
	front = iota
	back
)

func (rip *Ripper) pokemonOffset(number int, form int, facing int) (off int64) {
	base := rip.info.SpriteOffset
	n := number - 1
	if number == 201 {
		base = rip.info.UnownSpriteOffset
		n = form
	}
	off = readFarPointerAt(rip.r, base, 2*n+facing)

	if rip.info.Title == "PM_CRYSTAL" {
		off += 0x36 << 14
	} else {
		switch off >> 14 {
		case 0x13, 0x14:
			off += 0xC << 14
		case 0x1F:
			off += 0xF << 14
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
