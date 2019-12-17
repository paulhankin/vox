package vox

import (
	"fmt"
	"image/color"
	"reflect"
	"testing"
)

func compareStruct(got interface{}, want interface{}) error {
	gotVal := reflect.ValueOf(got)
	wantVal := reflect.ValueOf(want)

	for i := 0; i < wantVal.NumField(); i++ {
		tf := wantVal.Type().Field(i)
		gotf := gotVal.Field(i)
		wantf := wantVal.Field(i)

		// want uses zero fields to indicate "ignore"
		if reflect.DeepEqual(wantf.Interface(), reflect.Zero(tf.Type).Interface()) {
			continue
		}
		if !reflect.DeepEqual(wantf.Interface(), gotf.Interface()) {
			return fmt.Errorf("field %q: got %+v, want %+v", tf.Name, gotf.Interface(), wantf.Interface())
		}
	}
	return nil
}

func TestParse(t *testing.T) {
	main, err := ParseFile("testdata/test.vox")
	if err != nil {
		t.Fatal(err)
	}

	if len(main.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(main.Models))
	}

	model := main.Models[0]

	wantSize := [3]int{30, 20, 10}
	gotSize := [3]int{model.X, model.Y, model.Z}
	if wantSize != gotSize {
		t.Errorf("main.Size = %v, want %v", gotSize, wantSize)
	}

	wantMatls := map[int]Material{
		166: Material{
			Color:     color.RGBA{0x33, 0x66, 0x66, 0xff},
			Type:      MaterialMetal,
			Roughness: 63,
			Specular:  50,
		},
		220: Material{
			Color:       color.RGBA{0x88, 0, 0, 0xff},
			Type:        MaterialGlass,
			Roughness:   78,
			IOR:         1.8,
			Attenuation: 39,
		},
	}
	for idx, wantMat := range wantMatls {
		gotMat := main.Materials[idx]
		if err := compareStruct(gotMat, wantMat); err != nil {
			t.Errorf("material %d: want %+v, got %+v: %v", idx, wantMat, gotMat, err)
		}
	}
}

func TestSceneParse(t *testing.T) {
	// scene.vox contains multiple objects on a few different layers in a scene.
	main, err := ParseFile("testdata/scene.vox")
	if err != nil {
		t.Fatal(err)
	}
	if main == nil {
		t.Fatal("no main")
	}
	// TODO: verify scene structure etc.
}

func TestNewAttrsParse(t *testing.T) {
	// scene.vox contains multiple objects on a few different layers in a scene.
	main, err := ParseFile("testdata/newattrs.vox")
	if err != nil {
		t.Fatal(err)
	}
	if main == nil {
		t.Fatal("no main")
	}
	// TODO: verify scene structure etc.
}

func countVoxels(dw *DenseWorld) (int, error) {
	count := 0
	for i := dw.Min[0]; i < dw.Max[0]; i++ {
		for j := dw.Min[1]; j < dw.Max[1]; j++ {
			for k := dw.Min[2]; k < dw.Max[2]; k++ {
				c, ok := dw.MaterialIndex([3]int{i, j, k})
				if !ok {
					return 0, fmt.Errorf("failed to read voxel %v", [3]int{i, j, k})
				}
				if c != 0 {
					count++
				}
			}
		}
	}
	return count, nil
}

func TestDenseWorldFromModel(t *testing.T) {
	main, err := ParseFile("testdata/scene.vox")
	if err != nil {
		t.Fatal(err)
	}

	mod := main.Models[0]

	// Apply a bunch of rotations and translations, checking we can build the world each time.
	translations := [][3]int32{[3]int32{0, 0, 0}, [3]int32{-100, 0, 5}, [3]int32{213, 42, 64}, [3]int32{-500, 600, -700}}
	vxCount := -1
	for m := Matrix3x3(0); m < 128; m++ {
		if !m.Valid() {
			continue
		}
		for _, tr := range translations {
			tf := TransformFrame{m, tr}
			dw, err := DenseWorldFromModel(tf, mod)
			if err != nil {
				t.Errorf("%#v: failed to create dense world: %v", tf, err)
				continue
			}
			vc, err := countVoxels(dw)
			if err != nil {
				t.Errorf("%#v: failed to count voxels: %v", tf, err)
			}
			if vxCount == -1 {
				vxCount = vc
			}
			if vxCount != vc {
				t.Errorf("%#v: unexpected voxel count %d (previously found %d)", tf, vc, vxCount)
			}
		}
	}
	if vxCount < 10 {
		t.Fatalf("unexpectedly low number of voxels found: %d", vxCount)
	}
	if vxCount > int(mod.X)*int(mod.Y)*int(mod.Z) {
		t.Fatalf("found %d voxels, but that's impossible because the original model was of size %d,%d,%d", vxCount, mod.X, mod.Y, mod.Z)
	}
}
