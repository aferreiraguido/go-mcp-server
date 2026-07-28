package analysis

import "testing"

// hasWholeWord is the key anti-false-positive piece
// (lesson from rust iter 4: `add` inside `address` must NOT match).
func TestHasWholeWord(t *testing.T) {
	cases := []struct {
		line, word   string
		requireParen bool
		want         bool
	}{
		{"add(1, 2)", "add", false, true},
		{"address(1, 2)", "add", false, false},     // boundary: substring no casa
		{"x := padded + 1", "added", false, false}, // not present
		{"x := _add_(y)", "add", false, false},     // underscore IS ident-char: no match
		{"foo(bar)", "foo", true, true},
		{"foo(bar)", "foo", false, true},
		{"x := foo", "foo", true, false},   // no parens → not a call
		{"fooBar(x)", "foo", false, false}, // camelCase does not match
		{"Bar(foo)", "foo", true, false},   // foo as argument is NOT a call
		{"Bar(foo)", "foo", false, true},   // but IS a whole-word reference
		{"Bar(foo)", "Bar", true, true},    // Bar IS the call
		{"", "add", false, false},
		{"add(1)", "", false, false},
	}
	for _, c := range cases {
		if got := hasWholeWord(c.line, c.word, c.requireParen); got != c.want {
			t.Errorf("hasWholeWord(%q, %q, %v) = %v, want %v",
				c.line, c.word, c.requireParen, got, c.want)
		}
	}
}
