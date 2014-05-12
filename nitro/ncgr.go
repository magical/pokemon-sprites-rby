package nitro

import (
	"io"
	"image"
	"image/color"
)

type NCGR struct {
	header Header
	char   _CHAR
	Data   []byte
}

type _CHAR struct {
	Height, Width uint16

	// Pixel format
	BitDepth uint32
	// VRAM mode; see GBATEK
	// http://nocash.emubase.de/gbatek.htm#dsvideoobjs
	VRAMMode uint32
	Tiled    uint32

	DataSize   uint32
	DataOffset uint32
}

func ReadNCGR(r io.Reader) (*NCGR, error) {
	ncgr := new(NCGR)
	err := readNitroHeader(r, "RGCN", &ncgr.header)
	if err != nil {
		return nil, err
	}
	chunk, err := readChunk(r, "RAHC", &ncgr.char)
	if err != nil {
		return nil, err
	}
	ncgr.Data = make([]byte, ncgr.char.DataSize)
	_, err = chunk.Read(ncgr.Data)
	if err != nil {
		return nil, err
	}
	return ncgr, nil
}

// Decrypt decrypts the pixel data in the NCGR. This method is used
// for Pokémon and trainer sprites in D/P and HG/SS.
func (ncgr *NCGR) Decrypt() {
	seed := uint32(ncgr.Data[0]) + uint32(ncgr.Data[1])<<8
	for i := 0; i < len(ncgr.Data); i += 2 {
		ncgr.Data[i+0] ^= uint8(seed)
		ncgr.Data[i+1] ^= uint8(seed >> 8)
		seed = seed*0x41C64E6D + 0x6073
	}
}

// DecryptReverse decrypts the pixel data in the NCGR. This method
// is used for Pokémon sprites in Pt.
func (ncgr *NCGR) DecryptReverse() {
	seed := uint32(ncgr.Data[len(ncgr.Data)-2]) +
		uint32(ncgr.Data[len(ncgr.Data)-1])<<8
	for i := len(ncgr.Data); i > 0; i -= 2 {
		ncgr.Data[i-2] ^= uint8(seed)
		ncgr.Data[i-1] ^= uint8(seed >> 8)
		seed = seed*0x41C64E6D + 0x6073
	}
}

func (ncgr *NCGR) Bounds() image.Rectangle {
	if ncgr.char.Height != 0xFFFF {
		// Easy case
		w := int(ncgr.char.Width) * 8
		h := int(ncgr.char.Height) * 8
		return image.Rect(0, 0, w, h)
	}
	// No dimensions, so we'll just have to guess
	sz := 0
	switch ncgr.char.BitDepth {
	case 3:
		// 4 bits per pixel
		sz = len(ncgr.Data) * 2
	case 4:
		// 8 bits per pixel
		sz = len(ncgr.Data)
	default:
		panic("unknown bit depth")
	}
	w := 64
	h := (sz + w - 1) / w
	return image.Rect(0, 0, w, h)
}

// Pixels returns a copy of the pixels
func (ncgr *NCGR) Pixels() []byte {
	switch ncgr.char.BitDepth {
	case 3:
		// 4 bits per pixel
		pix := make([]byte, len(ncgr.Data)*2)
		for i, b := range ncgr.Data {
			pix[i*2+0] = b & 0xF
			pix[i*2+1] = b >> 4
		}
		return pix
	case 4:
		// 8 bits per pixel
		pix := make([]byte, len(ncgr.Data))
		copy(pix, ncgr.Data)
		return pix
	default:
		panic("unknown bit depth")
	}
}

func (ncgr *NCGR) Image(pal color.Palette) *image.Paletted {
	r := ncgr.Bounds()
	w, h := r.Dx(), r.Dy()
	pix := ncgr.Pixels()
	if len(pix) < w*h {
		pix = append(pix, make([]byte, len(pix)-w*h)...)
	}
	if ncgr.char.Tiled & 0xFF == 0 {
		pix2 := make([]uint8, len(pix))
		untile(pix2, pix, w, h)
		pix = pix2
	}
	return &image.Paletted{
		Pix:     pix,
		Stride:  w,
		Rect:    r,
		Palette: pal,
	}
}

func untile(dst, src []uint8, w, h int) {
	si := 0
	for y := 0; y < h; y += 8 {
		for x := 0; x < w; x += 8 {
			for ty := 0; ty < 8; ty++ {
				for tx := 0; tx < 8; tx++ {
					di := (y+ty)*w + (x+tx)*1
					dst[di] = src[si]
					si++
				}
			}
		}
	}
}
