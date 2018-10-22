package vox

import "fmt"

// A DenseWorld is an arbitrarily sized voxel model.
type DenseWorld struct {
	Min, Max [3]int // The range of coordinates that are valid.

	// A slice large enough to contain every voxel.
	// Voxels[0] is the voxel Min, and the last
	// element in the slice is the voxel Max.
	// The elements are stored in X, Y, Z min to max significance.
	Voxels []uint8
}

// NewDenseWorld creates a new dense world for the given cuboid.
func NewDenseWorld(min, max [3]int) (*DenseWorld, error) {
	if max[0] < min[0] || max[1] < min[1] || max[2] < min[2] {
		return nil, fmt.Errorf("the upper bounds of the cuboid %v must be at least as large as the lower bounds %v", max, min)
	}
	sx := max[0] - min[0] + 1
	sy := max[1] - min[1] + 1
	sz := max[2] - min[2] + 1
	return &DenseWorld{min, max, make([]uint8, sx*sy*sz)}, nil
}

// Cuboid returns the size of the world.
func (d *DenseWorld) Cuboid() (min, max [3]int) {
	return d.Min, d.Max
}

// Resize changes the size of the world, copying voxels
// where the areas overlap. This is an expensive operation.
func (d *DenseWorld) Resize(min, max [3]int) error {
	if min == d.Min && max == d.Max {
		return nil
	}
	ndw, err := NewDenseWorld(min, max)
	if err != nil {
		return err
	}
	SX := d.Max[0] - d.Min[0] + 1
	SY := d.Max[1] - d.Min[1] + 1
	for i, c := range d.Voxels {
		x := i % SX
		y := ((i - x) / SX) % SY
		z := ((i - x) / SX) / SY
		ndw.SetMaterialIndex([3]int{x + d.Min[0], y + d.Min[1], z + d.Min[2]}, c)
	}
	*d = *ndw
	return nil
}

// MaterialIndex returns the given voxel material.
func (d *DenseWorld) MaterialIndex(c [3]int) (uint8, bool) {
	SX := d.Max[0] - d.Min[0] + 1
	SY := d.Max[1] - d.Min[1] + 1
	SZ := d.Max[2] - d.Min[2] + 1

	x := c[0] - d.Min[0]
	y := c[1] - d.Min[1]
	z := c[2] - d.Min[2]

	if x < 0 || x >= SX || y < 0 || y >= SY || z < 0 || z >= SZ {
		// out of range
		return 0, false
	}

	return d.Voxels[z*(SX*SY)+y*SX+x], true
}

// SetMaterialIndex sets the given voxel to the given material index.
// It reports if the assignment succeeded.
func (d *DenseWorld) SetMaterialIndex(c [3]int, matIdx uint8) bool {
	SX := d.Max[0] - d.Min[0] + 1
	SY := d.Max[1] - d.Min[1] + 1
	SZ := d.Max[2] - d.Min[2] + 1

	x := c[0] - d.Min[0]
	y := c[1] - d.Min[1]
	z := c[2] - d.Min[2]

	if x < 0 || x >= SX || y < 0 || y >= SY || z < 0 || z >= SZ {
		// out of range
		return false
	}

	d.Voxels[z*(SX*SY)+y*SX+x] = matIdx
	return true
}

// DenseWorldFromModel takes a magicavoxel transform and a model, and builds
// a DenseWorld from it.
func DenseWorldFromModel(tf TransformFrame, m Model) (*DenseWorld, error) {
	mat := tf.R
	matInv := mat.Inverse()
	_ = matInv
	return nil, fmt.Errorf("unimplemented")
}
