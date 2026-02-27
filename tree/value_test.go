package tree_test

import (
	"fmt"
	"math"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/keypath"
	"github.com/tarantool/go-config/meta"
	"github.com/tarantool/go-config/tree"
)

var _ = math.MaxInt64

type testStringer struct{}

func (testStringer) String() string { return "I am a Stringer" }

func TestValue_Get_Int(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("int"), 42)

	node := root.Get(keypath.NewKeyPath("int"))
	require.NotNil(t, node)

	val := tree.NewValue(node, keypath.NewKeyPath("int"))

	var i int

	err := val.Get(&i)
	require.NoError(t, err)
	assert.Equal(t, 42, i)
}

func TestValue_Get_IntToStringConversion(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("int"), 42)

	node := root.Get(keypath.NewKeyPath("int"))
	require.NotNil(t, node)

	val := tree.NewValue(node, keypath.NewKeyPath("int"))

	var s string

	err := val.Get(&s)
	require.NoError(t, err)
	assert.Equal(t, "42", s)
}

func TestValue_Get_String(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("str"), "hello")

	node := root.Get(keypath.NewKeyPath("str"))
	require.NotNil(t, node)

	val := tree.NewValue(node, keypath.NewKeyPath("str"))

	var str string

	err := val.Get(&str)
	require.NoError(t, err)
	assert.Equal(t, "hello", str)
}

func TestValue_Get_Bool(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("bool"), true)

	node := root.Get(keypath.NewKeyPath("bool"))
	require.NotNil(t, node)

	val := tree.NewValue(node, keypath.NewKeyPath("bool"))

	var b bool

	err := val.Get(&b)
	require.NoError(t, err)
	assert.True(t, b)
}

func TestValue_Get_Float(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("float"), 3.14)

	node := root.Get(keypath.NewKeyPath("float"))
	require.NotNil(t, node)

	val := tree.NewValue(node, keypath.NewKeyPath("float"))

	var f float64

	err := val.Get(&f)
	require.NoError(t, err)
	assert.InDelta(t, 3.14, f, 0.0001)
}

func TestValue_Get_BoolFromString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("boolstr"), "true")

	node := root.Get(keypath.NewKeyPath("boolstr"))
	val := tree.NewValue(node, keypath.NewKeyPath("boolstr"))

	var b bool

	err := val.Get(&b)
	require.NoError(t, err)
	assert.True(t, b)
}

func TestValue_Get_BoolFromBool(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("booltrue"), true)

	node := root.Get(keypath.NewKeyPath("booltrue"))
	val := tree.NewValue(node, keypath.NewKeyPath("booltrue"))

	var got bool

	err := val.Get(&got)
	require.NoError(t, err)
	assert.True(t, got)

	root.Set(keypath.NewKeyPath("boolfalse"), false)

	node = root.Get(keypath.NewKeyPath("boolfalse"))
	val = tree.NewValue(node, keypath.NewKeyPath("boolfalse"))

	err = val.Get(&got)
	require.NoError(t, err)
	assert.False(t, got)
}

func TestValue_Get_IntFromString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("intstr"), "123")

	node := root.Get(keypath.NewKeyPath("intstr"))
	val := tree.NewValue(node, keypath.NewKeyPath("intstr"))

	var i int

	err := val.Get(&i)
	require.NoError(t, err)
	assert.Equal(t, 123, i)
}

func TestValue_Get_FloatFromString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("floatstr"), "45.6")

	node := root.Get(keypath.NewKeyPath("floatstr"))
	val := tree.NewValue(node, keypath.NewKeyPath("floatstr"))

	var f float64

	err := val.Get(&f)
	require.NoError(t, err)
	assert.InDelta(t, 45.6, f, 0.0001)
}

func TestValue_Get_DurationFromString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("duration"), "5s")

	node := root.Get(keypath.NewKeyPath("duration"))
	val := tree.NewValue(node, keypath.NewKeyPath("duration"))

	var d time.Duration

	err := val.Get(&d)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, d)
}

func TestValue_Get_Slice(t *testing.T) {
	t.Parallel()

	root := tree.New()

	root.Set(keypath.NewKeyPath("numbers"), []any{1, 2, 3})
	root.Set(keypath.NewKeyPath("strings"), []any{"a", "b", "c"})

	node := root.Get(keypath.NewKeyPath("numbers"))
	val := tree.NewValue(node, keypath.NewKeyPath("numbers"))

	var nums []int

	err := val.Get(&nums)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, nums)

	node = root.Get(keypath.NewKeyPath("strings"))
	val = tree.NewValue(node, keypath.NewKeyPath("strings"))

	var strs []string

	err = val.Get(&strs)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, strs)
}

func TestValue_Get_Map(t *testing.T) {
	t.Parallel()

	root := tree.New()

	root.Set(keypath.NewKeyPath("person/name"), "Alice")
	root.Set(keypath.NewKeyPath("person/age"), 30)
	root.Set(keypath.NewKeyPath("person/active"), true)

	node := root.Get(keypath.NewKeyPath("person"))
	val := tree.NewValue(node, keypath.NewKeyPath("person"))

	var m map[string]any

	err := val.Get(&m)
	require.NoError(t, err)
	assert.Equal(t, "Alice", m["name"])
	assert.Equal(t, 30, m["age"])
	assert.Equal(t, true, m["active"])
}

func TestValue_Get_Struct(t *testing.T) {
	t.Parallel()

	type Person struct {
		Name   string `yaml:"name"`
		Age    int    `yaml:"age"`
		Active bool   `yaml:"active"`
		Extra  string
	}

	root := tree.New()
	root.Set(keypath.NewKeyPath("person/name"), "Bob")
	root.Set(keypath.NewKeyPath("person/age"), 25)
	root.Set(keypath.NewKeyPath("person/active"), false)
	root.Set(keypath.NewKeyPath("person/Extra"), "something")

	node := root.Get(keypath.NewKeyPath("person"))
	val := tree.NewValue(node, keypath.NewKeyPath("person"))

	var person Person

	err := val.Get(&person)
	require.NoError(t, err)
	assert.Equal(t, "Bob", person.Name)
	assert.Equal(t, 25, person.Age)
	assert.False(t, person.Active)
	assert.Equal(t, "something", person.Extra)
}

func TestValue_Get_NestedStruct(t *testing.T) {
	t.Parallel()

	type Address struct {
		City string `yaml:"city"`
		Zip  int    `yaml:"zip"`
	}

	type User struct {
		Name    string  `yaml:"name"`
		Address Address `yaml:"address"`
	}

	root := tree.New()
	root.Set(keypath.NewKeyPath("user/name"), "Charlie")
	root.Set(keypath.NewKeyPath("user/address/city"), "Moscow")
	root.Set(keypath.NewKeyPath("user/address/zip"), 123456)

	node := root.Get(keypath.NewKeyPath("user"))
	val := tree.NewValue(node, keypath.NewKeyPath("user"))

	var user User

	err := val.Get(&user)
	require.NoError(t, err)
	assert.Equal(t, "Charlie", user.Name)
	assert.Equal(t, "Moscow", user.Address.City)
	assert.Equal(t, 123456, user.Address.Zip)
}

func TestValue_Get_Pointer(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("value"), "pointer test")

	node := root.Get(keypath.NewKeyPath("value"))
	val := tree.NewValue(node, keypath.NewKeyPath("value"))

	var ptr *string

	err := val.Get(&ptr)
	require.NoError(t, err)
	require.NotNil(t, ptr)
	assert.Equal(t, "pointer test", *ptr)
}

func TestValue_Get_NonPointerError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("str"), "hello")

	node := root.Get(keypath.NewKeyPath("str"))
	val := tree.NewValue(node, keypath.NewKeyPath("str"))

	var s string

	err := val.Get(s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pointer")
}

func TestValue_Get_NilPointerError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("str"), "hello")

	node := root.Get(keypath.NewKeyPath("str"))
	val := tree.NewValue(node, keypath.NewKeyPath("str"))

	err := val.Get(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pointer")
}

func TestValue_Get_TypeMismatchError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("str"), "hello")

	node := root.Get(keypath.NewKeyPath("str"))
	val := tree.NewValue(node, keypath.NewKeyPath("str"))

	var i int

	err := val.Get(&i)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "convert")
}

func TestValue_Get_IntFromInvalidString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("notint"), "abc")

	node := root.Get(keypath.NewKeyPath("notint"))
	val := tree.NewValue(node, keypath.NewKeyPath("notint"))

	var i int

	err := val.Get(&i)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "convert")
}

func TestValue_Get_FloatFromInvalidString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("notfloat"), "xyz")

	node := root.Get(keypath.NewKeyPath("notfloat"))
	val := tree.NewValue(node, keypath.NewKeyPath("notfloat"))

	var f float64

	err := val.Get(&f)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "convert")
}

func TestValue_Get_FloatFromVariousTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  any
		want float64
	}{
		{"float32", float32(3.14), float64(float32(3.14))},
		{"int8", int8(42), 42},
		{"int16", int16(1000), 1000},
		{"int32", int32(99999), 99999},
		{"int64", int64(-5), -5},
		{"uint8", uint8(255), 255},
		{"uint16", uint16(65535), 65535},
		{"uint32", uint32(4294967295), 4294967295},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := tree.New()
			root.Set(keypath.NewKeyPath("val"), tt.src)

			node := root.Get(keypath.NewKeyPath("val"))
			val := tree.NewValue(node, keypath.NewKeyPath("val"))

			var got float64

			err := val.Get(&got)
			require.NoError(t, err)
			assert.InDelta(t, tt.want, got, 0.0001)
		})
	}
}

func TestValue_Get_BoolFromInvalidString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("notbool"), "maybe")

	node := root.Get(keypath.NewKeyPath("notbool"))
	val := tree.NewValue(node, keypath.NewKeyPath("notbool"))

	var b bool

	err := val.Get(&b)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "convert")
}

func TestValue_Get_DurationFromInvalidString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("notduration"), "10x")

	node := root.Get(keypath.NewKeyPath("notduration"))
	val := tree.NewValue(node, keypath.NewKeyPath("notduration"))

	var d time.Duration

	err := val.Get(&d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestValue_Get_UintFromNegativeFloatString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("negfloat"), "-1.5")

	node := root.Get(keypath.NewKeyPath("negfloat"))
	val := tree.NewValue(node, keypath.NewKeyPath("negfloat"))

	var u uint

	err := val.Get(&u)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid syntax")
}

func TestValue_Get_DurationFromUnsupportedType(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("notduration"), []any{1, 2})

	node := root.Get(keypath.NewKeyPath("notduration"))
	val := tree.NewValue(node, keypath.NewKeyPath("notduration"))

	var dur time.Duration

	err := val.Get(&dur)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot convert")
}

func TestValue_Get_Overflow(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("big"), 1000)

	node := root.Get(keypath.NewKeyPath("big"))
	val := tree.NewValue(node, keypath.NewKeyPath("big"))

	var u8 uint8

	err := val.Get(&u8)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overflow")

	var i8 int8

	err = val.Get(&i8)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overflow")

	root.Set(keypath.NewKeyPath("huge"), 1e50)

	node = root.Get(keypath.NewKeyPath("huge"))
	val = tree.NewValue(node, keypath.NewKeyPath("huge"))

	var f32 float32

	err = val.Get(&f32)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overflow")
}

func TestValue_Get_MapToStringConversion(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("map/str"), "hello")
	root.Set(keypath.NewKeyPath("map/num"), "42")
	root.Set(keypath.NewKeyPath("map/bool"), "true")

	node := root.Get(keypath.NewKeyPath("map"))
	val := tree.NewValue(node, keypath.NewKeyPath("map"))

	var m map[string]string

	err := val.Get(&m)
	require.NoError(t, err)
	assert.Equal(t, "hello", m["str"])
	assert.Equal(t, "42", m["num"])
	assert.Equal(t, "true", m["bool"])
}

func TestValue_Get_MapToIntConversionError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("map/str"), "hello")
	root.Set(keypath.NewKeyPath("map/num"), "42")
	root.Set(keypath.NewKeyPath("map/bool"), "true")

	node := root.Get(keypath.NewKeyPath("map"))
	val := tree.NewValue(node, keypath.NewKeyPath("map"))

	var m map[string]int

	err := val.Get(&m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "convert")
}

func TestValue_Get_MapFromSliceError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("notmap"), []any{1, 2, 3})

	node := root.Get(keypath.NewKeyPath("notmap"))
	val := tree.NewValue(node, keypath.NewKeyPath("notmap"))

	var m map[string]any

	err := val.Get(&m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a map")
}

func TestValue_Get_MapWithNonStringKeyError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("goodmap/foo"), "bar")

	node := root.Get(keypath.NewKeyPath("goodmap"))
	val := tree.NewValue(node, keypath.NewKeyPath("goodmap"))

	var badMap map[int]string

	err := val.Get(&badMap)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must have string keys")
}

func TestValue_Get_Int8ToIntConversion(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("val"), int8(42))

	node := root.Get(keypath.NewKeyPath("val"))
	val := tree.NewValue(node, keypath.NewKeyPath("val"))

	var i int

	err := val.Get(&i)
	require.NoError(t, err)
	assert.Equal(t, 42, i)
}

func TestValue_Get_Int16(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("val16"), int16(1000))

	node := root.Get(keypath.NewKeyPath("val16"))
	val := tree.NewValue(node, keypath.NewKeyPath("val16"))

	var i16 int16

	err := val.Get(&i16)
	require.NoError(t, err)
	assert.Equal(t, int16(1000), i16)
}

func TestValue_Get_Int32(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("val32"), int32(100000))

	node := root.Get(keypath.NewKeyPath("val32"))
	val := tree.NewValue(node, keypath.NewKeyPath("val32"))

	var i32 int32

	err := val.Get(&i32)
	require.NoError(t, err)
	assert.Equal(t, int32(100000), i32)
}

func TestValue_Get_Uint8(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("valu8"), uint8(200))

	node := root.Get(keypath.NewKeyPath("valu8"))
	val := tree.NewValue(node, keypath.NewKeyPath("valu8"))

	var u8 uint8

	err := val.Get(&u8)
	require.NoError(t, err)
	assert.Equal(t, uint8(200), u8)
}

func TestValue_Get_Uint16(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("valu16"), uint16(50000))

	node := root.Get(keypath.NewKeyPath("valu16"))
	val := tree.NewValue(node, keypath.NewKeyPath("valu16"))

	var u16 uint16

	err := val.Get(&u16)
	require.NoError(t, err)
	assert.Equal(t, uint16(50000), u16)
}

func TestValue_Get_Uint32(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("valu32"), uint32(3000000000))

	node := root.Get(keypath.NewKeyPath("valu32"))
	val := tree.NewValue(node, keypath.NewKeyPath("valu32"))

	var u32 uint32

	err := val.Get(&u32)
	require.NoError(t, err)
	assert.Equal(t, uint32(3000000000), u32)
}

func TestValue_Get_DurationFromInt64(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("sec"), int64(5))

	node := root.Get(keypath.NewKeyPath("sec"))
	val := tree.NewValue(node, keypath.NewKeyPath("sec"))

	var d time.Duration

	err := val.Get(&d)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, d)
}

func TestValue_Get_DurationFromUint64(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("usec"), uint64(5))

	node := root.Get(keypath.NewKeyPath("usec"))
	val := tree.NewValue(node, keypath.NewKeyPath("usec"))

	var d time.Duration

	err := val.Get(&d)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, d)
}

func TestValue_Get_DurationFromFloat(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("floatsec"), 2.5)

	node := root.Get(keypath.NewKeyPath("floatsec"))
	val := tree.NewValue(node, keypath.NewKeyPath("floatsec"))

	var d time.Duration

	err := val.Get(&d)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(2500000000), d)
}

func TestValue_Get_StringFromBytes(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("bytes"), []byte("hello bytes"))

	node := root.Get(keypath.NewKeyPath("bytes"))
	val := tree.NewValue(node, keypath.NewKeyPath("bytes"))

	var s string

	err := val.Get(&s)
	require.NoError(t, err)
	assert.Equal(t, "hello bytes", s)
}

func TestValue_Get_StringFromStringer(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("stringer"), testStringer{})

	node := root.Get(keypath.NewKeyPath("stringer"))
	val := tree.NewValue(node, keypath.NewKeyPath("stringer"))

	var s string

	err := val.Get(&s)
	require.NoError(t, err)
	assert.Equal(t, "I am a Stringer", s)
}

func TestValue_Get_StringFromInt(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("int"), 123)

	node := root.Get(keypath.NewKeyPath("int"))
	val := tree.NewValue(node, keypath.NewKeyPath("int"))

	var s string

	err := val.Get(&s)
	require.NoError(t, err)
	assert.Equal(t, "123", s)
}

func TestValue_Get_BoolFromZero(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("zero"), 0)

	node := root.Get(keypath.NewKeyPath("zero"))
	val := tree.NewValue(node, keypath.NewKeyPath("zero"))

	var b bool

	err := val.Get(&b)
	require.NoError(t, err)
	assert.False(t, b)
}

func TestValue_Get_BoolFromOne(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("one"), 1)

	node := root.Get(keypath.NewKeyPath("one"))
	val := tree.NewValue(node, keypath.NewKeyPath("one"))

	var b bool

	err := val.Get(&b)
	require.NoError(t, err)
	assert.True(t, b)
}

func TestValue_Get_BoolFromNegative(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("neg"), -5)

	node := root.Get(keypath.NewKeyPath("neg"))
	val := tree.NewValue(node, keypath.NewKeyPath("neg"))

	var b bool

	err := val.Get(&b)
	require.NoError(t, err)
	assert.True(t, b)
}

func TestValue_Get_BoolFromUint(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("uintval"), uint(10))

	node := root.Get(keypath.NewKeyPath("uintval"))
	val := tree.NewValue(node, keypath.NewKeyPath("uintval"))

	var b bool

	err := val.Get(&b)
	require.NoError(t, err)
	assert.True(t, b)
}

func TestValue_Get_NilSource(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("nilval"), nil)

	node := root.Get(keypath.NewKeyPath("nilval"))
	val := tree.NewValue(node, keypath.NewKeyPath("nilval"))

	var s string

	err := val.Get(&s)
	require.NoError(t, err)
	assert.Empty(t, s)

	var i int

	err = val.Get(&i)
	require.NoError(t, err)
	assert.Equal(t, 0, i)

	var p *int

	err = val.Get(&p)
	require.NoError(t, err)
	require.Nil(t, p)
}

func TestValue_Meta(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("test"), "value")

	node := root.Get(keypath.NewKeyPath("test"))
	require.NotNil(t, node)

	node.Source = "file.yaml"
	node.Revision = "42"

	val := tree.NewValue(node, keypath.NewKeyPath("test"))
	mi := val.Meta()
	assert.Equal(t, keypath.NewKeyPath("test"), mi.Key)
	assert.Equal(t, "file.yaml", mi.Source.Name)
	assert.Equal(t, meta.UnknownSource, mi.Source.Type)
	assert.Equal(t, meta.RevisionType("42"), mi.Revision)
}

func TestValue_Get_Uint64ToInt64Overflow(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("large"), uint64(1<<63))

	node := root.Get(keypath.NewKeyPath("large"))
	val := tree.NewValue(node, keypath.NewKeyPath("large"))

	var i64 int64

	err := val.Get(&i64)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overflow")
}

func TestValue_Get_UintToIntOverflow(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("large"), uint(1<<63))

	node := root.Get(keypath.NewKeyPath("large"))
	val := tree.NewValue(node, keypath.NewKeyPath("large"))

	var i int

	err := val.Get(&i)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overflow")
}

func TestValue_Get_IntFromBool(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("booltrue"), true)

	node := root.Get(keypath.NewKeyPath("booltrue"))
	val := tree.NewValue(node, keypath.NewKeyPath("booltrue"))

	var got int

	err := val.Get(&got)
	require.NoError(t, err)
	assert.Equal(t, 1, got)

	root.Set(keypath.NewKeyPath("boolfalse"), false)

	node = root.Get(keypath.NewKeyPath("boolfalse"))
	val = tree.NewValue(node, keypath.NewKeyPath("boolfalse"))

	err = val.Get(&got)
	require.NoError(t, err)
	assert.Equal(t, 0, got)
}

func TestValue_Get_Int16Overflow(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("large"), 32768)

	node := root.Get(keypath.NewKeyPath("large"))
	val := tree.NewValue(node, keypath.NewKeyPath("large"))

	var i16 int16

	err := val.Get(&i16)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overflow")

	root.Set(keypath.NewKeyPath("small"), -32769)

	node = root.Get(keypath.NewKeyPath("small"))
	val = tree.NewValue(node, keypath.NewKeyPath("small"))

	err = val.Get(&i16)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overflow")
}

func TestValue_Get_Int32Overflow(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("large"), int64(2147483648))

	node := root.Get(keypath.NewKeyPath("large"))
	val := tree.NewValue(node, keypath.NewKeyPath("large"))

	var i32 int32

	err := val.Get(&i32)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overflow")

	root.Set(keypath.NewKeyPath("small"), int64(-2147483649))

	node = root.Get(keypath.NewKeyPath("small"))
	val = tree.NewValue(node, keypath.NewKeyPath("small"))

	err = val.Get(&i32)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overflow")
}

func TestValue_Get_UintFromNegativeInt(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("neg"), -5)

	node := root.Get(keypath.NewKeyPath("neg"))
	val := tree.NewValue(node, keypath.NewKeyPath("neg"))

	var u uint

	err := val.Get(&u)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "negative")
}

func TestValue_Get_UintFromNegativeFloat(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("neg"), -3.14)

	node := root.Get(keypath.NewKeyPath("neg"))
	val := tree.NewValue(node, keypath.NewKeyPath("neg"))

	var u uint

	err := val.Get(&u)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "negative")
}

func TestValue_Get_UintFromNegativeFloat32(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("neg"), float32(-3.14))

	node := root.Get(keypath.NewKeyPath("neg"))
	val := tree.NewValue(node, keypath.NewKeyPath("neg"))

	var u uint

	err := val.Get(&u)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "negative")
}

func TestValue_Get_Uint16Overflow(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("large"), 65536)

	node := root.Get(keypath.NewKeyPath("large"))
	val := tree.NewValue(node, keypath.NewKeyPath("large"))

	var u16 uint16

	err := val.Get(&u16)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overflow")
}

func TestValue_Get_Uint32Overflow(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("large"), uint64(4294967296))

	node := root.Get(keypath.NewKeyPath("large"))
	val := tree.NewValue(node, keypath.NewKeyPath("large"))

	var u32 uint32

	err := val.Get(&u32)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overflow")
}

func TestValue_Get_UintFromBool(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("booltrue"), true)

	node := root.Get(keypath.NewKeyPath("booltrue"))
	val := tree.NewValue(node, keypath.NewKeyPath("booltrue"))

	var got uint

	err := val.Get(&got)
	require.NoError(t, err)
	assert.Equal(t, uint(1), got)

	root.Set(keypath.NewKeyPath("boolfalse"), false)

	node = root.Get(keypath.NewKeyPath("boolfalse"))
	val = tree.NewValue(node, keypath.NewKeyPath("boolfalse"))

	err = val.Get(&got)
	require.NoError(t, err)
	assert.Equal(t, uint(0), got)
}

func TestValue_Get_UintFromInvalidString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("invalid"), "abc")

	node := root.Get(keypath.NewKeyPath("invalid"))
	val := tree.NewValue(node, keypath.NewKeyPath("invalid"))

	var u uint

	err := val.Get(&u)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "convert")
}

func TestValue_Get_UintFromVariousTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  any
		want uint64
	}{
		{"uint", uint(42), 42},
		{"uint8", uint8(255), 255},
		{"uint16", uint16(65535), 65535},
		{"uint32", uint32(4294967295), 4294967295},
		{"float32 positive", float32(3.14), 3},
		{"string positive", "12345", 12345},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := tree.New()
			root.Set(keypath.NewKeyPath("val"), tt.src)

			node := root.Get(keypath.NewKeyPath("val"))
			val := tree.NewValue(node, keypath.NewKeyPath("val"))

			var got uint64

			err := val.Get(&got)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValue_Get_UintFromUnsupportedType(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("val"), []int{1, 2, 3})

	node := root.Get(keypath.NewKeyPath("val"))
	val := tree.NewValue(node, keypath.NewKeyPath("val"))

	var u uint

	err := val.Get(&u)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "convert to uint")
}

func TestValue_Get_Float32Overflow(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("huge"), 1e100)

	node := root.Get(keypath.NewKeyPath("huge"))
	val := tree.NewValue(node, keypath.NewKeyPath("huge"))

	var f32 float32

	err := val.Get(&f32)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overflow")

	root.Set(keypath.NewKeyPath("negHuge"), -1e100)

	node = root.Get(keypath.NewKeyPath("negHuge"))
	val = tree.NewValue(node, keypath.NewKeyPath("negHuge"))

	err = val.Get(&f32)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overflow")
}

func TestValue_Get_FloatFromUnsupportedType(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("unsupported"), []any{1, 2})

	node := root.Get(keypath.NewKeyPath("unsupported"))
	val := tree.NewValue(node, keypath.NewKeyPath("unsupported"))

	var f float64

	err := val.Get(&f)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "convert")
}

func TestValue_Get_BoolFromUnsupportedType(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("unsupported"), []any{1, 2})

	node := root.Get(keypath.NewKeyPath("unsupported"))
	val := tree.NewValue(node, keypath.NewKeyPath("unsupported"))

	var b bool

	err := val.Get(&b)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "convert")
}

func TestValue_Get_SliceFromNonSliceError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("notslice"), "string")

	node := root.Get(keypath.NewKeyPath("notslice"))
	val := tree.NewValue(node, keypath.NewKeyPath("notslice"))

	var s []int

	err := val.Get(&s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a slice")
}

func TestValue_Get_MapKeyNotStringError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("badmap"), map[int]string{1: "foo"})

	node := root.Get(keypath.NewKeyPath("badmap"))
	val := tree.NewValue(node, keypath.NewKeyPath("badmap"))

	var m map[string]string

	err := val.Get(&m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key is not string")
}

func TestValue_Get_StructSourceNotMapError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("notmap"), "string")

	node := root.Get(keypath.NewKeyPath("notmap"))
	val := tree.NewValue(node, keypath.NewKeyPath("notmap"))

	type S struct {
		Field string `yaml:"field"`
	}

	var s S

	err := val.Get(&s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a map")
}

func TestValue_Get_StructMapKeyNotStringError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("badmap"), map[int]string{1: "foo"})

	node := root.Get(keypath.NewKeyPath("badmap"))
	val := tree.NewValue(node, keypath.NewKeyPath("badmap"))

	type S struct {
		Field string `yaml:"field"`
	}

	var s S

	err := val.Get(&s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must have string keys")
}

func TestValue_Get_StructFieldDecodeError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("struct/int"), "abc")

	node := root.Get(keypath.NewKeyPath("struct"))
	val := tree.NewValue(node, keypath.NewKeyPath("struct"))

	type S struct {
		Int int `yaml:"int"`
	}

	var s S

	err := val.Get(&s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "convert")
}

func TestValue_Get_StructUnexportedField(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("struct/exported"), "hello")
	root.Set(keypath.NewKeyPath("struct/unexported"), "world")

	node := root.Get(keypath.NewKeyPath("struct"))
	val := tree.NewValue(node, keypath.NewKeyPath("struct"))

	type S struct {
		Exported   string `yaml:"exported"`
		unexported string
	}

	var s S

	err := val.Get(&s)
	require.NoError(t, err)
	assert.Equal(t, "hello", s.Exported)
	assert.Empty(t, s.unexported)
}

func TestValue_Get_StructYamlTagComma(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("struct/myfield"), "value")

	node := root.Get(keypath.NewKeyPath("struct"))
	val := tree.NewValue(node, keypath.NewKeyPath("struct"))

	type S struct {
		Field string `yaml:"myfield,omitempty"`
	}

	var s S

	err := val.Get(&s)
	require.NoError(t, err)
	assert.Equal(t, "value", s.Field)
}

func TestValue_Get_StructMissingField(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("struct/present"), "hello")

	node := root.Get(keypath.NewKeyPath("struct"))
	val := tree.NewValue(node, keypath.NewKeyPath("struct"))

	type S struct {
		Present string `yaml:"present"`
		Missing string `yaml:"missing"`
	}

	var s S

	err := val.Get(&s)
	require.NoError(t, err)
	assert.Equal(t, "hello", s.Present)
	assert.Empty(t, s.Missing)
}

func TestValue_Get_DurationOverflowInt64(t *testing.T) {
	t.Parallel()

	maxSafeSeconds := math.MaxInt64 / int64(time.Second)
	root := tree.New()
	root.Set(keypath.NewKeyPath("large"), maxSafeSeconds+1)

	node := root.Get(keypath.NewKeyPath("large"))
	val := tree.NewValue(node, keypath.NewKeyPath("large"))

	var d time.Duration

	err := val.Get(&d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overflow")
}

func TestValue_Get_DurationOverflowUint64(t *testing.T) {
	t.Parallel()

	maxSafeUintSeconds := uint64(math.MaxInt64 / int64(time.Second))
	root := tree.New()
	root.Set(keypath.NewKeyPath("large"), maxSafeUintSeconds+1)

	node := root.Get(keypath.NewKeyPath("large"))
	val := tree.NewValue(node, keypath.NewKeyPath("large"))

	var d time.Duration

	err := val.Get(&d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overflow")
}

func TestValue_Get_UnsupportedDestinationType(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("val"), 123)

	node := root.Get(keypath.NewKeyPath("val"))
	val := tree.NewValue(node, keypath.NewKeyPath("val"))

	var c complex64

	err := val.Get(&c)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestValue_Get_InterfaceImplemented(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("stringer"), testStringer{})

	node := root.Get(keypath.NewKeyPath("stringer"))
	val := tree.NewValue(node, keypath.NewKeyPath("stringer"))

	var iface fmt.Stringer

	err := val.Get(&iface)
	require.NoError(t, err)
	assert.Equal(t, "I am a Stringer", iface.String())
}

func TestValue_Get_InterfaceEmpty(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("val"), 42)

	node := root.Get(keypath.NewKeyPath("val"))
	val := tree.NewValue(node, keypath.NewKeyPath("val"))

	var iface any

	err := val.Get(&iface)
	require.NoError(t, err)
	assert.Equal(t, 42, iface)
}

func TestValue_Get_IntFromUint(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("val"), uint(42))

	node := root.Get(keypath.NewKeyPath("val"))
	val := tree.NewValue(node, keypath.NewKeyPath("val"))

	var i int

	err := val.Get(&i)
	require.NoError(t, err)
	assert.Equal(t, 42, i)
}

func TestValue_Get_Int64FromUint64(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("val"), uint64(1234567890))

	node := root.Get(keypath.NewKeyPath("val"))
	val := tree.NewValue(node, keypath.NewKeyPath("val"))

	var i64 int64

	err := val.Get(&i64)
	require.NoError(t, err)
	assert.Equal(t, int64(1234567890), i64)
}

func TestValue_Get_IntFromFloat(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("val"), 3.0)

	node := root.Get(keypath.NewKeyPath("val"))
	val := tree.NewValue(node, keypath.NewKeyPath("val"))

	var i int

	err := val.Get(&i)
	require.NoError(t, err)
	assert.Equal(t, 3, i)
}

func TestValue_Get_UintFromFloat(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("val"), 5.0)

	node := root.Get(keypath.NewKeyPath("val"))
	val := tree.NewValue(node, keypath.NewKeyPath("val"))

	var u uint

	err := val.Get(&u)
	require.NoError(t, err)
	assert.Equal(t, uint(5), u)
}

func TestValue_Get_IntFromVariousTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  any
		want int64
	}{
		{"int8", int8(42), 42},
		{"int16", int16(1000), 1000},
		{"int32", int32(99999), 99999},
		{"int64", int64(-5), -5},
		{"uint8", uint8(255), 255},
		{"uint16", uint16(65535), 65535},
		{"uint32", uint32(4294967295), 4294967295},
		{"float32", float32(3.14), 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := tree.New()
			root.Set(keypath.NewKeyPath("val"), tt.src)

			node := root.Get(keypath.NewKeyPath("val"))
			val := tree.NewValue(node, keypath.NewKeyPath("val"))

			var got int64

			err := val.Get(&got)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValue_Get_IntFromUnsupportedType(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("val"), []int{1, 2, 3})

	node := root.Get(keypath.NewKeyPath("val"))
	val := tree.NewValue(node, keypath.NewKeyPath("val"))

	var i int

	err := val.Get(&i)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "convert to int")
}

func TestValue_Get_BoolFromNumericTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		val  any
		want bool
	}{
		{"int8 positive", int8(5), true},
		{"int8 zero", int8(0), false},
		{"int16 positive", int16(256), true},
		{"int16 zero", int16(0), false},
		{"int32 positive", int32(1000), true},
		{"int32 zero", int32(0), false},
		{"int64 positive", int64(-1), true},
		{"int64 zero", int64(0), false},
		{"uint8 positive", uint8(1), true},
		{"uint8 zero", uint8(0), false},
		{"uint16 positive", uint16(65535), true},
		{"uint16 zero", uint16(0), false},
		{"uint32 positive", uint32(40000), true},
		{"uint32 zero", uint32(0), false},
		{"uint64 positive", uint64(999), true},
		{"uint64 zero", uint64(0), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := tree.New()
			root.Set(keypath.NewKeyPath("val"), tt.val)

			node := root.Get(keypath.NewKeyPath("val"))
			val := tree.NewValue(node, keypath.NewKeyPath("val"))

			var b bool

			err := val.Get(&b)
			require.NoError(t, err)
			assert.Equal(t, tt.want, b)
		})
	}
}

func TestValue_Get_SliceElementConversionError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("slice"), []any{1, "abc", 3})

	node := root.Get(keypath.NewKeyPath("slice"))
	val := tree.NewValue(node, keypath.NewKeyPath("slice"))

	var s []int

	err := val.Get(&s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slice element")
}

func TestValue_Get_UnsupportedDestinationKinds(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("val"), 123)

	node := root.Get(keypath.NewKeyPath("val"))
	val := tree.NewValue(node, keypath.NewKeyPath("val"))

	var arr [1]int

	err := val.Get(&arr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")

	var ch chan int

	err = val.Get(&ch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")

	var fn func()

	err = val.Get(&fn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")

	var up unsafe.Pointer

	err = val.Get(&up)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")

	var ui uintptr

	err = val.Get(&ui)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")

	var c64 complex64

	err = val.Get(&c64)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")

	var c128 complex128

	err = val.Get(&c128)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}
