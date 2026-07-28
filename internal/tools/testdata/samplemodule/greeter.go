package main

import (
	"fmt"
	"io"
)

// Speaker is anything that can speak a message.
type Speaker interface {
	// Speak says msg and returns what was said.
	Speak(msg string) string
	fmt.Stringer // embedded
}

// Greeter greets people by name.
type Greeter struct {
	Name   string `json:"name"`
	output io.Writer
	io.Writer // embedded field
}

// NewGreeter builds a Greeter for name writing to w.
func NewGreeter(name string, w io.Writer) *Greeter {
	return &Greeter{Name: name, output: w}
}

// Greet returns the greeting message.
func (g *Greeter) Greet() string {
	return "hello " + g.Name
}

// Speak implements Speaker.
func (g *Greeter) Speak(msg string) string {
	return g.Greet() + ": " + msg
}

// String implements fmt.Stringer (so *Greeter satisfies Speaker).
func (g *Greeter) String() string {
	return g.Greet()
}

// reset clears the greeter state. Unexported.
func (g *Greeter) reset() {
	g.Name = ""
}
