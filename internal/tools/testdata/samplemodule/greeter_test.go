package main

import "testing"

func TestGreet(t *testing.T) {
	g := NewGreeter("test", nil)
	if g.Greet() != "hello test" {
		t.Fatal("unexpected greeting")
	}
}
