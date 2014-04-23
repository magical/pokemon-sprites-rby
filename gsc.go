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
	"image/png"
	"io"
	"os"
	"strconv"
	"strings"
)

var ErrMalformed = errors.New("malformed data")

func Decode(reader io.Reader, width, height int) (image.Image, error) {
	var r = bufio.NewReader(reader)
	var m = image.NewPaletted(image.Rect(0, 0, width, height), nil)
	var data = make([]byte, 0, width*height*4)

	var readErr error
	readByte := func() (b byte) {
		if readErr == nil {
			b, readErr = r.ReadByte()
		}
		return b
	}

	for len(data) < width*height/4 {
		var control, num, seek int
		num = int(readByte())
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
	for i, x := 0, 0; x < width/8; x++ {
		for y := 0; y < height/8; y++ {
			for ty := 0; ty < 8; ty++ {
				pix := mingle(uint16(data[i]), uint16(data[i+1]))
				for tx := 7; tx >= 0; tx-- {
					xx := x*8 + tx
					yy := y*8 + ty
					m.SetColorIndex(xx, yy, uint8(pix&3))
					pix >>= 2
				}
				i += 2
			}
		}
	}
	return m, nil
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
	title      string
	statsPos   int64
	palettePos int64
	spritePos  int64
	unownPos   int64
}{
	"POKEMON_GLD": {
		title:      "POKEMON_GLD",
		statsPos:   0x51B0B,
		palettePos: 0xAD3D,
		spritePos:  0x48000,
		unownPos:   0x7C000,
	},
	"POKEMON_SLV": {
		// Same as Gold
		title:      "POKEMON_SLV",
		statsPos:   0x51B0B,
		palettePos: 0xAD3D,
		spritePos:  0x48000,
		unownPos:   0x7C000,
	},
	"PM_CRYSTAL": {
		title:      "PM_CRYSTAL",
		statsPos:   0x51424,
		palettePos: 0xA8CE,
		spritePos:  0x120000,
		unownPos:   0x124000,
	},
}

type Ripper struct {
}

var defaultPalette = color.Palette{
	color.Gray{0xFF},
	color.Gray{0x7F},
	color.Gray{0x3F},
	color.Gray{0},
}

// BUG: This should really return an error
func readFarPointer(r io.Reader) (bank int64, p int64) {
	var b [3]byte
	_, err := r.Read(b[:])
	if err != nil {
		panic(err)
	}
	bank = int64(b[0])
	p = int64(b[1]) + int64(b[2])<<8
	return
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

func main() {
	flag.Parse()

	game, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Println(err)
		return
	}
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
	game.Seek(info.statsPos, 0)
	err = binary.Read(game, binary.LittleEndian, stats[:])
	if err != nil {
		fmt.Println(err)
		return
	}
	width, height := int(stats[number].SpriteSize>>4), int(stats[number].SpriteSize&15)

	var palettes [252][4]colorRGB15
	game.Seek(info.palettePos+8, 0)
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
		game.Seek(info.unownPos, 0)
		if form != "" && 'a' <= form[0] && form[0] <= 'z' {
			game.Seek(int64(form[0]-'a')*6, 1)
		}
	} else {
		game.Seek(info.spritePos+6*int64(number), 0)
	}

	bank, p := readFarPointer(game)
	if title == "PM_CRYSTAL" {
		bank += 0x36
	} else {
		switch bank {
		case 0x13:
			bank = 0x1F
		case 0x14:
			bank = 0x20
		case 0x1f:
			bank = 0x2E
		}
	}
	p = bank<<14 + p&^0x4000
	_, err = game.Seek(p, 0)
	if err != nil {
		fmt.Println(err)
		return
	}
	m, err := Decode(game, width*8, height*8)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	m.(*image.Paletted).Palette = palette
	png.Encode(os.Stdout, m)
}
