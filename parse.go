package vox

import (
	"bufio"
	"bytes"
	"fmt"
	"image/color"
	"io"
	"log"
	"os"
)

const version = 150

// parseChunk reads a RIFF chunk from the input, returning the ID (MAIN, MATL, etc.)
// and the bytes that hold the contents of this chunk and any child contents.
func parseChunk(vr *voxReader) (ID string, contents, childContents []byte, err error) {
	id := vr.ReadBytes(4)
	N := vr.ReadInt32()
	M := vr.ReadInt32()
	if err := vr.Error(); err != nil {
		return "", nil, nil, err
	}
	c := vr.ReadBytes(int(N))
	cc := vr.ReadBytes(int(M))
	if err := vr.Error(); err != nil {
		return "", nil, nil, err
	}
	return string(id), c, cc, nil
}

func buildMain(models []Model, rgba []color.RGBA, mats []Material) (*Main, error) {
	if len(rgba) != 256 {
		return nil, fmt.Errorf("expected 256 palette entries, but found %d", len(rgba))
	}
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

// parsePackChunk reads a PACK chunk from the input,
// returning the int it contains.
func parsePackChunk(c []byte) (int, error) {
	vr := &voxReader{r: bytes.NewReader(c)}
	n := vr.ReadInt32()
	vr.RequireEOF()
	return int(n), vr.Error()
}

// parseSizeChunk parses a SIZE chunk from the input,
// returning the size it contains.
func parseSizeChunk(c []byte) ([3]int32, error) {
	vr := &voxReader{r: bytes.NewReader(c)}
	x := vr.ReadInt32()
	y := vr.ReadInt32()
	z := vr.ReadInt32()
	vr.RequireEOF()
	return [3]int32{x, y, z}, vr.Error()
}

// parseXYZIChunk parses an XYZI chunk from the input,
// returning the voxels it contains.
func parseXYZIChunk(c []byte) ([]Voxel, error) {
	vr := &voxReader{r: bytes.NewReader(c)}
	N := int(vr.ReadInt32())
	v := []Voxel{}
	for i := 0; i < N; i++ {
		x := vr.ReadUint8()
		y := vr.ReadUint8()
		z := vr.ReadUint8()
		idx := vr.ReadUint8()
		v = append(v, Voxel{x, y, z, idx})
	}
	vr.RequireEOF()
	return v, vr.Error()
}

// parseRGBAChunk parses an RGBA chunk from the input,
// returning the colors it contains.
func parseRGBAChunk(c []byte) ([]color.RGBA, error) {
	vr := &voxReader{r: bytes.NewReader(c)}
	r := make([]color.RGBA, 256)
	for i := range r {
		cr := vr.ReadUint8()
		cg := vr.ReadUint8()
		cb := vr.ReadUint8()
		ca := vr.ReadUint8()
		r[i] = color.RGBA{cr, cg, cb, ca}
	}
	vr.RequireEOF()
	return r, vr.Error()
}

// parseMatType returns the corresponding material from
// the string label in the MATL dict.
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

// parseMatlChunk parses a MATL chunk, returning the ID of the
// material and its properties.
func parseMatlChunk(c []byte) (int, Material, error) {
	vr := &voxReader{r: bytes.NewReader(c)}
	matID := vr.ReadInt32()
	if matID > 255 || matID < 0 {
		return 0, Material{}, fmt.Errorf("material index %d out of range", matID)
	}
	d := vr.ReadDict()
	vr.RequireEOF()
	if err := vr.Error(); err != nil {
		return 0, Material{}, fmt.Errorf("error reading MATL chunk: %v", err)
	}

	// TODO: some of these floats need renormalizing.
	matTypeS := d.ReadString("_type", "<missing>")
	weight := d.ReadFloat("_weight", 1) * 100
	rough := d.ReadFloat("_rough", 0) * 100
	spec := d.ReadFloat("_spec", 0) * 100
	ior := d.ReadFloat("_ior", 0) + 1.0
	att := d.ReadFloat("_att", 0) * 100
	flux := d.ReadFloat("_flux", 0) * 100
	plastic := d.ReadBool("_plastic", false)
	ldr := d.ReadFloat("_ldr", 0) * 100 // not in spec, but present in files

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

// parseMainChunks parses the child chunks of a MAIN chunk.
func parseMainChunks(vr *voxReader) (*Main, error) {
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
		id, c, cc, err := parseChunk(vr)
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

// parseMainChunk parses the top-level MAIN chunk in the .vox file.
func parseMainChunk(vr *voxReader) (*Main, error) {
	id, contents, childContents, err := parseChunk(vr)
	if err != nil {
		return nil, err
	}
	if id != "MAIN" {
		return nil, fmt.Errorf("missing MAIN chunk")
	}
	if len(contents) != 0 {
		return nil, fmt.Errorf("unexpected MAIN contents")
	}
	vr.RequireEOF()
	return parseMainChunks(&voxReader{r: bytes.NewReader(childContents)})
}

// Parse reads and parses a magicavoxel .vox file.
func Parse(r io.Reader) (*Main, error) {
	vr := &voxReader{r: r}
	id := vr.ReadBytes(4)
	ver := vr.ReadInt32()

	if err := vr.Error(); err != nil {
		return nil, fmt.Errorf("failed reading header: %v", err)
	}

	if bytes.Compare(id, []byte("VOX ")) != 0 {
		return nil, fmt.Errorf("not a magicavox file")
	}
	if ver != version {
		return nil, fmt.Errorf("vox file must be version %d, got %d", version, ver)
	}
	return parseMainChunk(vr)
}

// Parse reads and parses the file with the given name as a magicavoxel .vox file.
func ParseFile(filename string) (*Main, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	br := bufio.NewReader(f)
	return Parse(br)
}
