package collectors_test

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/v2"
	"github.com/tarantool/go-config/v2/collectors"
	"github.com/tarantool/go-config/v2/tree"
)

//go:embed testdata/config.yaml
var configYaml string

func TestNewYamlFormat(t *testing.T) {
	t.Parallel()

	format := collectors.NewYamlFormat()
	require.NotNil(t, format)

	assert.Equal(t, "yaml", format.Name())
	assert.True(t, format.KeepOrder())
}

func TestYaml_From(t *testing.T) {
	t.Parallel()

	reader := strings.NewReader(configYaml)
	require.NotNil(t, reader)

	format := collectors.NewYamlFormat().From(reader)
	require.NotNil(t, format)
}

func TestYaml_Parse(t *testing.T) {
	t.Parallel()

	reader := strings.NewReader(configYaml)

	format := collectors.NewYamlFormat().From(reader)
	require.NotNil(t, format)

	root, err := format.Parse()
	require.NotNil(t, root)
	require.NoError(t, err)

	node := root.Get(config.NewKeyPath("storage/provider"))
	require.NotNil(t, node)

	val, ok := node.Value.(string)
	require.True(t, ok)
	assert.Equal(t, "etcd", val)

	node = root.Get(config.NewKeyPath("initial-settings/clusters/0/name"))
	require.NotNil(t, node)

	val, ok = node.Value.(string)
	require.True(t, ok)
	assert.Equal(t, "default-cluster", val)
}

func TestYaml_Parse_EmptyMapping(t *testing.T) {
	t.Parallel()

	data := []byte("groups:\n  storages:\n    replicasets:\n      r-001:\n        instances:\n          inst1: {}")

	format := collectors.NewYamlFormat().From(bytes.NewReader(data))
	require.NotNil(t, format)

	root, err := format.Parse()
	require.NoError(t, err)
	require.NotNil(t, root)

	node := root.Get(config.NewKeyPath("groups/storages/replicasets/r-001/instances/inst1"))
	require.NotNil(t, node)

	val, ok := node.Value.(map[string]any)
	require.True(t, ok)
	assert.Empty(t, val)
}

func TestYaml_Parse_Ranges(t *testing.T) {
	t.Parallel()

	data := []byte(`app:
  name: "demo"
  ports: [80, 443]
  tags:
    - a
    - b
  empty: {}
café: crème
base: &b
  x: 1
copy: *b
tagged: !!str
none:
list: [*b, x]
scalar: &s v
sref: *s
empty: &e {}
eref: *e
`)

	root, err := collectors.NewYamlFormat().From(bytes.NewReader(data)).Parse()
	require.NoError(t, err)

	tests := []struct {
		path string
		want tree.Range
	}{
		{"", tree.NewRange(1, 1, 18, 9)},
		{"app", tree.NewRange(2, 3, 7, 12)},
		{"app/name", tree.NewRange(2, 9, 2, 15)},
		{"app/ports", tree.NewRange(3, 10, 3, 19)},
		{"app/ports/1", tree.NewRange(3, 15, 3, 18)},
		{"app/tags", tree.NewRange(5, 5, 6, 8)},
		{"app/tags/0", tree.NewRange(5, 7, 5, 8)},
		{"app/empty", tree.NewRange(7, 10, 7, 12)},
		{"café", tree.NewRange(8, 7, 8, 12)},
		{"base", tree.NewRange(9, 7, 10, 7)},
		{"copy", tree.NewRange(11, 7, 11, 9)},
		{"copy/x", tree.NewRange(10, 6, 10, 7)},
		{"tagged", tree.NewRange(12, 9, 12, 14)},
		{"none", tree.NewRange(13, 6, 13, 6)},
		{"list/0", tree.NewRange(14, 8, 14, 10)},
		{"list/0/x", tree.NewRange(10, 6, 10, 7)},
		{"sref", tree.NewRange(16, 7, 16, 9)},
		{"eref", tree.NewRange(18, 7, 18, 9)},
	}

	for _, tt := range tests {
		node := root.Get(config.NewKeyPath(tt.path))
		require.NotNil(t, node, tt.path)
		assert.Equal(t, tt.want, node.Range, tt.path)
	}
}

func TestYaml_Parse_AliasCycle(t *testing.T) {
	t.Parallel()

	_, err := collectors.NewYamlFormat().From(strings.NewReader("a: &x [1, *x]\n")).Parse()
	require.ErrorIs(t, err, collectors.ErrUnmarshall)
	require.ErrorIs(t, err, collectors.ErrYamlAliasCycle)
}

func TestYaml_Parse_AliasBomb(t *testing.T) {
	t.Parallel()

	var data strings.Builder

	data.WriteString("l0: &l0 [" + strings.Repeat("x, ", 8) + "x]\n")

	for level := 1; level <= 6; level++ {
		ref := fmt.Sprintf("*l%d", level-1)
		fmt.Fprintf(&data, "l%d: &l%d [%s]\n", level, level, strings.Repeat(ref+", ", 8)+ref)
	}

	_, err := collectors.NewYamlFormat().From(strings.NewReader(data.String())).Parse()
	require.ErrorIs(t, err, collectors.ErrUnmarshall)
	require.ErrorIs(t, err, collectors.ErrYamlExcessiveAliasing)
}

func BenchmarkYamlFormat_Parse(b *testing.B) {
	var block, flow strings.Builder

	for i := range 2000 {
		fmt.Fprintf(&block, "section%d:\n  name: \"item %d\"\n  tags: [a, b, c]\n  script: |\n    echo %d\n", i, i, i)
	}

	flow.WriteString("[")

	for i := range 4000 {
		fmt.Fprintf(&flow, "{\"k%d\": \"v%d\"}, ", i, i)
	}

	flow.WriteString("{}]\n")

	for name, data := range map[string]string{
		"config":    configYaml,
		"block":     block.String(),
		"flow_line": flow.String(),
	} {
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()

			for b.Loop() {
				_, err := collectors.NewYamlFormat().From(strings.NewReader(data)).Parse()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestYaml_Parse_Invalid(t *testing.T) {
	t.Parallel()

	format := collectors.NewYamlFormat().From(nil)
	require.NotNil(t, format)

	root, err := format.Parse()
	require.Nil(t, root)
	require.Error(t, err)
	assert.Equal(t, err, collectors.ErrNoData)

	format = collectors.NewYamlFormat().From(nil)
	require.NotNil(t, format)

	root, err = format.Parse()
	require.Nil(t, root)
	require.Error(t, err)
	assert.Equal(t, err, collectors.ErrNoData)

	data := []byte("special: character: value")

	format = collectors.NewYamlFormat().From(bytes.NewReader(data))
	require.NotNil(t, format)

	root, err = format.Parse()
	require.Nil(t, root)
	require.Error(t, err)
}
