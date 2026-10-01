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

func TestYaml_Parse_TarantoolParserFormatting(t *testing.T) {
	t.Parallel()

	const (
		yesValue   = "yes"
		trueValue  = "true"
		falseValue = "false"
		titleYes   = "Yes"
		upperYes   = "YES"
		mixedYes   = "yEs"
		yesterday  = "yesterday"
		nobody     = "nobody"
	)

	tests := []struct {
		input          string
		yamlValue      any
		tarantoolValue any
	}{
		{"0", int64(0), int64(0)},
		{"00", int64(0), int64(0)},
		{"020", int64(16), int64(20)},
		{"00020", int64(16), int64(20)},
		{"+020", int64(16), int64(20)},
		{"-020", int64(-16), int64(-20)},
		{"08", float64(8), int64(8)},
		{"09", float64(9), int64(9)},
		{"+09", float64(9), int64(9)},
		{"-09", float64(-9), int64(-9)},
		{"09223372036854775807", float64(9223372036854775807), int64(9223372036854775807)},
		{"-09223372036854775808", float64(-9223372036854775808), int64(-9223372036854775808)},
		{"018446744073709551615", float64(18446744073709551615), uint64(18446744073709551615)},
		{"020.5", float64(20.5), float64(20.5)},
		{"020e2", float64(2000), float64(2000)},
		{"0x20", int64(32), int64(32)},
		{"-0x20", int64(-32), int64(-32)},
		{"0o20", int64(16), int64(16)},
		{"+0o20", int64(16), int64(16)},
		{"-0o20", int64(-16), int64(-16)},
		{"0b10", int64(2), int64(2)},
		{"-0b10", int64(-2), int64(-2)},
		{"\"020\"", "020", "020"},
		{"'020'", "020", "020"},
		{"!!str 020", "020", "020"},
		{"!!int 020", int64(16), int64(20)},
		{"!!float 020", float64(20), float64(20)},
		{"0_20", int64(16), "0_20"},
		{"0_9", float64(9), "0_9"},
		{"1_000", int64(1000), "1_000"},
		{"-1_000", int64(-1000), "-1_000"},
		{"1__0", int64(10), "1__0"},
		{"1_000.5", float64(1000.5), "1_000.5"},
		{"1__0.5", float64(10.5), "1__0.5"},
		{"1.5e1_0", float64(1.5e10), "1.5e1_0"},
		{"0x2_0", int64(32), "0x2_0"},
		{"0o2_0", int64(16), "0o2_0"},
		{"0b1_0", int64(2), "0b1_0"},
		{"!!int 1_0", int64(10), "1_0"},
		{"!!float 1_0", float64(10), "1_0"},
		{yesValue, yesValue, true},
		{titleYes, titleYes, titleYes},
		{upperYes, upperYes, upperYes},
		{"no", "no", false},
		{"No", "No", "No"},
		{"NO", "NO", "NO"},
		{mixedYes, mixedYes, mixedYes},
		{"nO", "nO", "nO"},
		{trueValue, true, true},
		{falseValue, false, false},
		{"\"true\"", trueValue, trueValue},
		{"'true'", trueValue, trueValue},
		{"\"false\"", falseValue, falseValue},
		{"'false'", falseValue, falseValue},
		{"!!str true", trueValue, trueValue},
		{"!!str false", falseValue, falseValue},
		{"True", true, "True"},
		{"TRUE", true, "TRUE"},
		{"False", false, "False"},
		{"FALSE", false, "FALSE"},
		{"\"yes\"", yesValue, yesValue},
		{"\"no\"", "no", "no"},
		{"'yes'", yesValue, yesValue},
		{"'no'", "no", "no"},
		{"!!str yes", yesValue, yesValue},
		{"!!str no", "no", "no"},
		{"!!bool yes", yesValue, true},
		{"!!bool no", "no", false},
		{"!!bool \"yes\"", yesValue, true},
		{"!!bool 'no'", "no", false},
		{"!custom yes", yesValue, yesValue},
		{"!custom no", "no", "no"},
		{"|-\n  yes", yesValue, yesValue},
		{">-\n  no", "no", "no"},
		{yesterday, yesterday, yesterday},
		{nobody, nobody, nobody},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			data := "number: " + tt.input + "\n"
			root, err := collectors.NewYamlFormat().From(strings.NewReader(data)).Parse()
			require.NoError(t, err)

			node := root.Get(config.NewKeyPath("number"))
			require.NotNil(t, node)
			assert.Equal(t, tt.yamlValue, node.Value, "default YAML format")

			root, err = collectors.NewYamlFormat(collectors.WithTarantoolParserFormatting()).
				From(strings.NewReader(data)).Parse()
			require.NoError(t, err)

			node = root.Get(config.NewKeyPath("number"))
			require.NotNil(t, node)
			assert.Equal(t, tt.tarantoolValue, node.Value, "Tarantool format")
		})
	}
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

func TestYaml_Parse_TypeFixed(t *testing.T) {
	t.Parallel()

	data := []byte(`plain_bool: false
plain_str: 5s
plain_yes: yes
double: "false"
single: 'false'
literal: |
  false
folded: >
  false
str_tag: !!str false
bool_tag: !!bool "true"
anchor: &q "3301"
alias: *q
list: [1, "2"]
`)

	root, err := collectors.NewYamlFormat().From(bytes.NewReader(data)).Parse()
	require.NoError(t, err)

	tests := []struct {
		path  string
		fixed bool
	}{
		{"plain_bool", false},
		{"plain_str", false},
		{"plain_yes", false},
		{"double", true},
		{"single", true},
		{"literal", true},
		{"folded", true},
		{"str_tag", true},
		{"bool_tag", true},
		{"anchor", true},
		{"alias", true},
		{"list/0", false},
		{"list/1", true},
	}

	for _, tt := range tests {
		node := root.Get(config.NewKeyPath(tt.path))
		require.NotNil(t, node, tt.path)
		assert.Equal(t, tt.fixed, node.TypeFixed(), tt.path)
	}

	assert.Equal(t, "false", root.Get(config.NewKeyPath("double")).Value)
	assert.Equal(t, true, root.Get(config.NewKeyPath("bool_tag")).Value)
}

func TestYaml_Parse_EmptyAsString(t *testing.T) {
	t.Parallel()

	data := []byte(`empty:
tilde: ~
null_word: null
null_tag: !!null
quoted: ""
list:
- 
- x
flow: {b: }
set:
  ? b
`)

	tests := []struct {
		path          string
		yamlSpec      any
		emptyAsString any
	}{
		{"empty", nil, ""},
		{"tilde", nil, nil},
		{"null_word", nil, nil},
		{"null_tag", nil, ""},
		{"quoted", "", ""},
		{"list/0", nil, ""},
		{"list/1", "x", "x"},
		{"flow/b", nil, ""},
		{"set/b", nil, ""},
	}

	spec, err := collectors.NewYamlFormat().From(bytes.NewReader(data)).Parse()
	require.NoError(t, err)

	standalone, err := collectors.NewYamlFormat(collectors.EmptyAsString()).From(bytes.NewReader(data)).Parse()
	require.NoError(t, err)

	tarantool, err := collectors.NewYamlFormat(collectors.WithTarantoolParserFormatting()).
		From(bytes.NewReader(data)).Parse()
	require.NoError(t, err)

	for _, tt := range tests {
		node := spec.Get(config.NewKeyPath(tt.path))
		require.NotNil(t, node, tt.path)
		assert.Equal(t, tt.yamlSpec, node.Value, "default: %s", tt.path)

		node = standalone.Get(config.NewKeyPath(tt.path))
		require.NotNil(t, node, tt.path)
		assert.Equal(t, tt.emptyAsString, node.Value, "EmptyAsString: %s", tt.path)

		node = tarantool.Get(config.NewKeyPath(tt.path))
		require.NotNil(t, node, tt.path)
		assert.Equal(t, tt.emptyAsString, node.Value, "Tarantool format: %s", tt.path)
	}
}

func TestYaml_Parse_EmptyAsString_EmptyDocument(t *testing.T) {
	t.Parallel()

	root, err := collectors.NewYamlFormat(collectors.EmptyAsString()).
		From(strings.NewReader("---\n")).Parse()
	require.NoError(t, err)
	assert.IsType(t, "", root.Value, "the empty document is a string, not null")
	assert.Empty(t, root.Value)
}

func TestYaml_Parse_TarantoolParserFormatting_EmptyDocument(t *testing.T) {
	t.Parallel()

	root, err := collectors.NewYamlFormat(collectors.WithTarantoolParserFormatting()).
		From(strings.NewReader("---\n")).Parse()
	require.NoError(t, err)
	assert.IsType(t, "", root.Value, "the empty document is a string, not null")
	assert.Empty(t, root.Value)
}
