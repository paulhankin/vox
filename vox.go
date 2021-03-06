// Package vox provides types for describing magicavoxel models,
// and a parser for .vox files.
package vox

import (
	"fmt"
	"image/color"
	"strings"
)

// Main contains the models, palette and materials
// in a magicavoxel .vox file.
type Main struct {
	Models    []Model
	Materials []Material
	Scene     Scene
}

// A Voxel is a single voxel in a model.
type Voxel struct {
	X, Y, Z    uint8
	ColorIndex uint8
}

func (v Voxel) String() string {
	return fmt.Sprintf("V{%d,%d,%d:%d}", v.X, v.Y, v.Z, v.ColorIndex)
}

// A Model is one model in a .vox file.
type Model struct {
	X, Y, Z int // Size
	V       []Voxel
}

// A Layer groups scene nodes.
type Layer struct {
	Index  int32
	Name   string
	Hidden bool
}

// A Scene describes how the various models in the file are arranged
// together.
type Scene struct {
	Layers []Layer
	Node   *TransformNode // The top node of the scene graph
}

// AnyNode is a type that can be any of the scene node types.
type AnyNode interface {
	isNode()
}

// Node is the information contained in every scene node.
type Node struct {
	Name   string
	Hidden bool
}

func (n Node) isNode() {}

// A TransformNode is a node in the scene graph that moves and rotates
// its children.
type TransformNode struct {
	Node
	Layer      *Layer           // The layer this node belongs to (or nil if it's the root node).
	Transforms []TransformFrame // Currently must be a single element.
	Child      AnyNode          // Child nodes that are affected by this transformation.
}

func (tn *TransformNode) isSceneNode() {}

func (tn *TransformNode) String() string {
	parts := []string{}
	if tn.Name != "" {
		parts = append(parts, fmt.Sprintf("name:%q", tn.Name))
	}
	if tn.Hidden {
		parts = append(parts, "hidden")
	}
	if tn.Layer != nil {
		parts = append(parts, fmt.Sprintf("layer:%d", tn.Layer.Index))
	}
	for _, tr := range tn.Transforms {
		parts = append(parts, tr.String())
	}
	parts = append(parts, fmt.Sprintf("child:%p", tn.Child))
	return fmt.Sprintf("Transform{%s}", strings.Join(parts, ", "))
}

// TransformFrame describes how a transform node affects
// its children.
type TransformFrame struct {
	R Matrix3x3 // Rotation
	T [3]int32  // Translation
}

func (tn TransformFrame) String() string {
	return fmt.Sprintf("%#v", tn)
}

// A GroupNode is a node that can group one or more subnodes.
type GroupNode struct {
	Node
	Children []AnyNode
}

func (gn *GroupNode) isSceneNode() {}

func (gn *GroupNode) String() string {
	parts := []string{}
	if gn.Name != "" {
		parts = append(parts, fmt.Sprintf("name:%q", gn.Name))
	}
	if gn.Hidden {
		parts = append(parts, "hidden")
	}
	for _, c := range gn.Children {
		parts = append(parts, fmt.Sprintf("child:%p", c))
	}
	return fmt.Sprintf("Group{%s}", strings.Join(parts, ", "))
}

// A ShapeNode is a terminal node in the scene graph that refers
// to a voxel model.
type ShapeNode struct {
	Node
	Models []*Model // Currently must be a single model.
}

func (sn *ShapeNode) String() string {
	parts := []string{}
	if sn.Name != "" {
		parts = append(parts, fmt.Sprintf("name:%q", sn.Name))
	}
	if sn.Hidden {
		parts = append(parts, "hidden")
	}
	for _, m := range sn.Models {
		parts = append(parts, fmt.Sprintf("model:%p", m))
	}
	return fmt.Sprintf("Shape{%s}", strings.Join(parts, ", "))
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
	Color color.RGBA
	Type  MaterialType

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
	LDR         float32
}

func (mt MaterialType) String() string {
	switch mt {
	case MaterialDiffuse:
		return "diffuse"
	case MaterialMetal:
		return "metal"
	case MaterialGlass:
		return "glass"
	case MaterialEmissive:
		return "emissive"
	}
	return fmt.Sprintf("MaterialType(%d)", mt)
}

func (m Material) String() string {
	parts := []string{fmt.Sprintf("rgba:%02x%02x%02x%02x", m.Color.R, m.Color.G, m.Color.B, m.Color.A), m.Type.String()}
	if m.Weight != 100 {
		parts = append(parts, fmt.Sprintf("w:%.1f", m.Weight))
	}
	if m.Plastic && m.Type == MaterialMetal {
		parts = append(parts, fmt.Sprintf("plastic"))
	}
	all := 1 + 2 + 4 + 8
	for _, v := range []struct {
		s   string
		v   float32
		def float32
		mt  int
	}{
		{"rough", m.Roughness, 0, all},
		{"spec", m.Specular, 0, all},
		{"ior", m.IOR, 1.0, (1 << uint(MaterialGlass))},
		{"attn", m.Attenuation, 100, (1 << uint(MaterialGlass))},
		{"flux", m.Flux, 0, (1 << uint(MaterialEmissive))},
		{"ldr", m.LDR, 0, (1 << uint(MaterialEmissive))},
	} {
		if v.v != v.def && (1<<uint(m.Type))&v.mt != 0 {
			parts = append(parts, fmt.Sprintf("%s:%.1f", v.s, v.v))
		}
	}
	return fmt.Sprintf("Mat{%s}", strings.Join(parts, ", "))
}
