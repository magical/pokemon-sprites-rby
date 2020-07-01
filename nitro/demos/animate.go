// +build ignore

package main

import (
	"fmt"
	"image/gif"
	"os"
	"strconv"

	"github.com/magical/sprites/nitro"
)

func die(v ...interface{}) {
	fmt.Fprintln(os.Stderr, v...)
	os.Exit(1)
}

func main() {
	if len(os.Args) != 3 {
		die("Usage: animate pokegra.narc 2")
	}
	filename := os.Args[1]
	poke, err := strconv.ParseInt(os.Args[2], 0, 64)
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
	base := int(poke) * 20
	dir := 9 // 0 for fontsprite, 9 for backsprite
	ncgr, err := narc.OpenNCGR(base + dir + 2)
	if err != nil {
		die("OpenNCGR:", err)
	}
	nclr, err := narc.OpenNCLR(base + 18)
	if err != nil {
		die("OpenNCLR:", err)
	}
	ncer, err := narc.OpenNCER(base + dir + 4)
	if err != nil {
		die("OpenNCER:", err)
	}
	nanr, err := narc.OpenNANR(base + dir + 5)
	if err != nil {
		die("OpenNANR:", err)
	}
	nmcr, err := narc.OpenNMCR(base + dir + 6)
	if err != nil {
		die("OpenNMCR:", err)
	}
	nmar, err := narc.OpenNMAR(base + dir + 7)
	if err != nil {
		die("OpenNMAR:", err)
	}
	g := nitro.NewAnimation(ncgr, nclr, ncer, nanr, nmcr, nmar).Render()
	//fmt.Fprintln(os.Stderr, len(g.Image))
	if err := gif.EncodeAll(os.Stdout, g); err != nil {
		die(err)
	}
}
