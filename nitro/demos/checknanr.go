package main

import (
	"fmt"
	"log"
	"os"

	"github.com/magical/sprites/nitro"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s path/to/a.narc", os.Args[0])
		return
	}
	filename := os.Args[1]

	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	narc, err := nitro.ReadNARC(f)
	if err != nil {
		log.Fatal(err)
	}
	for i := 0; i < narc.FileCount(); i++ {
		r, err := narc.Open(i)
		if err != nil {
			//log.Printf("%d: %v", i, err)
			continue
		}
		n, err := nitro.ReadNANR(r)
		if err != nil {
			//log.Printf("%d: %v", i, err)
			continue
		}
		for _, c := range n.Acells {
			if c.PlayMode != 2 {
				fmt.Printf("%d: % #v\n", i, c)
			}
		}
	}
}
