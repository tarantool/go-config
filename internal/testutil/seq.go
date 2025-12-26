// Package testutil provides test helpers for working with Go iterators (iter.Seq and iter.Seq2).
// These helpers simplify comparing iterator outputs with expected slices, checking emptiness,
// and verifying lengths in tests.
package testutil

import (
	"iter"
	"slices"
	"testing"

	"github.com/shoenig/test"
)

// TestIterSeqCompare compares the output of an iter.Seq with an expected slice.
// It collects the iterator into a slice and uses test.Eq to assert equality.
// This is useful for testing functions that return iter.Seq.
func TestIterSeqCompare[V any](t *testing.T, expected []V, it iter.Seq[V]) {
	t.Helper()

	test.Eq(t, expected, slices.Collect(it))
}

// TestIterSeqEmpty asserts that an iter.Seq produces no elements.
// It collects the iterator and uses test.SliceEmpty to verify the slice is empty.
func TestIterSeqEmpty[V any](t *testing.T, it iter.Seq[V]) {
	t.Helper()

	test.SliceEmpty(t, slices.Collect(it))
}

// TestIterSeqLen asserts that an iter.Seq produces exactly the expected number of elements.
// It collects the iterator and uses test.SliceLen to verify the length.
func TestIterSeqLen[V any](t *testing.T, expected int, it iter.Seq[V]) {
	t.Helper()

	test.SliceLen(t, expected, slices.Collect(it))
}
