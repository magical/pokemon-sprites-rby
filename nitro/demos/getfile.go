// +build ignore

package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/magical/sprites/nitro"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: getfile path/to/a.narc number >file")
		return
	}
	filename := os.Args[1]
	number, err := strconv.ParseInt(os.Args[2], 0, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	f, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	narc, err := nitro.ReadNARC(f)
	if err != nil {
		panic(err)
	}
	r, err := narc.Open(int(number))
	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, r)
}
