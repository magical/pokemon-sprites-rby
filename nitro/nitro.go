package nitro

import "errors"

type Header struct {
	Magic      [4]byte
	BOM        uint16
	Version    uint16
	Size       uint32
	HeaderSize uint16
	ChunkCount uint16
}

// These constants are copied from os.
const (
	seekSet = 0
	seekCur = 1
)

var (
	errBadMagic     = errors.New("bad magic")
	errInvalidChunk = errors.New("invalid chunk")
)
