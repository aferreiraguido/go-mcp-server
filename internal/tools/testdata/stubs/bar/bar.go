// Package bar is a stub external dependency for the samplemodule fixture.
// It lets go/packages type-check the fixture offline (replace directive).
package bar

// Baz returns a fixed string.
func Baz() string {
	return "baz"
}
