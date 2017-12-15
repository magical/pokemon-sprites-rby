// +build ignore

package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"log"
	"os"

	"github.com/magical/png"
	"github.com/magical/sprites/gb"
)

// TODO: get rid of this dumb hack
var fakeGbcPalettes []color.Palette

// GrayPalette is a simple linear grayscale palette.
var grayPalette = color.Palette{
	color.Gray{255},
	color.Gray{170},
	color.Gray{85},
	color.Gray{0},
}

// GameboyPalette is supposed to emulate the yellow-green look of the original
// gameboy.
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

// gbPokemonRedPalette is the default GBC palette for Pokemon Red.
var gbPokemonRedPalette = color.Palette{
	color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	color.NRGBA{R: 0xff, G: 0x84, B: 0x84, A: 0xff},
	color.NRGBA{R: 0x94, G: 0x3a, B: 0x3a, A: 0xff},
	color.NRGBA{R: 0x0, G: 0x0, B: 0x0, A: 0xff},
}

//{color.NRGBA{R:0xff, G:0xff, B:0xff, A:0xff}, color.NRGBA{R:0x7b, G:0xff, B:0x31, A:0xff}, color.NRGBA{R:0x0, G:0x84, B:0x0, A:0xff}, color.NRGBA{R:0x0, G:0x0, B:0x0, A:0xff}},
//{color.NRGBA{R:0xff, G:0xff, B:0xff, A:0xff}, color.NRGBA{R:0xff, G:0x84, B:0x84, A:0xff}, color.NRGBA{R:0x94, G:0x3a, B:0x3a, A:0xff}, color.NRGBA{R:0x0, G:0x0, B:0x0, A:0xff}},

// gbPokemonGreenPalette is the default GBC palette for Pokemon Green.
var gbPokemonGreenPalette = color.Palette{
	color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	color.NRGBA{R: 0x7b, G: 0xff, B: 0x31, A: 0xff},
	color.NRGBA{R: 0x0, G: 0x63, B: 0xc5, A: 0xff},
	color.NRGBA{R: 0x0, G: 0x0, B: 0x0, A: 0xff},
}

//{color.NRGBA{R:0xff, G:0xff, B:0xff, A:0xff}, color.NRGBA{R:0xff, G:0x84, B:0x84, A:0xff}, color.NRGBA{R:0x94, G:0x3a, B:0x3a, A:0xff}, color.NRGBA{R:0x0, G:0x0, B:0x0, A:0xff}},
//{color.NRGBA{R:0xff, G:0xff, B:0xff, A:0xff}, color.NRGBA{R:0x7b, G:0xff, B:0x31, A:0xff}, color.NRGBA{R:0x0, G:0x63, B:0xc5, A:0xff}, color.NRGBA{R:0x0, G:0x0, B:0x0, A:0xff}},

// gbPokemonBluePalette is the default GBC palette for Pokemon Blue.
var gbPokemonBluePalette = color.Palette{
	color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	color.NRGBA{R: 0x63, G: 0xa5, B: 0xff, A: 0xff},
	color.NRGBA{R: 0x0, G: 0x0, B: 0xff, A: 0xff},
	color.NRGBA{R: 0x0, G: 0x0, B: 0x0, A: 0xff},
}

//{color.NRGBA{R:0xff, G:0xff, B:0xff, A:0xff}, color.NRGBA{R:0xff, G:0x84, B:0x84, A:0xff}, color.NRGBA{R:0x94, G:0x3a, B:0x3a, A:0xff}, color.NRGBA{R:0x0, G:0x0, B:0x0, A:0xff}},
//{color.NRGBA{R:0xff, G:0xff, B:0xff, A:0xff}, color.NRGBA{R:0x63, G:0xa5, B:0xff, A:0xff}, color.NRGBA{R:0x0, G:0x0, B:0xff, A:0xff}, color.NRGBA{R:0x0, G:0x0, B:0x0, A:0xff}},

// gbPokemonYellowPalette is the default GBC palette for Pokemon Yellow (Japanese).
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
		rip, err := gb.NewRBYRipper(f)
		if err != nil {
			continue
		}

		if rip.CGBPalettes() != nil {
			fakeGbcPalettes = rip.CGBPalettes()
			break
		}
	}

	for _, filename := range flag.Args() {
		f, err := os.Open(filename)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		rip, err := gb.NewRBYRipper(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		header := rip.Header()
		path := "out"
		os.MkdirAll(path, 0777)
		var gbcPalette color.Palette
		if rip.CGBPalettes() == nil {
			switch header.Version {
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
			dst, err := os.Create(path + "/" + header.Lang + "-" + header.Version + "-" + sys.system + ".png")
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

func montage(rip *gb.RBYRipper, w io.Writer, pal color.Palette, sys string) {
	oldPalettes := rip.CGBPalettes()
	if sys == "fakegbc" {
		rip.SetCGBPalettes(fakeGbcPalettes)
	}

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

	if sys == "fakegbc" {
		rip.SetCGBPalettes(oldPalettes)
	}
}
