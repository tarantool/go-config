package collectors

import (
	"bytes"
	"strconv"
	"testing"

	"go.yaml.in/yaml/v3"
)

// FuzzYamlRanges checks the ranges of anything yaml.v3 accepts the way
// TestYamlRanges_Suite checks yaml-test-suite. testdata/fuzz/FuzzYamlRanges
// keeps the inputs it has failed on.
func FuzzYamlRanges(f *testing.F) {
	for _, src := range yamlTestSuite(f) {
		f.Add(src)
	}

	f.Fuzz(func(t *testing.T, src []byte) {
		// yaml.v3 reports positions past a U+FEFF inside the stream that the
		// text does not have.
		if bytes.Contains(normalizeYamlSource(src), []byte("\xef\xbb\xbf")) {
			return
		}

		name := strconv.Quote(string(src))

		for _, doc := range decodeYamlStreamSafely(src) {
			ranges := newYamlRanges(src, doc)

			checkYamlRangeTree(t, name, ranges, doc)

			walkYaml(doc, func(node *yaml.Node) {
				checkYamlRangeValue(t, name, ranges, node)
			})
		}
	})
}

// decodeYamlStreamSafely is decodeYamlStream for input that makes yaml.v3
// itself panic, which is not what the fuzzing looks for.
func decodeYamlStreamSafely(src []byte) []*yaml.Node {
	var docs []*yaml.Node

	func() {
		defer func() { _ = recover() }()

		docs = decodeYamlStream(src)
	}()

	return docs
}
