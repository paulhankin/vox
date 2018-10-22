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
