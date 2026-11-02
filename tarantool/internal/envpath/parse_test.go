package envpath_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tarantool/go-config/v2/tarantool/internal/envpath"
)

const (
	word     = "abc"
	jsonWord = `["a"]`
	endpoint = "http://a:2379"
)

func newType(kind envpath.Kind, items, values *envpath.Type, variants []*envpath.Type, enum []any) *envpath.Type {
	return &envpath.Type{Kind: kind, Items: items, Values: values, Variants: variants, Enum: enum}
}

func scalar(kind envpath.Kind) *envpath.Type {
	return newType(kind, nil, nil, nil, nil)
}

func arrayOf(items *envpath.Type) *envpath.Type {
	return newType(envpath.KindArray, items, nil, nil, nil)
}

func mapOf(values *envpath.Type) *envpath.Type {
	return newType(envpath.KindMap, nil, values, nil, nil)
}

func unionOf(variants ...*envpath.Type) *envpath.Type {
	return newType(envpath.KindUnion, nil, nil, variants, nil)
}

type parseCase struct {
	raw  string
	want any
	ok   bool
}

func checkParse(t *testing.T, typ *envpath.Type, cases []parseCase) {
	t.Helper()

	for _, tc := range cases {
		got, ok := typ.Parse(tc.raw)
		if assert.Equal(t, tc.ok, ok, "Parse(%q) ok", tc.raw) && ok {
			assert.Equal(t, tc.want, got, "Parse(%q)", tc.raw)
		}
	}
}

func TestParse_Empty(t *testing.T) {
	t.Parallel()

	for _, typ := range []*envpath.Type{
		scalar(envpath.KindString), scalar(envpath.KindInteger), scalar(envpath.KindAny),
		scalar(envpath.KindRecord), arrayOf(scalar(envpath.KindString)),
	} {
		got, ok := typ.Parse("")
		assert.True(t, ok, "an empty value is unset, not invalid")
		assert.Nil(t, got)
	}
}

func TestParse_String(t *testing.T) {
	t.Parallel()

	checkParse(t, scalar(envpath.KindString), []parseCase{
		{word, word, true},
		{"123", "123", true},
		{" true ", " true ", true},
		{jsonWord, jsonWord, true},
	})
}

func TestParse_Number(t *testing.T) {
	t.Parallel()

	checkParse(t, scalar(envpath.KindNumber), []parseCase{
		{"5", int64(5), true},
		{" 5 ", int64(5), true},
		{"-5", int64(-5), true},
		{"+5", int64(5), true},
		{"010", int64(10), true},
		{"0x10", int64(16), true},
		{"1.5", 1.5, true},
		{".5", 0.5, true},
		{"1e3", 1000.0, true},
		{"18446744073709551615", uint64(math.MaxUint64), true},
		{"0b11", nil, false},
		{"1_000", nil, false},
		{"inf", nil, false},
		{"NaN", nil, false},
		{"5s", nil, false},
		{word, nil, false},
		{"1e400", nil, false},
	})
}

func TestParse_Integer(t *testing.T) {
	t.Parallel()

	checkParse(t, scalar(envpath.KindInteger), []parseCase{
		{"42", int64(42), true},
		{" 42\t", int64(42), true},
		{"-42", int64(-42), true},
		{"010", int64(10), true},
		{"0x1F", int64(31), true},
		{"0b101", int64(5), true},
		{"-9223372036854775808", int64(math.MinInt64), true},
		{"9223372036854775808", uint64(1 << 63), true},
		{"-9223372036854775809", nil, false},
		{"1.5", nil, false},
		{"1e3", nil, false},
		{"1_000", nil, false},
		{"0x", nil, false},
		{"-", nil, false},
		{word, nil, false},
	})
}

func TestParse_Boolean(t *testing.T) {
	t.Parallel()

	checkParse(t, scalar(envpath.KindBoolean), []parseCase{
		{"true", true, true},
		{"TRUE", true, true},
		{"True", true, true},
		{"1", true, true},
		{"false", false, true},
		{"FaLsE", false, true},
		{"0", false, true},
		{"yes", nil, false},
		{"t", nil, false},
		{"2", nil, false},
		{" true", nil, false},
	})
}

func TestParse_Any(t *testing.T) {
	t.Parallel()

	checkParse(t, scalar(envpath.KindAny), []parseCase{
		{`{"a": [1, 2.5, "x", true, null]}`, map[string]any{"a": []any{int64(1), 2.5, "x", true, nil}}, true},
		{`"x"`, "x", true},
		{"5", int64(5), true},
		{"null", nil, true},
		{word, nil, false},
		{`{"a": 1} trailing`, nil, false},
		{`[1e400]`, nil, false},
	})
}

func TestParse_Array(t *testing.T) {
	t.Parallel()

	checkParse(t, arrayOf(scalar(envpath.KindString)), []parseCase{
		{endpoint, []any{endpoint}, true},
		{endpoint + ",http://b:2379", []any{endpoint, "http://b:2379"}, true},
		{`["` + endpoint + `"]`, []any{endpoint}, true},
		{"a,b,", []any{"a", "b"}, true},
		{",", []any{}, true},
		{"[]", []any{}, true},
		{"a,,b", nil, false},
		{",a", nil, false},
		{`{"a": 1}`, nil, false},
		{"[broken", nil, false},
	})

	checkParse(t, arrayOf(scalar(envpath.KindInteger)), []parseCase{
		{"1,2,3", []any{int64(1), int64(2), int64(3)}, true},
		{"[1, 2]", []any{int64(1), int64(2)}, true},
		{`["1"]`, []any{"1"}, true},
		{"1,x", nil, false},
	})

	records := arrayOf(scalar(envpath.KindRecord))
	checkParse(t, records, []parseCase{
		{`[{"uri": "127.0.0.1:3301"}]`, []any{map[string]any{"uri": "127.0.0.1:3301"}}, true},
		{"127.0.0.1:3301", nil, false},
	})

	checkParse(t, arrayOf(scalar(envpath.KindAny)), []parseCase{
		{"[1]", []any{int64(1)}, true},
		{"a,b", nil, false},
	})

	checkParse(t, arrayOf(unionOf(scalar(envpath.KindNumber), scalar(envpath.KindString))), []parseCase{
		{"1,x", []any{int64(1), "x"}, true},
	})
}

func TestParse_Map(t *testing.T) {
	t.Parallel()

	checkParse(t, mapOf(scalar(envpath.KindString)), []parseCase{
		{"a=b,c=d", map[string]any{"a": "b", "c": "d"}, true},
		{"a=b=c", map[string]any{"a": "b=c"}, true},
		{"a=1,a=2", map[string]any{"a": "2"}, true},
		{"a=,b=2", map[string]any{"b": "2"}, true},
		{"a=1,a=", map[string]any{}, true},
		{`{"a": "b"}`, map[string]any{"a": "b"}, true},
		{"a", nil, false},
		{"=b", nil, false},
		{"a=b,", nil, false},
		{jsonWord, nil, false},
	})

	checkParse(t, mapOf(scalar(envpath.KindBoolean)), []parseCase{
		{"a=true,b=0", map[string]any{"a": true, "b": false}, true},
		{"a=yes", nil, false},
	})

	checkParse(t, mapOf(scalar(envpath.KindAny)), []parseCase{
		{`{"role": {"a": 1}}`, map[string]any{"role": map[string]any{"a": int64(1)}}, true},
		{"a=b", nil, false},
	})
}

func TestParse_Record(t *testing.T) {
	t.Parallel()

	checkParse(t, scalar(envpath.KindRecord), []parseCase{
		{`{"uri": "x"}`, nil, false},
		{"x", nil, false},
	})
}

func TestParse_Union(t *testing.T) {
	t.Parallel()

	// A duration or byte_size field: the number wins when there is one.
	checkParse(t, unionOf(scalar(envpath.KindNumber), scalar(envpath.KindString)), []parseCase{
		{"5", int64(5), true},
		{"0.5", 0.5, true},
		{"30s", "30s", true},
	})

	// The order of the variants does not let a string keep a number.
	checkParse(t, unionOf(scalar(envpath.KindString), scalar(envpath.KindNumber)), []parseCase{
		{"5", int64(5), true},
		{"info", "info", true},
	})

	// config.context.*: string, number, boolean or a record.
	checkParse(t, unionOf(scalar(envpath.KindString), scalar(envpath.KindNumber),
		scalar(envpath.KindBoolean), scalar(envpath.KindRecord)), []parseCase{
		{"true", true, true},
		{"1", int64(1), true},
		{word, word, true},
	})

	// audit_log.spaces: an array or a map.
	checkParse(t, unionOf(arrayOf(scalar(envpath.KindString)), mapOf(scalar(envpath.KindRecord))), []parseCase{
		{"a,b", []any{"a", "b"}, true},
		{jsonWord, []any{"a"}, true},
		{`{"a": {"extract_key": true}}`, map[string]any{"a": map[string]any{"extract_key": true}}, true},
	})

	// A variant counts only with a value its enum allows.
	enumNumber := newType(envpath.KindNumber, nil, nil, nil, []any{1.0, 2.0})
	checkParse(t, unionOf(enumNumber, scalar(envpath.KindString)), []parseCase{
		{"1", int64(1), true},
		{"3", "3", true},
	})

	enumString := newType(envpath.KindString, nil, nil, nil, []any{"a"})
	checkParse(t, unionOf(enumString, scalar(envpath.KindBoolean)), []parseCase{
		{"a", "a", true},
		{"b", nil, false},
	})
}
