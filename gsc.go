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
	"image/png"
	"io"
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
	data, err := decodeTiles(reader, w*h*16)
	if err != nil {
		return nil, err
	}
	var m = image.NewPaletted(image.Rect(0, 0, w*8, h*8), nil)
	untile(m, data, w, h)
	return m, nil
}

func decodeTiles(reader io.Reader, sizehint int) ([]byte, error) {
	var r = bufio.NewReader(reader)
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
		if num == 0xff {
			break
		}
		if len(data) > 0xffff {
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
			var b [2]byte
			r.Read(b[:])
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
	if readErr != nil {
		return nil, readErr
	}
	return data, readErr
}

func untile(m *image.Paletted, data []byte, w, h int) {
	for i, x := 0, 0; x < w*8; x += 8 {
		for y := 0; y < h*8; y += 8 {
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

var romtab = map[string]struct {
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
}{
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

// BUG: This should really return an error
func readFarPointer(r io.Reader) int64 {
	var b [3]byte
	_, err := r.Read(b[:])
	if err != nil {
		panic(err)
	}
	bank := int64(b[0])
	return bank<<14 + int64(b[2])&0x3F<<8 + int64(b[1])
}

func readNearPointer(r io.Reader, bank int64) int64 {
	var b [2]byte
	_, err := r.Read(b[:])
	if err != nil {
		panic(err)
	}
	p := bank<<14 + int64(b[1])&0x3F<<8 + int64(b[0])
	//fmt.Fprintf(os.Stderr, "Read pointer %#x (%+x)\n", p, b)
	return p
}

// Seek to a position designated by a pointer in an array at base.
func seekIndirect(r io.ReadSeeker, base int64, n int) {
	p := base + int64(n)*2
	//fmt.Fprintf(os.Stderr, "Seeking to %x+%d*2 = %x\n", base, n, p)
	r.Seek(p, 0)
	p = readNearPointer(r, base>>14)
	r.Seek(p, 0)
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
	form := flag.Arg(2)

	var header [0x150]byte
	game.Read(header[:])
	title := string(header[0x134:0x13F])
	title = strings.TrimRight(title, "\x00")

	info, ok := romtab[title]
	if !ok {
		fmt.Println("Couldn't recognize ROM")
		return
	}

	if number > 250 {
		fmt.Println("Pokemon number out of range")
		return
	}

	// Check that the file is seekable. After this we will assume that all
	// seeks succeed.
	if _, err := game.Seek(0, 0); err != nil {
		fmt.Println(err)
	}

	// Base stats structure. We only care about SpriteSize, but what the heck.
	var stats [251]struct {
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
	game.Seek(info.StatsPos, 0)
	err = binary.Read(game, binary.LittleEndian, stats[:])
	if err != nil {
		fmt.Println(err)
		return
	}
	// The high and low nibbles of SpriteSize give the width and height of
	// the sprite in 8x8 tiles. Not sure which is which, but it doesn't
	// matter because they always match.
	wh := int(stats[number].SpriteSize) >> 4

	var palettes [252][4]colorRGB15
	game.Seek(info.PalettePos+8, 0)
	err = binary.Read(game, binary.LittleEndian, &palettes)
	if err != nil {
		fmt.Println(err)
		return
	}

	palette := color.Palette{
		color.Gray{0xFF},
		palettes[number][0],
		palettes[number][1],
		color.Gray{0},
	}

	if number == 200 {
		game.Seek(info.UnownSpritePos, 0)
		if form != "" && 'a' <= form[0] && form[0] <= 'z' {
			game.Seek(int64(form[0]-'a')*6, 1)
		}
	} else {
		game.Seek(info.SpritePos+6*int64(number), 0)
	}
	spritePos := readFarPointer(game)
	if title == "PM_CRYSTAL" {
		spritePos += 0x36 << 14
	} else {
		switch spritePos >> 14 {
		case 0x13, 0x14:
			spritePos += 0xC << 14
		case 0x1F:
			spritePos += (0x2E - 0x1F) << 14
		}
	}

	if info.AnimPos != 0 {
		game.Seek(spritePos, 0)
		tiledata, err := decodeTiles(game, wh*wh*16)
		if err != nil {
			fmt.Println(err)
			return
		}
		var tiles []*image.Paletted
		for i := 0; i < len(tiledata); i += 16 {
			m := image.NewPaletted(image.Rect(0, 0, 8, 8), palette)
			untile(m, tiledata[i:i+16], 1, 1) // seems like overkill
			tiles = append(tiles, m)
		}

		// TODO: Kinda want to just slurp in all the animation data for
		// every sprite at once. It's stored in just a couple banks.

		seekIndirect(game, info.AnimPos, number)
		animdata, err := bufio.NewReader(game).ReadBytes('\xFF')
		if err != nil {
			fmt.Println(err)
			return
		}
		//fmt.Fprintf(os.Stderr, "%x\n", animdata)

		seekIndirect(game, info.ExtraPos, number)
		extradata, err := bufio.NewReader(game).ReadBytes('\xFF')
		if err != nil {
			fmt.Println(err)
			return
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

		seekIndirect(game, info.BitmapsPos, number)
		bitmaplen := (wh*wh + 7) / 8 // 1 pixel per tile
		bitmapdata := make([]byte, bitmaplen*nframes)
		_, err = game.Read(bitmapdata)
		if err != nil {
			fmt.Println(err)
			return
		}

		var frameptrs []int64
		game.Seek(info.FramesPos+int64(number)*2, 0)
		bank := info.FramesPos >> 14
		if number > 150 {
			bank++
		}
		game.Seek(readNearPointer(game, bank), 0)
		//seekIndirect(game, info.FramesPos, number)
		for i := 0; i < nframes; i++ {
			frameptrs = append(frameptrs, readNearPointer(game, bank))
		}
		var frames []*image.Paletted
		rect := image.Rect(0, 0, wh*8, wh*8)
		m := image.NewPaletted(rect, palette)
		untile(m, tiledata, wh, wh)
		frames = append(frames, m)
		for i := 0; i < nframes; i++ {
			m = image.NewPaletted(rect, palette)
			game.Seek(frameptrs[i], 0)
			fr := bufio.NewReader(game)
			bn, _ := fr.ReadByte()
			//fmt.Fprintf(os.Stderr, "bitmap %d\n", bn)
			bitindex := uint(bn) * uint(bitmaplen) * 8
			ti := 0
			for x := 0; x < wh; x++ {
				for y := 0; y < wh; y++ {
					tile := tiles[ti]
					ti++
					bit := bitmapdata[bitindex/8] >> (bitindex % 8) & 1
					bitindex++
					if bit != 0 {
						a, _ := fr.ReadByte()
						tile = tiles[a]
					}
					draw.Draw(m, tile.Rect.Add(image.Pt(x*8, y*8)), tile, image.ZP, draw.Src)
				}
			}
			frames = append(frames, m)
		}
		nframes++
		m = image.NewPaletted(image.Rect(0, 0, wh*8*nframes, wh*8), palette)
		for i := 0; i < nframes; i++ {
			r := frames[i].Rect.Add(image.Pt(wh*8*i, 0))
			draw.Draw(m, r, frames[i], image.ZP, draw.Src)
		}
		png.Encode(os.Stdout, m)
	} else {
		game.Seek(spritePos, 0)
		m, err := Decode(game, wh, wh)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		m.Palette = palette
		png.Encode(os.Stdout, m)
	}
}

func decodeAnim(animdata []byte, frames []*image.Paletted, w, h int) {
	var anim []*image.Paletted
	var loopcount int
	for pc := 0; ; pc += 2 {
		switch animdata[pc] {
		case 0xFF:
			break
		case 0xFE:
			loopcount = int(animdata[pc])
		case 0xFD:
			if loopcount > 0 {
				pc = int(animdata[pc]) * 2
			}
		default:
			framenum, duration := animdata[pc], int(animdata[pc+1])
			anim = append(anim, frames[framenum])
			for i := 1; i < duration; i++ {
				anim = append(anim, frames[framenum])
			}
		}
	}
}
