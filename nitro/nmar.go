package nitro

import "io"

type NMAR struct {
	h    Header
	abnk _ABNK // see abnk.go

	Cells []Acell
}

func ReadNMAR(r io.Reader) (*NMAR, error) {
	nmar := new(NMAR)
	if err := readNitroHeader(r, "RAMN", &nmar.h); err != nil {
		return nil, err
	}
	if err := readABNK(r, &nmar.abnk, &nmar.Cells); err != nil {
		return nil, err
	}
	return nmar, nil
}
