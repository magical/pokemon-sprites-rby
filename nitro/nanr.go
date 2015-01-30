package nitro

import (
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
)

// An NANR (nitro animation resource) sequences cells into animations.
type NANR struct {
	h    Header
	abnk _ABNK // see abnk.go

	Acells []Acell
	//Frames []Frame
}

// An animated cell.
type Acell struct {
	LoopStart int
	PlayMode  int // invalid, forward, forward loop, forward-reverse, forward-reverse loop
	Frames    []Frame
}

// A frame of an animated cell.
type Frame struct {
	Duration uint // 60 fps
	Cell     int
	Rotate   uint16 // angle in units of (tau/65536)
	ScaleX   int32  // in units of 1/4096
	ScaleY   int32  // in units of 1/4096
	X        int
	Y        int
}

func ReadNANR(r io.Reader) (*NANR, error) {
	nanr := new(NANR)
	if err := readNitroHeader(r, "RNAN", &nanr.h); err != nil {
		return nil, err
	}
	if nanr.h.ChunkCount != 1 && nanr.h.ChunkCount != 3 {
		return nil, errors.New("NANR: too many chunks")
	}
	if err := readABNK(r, &nanr.abnk, &nanr.Acells); err != nil {
		return nil, err
	}
	return nanr, nil
}

func readABNK(r io.Reader, abnk *_ABNK, acells *[]Acell) error {
	chunk, err := readChunk(r, "KNBA", abnk)
	if err != nil {
		return err
	}

	cellsize := binary.Size(&acell{}) * int(abnk.CellCount)
	framesize := binary.Size(&frame{}) * int(abnk.FrameCount)

	if abnk.CellOffset != 0x18 || int(abnk.FrameOffset) != 0x18+cellsize || int(abnk.FrameDataOffset) != 0x18+cellsize+framesize {
		return errors.New("ABNK: invalid offsets")
	}

	acellsraw := make([]acell, abnk.CellCount)
	framesraw := make([]frame, abnk.FrameCount)
	if err := binary.Read(chunk, le, acellsraw); err != nil {
		return err
	}
	if err := binary.Read(chunk, le, framesraw); err != nil {
		return err
	}
	framedata, err := ioutil.ReadAll(chunk)
	if err != nil {
		return err
	}

	for _, c := range acellsraw {
		if c.FrameOffset%8 != 0 {
			return errors.New("ABNK: unaligned frame")
		}
	}

	// Frame data is variable-sized so we can't slurp it up with binary.Read.
	// Fortunately it's not too difficult to parse by hand.

	*acells = make([]Acell, abnk.CellCount)
	frames := make([]Frame, abnk.FrameCount)
	for i, araw := range acellsraw {
		c := &(*acells)[i]
		c.LoopStart = int(araw.LoopStart)
		c.PlayMode = int(araw.PlayMode)
		c.Frames = frames[araw.FrameOffset/8 : araw.FrameOffset/8+uint32(araw.FrameCount)]
		framesraw := framesraw[araw.FrameOffset/8 : araw.FrameOffset/8+uint32(araw.FrameCount)]
		for i, fraw := range framesraw {
			f := parseFrame(araw.FrameType, framedata[fraw.DataOffset:])
			f.Duration = uint(fraw.Duration)
			c.Frames[i] = f
		}
	}

	return nil
}

func parseFrame(typ uint16, b []byte) Frame {
	var f Frame
	f.ScaleX = 1<<12
	f.ScaleY = 1<<12
	switch typ {
	case 0:
		f.Cell = int(le.Uint16(b[0:]))
	case 1:
		f.Cell = int(le.Uint16(b[0:]))
		f.Rotate = le.Uint16(b[2:])
		f.ScaleX = int32(le.Uint32(b[4:]))
		f.ScaleY = int32(le.Uint32(b[8:]))
		f.X = int(int16(le.Uint16(b[12:])))
		f.Y = int(int16(le.Uint16(b[14:])))
	case 2:
		f.Cell = int(le.Uint16(b[0:]))
		f.X = int(int16(le.Uint16(b[4:])))
		f.Y = int(int16(le.Uint16(b[6:])))
	}
	return f
}
