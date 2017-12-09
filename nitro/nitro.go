package nitro

import (
	"encoding/binary"
	"errors"
	"io"
)

// A Header is the common header used in all nitro formats.
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

/* TODO: Decide which prefix to use in errors, "nitro:" or e.g. "NCER:" */

var (
	errBadMagic     = errors.New("nitro: bad magic")
	errHeader       = errors.New("nitro: invalid header")
	errInvalidChunk = errors.New("nitro: invalid chunk")
)

var le = binary.LittleEndian

func readNitroHeader(r io.Reader, magic string, header *Header) error {
	err := binary.Read(r, binary.LittleEndian, header)
	if err != nil {
		return err
	}
	if string(header.Magic[:]) != magic {
		return errBadMagic
	}
	if header.HeaderSize != 16 {
		return errHeader
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
	if string(chunk.Magic[:]) != magic {
		return nil, errInvalidChunk
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
