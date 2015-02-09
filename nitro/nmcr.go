package nitro

import (
	"encoding/binary"
	"errors"
	"io"
)

// A NMCR (nitro multi cell resource) composes animated cells.
type NMCR struct {
	h    Header
	mcbk _MCBK

	mcells []mcell
	mobjs  []mobj
}

type _MCBK struct {
	CellCount  uint16
	_          uint16
	CellOffset uint32
	ObjOffset  uint32
	_          uint32
	_          uint32
}

type mcell struct {
	MobjCount  uint16
	AcellCount uint16
	MobjOffset uint32
}

type mobj struct {
	AcellIndex uint16
	X          int16
	Y          int16
	Flags      uint16 // num anims, visible, play mode
	// Play modes: reset, continue, pause
}

func ReadNMCR(r io.Reader) (*NMCR, error) {
	nmcr := new(NMCR)
	if err := readNitroHeader(r, "RCMN", &nmcr.h); err != nil {
		return nil, err
	}
	if err := readMCBK(r, nmcr); err != nil {
		return nil, err
	}
	return nmcr, nil
}

func readMCBK(r io.Reader, nmcr *NMCR) error {
	chunk, err := readChunk(r, "KBCM", &nmcr.mcbk)
	if err != nil {
		return err
	}

	mcells := make([]mcell, nmcr.mcbk.CellCount)
	if err := binary.Read(chunk, le, mcells); err != nil {
		return err
	}

	var objCount int
	for _, c := range mcells {
		if c.MobjOffset%8 != 0 {
			return errors.New("NMCR: unaligned obj")
		}
		objCount += int(c.MobjCount)
	}

	mobjs := make([]mobj, objCount)
	if err := binary.Read(chunk, le, mobjs); err != nil {
		return err
	}

	nmcr.mcells = mcells
	nmcr.mobjs = mobjs
	return nil
}

func (nmcr *NMCR) Len() int {
	return int(nmcr.mcbk.CellCount)
}

func (nmcr *NMCR) Mcell(i int) []mobj {
	if i < 0 || i >= nmcr.Len() {
		panic("NMCR: cell out of range")
	}
	c := nmcr.mcells[i]
	return nmcr.mobjs[c.MobjOffset/8 : c.MobjOffset/8+uint32(c.MobjCount)]
}

func (o *mobj) PlayMode() int {
	return int(o.Flags&0xF)
}
