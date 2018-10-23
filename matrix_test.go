package vox

import (
	"fmt"
	"testing"
)

type mat struct {
	m [3][3]int
}

func matFromMatrix3x3(m Matrix3x3) mat {
	r := mat{}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			r.m[i][j] = m.Get(i, j)
		}
	}
	return r
}

func matToMatrix3x3(m mat) Matrix3x3 {
	var r Matrix3x3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			x := m.m[i][j]
			if x == 0 {
				continue
			}
			if x < 0 {
				r |= 1 << uint(i+4)
			}
			if i < 2 {
				r |= Matrix3x3(j) << uint(2*i)
			}
		}
	}
	return r
}

func (a mat) mul(b mat) mat {
	var r mat
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			for k := 0; k < 3; k++ {
				r.m[i][k] += a.m[i][j] * b.m[j][k]
			}
		}
	}
	return r
}

func (m mat) mulVec(v [3]int) [3]int {
	var r [3]int
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			r[i] += m.m[i][j] * v[j]
		}
	}
	return r
}

func (m mat) valid() bool {
	rows := 0
	cols := 0
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if m.m[i][j] == 0 {
				continue
			}
			if m.m[i][j] > 1 || m.m[i][j] < -1 {
				return false
			}
			if rows&(1<<uint(i)) != 0 {
				return false
			}
			if cols&(1<<uint(j)) != 0 {
				return false
			}
			rows |= 1 << uint(i)
			cols |= 1 << uint(j)
		}
	}
	return rows == 7 && cols == 7
}

func TestValid(t *testing.T) {
	count := 0
	for m := Matrix3x3(0); m < 128; m++ {
		r := matFromMatrix3x3(m)
		got := m.Valid()
		want := r.valid()
		if got != want {
			t.Errorf("%x.Valid() == %v, want %v, %v", m, got, r, want)
		}
		if got {
			count++
		}
	}
	if count != 48 {
		t.Errorf("Found %d valid Matrix3x3, but expected 48", count)
	}
}

func TestIdentity(t *testing.T) {
	id := Matrix3x3Identity
	if r := id.Mul(id); r != id {
		t.Errorf("id * id = %x, want %x", r, id)
	}
	if r := id.Inverse(); r != id {
		t.Errorf("id.Inverse() = %x, want %x", r, id)
	}

}

func TestInverses(t *testing.T) {
	for m := Matrix3x3(0); m < 128; m++ {
		if !m.Valid() {
			continue
		}
		inv := m.Inverse()
		if !inv.Valid() {
			t.Errorf("%x.Inverse().Valid() == false, want true", m)
			continue
		}
		if r := m.Mul(inv); r != 0x04 {
			t.Errorf("%x mul %x.Inverse() [%x] = %x, want 0x04", m, m, inv, r)
		}
		if r := inv.Mul(m); r != 0x04 {
			t.Errorf("%x.Inverse() [%x] mul %x = %x, want 0x04", m, inv, m, r)
		}
	}
}

func TestMul(t *testing.T) {
	for a := Matrix3x3(0); a < 128; a++ {
		if !a.Valid() {
			continue
		}
		prods := map[Matrix3x3]bool{}
		ma := matFromMatrix3x3(a)
		for b := Matrix3x3(0); b < 128; b++ {
			if !b.Valid() {
				continue
			}
			mb := matFromMatrix3x3(b)
			ab := a.Mul(b)
			if !ab.Valid() {
				t.Errorf("%x * %x = %x isn't Valid()", a, b, ab)
			}
			prods[ab] = true
			mab := ma.mul(mb)
			abWant := matToMatrix3x3(mab)
			mabWant := matFromMatrix3x3(ab)

			if ab != abWant {
				t.Errorf("%x * %x = %x, want %x", a, b, ab, abWant)
			}
			if mab != mabWant {
				t.Errorf("%v * %v = %v, want %v", ma, mb, mab, mabWant)
			}
		}
		if len(prods) != 48 {
			t.Errorf("The number of products of valid matrices with %x is %d, want 48", a, len(prods))
		}
	}
}

func abseq(a, b int) bool {
	return a == b || a == -b
}

// vecSeemsPlausible checks that b is a permutation of a, possibly with
// sign changes. That's always true if b is supposed to be a multiplied by
// a Matrix3x3.
func vecSeemsPlausible(a, b [3]int) error {
	// We iterate over i, j, k, permutations of [0, 1, 2]
	for i := 0; i < 3; i++ {
		for j2 := 0; j2 < 2; j2++ {
			j := j2
			if j >= i {
				j++
			}
			k := 3 - i - j
			if abseq(a[0], b[i]) && abseq(a[1], b[j]) && abseq(a[2], b[k]) {
				return nil
			}
		}
	}
	return fmt.Errorf("%v is not a permuation (with sign changes) of %v", a, b)
}

func TestMulVec(t *testing.T) {
	for m := Matrix3x3(0); m < 128; m++ {
		if !m.Valid() {
			continue
		}
		ma := matFromMatrix3x3(m)

		// We multiply m with all [x, y, z] where each of x, y, z
		// are taken from vals.
		vals := []int{-5, -2, -1, 0, 1, 2, 5}
		for _, x := range vals {
			for _, y := range vals {
				for _, z := range vals {
					v := [3]int{x, y, z}
					got := m.MulVec(v)
					want := ma.mulVec(v)
					if got != want {
						t.Errorf("%x * %v = %v, want %v", m, v, got, want)
						continue
					}
					if err := vecSeemsPlausible(v, got); err != nil {
						t.Errorf("%x * %v = %v. %v", m, v, got, err)
					}
				}
			}
		}
	}
}
