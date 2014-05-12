package nitro

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

type NARC struct {
	header Header
	fatb   _FATB
	fntb   _FNTB
	fimg   _FIMG

	records    []fatRecord
	data       *io.SectionReader
	dataOffset int64
}

type _FATB struct {
	FileCount uint32
}

type _FNTB struct {
	Magic [4]byte
	Size  uint32

	// ...
}

type _FIMG struct {
	Magic [4]byte
	Size  uint32
}

type fatRecord struct {
	Start uint32
	End   uint32
}

type chunkError struct {
	got  string
	want string
}

func (err chunkError) Error() string {
	return "invalid chunk: expected " + err.want + ", got " + err.got
}

type ReadSeekerAt interface {
	io.Reader
	io.ReaderAt
	io.Seeker
}

func ReadNARC(r ReadSeekerAt) (*NARC, error) {
	narc := new(NARC)

	err := readNitroHeader(r, "NARC", &narc.header)
	if err != nil {
		return nil, err
	}

	chunk, err := readChunk(r, "BTAF", &narc.fatb)
	if err != nil {
		return nil, err
	}

	narc.records = make([]fatRecord, narc.fatb.FileCount)
	err = binary.Read(chunk, binary.LittleEndian, &narc.records)
	if err != nil {
		return nil, err
	}

	err = binary.Read(r, binary.LittleEndian, &narc.fntb)
	if err != nil {
		return nil, err
	}
	if string(narc.fntb.Magic[:]) != "BTNF" {
		return nil, chunkError{string(narc.fntb.Magic[:]), "BTNF"}
	}
	_, err = r.Seek(int64(narc.fntb.Size)-int64(binary.Size(&narc.fntb)), seekCur)
	if err != nil {
		return nil, err
	}

	err = binary.Read(r, binary.LittleEndian, &narc.fimg)
	if err != nil {
		return nil, err
	}
	if string(narc.fimg.Magic[:]) != "GMIF" {
		return nil, chunkError{string(narc.fimg.Magic[:]), "GMIF"}
	}

	off, err := r.Seek(0, seekCur)
	size := int64(narc.fimg.Size) - 8
	if err != nil {
		return nil, err
	}
	narc.data = io.NewSectionReader(r, off, size)

	return narc, nil
}

func (narc *NARC) FileCount() int {
	return len(narc.records)
}

func (narc *NARC) Open(n int) (readerSize, error) {
	sr, err := narc.OpenRaw(n)
	if err != nil {
		return nil, err
	}
	ok := isLZ(sr)
	sr.Seek(0, seekSet)
	if !ok {
		return sr, nil
	}
	data, err := decode11(bufio.NewReader(sr))
	if err != nil {
		return nil, err
	}
	return &bytesReaderSize{*bytes.NewReader(data), len(data)}, nil
}

type bytesReaderSize struct {
	bytes.Reader
	size int
}

func (b *bytesReaderSize) Size() int64 {
	return int64(b.size)
}

func (narc *NARC) OpenRaw(n int) (*io.SectionReader, error) {
	if n < 0 || n > len(narc.records) {
		return nil, errors.New("NARC.Open: no such file")
	}
	rec := narc.records[n]
	size := int64(rec.End) - int64(rec.Start)
	if size < 0 {
		size = 0
	}
	return io.NewSectionReader(narc.data, int64(rec.Start), size), nil
}

func (narc *NARC) OpenNCGR(n int) (*NCGR, error) {
	r, err := narc.Open(n)
	if err != nil {
		return nil, err
	}
	return ReadNCGR(r)
}

func (narc *NARC) OpenNCLR(n int) (*NCLR, error) {
	r, err := narc.Open(n)
	if err != nil {
		return nil, err
	}
	return ReadNCLR(r)
}
