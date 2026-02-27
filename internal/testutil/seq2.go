package testutil

import (
	"iter"
	"slices"

	"github.com/stretchr/testify/assert"
)

// TestIterSeq2Pair holds a key-value pair from an iter.Seq2 iterator.
type TestIterSeq2Pair[K any, V any] struct {
	Key   K
	Value V
}

func testSeq2ToSeq[K any, V any](it iter.Seq2[K, V]) iter.Seq[TestIterSeq2Pair[K, V]] {
	return func(yield func(TestIterSeq2Pair[K, V]) bool) {
		for key, val := range it {
			if !yield(TestIterSeq2Pair[K, V]{Key: key, Value: val}) {
				return
			}
		}
	}
}

func testSeq2ToSlice[K any, V any](it iter.Seq2[K, V]) []TestIterSeq2Pair[K, V] {
	return slices.Collect(testSeq2ToSeq(it))
}

// TestIterSeq2Compare compares the output of an iter.Seq2 with an expected slice of key-value pairs.
func TestIterSeq2Compare[K any, V any](t TB, expected []TestIterSeq2Pair[K, V], it iter.Seq2[K, V]) {
	t.Helper()

	assert.Equal(t, expected, testSeq2ToSlice(it))
}

// TestIterSeq2UnorderedCompare compares the output of an iter.Seq2 with an expected slice of key-value pairs unordered.
func TestIterSeq2UnorderedCompare[K any, V any](t TB, expected []TestIterSeq2Pair[K, V], it iter.Seq2[K, V]) {
	t.Helper()

	assert.ElementsMatch(t, expected, testSeq2ToSlice(it))
}

// TestIterSeq2Empty asserts that an iter.Seq2 produces no elements.
func TestIterSeq2Empty[K any, V any](t TB, it iter.Seq2[K, V]) {
	t.Helper()

	assert.Empty(t, testSeq2ToSlice(it))
}

// TestIterSeq2Len asserts that an iter.Seq2 produces exactly the expected number of key-value pairs.
func TestIterSeq2Len[K any, V any](t TB, expected int, it iter.Seq2[K, V]) {
	t.Helper()

	assert.Len(t, testSeq2ToSlice(it), expected)
}
