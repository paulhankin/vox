package vox

import "image/color"

// Main contains the models, palette and materials
// in a magicavoxel .vox file.
type Main struct {
	Models    []Model
	RGBA      []color.RGBA
	Materials []Material
}

type Voxel struct {
	X, Y, Z    uint8
	ColorIndex uint8
}

type Model []Voxel

type MaterialType int

type Material struct {
	Type MaterialType

	// Material weight (0, 1]
	Weight float64

	Plastic      float64
	Roughness    float64
	Specular     float64
	IOR          float64
	Attenuation  float64
	Power        float64
	Glow         float64
	IsTotalPower bool
}
