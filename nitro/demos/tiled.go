// +build ignore

package main

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"strconv"

	"github.com/magical/sprites/nitro"
)

func die(v interface{}) {
	fmt.Fprintln(os.Stderr, v)
	os.Exit(1)
}

func main() {
	if len(os.Args) != 4 {
		fmt.Println("Usage: tiled path/to/a.narc number palette")
		return
	}
	filename := os.Args[1]
	number, err := strconv.ParseInt(os.Args[2], 0, 0)
	if err != nil {
		die(err)
	}
	npalette, err := strconv.ParseInt(os.Args[3], 0, 0)
	if err != nil {
		die(err)
	}

	f, err := os.Open(filename)
	if err != nil {
		die(err)
	}
	defer f.Close()
	narc, err := nitro.ReadNARC(f)
	if err != nil {
		die(err)
	}
	n, err := narc.OpenNCGR(int(number))
	if err != nil {
		die(err)
	}
	nclr, err := narc.OpenNCLR(int(npalette))
	if err != nil {
		die(err)
	}
	pal := nclr.Palette(0)
	fmt.Fprintln(os.Stderr, n.Bounds())
	//m := n.Tile(0, 96, 96
	//m.Palette = nclr.Palette(0)
	m := image.NewPaletted(image.Rect(0, 0, 96, 96), pal)
	//m := nitro.NewTiled(image.Rect(0, 0, 96, 96), pal)
	draw.Draw(m, image.Rect(0, 0, 64, 64), n.Tile(0, 64, 64, pal), image.ZP, draw.Src)
	draw.Draw(m, image.Rect(64, 0, 96, 64), n.Tile(8, 32, 64, pal), image.ZP, draw.Src)
	draw.Draw(m, image.Rect(0, 64, 64, 96), n.Tile(256, 64, 32, pal), image.ZP, draw.Src)
	draw.Draw(m, image.Rect(64, 64, 96, 96), n.Tile(264, 32, 32, pal), image.ZP, draw.Src)
	png.Encode(os.Stdout, m)
}
