package vox

import (
	"bufio"
	"bytes"
	"fmt"
	"image/color"
	"io"
	"log"
	"os"
	"sort"
)

const version = 150

// parseChunk reads a RIFF chunk from the input, returning the ID (MAIN, MATL, etc.)
// and the bytes that hold the contents of this chunk and any child contents.
func parseChunk(vr *voxReader) (ID string, contents, childContents []byte, err error) {
	id := vr.ReadBytes(4)
	N := vr.ReadInt32()
	M := vr.ReadInt32()
	if err := vr.Error(); err != nil {
		return "", nil, nil, err
	}
	c := vr.ReadBytes(int(N))
	cc := vr.ReadBytes(int(M))
	if err := vr.Error(); err != nil {
		return "", nil, nil, err
	}
	return string(id), c, cc, nil
}

func buildMain(models []Model, rgba []color.RGBA, mats []Material, scene Scene) (*Main, error) {
	if len(rgba) != 256 {
		return nil, fmt.Errorf("expected 256 palette entries, but found %d", len(rgba))
	}
	for i := 1; i < 256; i++ {
		mats[i].Color = rgba[i-1]
	}
	return &Main{
		Models:    models,
		Materials: mats,
		Scene:     scene,
	}, nil
}

type addChilder interface {
	addChild(c AnyNode) error
}

func (t *TransformNode) addChild(c AnyNode) error {
	if t.Child != nil {
		return fmt.Errorf("can't add two children nodes to transform node")
	}
	t.Child = c
	return nil
}

func (g *GroupNode) addChild(c AnyNode) error {
	g.Children = append(g.Children, c)
	return nil
}

type nodeCounter interface {
	nodeCount(visited map[AnyNode]bool) (int, error)
}

func (t *TransformNode) nodeCount(visited map[AnyNode]bool) (int, error) {
	if visited[AnyNode(t)] {
		return 0, fmt.Errorf("cycle found")
	}
	visited[AnyNode(t)] = true
	r := 1
	if t.Child != nil {
		rc, err := t.Child.(nodeCounter).nodeCount(visited)
		if err != nil {
			return 0, err
		}
		r += rc
	}
	return r, nil
}

func (g *GroupNode) nodeCount(visited map[AnyNode]bool) (int, error) {
	if visited[AnyNode(g)] {
		return 0, fmt.Errorf("cycle found")
	}
	visited[AnyNode(g)] = true
	r := 1
	for _, c := range g.Children {
		rc, err := c.(nodeCounter).nodeCount(visited)
		if err != nil {
			return 0, err
		}
		r += rc
	}
	return r, nil
}

func (s *ShapeNode) nodeCount(visited map[AnyNode]bool) (int, error) {
	if visited[AnyNode(s)] {
		return 0, fmt.Errorf("cycle found")
	}
	return 1, nil
}

func buildScene(sceneIDs map[int32]AnyNode, sceneChildrenIDs map[int32][]int32, sceneLayers map[int32]int32, layerIDs map[int32]*Layer) (Scene, error) {
	scene := Scene{}
	for _, layer := range layerIDs {
		scene.Layers = append(scene.Layers, *layer)
	}
	sort.Slice(scene.Layers, func(i, j int) bool { return scene.Layers[i].Index < scene.Layers[j].Index })

	// The root node in the scene is a transform node with layer -1.
	var top *TransformNode
	for k, v := range sceneIDs {
		if tn, ok := v.(*TransformNode); ok {
			if lid, ok := sceneLayers[k]; ok && lid == -1 {
				if top != nil {
					return scene, fmt.Errorf("scene has two root nodes")
				}
				top = tn
			}
		}
	}
	if top == nil {
		return scene, fmt.Errorf("failed to find root node in the scene graph")
	}
	for scid, children := range sceneChildrenIDs {
		node, ok := sceneIDs[scid]
		if !ok {
			return Scene{}, fmt.Errorf("node %d has children, but doesn't exist in the scene graph", scid)
		}
		cs, ok := node.(addChilder)
		if !ok {
			return Scene{}, fmt.Errorf("node %d has type %T, which we can't add children to", scid, node)
		}
		for _, c := range children {
			cn, ok := sceneIDs[c]
			if !ok {
				return Scene{}, fmt.Errorf("node %d has child %d, but no such node exists", scid, c)
			}
			if err := cs.addChild(cn); err != nil {
				return Scene{}, err
			}
		}
	}

	nc, err := top.nodeCount(map[AnyNode]bool{})
	if err != nil {
		return Scene{}, err
	}
	if nc != len(sceneIDs) {
		return Scene{}, fmt.Errorf("not all nodes are in scene. %d nodes, but only %d in scene", len(sceneIDs), nc)
	}

	scene.Node = top
	return scene, nil
}

type state int

const (
	statePack state = iota
	stateSize
	stateXYZI
	stateRGBA
	stateSceneGraph
	stateLAYR
	stateMatt
)

// parsePackChunk reads a PACK chunk from the input,
// returning the int it contains.
func parsePackChunk(c []byte) (int, error) {
	vr := &voxReader{r: bytes.NewReader(c)}
	n := vr.ReadInt32()
	vr.RequireEOF("PACK")
	return int(n), vr.Error()
}

// parseSizeChunk parses a SIZE chunk from the input,
// returning the size it contains.
func parseSizeChunk(c []byte) ([3]int32, error) {
	vr := &voxReader{r: bytes.NewReader(c)}
	x := vr.ReadInt32()
	y := vr.ReadInt32()
	z := vr.ReadInt32()
	vr.RequireEOF("SIZE")
	return [3]int32{x, y, z}, vr.Error()
}

// parseXYZIChunk parses an XYZI chunk from the input,
// returning the voxels it contains.
func parseXYZIChunk(c []byte) ([]Voxel, error) {
	vr := &voxReader{r: bytes.NewReader(c)}
	N := int(vr.ReadInt32())
	v := []Voxel{}
	for i := 0; i < N; i++ {
		x := vr.ReadUint8()
		y := vr.ReadUint8()
		z := vr.ReadUint8()
		idx := vr.ReadUint8()
		v = append(v, Voxel{x, y, z, idx})
	}
	vr.RequireEOF("XYZI")
	return v, vr.Error()
}

// parseRGBAChunk parses an RGBA chunk from the input,
// returning the colors it contains.
func parseRGBAChunk(c []byte) ([]color.RGBA, error) {
	vr := &voxReader{r: bytes.NewReader(c)}
	r := make([]color.RGBA, 256)
	for i := range r {
		cr := vr.ReadUint8()
		cg := vr.ReadUint8()
		cb := vr.ReadUint8()
		ca := vr.ReadUint8()
		r[i] = color.RGBA{cr, cg, cb, ca}
	}
	vr.RequireEOF("RGBA")
	return r, vr.Error()
}

// parsenTRNChunk parses a nTRN (transform) node chunk from the input,
// returning the ids it contains, along with the partially filled-in
// TransformNode.
func parsenTRNChunk(c []byte) (id, childID, layerID int32, n *TransformNode, err error) {
	vr := &voxReader{r: bytes.NewReader(c)}
	id = vr.ReadInt32()
	attr := vr.ReadDict()
	childID = vr.ReadInt32()
	reserved := vr.ReadInt32()
	layerID = vr.ReadInt32()
	nFrame := vr.ReadInt32()
	frames := []*dict{}
	for i := 0; i < int(nFrame); i++ {
		frames = append(frames, vr.ReadDict())
	}

	if err = vr.Error(); err != nil {
		return 0, 0, 0, nil, err
	}

	if reserved != -1 {
		return 0, 0, 0, nil, fmt.Errorf("reserved field in nTRN must be -1, got %d", reserved)
	}

	if nFrame != 1 {
		return 0, 0, 0, nil, fmt.Errorf("must have one frame in nTRN chunk, got %d", nFrame)
	}

	name := attr.ReadString("_name", "")
	hidden := attr.ReadBool("_hidden", false)

	if err := attr.Error(); err != nil {
		return 0, 0, 0, nil, fmt.Errorf("error reading nTRN chunk: %v", err)
	}

	if err := attr.AssertNoUnreadFields(); err != nil {
		return 0, 0, 0, nil, fmt.Errorf("unexpected field or fields in nTRN attributes: %v", err)
	}

	r := frames[0].ReadMatrix3x3("_r", Matrix3x3Identity)
	t := frames[0].Read3xInt32("_t", [3]int32{0, 0, 0})

	if err := frames[0].Error(); err != nil {
		return 0, 0, 0, nil, fmt.Errorf("error reading nTRN frame chunk: %v", err)
	}

	if err := frames[0].AssertNoUnreadFields(); err != nil {
		return 0, 0, 0, nil, fmt.Errorf("unexpected field or fields in nTRN frame: %v", err)
	}

	vr.RequireEOF("nTRN")

	return id, childID, layerID, &TransformNode{
		Node: Node{Name: name, Hidden: hidden},
		Transforms: []TransformFrame{
			{
				R: r,
				T: t,
			},
		},
	}, vr.Error()
}

// parsenGRPChunk parses a nGRP (group) node chunk from the input,
// returning the ids it contains, along with the partially filled-in
// GroupNode.
func parsenGRPChunk(c []byte) (id int32, childIDs []int32, g *GroupNode, err error) {
	vr := &voxReader{r: bytes.NewReader(c)}
	id = vr.ReadInt32()
	attr := vr.ReadDict()
	nChild := vr.ReadInt32()
	childIDs = []int32{}
	for i := 0; i < int(nChild); i++ {
		childIDs = append(childIDs, vr.ReadInt32())
	}

	if err := vr.Error(); err != nil {
		return 0, nil, nil, fmt.Errorf("error reading nGRP chunk: %v", err)
	}

	name := attr.ReadString("_name", "")
	hidden := attr.ReadBool("_hidden", false)

	if err := attr.Error(); err != nil {
		return 0, nil, nil, fmt.Errorf("error reading nGRP chunk: %v", err)
	}
	if err := attr.AssertNoUnreadFields(); err != nil {
		return 0, nil, nil, fmt.Errorf("unexpected fields in nGRP chunk attributes: %v", err)
	}

	vr.RequireEOF("nGRP")

	return id, childIDs, &GroupNode{Node: Node{name, hidden}}, vr.Error()
}

// parsenSHPChunk parses a nSHP (shape) node chunk from the input,
// returning the model IDs it contains, along with the partially filled-in
// ShapeNode.
func parsenSHPChunk(c []byte) (id int32, modelIDs []int32, s *ShapeNode, err error) {
	vr := &voxReader{r: bytes.NewReader(c)}
	id = vr.ReadInt32()
	attr := vr.ReadDict()
	nModel := vr.ReadInt32()
	modelIDs = []int32{}
	for i := 0; i < int(nModel); i++ {
		modelIDs = append(modelIDs, vr.ReadInt32())
		_ = vr.ReadDict()
	}
	if err := vr.Error(); err != nil {
		return 0, nil, nil, fmt.Errorf("error reading nSHP chunk: %v", err)
	}

	name := attr.ReadString("_name", "")
	hidden := attr.ReadBool("_hidden", false)

	if err := attr.Error(); err != nil {
		return 0, nil, nil, fmt.Errorf("error reading nSHP chunk: %v", err)
	}
	if err := attr.AssertNoUnreadFields(); err != nil {
		return 0, nil, nil, fmt.Errorf("unexpected fields in nSHP chunk attributes: %v", err)
	}

	vr.RequireEOF("nSHP")
	return id, modelIDs, &ShapeNode{Node: Node{name, hidden}}, vr.Error()
}

// parseLAYRChunk parses a LAYR (layer) chunk from the input,
// returning its ID and the layer information it contains.
func parseLAYRChunk(c []byte) (int32, *Layer, error) {
	vr := &voxReader{r: bytes.NewReader(c)}
	id := vr.ReadInt32()
	attr := vr.ReadDict()
	reserved := vr.ReadInt32()

	if err := vr.Error(); err != nil {
		return 0, nil, fmt.Errorf("error reading LAYR chunk: %v", err)
	}
	if reserved != -1 {
		return 0, nil, fmt.Errorf("reserved field in LAYR chunk must be -1, got %d", reserved)
	}

	name := attr.ReadString("_name", "")
	hidden := attr.ReadBool("_hidden", false)

	if err := attr.Error(); err != nil {
		return 0, nil, fmt.Errorf("error reading LAYR chunk: %v", err)
	}
	if err := attr.AssertNoUnreadFields(); err != nil {
		return 0, nil, fmt.Errorf("unexpected fields in LAYR chunk attributes: %v", err)
	}

	vr.RequireEOF("LAYR")

	return id, &Layer{
		Index:  id,
		Name:   name,
		Hidden: hidden,
	}, vr.Error()
}

// parseMatType returns the corresponding material from
// the string label in the MATL dict.
func parseMatType(s string) (MaterialType, error) {
	switch s {
	case "_diffuse":
		return MaterialDiffuse, nil
	case "_metal":
		return MaterialMetal, nil
	case "_glass":
		return MaterialGlass, nil
	case "_emit":
		return MaterialEmissive, nil
	}
	return MaterialDiffuse, fmt.Errorf("unknown material %q", s)
}

// parseMatlChunk parses a MATL chunk, returning the ID of the
// material and its properties.
func parseMatlChunk(c []byte) (int, Material, error) {
	vr := &voxReader{r: bytes.NewReader(c)}
	matID := vr.ReadInt32()
	if matID > 255 || matID < 0 {
		return 0, Material{}, fmt.Errorf("material index %d out of range", matID)
	}
	d := vr.ReadDict()
	vr.RequireEOF("MATL")
	if err := vr.Error(); err != nil {
		return 0, Material{}, fmt.Errorf("error reading MATL chunk: %v", err)
	}

	// TODO: some of these floats need renormalizing.
	matTypeS := d.ReadString("_type", "<missing>")
	weight := d.ReadFloat("_weight", 1) * 100
	rough := d.ReadFloat("_rough", 0) * 100
	spec := d.ReadFloat("_spec", 0) * 100
	ior := d.ReadFloat("_ior", 0) + 1.0
	att := d.ReadFloat("_att", 0) * 100
	flux := d.ReadFloat("_flux", 0) * 100
	plastic := d.ReadBool("_plastic", false)
	ldr := d.ReadFloat("_ldr", 0) * 100 // not in spec, but present in files

	if err := d.Error(); err != nil {
		return 0, Material{}, fmt.Errorf("dict error reading MATL chunk: %v", err)
	}

	if err := d.AssertNoUnreadFields(); err != nil {
		return 0, Material{}, fmt.Errorf("dict error -- unknown field: %v", err)
	}

	matType, err := parseMatType(matTypeS)
	if err != nil {
		return 0, Material{}, fmt.Errorf("error reading MATL chunk: %v", err)
	}

	return int(matID), Material{
		Type:        matType,
		Weight:      weight,
		Roughness:   rough,
		Specular:    spec,
		IOR:         ior,
		Attenuation: att,
		Flux:        flux,
		Plastic:     plastic,
		LDR:         ldr,
	}, nil
}

// parseMainChunks parses the child chunks of a MAIN chunk.
func parseMainChunks(vr *voxReader) (*Main, error) {
	state := statePack
	pack := -1
	models := []Model{}
	var rgba []color.RGBA
	mats := []Material{}
	var size [3]int32

	// map ids to scene nodes
	sceneIDs := map[int32]AnyNode{}
	// map scene node ids to their children.
	sceneChildren := map[int32][]int32{}
	// map scene node ids to their layer.
	sceneLayer := map[int32]int32{}
	// map layer IDs to the corresponding lyaer.
	layerIDs := map[int32]*Layer{}

	ignoredChunks := map[string]bool{
		"rOBJ": true,
	}

	for {
		id, c, cc, err := parseChunk(vr)
		if err == io.EOF {
			if pack != -1 && len(models) != pack {
				return nil, fmt.Errorf("expected %d models, but got %d", pack, len(models))
			}
			scene, err := buildScene(sceneIDs, sceneChildren, sceneLayer, layerIDs)
			if err != nil {
				return nil, fmt.Errorf("error building scene graph: %v", err)
			}
			return buildMain(models, rgba, mats, scene)
		}
		if err != nil {
			return nil, err
		}
		switch id {
		case "PACK":
			if state != statePack {
				return nil, fmt.Errorf("PACK chunk must appear first in MAIN")
			}
			pack, err = parsePackChunk(c)
			if err != nil {
				return nil, err
			}
			state = stateSize
		case "SIZE":
			if state == statePack {
				state = stateSize
			}
			if state != stateSize {
				return nil, fmt.Errorf("misplaced SIZE chunk")
			}
			size, err = parseSizeChunk(c)
			if err != nil {
				return nil, err
			}
			state = stateXYZI
		case "XYZI":
			if state != stateXYZI {
				return nil, fmt.Errorf("misplaced XYZI chunk")
			}
			var vs []Voxel
			vs, err = parseXYZIChunk(c)
			if err != nil {
				return nil, err
			}
			models = append(models, Model{X: int(size[0]), Y: int(size[1]), Z: int(size[2]), V: vs})
			if pack != -1 && len(models) == pack {
				state = stateSceneGraph
			} else {
				state = stateSize
			}
		case "nTRN":
			if state == stateSize {
				if pack != -1 {
					return nil, fmt.Errorf("missing models: expected %d but found %d", pack, len(models))
				}
				state = stateSceneGraph
			}
			if state != stateSceneGraph {
				return nil, fmt.Errorf("misplaced nTRN chunk")
			}
			id, childID, layerID, node, err := parsenTRNChunk(c)
			if err != nil {
				return nil, err
			}
			if _, ok := sceneIDs[id]; ok {
				return nil, fmt.Errorf("node %d appears twice", id)
			}
			sceneIDs[id] = node
			sceneChildren[id] = []int32{childID}
			sceneLayer[id] = layerID
		case "nGRP":
			if state != stateSceneGraph {
				return nil, fmt.Errorf("misplaced nGRP chunk")
			}
			id, childIDs, node, err := parsenGRPChunk(c)
			if err != nil {
				return nil, err
			}
			if _, ok := sceneIDs[id]; ok {
				return nil, fmt.Errorf("node %d appears twice", id)
			}
			sceneIDs[id] = node
			sceneChildren[id] = childIDs
		case "nSHP":
			if state != stateSceneGraph {
				return nil, fmt.Errorf("misplaced nSHP chunk")
			}
			id, modelIDs, node, err := parsenSHPChunk(c)
			if err != nil {
				return nil, err
			}
			if _, ok := sceneIDs[id]; ok {
				return nil, fmt.Errorf("node %d appears twice", id)
			}
			for _, modelID := range modelIDs {
				if modelID < 0 || int(modelID) >= len(models) {
					return nil, fmt.Errorf("nSHP node refers to missing model ID %d", modelID)
				}
				node.Models = append(node.Models, &models[int(modelID)])
			}
			sceneIDs[id] = node
		case "LAYR":
			if state == stateSceneGraph {
				state = stateLAYR
			}
			if state != stateLAYR {
				return nil, fmt.Errorf("misplaced LAYR chunk")
			}
			id, layer, err := parseLAYRChunk(c)
			if err != nil {
				return nil, err
			}
			if _, ok := layerIDs[id]; ok {
				return nil, fmt.Errorf("two LAYR chunks have id %d", id)
			}
			layerIDs[id] = layer
		case "RGBA":
			if state == stateLAYR {
				// We've just finished parsing the layers
				state = stateRGBA
			}
			if state != stateRGBA {
				return nil, fmt.Errorf("misplaced RGBA chunk")
			}
			rgba, err = parseRGBAChunk(c)
			state = stateMatt
		case "MATL":
			if state != stateMatt {
				return nil, fmt.Errorf("misplaced MATL chunk")
			}
			idx, mat, err := parseMatlChunk(c)
			if err != nil {
				return nil, err
			}
			for len(mats) <= idx {
				mats = append(mats, Material{})
			}
			mats[idx] = mat
		default:
			if !ignoredChunks[id] {
				log.Printf("unexpected chunk %s\n", id)
				ignoredChunks[id] = true // stop the error appearing multiple times
			}
			continue
		}
		if len(cc) != 0 {
			return nil, fmt.Errorf("unexpected child chunks of chunk %q", id)
		}
	}
}

// parseMainChunk parses the top-level MAIN chunk in the .vox file.
func parseMainChunk(vr *voxReader) (*Main, error) {
	id, contents, childContents, err := parseChunk(vr)
	if err != nil {
		return nil, err
	}
	if id != "MAIN" {
		return nil, fmt.Errorf("missing MAIN chunk")
	}
	if len(contents) != 0 {
		return nil, fmt.Errorf("unexpected MAIN contents")
	}
	vr.RequireEOF("MAIN")
	return parseMainChunks(&voxReader{r: bytes.NewReader(childContents)})
}

// Parse reads and parses a magicavoxel .vox file.
func Parse(r io.Reader) (*Main, error) {
	vr := &voxReader{r: r}
	id := vr.ReadBytes(4)
	ver := vr.ReadInt32()

	if err := vr.Error(); err != nil {
		return nil, fmt.Errorf("failed reading header: %v", err)
	}

	if bytes.Compare(id, []byte("VOX ")) != 0 {
		return nil, fmt.Errorf("not a magicavox file")
	}
	if ver != version {
		return nil, fmt.Errorf("vox file must be version %d, got %d", version, ver)
	}
	return parseMainChunk(vr)
}

// Parse reads and parses the file with the given name as a magicavoxel .vox file.
func ParseFile(filename string) (*Main, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	br := bufio.NewReader(f)
	return Parse(br)
}
