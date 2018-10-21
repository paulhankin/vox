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

	"github.com/paulhankin/vox"
)

func quitf(f string, args ...interface{}) {
	log.Fatalf(f, args...)
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
	fmt.Printf("%+v\n\n", main.Scene)
	fmt.Printf("%+v\n\n", main.Models)
	for i, m := range main.Materials {
		fmt.Printf("%3d: %s\n", i, m)
	}
}
