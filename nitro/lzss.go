package nitro

import (
	"errors"
	"io"
)

var errMalformed = errors.New("LZSS: malformed data")

// decode lzss10
func decode10(r io.ByteReader) ([]byte, error) {
	var err error
	var nextbyte = func() (b byte) {
		if err == nil {
			b, err = r.ReadByte()
		}
		return
	}
	magic := nextbyte()
	if magic != 0x10 {
		return nil, errMalformed
	}
	size := int(nextbyte()) +
		int(nextbyte())<<8 +
		int(nextbyte())<<16
	if err != nil {
		return nil, err
	}
	data := make([]byte, 0, size)
	for len(data) < size && err == nil {
		bits := nextbyte()
		for i := 0; i < 8 && len(data) < size; i, bits = i+1, bits<<1 {
			if bits&0x80 == 0 {
				data = append(data, nextbyte())
				continue
			}
			n := int(nextbyte())<<8 + int(nextbyte())
			count := n>>12 + 3
			disp := n&0xFFF + 1
			if disp > len(data) {
				return nil, errMalformed
			}
			if len(data)+count > size {
				count = size - len(data)
			}
			for j := 0; j < count; j++ {
				data = append(data, data[len(data)-disp])
			}
		}
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// decode lzss11
func decode11(r io.ByteReader) ([]byte, error) {
	var err error
	var nextbyte = func() (b byte) {
		if err == nil {
			b, err = r.ReadByte()
		}
		return
	}
	magic := nextbyte()
	if magic != 0x11 {
		return nil, errMalformed
	}
	size := int(nextbyte()) +
		int(nextbyte())<<8 +
		int(nextbyte())<<16
	if err != nil {
		return nil, err
	}
	data := make([]byte, 0, size)
	for len(data) < size && err == nil {
		bits := nextbyte()
		for i := 0; i < 8 && len(data) < size; i, bits = i+1, bits<<1 {
			if bits&0x80 == 0 {
				data = append(data, nextbyte())
				continue
			}
			n := int(nextbyte())<<8 + int(nextbyte())
			count := 0
			switch n >> 12 {
			default:
				count = 1
			case 0:
				n = n&0xFFF<<8 + int(nextbyte())
				count = 0x11
			case 1:
				// n doesn't exceed 28 bits
				n = n&0xFFF<<16 + int(nextbyte())<<8 + int(nextbyte())
				count = 0x111
			}
			count += n >> 12
			disp := n&0xFFF + 1
			if disp > len(data) {
				return nil, errMalformed
			}
			if len(data)+count > size {
				count = size - len(data)
			}
			for j := 0; j < count; j++ {
				data = append(data, data[len(data)-disp])
			}
		}
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

type readerSize interface {
	io.Reader
	Size() int64
}

// Is LZ reports whether a reader is likely LZ-compressed
func isLZ(r readerSize) bool {
	var b [4]byte
	n, err := r.Read(b[:])
	if n < 4 || err != nil {
		return false
	}
	if b[0] != 0x10 && b[0] != 0x11 {
		return false
	}
	size := int64(b[1]) + int64(b[2])<<8 + int64(b[3])<<16
	if size < r.Size() {
		return false
	}
	return true
}
