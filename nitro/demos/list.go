// +build ignore

package main

import (
	"fmt"
	"os"

	"github.com/magical/sprites/nitro"
)

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
	for i := 0; i < narc.FileCount(); i++ {
		r, err := narc.Open(i)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%d: %d\n", i, r.Size())
	}
}
