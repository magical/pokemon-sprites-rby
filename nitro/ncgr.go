package nitro

import (
	"io"
)

type NCGR struct {
	header Header
	char   _CHAR
	data   []byte
}

type _CHAR struct {
	Magic uint32
	Size  uint32

	Height, Width uint16

	// Pixel format
	Format uint32
	// VRAM mode; see GBATEK
	// http://nocash.emubase.de/gbatek.htm#dsvideoobjs
	Mode  uint32
	Tiled uint32

	DataSize uint32
	Unknown  uint32
}

func ReadNCGR(io.Reader) (*NCGR, error) {
	ncgr := new(NCGR)
	//ncgr.data, err := ReadChunk("CHAR", &ncgr._CHAR)
	return ncgr, nil
}

// Crypt decrypts (or encrypts) the pixel data in the NCGR. This method is used
// for Pokémon and trainer sprites in D/P and HG/SS.
func (ncgr *NCGR) Crypt() {
	seed := uint32(ncgr.data[0]) + uint32(ncgr.data[1])<<8
	for i := 0; i < len(ncgr.data); i += 2 {
		ncgr.data[i] ^= uint8(seed >> 16)
		ncgr.data[i+1] ^= uint8(seed >> 24)
		seed = seed*0x41C64E6D + 0x6073
	}
}

// CryptReverse decrypts (or encrypts) the pixel data in the NCGR. This method
// is used for Pokémon sprites in Pt.
func (ncgr *NCGR) CryptReverse() {
	seed := uint32(ncgr.data[len(ncgr.data)-2]) +
		uint32(ncgr.data[len(ncgr.data)-1])<<8
	for i := len(ncgr.data) - 2; i >= 0; i -= 2 {
		ncgr.data[i] ^= uint8(seed >> 16)
		ncgr.data[i+1] ^= uint8(seed >> 24)
		seed = seed*0x41C64E6D + 0x6073
	}
}
