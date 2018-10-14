package vox

import (
	"bytes"
	"fmt"
	"image/color"
	"io"
)

const version = 150

type errReader struct {
	r   io.Reader
	err error
}

func (er *errReader) Error() error {
	return er.err
}

func (er *errReader) ReadBytes(n int) []byte {
	r := make([]byte, n)
	if er.err != nil {
		return r
	}
	_, er.err = io.ReadFull(er.r, r)
	return r
}

func (er *errReader) ReadInt32() int32 {
	b := er.ReadBytes(4)
	u := uint32(b[0]) + uint32(b[1])<<8 + uint32(b[2])<<16 + uint32(b[3])<<24
	return int32(u)
}

func parseChunk(er *errReader) (ID string, contents, childContents []byte, err error) {
	id := er.ReadBytes(4)
	N := er.ReadInt32()
	M := er.ReadInt32()
	if err := er.Error(); err != nil {
		return "", nil, nil, err
	}
	c := er.ReadBytes(int(N))
	cc := er.ReadBytes(int(M))
	if err := er.Error(); err != nil {
		return "", nil, nil, err
	}
	return string(id), c, cc, nil
}

func parseMainChunks(er *errReader) (*Main, error) {
	readPack := false
	pack := 1
	models := []Model{}
	var rgba []color.RGBA
	mats := []Material{}

	for {
		id, c, cc, err := parseChunk(er)
		if err == os.EOF {

		}
	}
}

func parseMainChunk(er *errReader) (*Main, error) {
	id, contents, childContents, err := parseChunk(er)
	if err != nil {
		return nil, err
	}
	if id != "MAIN" {
		return nil, fmt.Errorf("missing MAIN chunk")
	}
	if len(contents) != 0 {
		return nil, fmt.Errorf("unexpected MAIN contents")
	}
	return nil, parseMainChunks(&errReader{r: bytes.NewReader(childContents)})
}

// Parse reads and parses a magicavoxel .vox file.
func Parse(r io.Reader) (*Main, error) {
	er := &errReader{r: r}
	id := er.ReadBytes(4)
	ver := er.ReadInt32()

	if err := er.Error(); err != nil {
		return nil, fmt.Errorf("failed reading header: %v", err)
	}

	if bytes.Compare(id, []byte("VOX ")) != 0 {
		return nil, fmt.Errorf("not a magicavox file")
	}
	if ver != version {
		return nil, fmt.Errorf("vox file must be version %d, got %d", version, ver)
	}
	return parseMainChunk(er)
}
