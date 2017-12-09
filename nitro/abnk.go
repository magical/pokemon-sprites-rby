package nitro

// ABNK chunk used in NANR and NMAR.

type _ABNK struct {
	CellCount  uint16 // number of cells
	FrameCount uint16 // total number of frames

	CellOffset      uint32
	FrameOffset     uint32
	FrameDataOffset uint32

	_ uint32
	_ uint32
}

type acell struct {
	FrameCount  uint16
	LoopStart   uint16 // index of first frame
	FrameType   uint16 // frameData{0,1,2}
	CellType    uint16 // NANR or NMAR
	PlayMode    uint32 // invalid, forward, forward loop, reverse, reverse loop
	FrameOffset uint32
}

type frame struct {
	DataOffset uint32
	Duration   uint16 // 60 fps
	_          uint16 // padding
}

// A frame.
type frameData0 struct {
	Index uint16
}

// A frame with rotation, scaling, and translation.
type frameData1 struct {
	Index  uint16
	Theta  uint16 // angle in units of (tau/65536)
	ScaleX int32  // 1.15.16 fixed-point
	ScaleY int32
	X      int16
	Y      int16
}

// A frame with translation.
type frameData2 struct {
	Index uint16
	_     uint16 // padding
	X     int16
	Y     int16
}
