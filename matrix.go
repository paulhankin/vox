package vox

// Matrix3x3 is an encoded 3x3 orthogonal matrix with entries 0, +1, -1.
type Matrix3x3 uint8

// Matrix3x3Identity represents the identity matrix.
const Matrix3x3Identity = Matrix3x3(4)

func eqm3(a int, b Matrix3x3) int {
	if a == int(b) {
		return 1
	}
	return 0
}

func signm3(x Matrix3x3) int {
	if x == 0 {
		return 1
	}
	return -1
}

// Valid reports whether m is a valid 3x3 matrix.
func (m Matrix3x3) Valid() bool {
	// We can't have the top bit set, and the position of
	// the non-zero entry in the first and second rows
	// can't be the same. Neither can be 3.
	one1 := m & 3
	one2 := (m >> 2) & 3
	return m&128 == 0 && one1 != one2 && one1 != 3 && one2 != 3
}

// Get returns the ith row, jth column of the decoded matrix.
// It must be that 0 <= i, j <= 2, and it returns 0, 1 or -1.
func (m Matrix3x3) Get(i, j int) int {
	// From https://github.com/ephtracy/voxel-model/blob/master/MagicaVoxel-file-format-vox-extension.txt
	// bit | value
	// 0-1 : 1 : index of the non-zero entry in the first row
	// 2-3 : 2 : index of the non-zero entry in the second row
	// 4   : 0 : the sign in the first row (0 : positive; 1 : negative)
	// 5   : 1 : the sign in the second row (0 : positive; 1 : negative)
	// 6   : 1 : the sign in the third row (0 : positive; 1 : negative)
	if i == 0 {
		return eqm3(j, m&3) * signm3((m>>4)&1)
	} else if i == 1 {
		return eqm3(j, (m>>2)&3) * signm3((m>>5)&1)
	}
	return eqm3(j, 3-(m&3)-((m>>2)&3)) * signm3((m>>6)&1)
}

// Mul multiplies the two matrices together, returning the result.
func (a Matrix3x3) Mul(b Matrix3x3) Matrix3x3 {
	var r Matrix3x3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			for k := 0; k < 3; k++ {
				x := a.Get(i, j) * b.Get(j, k)
				if x == 0 {
					continue
				}
				if x < 0 {
					r |= 1 << uint(i+4)
				}
				if i < 2 {
					r |= Matrix3x3(k) << uint(2*i)
				}
			}
		}
	}
	return r
}

// MulVec multiplies the matrix and the given vector.
func (m Matrix3x3) MulVec(x [3]int) [3]int {
	var r [3]int
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			r[i] += m.Get(i, j) * x[j]
		}
	}
	return r
}

var matInverses [128]Matrix3x3

func init() {
	for m := Matrix3x3(0); m < 128; m++ {
		if !m.Valid() {
			continue
		}
		for r := Matrix3x3(0); r < 128; r++ {
			if !r.Valid() {
				continue
			}
			if m.Mul(r) == 0x04 {
				matInverses[int(m)] = r
			}
		}
	}
}

// Inverse returns the inverse of the given matrix.
func (m Matrix3x3) Inverse() Matrix3x3 {
	if m >= 128 {
		return 0
	}
	return matInverses[int(m)]
}
