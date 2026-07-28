// Command samplemodule is a tiny fixture program for go-mcp-server tests.
package main

import (
	"fmt"
	"io"

	"github.com/foo/bar"
)

// Version is the fixture version.
const Version = "1.0"

// Modes supported by the fixture.
const (
	ModeQuiet = iota
	ModeNormal
	ModeVerbose
)

// DefaultName is used when no name is provided.
var DefaultName = "world"

var counter int

func init() {
	counter = 0
}

func main() {
	g := NewGreeter(DefaultName, io.Discard)
	fmt.Println(g.Greet())
	bar.Baz()
}

// helper returns its argument unchanged. It is never called from tests.
func helper(s string) string {
	return s
}
