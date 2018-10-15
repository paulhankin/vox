package vox

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// dict contains data read from a DICT type that's stored in
// the magicavox files. The accessors (ReadFloat, ReadBool,
// ReadString, ...) never return an error, but any errors
// found (for example, because the value isn't parsable
// as a float or whatever) can be later retrieved using
// d.Error().
type dict struct {
	d    map[string]string
	read map[string]bool
	err  error
}

// Error returns any error reading from the dict.
func (d *dict) Error() error {
	return d.err
}

// AssertNoUnreadFields returns an error if any field in the
// dict has not been read. It's intended to identify missing
// fields in the parser.
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

// ReadFloat returns a float, read from the dict, defaulting
// to 'def'.
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

// ReadBool returns a boolean, read from the dict, defaulting to def.
func (d *dict) ReadBool(name string, def bool) bool {
	df := float32(0.0)
	if def {
		df = 1
	}
	return d.ReadFloat(name, df) != 0
}

// ReadString returns a string, read from the dict, defaulting to def.
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

// voxReader provides help for reading .vox-styled
// RIFF files.
// The reader can be used after error (although default
// values are returned), and errors are not explicitly
// returned. They can be checked at a convenient point
// using vr.Error().
type voxReader struct {
	r   io.Reader
	err error
}

// Error() returns the first error (if any) encountered
// by the reader.
func (vr *voxReader) Error() error {
	return vr.err
}

// RequireEOF checks that all input has been consumed.
func (vr *voxReader) RequireEOF() {
	if err := vr.Error(); err != nil {
		return
	}
	_ = vr.ReadUint8()
	err := vr.Error()
	if err == nil {
		vr.err = fmt.Errorf("expected EOF, but found at least one more byte")
	} else if err == io.EOF {
		vr.err = nil
	}
}

// ReadBytes reads n bytes from the input.
func (vr *voxReader) ReadBytes(n int) []byte {
	r := make([]byte, n)
	if vr.err != nil {
		return r
	}
	_, vr.err = io.ReadFull(vr.r, r)
	return r
}

// ReadUint8 reads a uint8 from the input.
func (vr *voxReader) ReadUint8() uint8 {
	return vr.ReadBytes(1)[0]
}

// ReadInt32 reads a int32 from the input.
func (vr *voxReader) ReadInt32() int32 {
	b := vr.ReadBytes(4)
	u := uint32(b[0]) + uint32(b[1])<<8 + uint32(b[2])<<16 + uint32(b[3])<<24
	return int32(u)
}

// ReadString reads a .vox-formatted STRING from
// the input.
func (vr *voxReader) ReadString() string {
	n := int(vr.ReadInt32())
	bs := vr.ReadBytes(n)
	return string(bs)
}

// ReadDict reads a .vox-formatted DICT from the input.
func (vr *voxReader) ReadDict() *dict {
	d := &dict{d: map[string]string{}, read: map[string]bool{}}
	n := int(vr.ReadInt32())
	for i := 0; i < n; i++ {
		key := vr.ReadString()
		val := vr.ReadString()
		d.d[key] = val
	}
	if err := vr.Error(); err != nil {
		d.err = err
	}
	return d
}
