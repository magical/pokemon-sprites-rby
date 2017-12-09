// +build ignore

package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"

	"github.com/magical/png"
	"github.com/magical/sprites/gb"
)

var (
	animFlag    bool
	framesFlag  bool
	trainerFlag bool
	batch       bool
	number      int
	outname     string
	profile     string
)

func main() {
	flag.BoolVar(&batch, "all", false, "rip all sprites")
	flag.BoolVar(&animFlag, "anim", false, "rip animation")
	flag.BoolVar(&framesFlag, "frames", false, "rip frames")
	flag.BoolVar(&trainerFlag, "trainer", false, "rip trainer")
	flag.IntVar(&number, "n", 0, "number of pokemon")
	flag.StringVar(&outname, "out", "", "output file or directory")
	flag.StringVar(&profile, "profile", "", "save profile data")
	flag.Parse()

	if profile != "" {
		f, err := os.Create(profile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	var err error
	if batch {
		err = ripBatch()
	} else {
		err = ripSingle()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func ripSingle() error {
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		return err
	}
	defer f.Close()

	rip, err := gb.NewRipper(f)
	if err != nil {
		return err
	}

	if trainerFlag {
		m, err := rip.Trainer(number)
		if err != nil {
			return err
		}
		return write(m, outname)
	} else if animFlag && rip.HasAnimations() {
		g, err := rip.PokemonAnimation(number)
		if err != nil {
			return err
		}
		return write(g, outname)
	} else if framesFlag && rip.HasAnimations() {
		frames, err := rip.PokemonFrames(number)
		if err != nil {
			return err
		}

		w, h := frames[0].Rect.Dx(), frames[0].Rect.Dy()
		m := image.NewPaletted(image.Rect(0, 0, w*len(frames), h), frames[0].Palette)
		for i := 0; i < len(frames); i++ {
			r := frames[i].Rect.Add(image.Pt(w*i, 0))
			draw.Draw(m, r, frames[i], image.ZP, draw.Src)
		}

		return write(m, outname)
	} else {
		m, err := rip.Pokemon(number)
		if err != nil {
			return err
		}
		return write(m, outname)
	}
}

func setPalette(v interface{}, pal color.Palette) {
	switch v := v.(type) {
	case *image.Paletted:
		v.Palette = pal
	case *gif.GIF:
		for _, m := range v.Image {
			m.Palette = pal
		}
	default:
		panic("unknown type")
	}
}

// Write writes a *image.Paletted or *gif.GIF to the file named by outname.
// If outname is "", it writes to os.Stdout.
func write(v interface{}, outname string) (err error) {
	f := os.Stdout
	if outname != "" {
		f, err = os.Create(outname)
		if err != nil {
			return err
		}
		defer f.Close()
	}
	switch v := v.(type) {
	case *image.Paletted:
		return png.EncodeWithSBIT(f, v, 5)
	case *gif.GIF:
		return gif.EncodeAll(f, v)
	default:
		panic("unexpected type")
	}
}

func ripBatch() error {
	for _, filename := range flag.Args() {
		err := ripBatchFilename(filename)
		if err != nil {
			log.Println(err)
		}
	}
	return nil
}

func ripBatchFilename(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	rip, err := gb.NewRipper(f)
	if err != nil {
		return err
	}
	version := rip.Version()
	outdir := filepath.Join(outname, version)
	var things = []struct {
		fn      func(rip *gb.Ripper, n int, form string, outname string) error
		dirname string
		ext     string
		enabled bool
	}{
		{ripPokemon, "", ".png", true},
		{ripPokemonBack, "back", ".png", true},
		{ripShinyPokemon, "shiny", ".png", true},
		{ripShinyPokemonBack, "back/shiny", ".png", true},
		{ripAnimation, "animated", ".gif", rip.HasAnimations()},
		{ripShinyAnimation, "animated/shiny", ".gif", rip.HasAnimations()},
	}
	for _, t := range things {
		if t.enabled {
			err = os.MkdirAll(filepath.Join(outdir, filepath.FromSlash(t.dirname)), 0777)
			if err != nil && !os.IsExist(err) {
				return err
			}
		}
	}
	for n := 1; n <= gb.MaxPokemon; n++ {
		for _, t := range things {
			if !t.enabled {
				continue
			}
			name := filepath.Join(filepath.FromSlash(t.dirname), strconv.Itoa(n))
			err := t.fn(rip, n, "", filepath.Join(outdir, name+t.ext))
			if err != nil {
				log.Printf("%s: %s", name, err)
			}
		}
	}
	for _, form := range gb.UnownForms {
		for _, t := range things {
			if !t.enabled {
				continue
			}
			name := filepath.Join(filepath.FromSlash(t.dirname), "201-"+form)
			err := t.fn(rip, 201, form, filepath.Join(outdir, name+t.ext))
			if err != nil {
				log.Printf("%s: %s", name, err)
			}
		}
	}
	os.MkdirAll(filepath.Join(outdir, "trainers"), 0777)
	for n := 1; n <= gb.MaxTrainer; n++ {
		name := filepath.Join("trainers", strconv.Itoa(n))
		m, err := rip.Trainer(n)
		if err != nil {
			log.Printf("%s: %s", name, err)
			continue
		}
		err = write(m, filepath.Join(outdir, name+".png"))
		if err != nil {
			log.Printf("%s: %s", name, err)
		}
	}
	return nil
}

func ripAnimation(rip *gb.Ripper, number int, form string, outname string) error {
	var g *gif.GIF
	var err error
	if number == 201 && form != "" {
		g, err = rip.UnownAnimation(form)
	} else {
		g, err = rip.PokemonAnimation(number)
	}
	if err != nil {
		return err
	}
	return write(g, outname)
}

func ripPokemon(rip *gb.Ripper, number int, form string, outname string) error {
	var m *image.Paletted
	var err error
	if number == 201 && form != "" {
		m, err = rip.Unown(form)
	} else {
		m, err = rip.Pokemon(number)
	}
	if err != nil {
		return err
	}
	return write(m, outname)
}

func ripPokemonBack(rip *gb.Ripper, number int, form string, outname string) error {
	var m *image.Paletted
	var err error
	if number == 201 && form != "" {
		m, err = rip.UnownBack(form)
	} else {
		m, err = rip.PokemonBack(number)
	}
	if err != nil {
		return err
	}
	return write(m, outname)
}

// TODO: It would be nice to just do a palette
//       swap instead of re-ripping the images.

func ripShinyPokemon(rip *gb.Ripper, number int, form string, outname string) error {
	var m *image.Paletted
	var err error
	if number == 201 && form != "" {
		m, err = rip.Unown(form)
	} else {
		m, err = rip.Pokemon(number)
	}
	if err != nil {
		return err
	}
	m.Palette = rip.ShinyPalette(number)
	if m.Palette == nil {
		return errors.New("couldn't get palette")
	}
	return write(m, outname)
}

func ripShinyPokemonBack(rip *gb.Ripper, number int, form string, outname string) error {
	var m *image.Paletted
	var err error
	if number == 201 && form != "" {
		m, err = rip.UnownBack(form)
	} else {
		m, err = rip.PokemonBack(number)
	}
	if err != nil {
		return err
	}
	m.Palette = rip.ShinyPalette(number)
	if m.Palette == nil {
		return errors.New("couldn't get palette")
	}
	return write(m, outname)
}

func ripShinyAnimation(rip *gb.Ripper, number int, form string, outname string) error {
	var g *gif.GIF
	var err error
	if number == 201 && form != "" {
		g, err = rip.UnownAnimation(form)
	} else {
		g, err = rip.PokemonAnimation(number)
	}
	if err != nil {
		return err
	}
	pal := rip.ShinyPalette(number)
	if pal == nil {
		return errors.New("couldn't get palette")
	}
	for _, m := range g.Image {
		m.Palette = pal
	}
	return write(g, outname)
}
