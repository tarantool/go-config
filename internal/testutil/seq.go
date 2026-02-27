// Package testutil provides test helpers including iterator comparisons (iter.Seq and iter.Seq2)
// and channel draining utilities.
package testutil

import (
	"iter"
	"slices"

	"github.com/stretchr/testify/assert"
)

// TestIterSeqCompare compares the output of an iter.Seq with an expected slice.
func TestIterSeqCompare[V any](t TB, expected []V, it iter.Seq[V]) {
	t.Helper()

	assert.Equal(t, expected, slices.Collect(it))
}

// TestIterSeqEmpty asserts that an iter.Seq produces no elements.
func TestIterSeqEmpty[V any](t TB, it iter.Seq[V]) {
	t.Helper()

	assert.Empty(t, slices.Collect(it))
}

// TestIterSeqLen asserts that an iter.Seq produces exactly the expected number of elements.
func TestIterSeqLen[V any](t TB, expected int, it iter.Seq[V]) {
	t.Helper()

	assert.Len(t, slices.Collect(it), expected)
}
