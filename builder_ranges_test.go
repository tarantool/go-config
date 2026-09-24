package config_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/v2"
	"github.com/tarantool/go-config/v2/collectors"
	"github.com/tarantool/go-config/v2/tree"
	"github.com/tarantool/go-config/v2/validator"
	"github.com/tarantool/go-config/v2/validators/jsonschema"
)

type yamlString string

func (s yamlString) Name() string                  { return "yaml" }
func (s yamlString) SourceType() config.SourceType { return config.FileSource }
func (s yamlString) Revision() config.RevisionType { return "" }
func (s yamlString) FetchStream(context.Context) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(string(s))), nil
}

type capturingValidator struct {
	root *tree.Node
}

func (v *capturingValidator) Validate(root *tree.Node) []validator.ValidationError {
	v.root = root

	return nil
}

func (v *capturingValidator) SchemaType() string { return "capture" }

func TestBuilder_YAMLRanges(t *testing.T) {
	t.Parallel()

	source, err := collectors.NewSource(t.Context(), yamlString(`svc:
  name: localhost
  backlog: 8080
  tags: [a, b]
`), collectors.NewYamlFormat())
	require.NoError(t, err)

	override := collectors.NewMap(map[string]any{"svc": map[string]any{"backlog": 9090}}).WithName("override")
	capture := &capturingValidator{root: nil}

	builder := config.NewBuilder()

	builder = builder.AddCollector(source)
	builder = builder.AddCollector(override)
	builder = builder.WithValidator(capture)

	_, errs := builder.Build(t.Context())
	require.Empty(t, errs)
	require.NotNil(t, capture.root)

	tests := []struct {
		path string
		want tree.Range
	}{
		{"svc", tree.NewRange(2, 3, 4, 15)},
		{"svc/name", tree.NewRange(2, 9, 2, 18)},
		{"svc/tags", tree.NewRange(4, 9, 4, 15)},
		{"svc/tags/1", tree.NewRange(4, 13, 4, 14)},
		// Overridden by a collector without ranges.
		{"svc/backlog", tree.NewZeroRange()},
	}

	for _, tt := range tests {
		node := capture.root.Get(config.NewKeyPath(tt.path))
		require.NotNil(t, node, tt.path)
		assert.Equal(t, tt.want, node.Range, tt.path)
	}
}

func TestBuilder_YAMLRangesAcrossSources(t *testing.T) {
	t.Parallel()

	base, err := collectors.NewSource(t.Context(), yamlString("app:\n  name: a\n"), collectors.NewYamlFormat())
	require.NoError(t, err)

	top, err := collectors.NewSource(t.Context(), yamlString("# top\napp:\n  backlog: 1\n"), collectors.NewYamlFormat())
	require.NoError(t, err)

	capture := &capturingValidator{root: nil}

	builder := config.NewBuilder()

	builder = builder.AddCollector(base)
	builder = builder.AddCollector(top)
	builder = builder.WithValidator(capture)

	_, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	// A map both sources define points at the one merged last.
	assert.Equal(t, tree.NewRange(2, 1, 3, 13), capture.root.Range)
	assert.Equal(t, tree.NewRange(3, 3, 3, 13), capture.root.Get(config.NewKeyPath("app")).Range)
	assert.Equal(t, tree.NewRange(2, 9, 2, 10), capture.root.Get(config.NewKeyPath("app/name")).Range)
}

func TestMergeCollector_ReplacedValueDropsRange(t *testing.T) {
	t.Parallel()

	source, err := collectors.NewSource(t.Context(), yamlString("quota: 80\nname: a\n"), collectors.NewYamlFormat())
	require.NoError(t, err)

	root := tree.New()
	require.NoError(t, config.MergeCollector(t.Context(), root, source))
	require.NoError(t, config.MergeCollector(t.Context(), root, collectors.NewMap(map[string]any{"quota": 9090})))

	assert.Equal(t, tree.NewZeroRange(), root.Get(config.NewKeyPath("quota")).Range)
	assert.Equal(t, tree.NewRange(2, 7, 2, 8), root.Get(config.NewKeyPath("name")).Range)
}

func TestBuilder_ValidationErrorRange(t *testing.T) {
	t.Parallel()

	schema, err := jsonschema.New([]byte(`{
		"type": "object",
		"properties": {
			"app": {
				"type": "object",
				"properties": {"listen": {"type": "integer", "minimum": 1024}}
			}
		}
	}`))
	require.NoError(t, err)

	source, err := collectors.NewSource(t.Context(), yamlString("app:\n  listen: 80\n"), collectors.NewYamlFormat())
	require.NoError(t, err)

	builder := config.NewBuilder()

	builder = builder.AddCollector(source)
	builder = builder.WithValidator(schema)

	_, errs := builder.Build(t.Context())
	require.Len(t, errs, 1)

	var validationErr *validator.ValidationError

	require.ErrorAs(t, errs[0], &validationErr)
	assert.Equal(t, "app/listen", validationErr.Path.String())
	assert.Equal(t, validator.RangeFromTree(tree.NewRange(2, 11, 2, 13)), validationErr.Range)
}
