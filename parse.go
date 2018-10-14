package vox

import (
	"bytes"
	"fmt"
	"image/color"
	"io"
	"log"
	"strconv"
	"strings"
)

const version = 150

type errReader struct {
	r   io.Reader
	err error
}

type dict struct {
	d    map[string]string
	read map[string]bool
	err  error
}

func (d *dict) Error() error {
	return d.err
}

func (d *dict) AssertNoUnreadFields() error {
	uk := []string{}
	for k := range d.d {
		if !d.read[k] {
			uk = append(uk, k)
		}
	}
	if len(uk) != 0 {
		return fmt.Errorf("unknown field[s] in dict: %s", strings.Join(uk, ", "))
	}
	return nil
}

func (d *dict) ReadFloat(name string, def float32) float32 {
	d.read[name] = true
	if d.err != nil {
		return def
	}
	r, ok := d.d[name]
	if !ok {
		return def
	}
	f, err := strconv.ParseFloat(r, 32)
	if err != nil {
		d.err = fmt.Errorf("error parsing float %q in field %q: %v", r, name, err)
	}
	return float32(f)
}

func (d *dict) ReadBool(name string, def bool) bool {
	df := float32(0.0)
	if def {
		df = 1
	}
	return d.ReadFloat(name, df) != 0
}

func (d *dict) ReadString(name string, def string) string {
	d.read[name] = true
	if d.err != nil {
		return def
	}
	r, ok := d.d[name]
	if !ok {
		return def
	}
	return r
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

func (er *errReader) ReadUint8() uint8 {
	if er.err != nil {
		return 0
	}
	r := make([]byte, 1)
	_, er.err = io.ReadFull(er.r, r)
	return uint8(r[0])
}

func (er *errReader) ReadInt32() int32 {
	b := er.ReadBytes(4)
	u := uint32(b[0]) + uint32(b[1])<<8 + uint32(b[2])<<16 + uint32(b[3])<<24
	return int32(u)
}

func (er *errReader) ReadString() string {
	n := int(er.ReadInt32())
	bs := er.ReadBytes(n)
	return string(bs)
}

func (er *errReader) Readdict() *dict {
	d := &dict{d: map[string]string{}, read: map[string]bool{}}
	n := int(er.ReadInt32())
	for i := 0; i < n; i++ {
		key := er.ReadString()
		val := er.ReadString()
		d.d[key] = val
	}
	if err := er.Error(); err != nil {
		d.err = err
	}
	return d
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

func buildMain(models []Model, rgba []color.RGBA, mats []Material) (*Main, error) {
	// TODO: error checking here!
	return &Main{
		Models:    models,
		RGBA:      rgba,
		Materials: mats,
	}, nil
}

type state int

const (
	statePack state = iota
	stateSize
	stateXYZI
	stateRGBA
	stateMatt
)

func parsePackChunk(c []byte) (int, error) {
	er := &errReader{r: bytes.NewReader(c)}
	n := er.ReadInt32()
	return int(n), er.Error()
}

func parseSizeChunk(c []byte) ([3]int32, error) {
	er := &errReader{r: bytes.NewReader(c)}
	x := er.ReadInt32()
	y := er.ReadInt32()
	z := er.ReadInt32()
	return [3]int32{x, y, z}, er.Error()
}

func parseXYZIChunk(c []byte) ([]Voxel, error) {
	er := &errReader{r: bytes.NewReader(c)}
	N := int(er.ReadInt32())
	v := []Voxel{}
	for i := 0; i < N; i++ {
		x := er.ReadUint8()
		y := er.ReadUint8()
		z := er.ReadUint8()
		idx := er.ReadUint8()
		v = append(v, Voxel{x, y, z, idx})
	}
	return v, er.Error()
}

func parseRGBAChunk(c []byte) ([]color.RGBA, error) {
	er := &errReader{r: bytes.NewReader(c)}
	r := make([]color.RGBA, 256)
	for i := range r {
		cr := er.ReadUint8()
		cg := er.ReadUint8()
		cb := er.ReadUint8()
		ca := er.ReadUint8()
		r[i] = color.RGBA{cr, cg, cb, ca}
	}
	return r, er.Error()
}

func parseMatType(s string) (MaterialType, error) {
	switch s {
	case "_diffuse":
		return MaterialDiffuse, nil
	case "_metal":
		return MaterialMetal, nil
	case "_glass":
		return MaterialGlass, nil
	case "_emit":
		return MaterialEmissive, nil
	}
	return MaterialDiffuse, fmt.Errorf("unknown material %q", s)
}

func parseMatlChunk(c []byte) (int, Material, error) {
	er := &errReader{r: bytes.NewReader(c)}
	matID := er.ReadInt32()
	if matID > 255 || matID < 0 {
		return 0, Material{}, fmt.Errorf("material index %d out of range", matID)
	}
	d := er.Readdict()
	if err := er.Error(); err != nil {
		return 0, Material{}, fmt.Errorf("error reading MATL chunk: %v", err)
	}

	// TODO: some of these floats need renormalizing.
	matTypeS := d.ReadString("_type", "<missing>")
	weight := d.ReadFloat("_weight", 1)
	rough := d.ReadFloat("_rough", 0)
	spec := d.ReadFloat("_spec", 0)
	ior := d.ReadFloat("_ior", 0)
	att := d.ReadFloat("_att", 0)
	flux := d.ReadFloat("_flux", 0)
	plastic := d.ReadBool("_plastic", false)
	ldr := d.ReadFloat("_ldr", 0) // not in spec, but present in files

	if err := d.Error(); err != nil {
		return 0, Material{}, fmt.Errorf("dict error reading MATL chunk: %v", err)
	}

	if err := d.AssertNoUnreadFields(); err != nil {
		return 0, Material{}, fmt.Errorf("dict error -- unknown field: %v", err)
	}

	matType, err := parseMatType(matTypeS)
	if err != nil {
		return 0, Material{}, fmt.Errorf("error reading MATL chunk: %v", err)
	}

	return int(matID), Material{
		Type:        matType,
		Weight:      weight,
		Roughness:   rough,
		Specular:    spec,
		IOR:         ior,
		Attenuation: att,
		Flux:        flux,
		Plastic:     plastic,
		LDR:         ldr,
	}, nil
}

func parseMainChunks(er *errReader) (*Main, error) {
	state := statePack
	pack := 1
	models := []Model{}
	var rgba []color.RGBA
	mats := []Material{}
	var size [3]int32

	ignoredChunks := map[string]bool{
		"nTRN": true,
		"nGRP": true,
		"nSHP": true,
		"LAYR": true,
		"rOBJ": true,
	}

	for {
		id, c, cc, err := parseChunk(er)
		if err == io.EOF {
			if len(models) != pack {
				return nil, fmt.Errorf("expected %d models, but got %d", pack, len(models))
			}
			return buildMain(models, rgba, mats)
		}
		if err != nil {
			return nil, err
		}
		switch id {
		case "PACK":
			if state != statePack {
				return nil, fmt.Errorf("PACK chunk must appear first in MAIN")
			}
			pack, err = parsePackChunk(c)
			if err != nil {
				return nil, err
			}
			state = stateSize
		case "SIZE":
			if state == statePack {
				state = stateSize
			}
			if state != stateSize {
				return nil, fmt.Errorf("misplaced SIZE chunk")
			}
			size, err = parseSizeChunk(c)
			if err != nil {
				return nil, err
			}
			state = stateXYZI
		case "XYZI":
			if state != stateXYZI {
				return nil, fmt.Errorf("misplaced XYZI chunk")
			}
			var vs []Voxel
			vs, err = parseXYZIChunk(c)
			if err != nil {
				return nil, err
			}
			models = append(models, Model{X: int(size[0]), Y: int(size[1]), Z: int(size[2]), V: vs})
			if len(models) == pack {
				state = stateRGBA
			} else {
				state = stateXYZI
			}
		case "RGBA":
			if state != stateRGBA {
				return nil, fmt.Errorf("misplaced RGBA chunk")
			}
			rgba, err = parseRGBAChunk(c)
			state = stateMatt
		case "MATL":
			if state != stateMatt {
				return nil, fmt.Errorf("misplaced MATL chunk")
			}
			idx, mat, err := parseMatlChunk(c)
			if err != nil {
				return nil, err
			}
			for len(mats) <= idx {
				mats = append(mats, Material{})
			}
			mats[idx] = mat
		default:
			if !ignoredChunks[id] {
				log.Printf("unexpected chunk %s", id)
				ignoredChunks[id] = true // stop the error appearing multiple times
			}
			continue
		}
		if len(cc) != 0 {
			return nil, fmt.Errorf("unexpected child chunks of chunk %q", id)
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
	return parseMainChunks(&errReader{r: bytes.NewReader(childContents)})
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
