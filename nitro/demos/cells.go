package main

import (
	"fmt"
	"image/color"
	"image/png"
	"os"

	"github.com/magical/sprites/nitro"
)

func die(v ...interface{}) {
	fmt.Fprintln(os.Stderr, v...)
	os.Exit(1)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s path/to/a.narc", os.Args[0])
		return
	}
	filename := os.Args[1]

	f, err := os.Open(filename)
	if err != nil {
		die(err)
	}
	defer f.Close()
	narc, err := nitro.ReadNARC(f)
	if err != nil {
		die(err)
	}
	const poke = 643
	g, err := narc.OpenNCGR(poke*20 + 2)
	if err != nil {
		die("OpenNCGR:", err)
	}
	p, err := narc.OpenNCLR(poke*20 + 18)
	if err != nil {
		die("OpenNCLR:", err)
	}
	c, err := narc.OpenNCER(poke*20 + 4)
	if err != nil {
		die("OpenNCER:", err)
	}
	pal := p.Palette(0)
	pal[0] = setAlpha(pal[0])
	for i := 0; i < c.Len(); i++ {
		m := c.Cell(i, g, pal)
		f, _ := os.Create(fmt.Sprintf("cell-%d.png", i))
		png.Encode(f, m)
		f.Close()
	}
}

func setAlpha(c color.Color) color.Color {
	nrgba := color.NRGBAModel.Convert(c).(color.NRGBA)
	nrgba.A = 0
	return nrgba
}
