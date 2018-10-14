// Package vox provides types for describing magicavoxel models,
// and a parser for .vox files.
package vox

import "image/color"

// Main contains the models, palette and materials
// in a magicavoxel .vox file.
type Main struct {
	Models    []Model
	RGBA      []color.RGBA
	Materials []Material
}

// A Voxel is a single voxel in a model.
type Voxel struct {
	X, Y, Z    uint8
	ColorIndex uint8
}

// A Model is one model in a .vox file.
type Model struct {
	X, Y, Z int // Size
	V       []Voxel
}

// MaterialType describes the nature of a material.
type MaterialType int

const (
	MaterialDiffuse  MaterialType = 0
	MaterialMetal    MaterialType = 1
	MaterialGlass    MaterialType = 2
	MaterialEmissive MaterialType = 3
)

// Material describes a material.
type Material struct {
	Type MaterialType

	// Material weight (0, 1]
	// Affects how much of a blend between the given
	// material type and a pure diffuse material.
	Weight float32

	Plastic     bool
	Roughness   float32
	Specular    float32
	IOR         float32
	Attenuation float32
	Flux        float32
}
