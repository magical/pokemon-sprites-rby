package nitro

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"reflect"
	"testing"
)

func openNCGR(filename string) (*NCGR, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ReadNCGR(f)
}

func openPNG(filename string) (image.Image, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func TestReadNCGR(T *testing.T) {
	ncgr, err := openNCGR("testdata/test.ncgr")
	if err != nil {
		T.Fatal(err)
	}
	m, err := openPNG("testdata/test.png")
	if err != nil {
		T.Fatal(err)
	}
	ncgr.DecryptReverse()
	want := m.(*image.Paletted)
	got := ncgr.Image(want.Palette)
	if !reflect.DeepEqual(got, want) {
		//fmt.Println(got)
		//fmt.Println(want)
		T.Errorf("testdata/test.ncgr does not equal testdata/test.png")
	}
}
