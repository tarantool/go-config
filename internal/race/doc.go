// Package race exposes whether the binary was built with the race detector
// enabled (-race). Callers use it to skip CPU-bound work under race
// instrumentation where race coverage adds nothing — e.g. one-shot decode
// sweeps that are slow enough to blow per-test timeouts when the race
// runtime amplifies allocator and recursive-call costs.
package race
