// +build ignore

package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"os"
	"strconv"

	"github.com/magical/png"
	"github.com/magical/sprites"
)

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

	rip, err := sprites.NewRipper(game)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	if rip.HasAnimations() {
		if true {
			g, err := rip.PokemonAnimation(number)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return
			}
			gif.EncodeAll(os.Stdout, g)
		} else {
			frames, err := rip.PokemonFrames(number)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return
			}

			w, h := frames[0].Rect.Dx(), frames[0].Rect.Dy()
			m := image.NewPaletted(image.Rect(0, 0, w*len(frames), h), frames[0].Palette)
			for i := 0; i < len(frames); i++ {
				r := frames[i].Rect.Add(image.Pt(w*i, 0))
				draw.Draw(m, r, frames[i], image.ZP, draw.Src)
			}
			png.EncodeWithSBIT(os.Stdout, m, 5)
		}
	} else {
		m, err := rip.Pokemon(number)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		png.EncodeWithSBIT(os.Stdout, m, 5)
	}
}
