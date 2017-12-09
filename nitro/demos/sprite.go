package main

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/magical/png"
	"github.com/magical/sprites/nitro"
)

var _ = fmt.Println

var Frame1Rect = image.Rect(0, 0, 80, 80)
var Frame2Rect = image.Rect(80, 0, 160, 80)

func main() {
	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	narc, err := nitro.ReadNARC(f)
	if err != nil {
		panic(err)
	}

	number := 3

	ncgr, err := narc.OpenNCGR(number*6 + 2)
	if err != nil {
		panic(err)
	}
	nclr, err := narc.OpenNCLR(number*6 + 4)
	if err != nil {
		panic(err)
	}
	ncgr.DecryptReverse()
	pal := nclr.Palette(0)
	pal[0] = setTransparent(pal[0])
	//fmt.Fprintln(os.Stderr, pal)
	m := ncgr.Image(pal).SubImage(Frame1Rect)
	//fmt.Fprintln(os.Stderr, m.(*image.Paletted).Pix)
	png.EncodeWithSBIT(os.Stdout, m, 5)
}

func setTransparent(c color.Color) color.Color {
	nrgba := color.NRGBAModel.Convert(c).(color.NRGBA)
	nrgba.A = 0
	return nrgba
}
