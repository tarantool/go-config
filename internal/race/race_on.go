//go:build race

package race

// Enabled reports whether the binary was built with -race.
func Enabled() bool { return true }
