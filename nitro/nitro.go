package nitro

import (
	"encoding/binary"
	"errors"
	"io"
)

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

func readNitroHeader(r io.Reader, magic string, header *Header) error {
	err := binary.Read(r, binary.LittleEndian, header)
	if err != nil {
		return err
	}
	if string(header.Magic[:]) != magic {
		return errBadMagic
	}
	return nil
}

func readChunk(r io.Reader, magic string, header interface{}) (io.Reader, error) {
	var chunk struct {
		Magic [4]byte
		Size  uint32
	}
	err := binary.Read(r, binary.LittleEndian, &chunk)
	if err != nil {
		return nil, err
	}
	r = io.LimitReader(r, int64(chunk.Size)-8)
	if header != nil {
		err = binary.Read(r, binary.LittleEndian, header)
		if err != nil {
			return r, err
		}
	}
	return r, err
}
