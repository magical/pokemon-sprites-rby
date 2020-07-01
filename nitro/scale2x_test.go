package nitro

import (
	"image"
	"image/draw"
	"image/png"
	"os"
	"testing"
)

func readPNG(t testing.TB, filename string) image.Image {
	f, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// Orig and golden images taken from test-1.png and test-2.png in the
// official scale2x distribution.

func TestScale2x(t *testing.T) {
	orig := readPNG(t, "testdata/scale2x.png")
	golden := readPNG(t, "testdata/scale2x.golden.png")

	pal := orig.(*image.Paletted).Palette
	r := image.Rect(0, 0, orig.Bounds().Dx()*2, orig.Bounds().Dy()*2)
	var images = []draw.Image{
		image.NewRGBA(r),
		image.NewPaletted(r, pal),
	}
	for _, actual := range images {
		scale2x(actual, image.ZP, orig, orig.Bounds())

		if actual.Bounds() != golden.Bounds() {
			t.Fatalf("%T: bounds do not match: got %v, want %v", actual, actual.Bounds(), golden.Bounds())
		}
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				if !equal(actual.At(x, y), golden.At(x, y)) {
					t.Fatalf("%T: pix at (%v,%v): got %v, want %v", actual, x, y, actual.At(x, y), golden.At(x, y))
				}
			}
		}
	}
}

func BenchmarkScale2xGeneric(b *testing.B) {
	orig := readPNG(b, "testdata/scale2x.png")
	r := image.Rect(0, 0, orig.Bounds().Dx()*2, orig.Bounds().Dy()*2)
	m := image.NewRGBA(r)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scale2x(m, image.ZP, orig, orig.Bounds())
	}
}

func BenchmarkScale2xPaletted(b *testing.B) {
	orig := readPNG(b, "testdata/scale2x.png")
	r := image.Rect(0, 0, orig.Bounds().Dx()*2, orig.Bounds().Dy()*2)
	pal := orig.(*image.Paletted).Palette
	m := image.NewPaletted(r, pal)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scale2x(m, image.ZP, orig, orig.Bounds())
	}
}
