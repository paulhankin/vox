/* Binary voxtext prints the contents of a magicavox .vox file to stdout.

It's not intended to be useful; simply as a quick test of the parsing code.

Usage:

voxtext myfile.vox
*/
package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/paulhankin/vox"
)

func quitf(f string, args ...interface{}) {
	log.Fatalf(f, args...)
}

func printScene(node vox.AnyNode, depth int) error {
	fmt.Printf("%s%s\n", strings.Repeat("  ", depth), node)
	var children []vox.AnyNode
	switch t := node.(type) {
	case *vox.TransformNode:
		children = append(children, t.Child)
	case *vox.GroupNode:
		children = append(children, t.Children...)
	case *vox.ShapeNode:
	default:
		return fmt.Errorf("found unexpected node of type %T\n", node)
	}
	for _, c := range children {
		if err := printScene(c, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		quitf("Expected input filename, got %v", args)
	}
	main, err := vox.ParseFile(args[0])
	if err != nil {
		quitf("Error parsing file: %s", err)
	}
	fmt.Printf("scene:\n")
	if err := printScene(main.Scene.Node, 4); err != nil {
		quitf("Error found in scene: %s", err)
	}
	fmt.Printf("\nlayers: %#v\n\n", main.Scene.Layers)
	fmt.Printf("models: %+v\n\n", main.Models)
	for i, m := range main.Materials {
		fmt.Printf("%3d: %s\n", i, m)
	}
}
