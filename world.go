package vox

import (
	"fmt"
	"math"
)

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

func addVec(a, b [3]int) [3]int {
	return [3]int{a[0] + b[0], a[1] + b[1], a[2] + b[2]}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// DenseWorldFromModel takes a magicavoxel transform and a model, and builds
// a DenseWorld from it.
func DenseWorldFromModel(tf TransformFrame, m Model) (*DenseWorld, error) {
	mat := tf.R

	v := [3]int{m.X, m.Y, m.Z}
	mv := mat.MulVec(v)
	mv[0] = abs(mv[0]) - 1
	mv[1] = abs(mv[1]) - 1
	mv[2] = abs(mv[2]) - 1
	// magicvoxel puts the majority of the voxel block on the positive side of the zero axis.
	min := [3]int{-(mv[0] / 2), -(mv[1] / 2), -(mv[2] / 2)}
	max := [3]int{mv[0] + min[0], mv[1] + min[1], mv[2] + min[2]}
	T := [3]int{int(tf.T[0]), int(tf.T[1]), int(tf.T[2])}
	min = addVec(min, T)
	max = addVec(max, T)
	dw, err := NewDenseWorld(min, max)
	if err != nil {
		return nil, err
	}

	// find the corner of the model that maps to the smallest point.
	minCorner := [3]int{math.MaxInt64, math.MaxInt64, math.MaxInt64}
	for i := 0; i <= 1; i++ {
		for j := 0; j <= 1; j++ {
			for k := 0; k <= 1; k++ {
				x := [3]int{i * (m.X - 1), j * (m.Y - 1), k * (m.Z - 1)}
				mx := mat.MulVec(x)
				if mx[0] <= minCorner[0] && mx[1] <= minCorner[1] && mx[2] <= minCorner[2] {
					minCorner = mx
				}
			}
		}
	}
	// The translation that maps the unrotated model into the dense world coordinate space.
	trn := [3]int{min[0] - minCorner[0], min[1] - minCorner[1], min[2] - minCorner[2]}

	for _, vox := range m.V {
		voxLoc := [3]int{int(vox.X), int(vox.Y), int(vox.Z)}
		rv := mat.MulVec(voxLoc)
		rv = addVec(rv, trn)
		ok := dw.SetMaterialIndex(rv, vox.ColorIndex)
		if !ok {
			return nil, fmt.Errorf("rotation/translation is messed up. %x * %v = %v, out of bounds %v, %v", mat, voxLoc, rv, min, max)
		}
	}

	return dw, nil
}
