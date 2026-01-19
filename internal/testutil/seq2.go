package testutil

import (
	"iter"
	"slices"

	"github.com/shoenig/test"
)

// TestIterSeq2Pair holds a key-value pair from an iter.Seq2 iterator.
// Used to convert iter.Seq2 outputs into slices for comparison.
type TestIterSeq2Pair[K any, V any] struct {
	Key   K
	Value V
}

// testSeq2ToSeq converts an iter.Seq2[K, V] into an iter.Seq[TestIterSeq2Pair[K, V]].
// This internal helper allows collecting key-value pairs into a slice for comparisons.
func testSeq2ToSeq[K any, V any](it iter.Seq2[K, V]) iter.Seq[TestIterSeq2Pair[K, V]] {
	return func(yield func(TestIterSeq2Pair[K, V]) bool) {
		for key, val := range it {
			if !yield(TestIterSeq2Pair[K, V]{Key: key, Value: val}) {
				return
			}
		}
	}
}

// testSeq2ToSlice collects an iter.Seq2[K, V] into a slice of TestIterSeq2Pair[K, V].
// This internal helper is used by the TestIterSeq2* functions.
func testSeq2ToSlice[K any, V any](it iter.Seq2[K, V]) []TestIterSeq2Pair[K, V] {
	return slices.Collect(testSeq2ToSeq(it))
}

// TestIterSeq2Compare compares the output of an iter.Seq2 with an expected slice of key-value pairs.
// It converts the iterator into a slice of TestIterSeq2Pair and uses test.Eq to assert equality.
// Order does matter.
// Useful for testing functions that return iter.Seq2.
func TestIterSeq2Compare[K any, V any](t test.T, expected []TestIterSeq2Pair[K, V], it iter.Seq2[K, V]) {
	t.Helper()

	test.Eq(t, expected, testSeq2ToSlice(it))
}

// TestIterSeq2UnorderedCompare compares the output of an iter.Seq2 with an expected slice of key-value pairs.
// It converts the iterator into a slice of TestIterSeq2Pair and uses test.Eq to assert equality.
// Order doesn't matter.
// Useful for testing functions that return iter.Seq2.
func TestIterSeq2UnorderedCompare[K any, V any](t test.T, expected []TestIterSeq2Pair[K, V], it iter.Seq2[K, V]) {
	t.Helper()

	test.SliceContainsAll(t, expected, testSeq2ToSlice(it))
}

// TestIterSeq2Empty asserts that an iter.Seq2 produces no elements.
// It converts the iterator into a slice of pairs and uses test.SliceEmpty to verify emptiness.
func TestIterSeq2Empty[K any, V any](t test.T, it iter.Seq2[K, V]) {
	t.Helper()

	test.SliceEmpty(t, testSeq2ToSlice(it))
}

// TestIterSeq2Len asserts that an iter.Seq2 produces exactly the expected number of key-value pairs.
// It converts the iterator into a slice of pairs and uses test.SliceLen to verify the length.
func TestIterSeq2Len[K any, V any](t test.T, expected int, it iter.Seq2[K, V]) {
	t.Helper()

	test.SliceLen(t, expected, testSeq2ToSlice(it))
}
