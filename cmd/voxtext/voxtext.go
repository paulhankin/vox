package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/paulhankin/vox"
)

var ()

func quitf(f string, args ...interface{}) {
	log.Fatalf(f, args...)
}

func parseVox(filename string) (*vox.Main, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	br := bufio.NewReader(f)
	return vox.Parse(br)
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		quitf("Expected input filename, got %v", args)
	}
	main, err := parseVox(args[0])
	if err != nil {
		quitf("Error parsing file: %s", err)
	}
	fmt.Printf("%+v\n", main)
}
