package tree_test

import (
	"fmt"
	"math"
	"testing"
	"time"
	"unsafe"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"

	"github.com/tarantool/go-config/internal/tree"
	"github.com/tarantool/go-config/meta"
	"github.com/tarantool/go-config/path"
)

var _ = math.MaxInt64

type testStringer struct{}

func (testStringer) String() string { return "I am a Stringer" }

func TestValue_Get_Int(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("int"), 42)

	node := root.Get(path.NewKeyPath("int"))
	must.NotNil(t, node)

	val := tree.NewValue(node, path.NewKeyPath("int"))

	var i int

	err := val.Get(&i)
	must.NoError(t, err)
	test.Eq(t, 42, i)
}

func TestValue_Get_IntToStringConversion(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("int"), 42)

	node := root.Get(path.NewKeyPath("int"))
	must.NotNil(t, node)

	val := tree.NewValue(node, path.NewKeyPath("int"))

	var s string

	err := val.Get(&s)
	must.NoError(t, err)
	test.Eq(t, "42", s)
}

func TestValue_Get_String(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("str"), "hello")

	node := root.Get(path.NewKeyPath("str"))
	must.NotNil(t, node)

	val := tree.NewValue(node, path.NewKeyPath("str"))

	var str string

	err := val.Get(&str)
	must.NoError(t, err)
	test.Eq(t, "hello", str)
}

func TestValue_Get_Bool(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("bool"), true)

	node := root.Get(path.NewKeyPath("bool"))
	must.NotNil(t, node)

	val := tree.NewValue(node, path.NewKeyPath("bool"))

	var b bool

	err := val.Get(&b)
	must.NoError(t, err)
	test.True(t, b)
}

func TestValue_Get_Float(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("float"), 3.14)

	node := root.Get(path.NewKeyPath("float"))
	must.NotNil(t, node)

	val := tree.NewValue(node, path.NewKeyPath("float"))

	var f float64

	err := val.Get(&f)
	must.NoError(t, err)
	test.Eq(t, 3.14, f)
}

func TestValue_Get_BoolFromString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("boolstr"), "true")

	node := root.Get(path.NewKeyPath("boolstr"))
	val := tree.NewValue(node, path.NewKeyPath("boolstr"))

	var b bool

	err := val.Get(&b)
	must.NoError(t, err)
	test.True(t, b)
}

func TestValue_Get_BoolFromBool(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("booltrue"), true)

	node := root.Get(path.NewKeyPath("booltrue"))
	val := tree.NewValue(node, path.NewKeyPath("booltrue"))

	var got bool

	err := val.Get(&got)
	must.NoError(t, err)
	test.True(t, got)

	root.Set(path.NewKeyPath("boolfalse"), false)

	node = root.Get(path.NewKeyPath("boolfalse"))
	val = tree.NewValue(node, path.NewKeyPath("boolfalse"))

	err = val.Get(&got)
	must.NoError(t, err)
	test.False(t, got)
}

func TestValue_Get_IntFromString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("intstr"), "123")

	node := root.Get(path.NewKeyPath("intstr"))
	val := tree.NewValue(node, path.NewKeyPath("intstr"))

	var i int

	err := val.Get(&i)
	must.NoError(t, err)
	test.Eq(t, 123, i)
}

func TestValue_Get_FloatFromString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("floatstr"), "45.6")

	node := root.Get(path.NewKeyPath("floatstr"))
	val := tree.NewValue(node, path.NewKeyPath("floatstr"))

	var f float64

	err := val.Get(&f)
	must.NoError(t, err)
	test.Eq(t, 45.6, f)
}

func TestValue_Get_DurationFromString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("duration"), "5s")

	node := root.Get(path.NewKeyPath("duration"))
	val := tree.NewValue(node, path.NewKeyPath("duration"))

	var d time.Duration

	err := val.Get(&d)
	must.NoError(t, err)
	test.Eq(t, 5*time.Second, d)
}

func TestValue_Get_Slice(t *testing.T) {
	t.Parallel()

	root := tree.New()

	root.Set(path.NewKeyPath("numbers"), []any{1, 2, 3})
	root.Set(path.NewKeyPath("strings"), []any{"a", "b", "c"})

	node := root.Get(path.NewKeyPath("numbers"))
	val := tree.NewValue(node, path.NewKeyPath("numbers"))

	var nums []int

	err := val.Get(&nums)
	must.NoError(t, err)
	test.Eq(t, []int{1, 2, 3}, nums)

	node = root.Get(path.NewKeyPath("strings"))
	val = tree.NewValue(node, path.NewKeyPath("strings"))

	var strs []string

	err = val.Get(&strs)
	must.NoError(t, err)
	test.Eq(t, []string{"a", "b", "c"}, strs)
}

func TestValue_Get_Map(t *testing.T) {
	t.Parallel()

	root := tree.New()

	root.Set(path.NewKeyPath("person/name"), "Alice")
	root.Set(path.NewKeyPath("person/age"), 30)
	root.Set(path.NewKeyPath("person/active"), true)

	node := root.Get(path.NewKeyPath("person"))
	val := tree.NewValue(node, path.NewKeyPath("person"))

	var m map[string]any

	err := val.Get(&m)
	must.NoError(t, err)
	test.Eq(t, "Alice", m["name"])
	test.Eq(t, 30, m["age"])
	test.Eq(t, true, m["active"])
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
	root.Set(path.NewKeyPath("person/name"), "Bob")
	root.Set(path.NewKeyPath("person/age"), 25)
	root.Set(path.NewKeyPath("person/active"), false)
	root.Set(path.NewKeyPath("person/Extra"), "something")

	node := root.Get(path.NewKeyPath("person"))
	val := tree.NewValue(node, path.NewKeyPath("person"))

	var person Person

	err := val.Get(&person)
	must.NoError(t, err)
	test.Eq(t, "Bob", person.Name)
	test.Eq(t, 25, person.Age)
	test.False(t, person.Active)
	test.Eq(t, "something", person.Extra)
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
	root.Set(path.NewKeyPath("user/name"), "Charlie")
	root.Set(path.NewKeyPath("user/address/city"), "Moscow")
	root.Set(path.NewKeyPath("user/address/zip"), 123456)

	node := root.Get(path.NewKeyPath("user"))
	val := tree.NewValue(node, path.NewKeyPath("user"))

	var user User

	err := val.Get(&user)
	must.NoError(t, err)
	test.Eq(t, "Charlie", user.Name)
	test.Eq(t, "Moscow", user.Address.City)
	test.Eq(t, 123456, user.Address.Zip)
}

func TestValue_Get_Pointer(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("value"), "pointer test")

	node := root.Get(path.NewKeyPath("value"))
	val := tree.NewValue(node, path.NewKeyPath("value"))

	var ptr *string

	err := val.Get(&ptr)
	must.NoError(t, err)
	must.NotNil(t, ptr)
	test.Eq(t, "pointer test", *ptr)
}

func TestValue_Get_NonPointerError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("str"), "hello")

	node := root.Get(path.NewKeyPath("str"))
	val := tree.NewValue(node, path.NewKeyPath("str"))

	var s string

	err := val.Get(s)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "pointer")
}

func TestValue_Get_NilPointerError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("str"), "hello")

	node := root.Get(path.NewKeyPath("str"))
	val := tree.NewValue(node, path.NewKeyPath("str"))

	err := val.Get(nil)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "pointer")
}

func TestValue_Get_TypeMismatchError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("str"), "hello")

	node := root.Get(path.NewKeyPath("str"))
	val := tree.NewValue(node, path.NewKeyPath("str"))

	var i int

	err := val.Get(&i)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "convert")
}

func TestValue_Get_IntFromInvalidString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("notint"), "abc")

	node := root.Get(path.NewKeyPath("notint"))
	val := tree.NewValue(node, path.NewKeyPath("notint"))

	var i int

	err := val.Get(&i)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "convert")
}

func TestValue_Get_FloatFromInvalidString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("notfloat"), "xyz")

	node := root.Get(path.NewKeyPath("notfloat"))
	val := tree.NewValue(node, path.NewKeyPath("notfloat"))

	var f float64

	err := val.Get(&f)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "convert")
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
			root.Set(path.NewKeyPath("val"), tt.src)

			node := root.Get(path.NewKeyPath("val"))
			val := tree.NewValue(node, path.NewKeyPath("val"))

			var got float64

			err := val.Get(&got)
			must.NoError(t, err)
			test.Eq(t, tt.want, got)
		})
	}
}

func TestValue_Get_BoolFromInvalidString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("notbool"), "maybe")

	node := root.Get(path.NewKeyPath("notbool"))
	val := tree.NewValue(node, path.NewKeyPath("notbool"))

	var b bool

	err := val.Get(&b)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "convert")
}

func TestValue_Get_DurationFromInvalidString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("notduration"), "10x")

	node := root.Get(path.NewKeyPath("notduration"))
	val := tree.NewValue(node, path.NewKeyPath("notduration"))

	var d time.Duration

	err := val.Get(&d)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "parse")
}

func TestValue_Get_UintFromNegativeFloatString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("negfloat"), "-1.5")

	node := root.Get(path.NewKeyPath("negfloat"))
	val := tree.NewValue(node, path.NewKeyPath("negfloat"))

	var u uint

	err := val.Get(&u)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "invalid syntax")
}

func TestValue_Get_DurationFromUnsupportedType(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("notduration"), []any{1, 2})

	node := root.Get(path.NewKeyPath("notduration"))
	val := tree.NewValue(node, path.NewKeyPath("notduration"))

	var dur time.Duration

	err := val.Get(&dur)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "cannot convert")
}

func TestValue_Get_Overflow(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("big"), 1000)

	node := root.Get(path.NewKeyPath("big"))
	val := tree.NewValue(node, path.NewKeyPath("big"))

	var u8 uint8

	err := val.Get(&u8)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "overflow")

	var i8 int8

	err = val.Get(&i8)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "overflow")

	root.Set(path.NewKeyPath("huge"), 1e50)

	node = root.Get(path.NewKeyPath("huge"))
	val = tree.NewValue(node, path.NewKeyPath("huge"))

	var f32 float32

	err = val.Get(&f32)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "overflow")
}

func TestValue_Get_MapToStringConversion(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("map/str"), "hello")
	root.Set(path.NewKeyPath("map/num"), "42")
	root.Set(path.NewKeyPath("map/bool"), "true")

	node := root.Get(path.NewKeyPath("map"))
	val := tree.NewValue(node, path.NewKeyPath("map"))

	var m map[string]string

	err := val.Get(&m)
	must.NoError(t, err)
	test.Eq(t, "hello", m["str"])
	test.Eq(t, "42", m["num"])
	test.Eq(t, "true", m["bool"])
}

func TestValue_Get_MapToIntConversionError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("map/str"), "hello")
	root.Set(path.NewKeyPath("map/num"), "42")
	root.Set(path.NewKeyPath("map/bool"), "true")

	node := root.Get(path.NewKeyPath("map"))
	val := tree.NewValue(node, path.NewKeyPath("map"))

	var m map[string]int

	err := val.Get(&m)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "convert")
}

func TestValue_Get_MapFromSliceError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("notmap"), []any{1, 2, 3})

	node := root.Get(path.NewKeyPath("notmap"))
	val := tree.NewValue(node, path.NewKeyPath("notmap"))

	var m map[string]any

	err := val.Get(&m)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "not a map")
}

func TestValue_Get_MapWithNonStringKeyError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("goodmap/foo"), "bar")

	node := root.Get(path.NewKeyPath("goodmap"))
	val := tree.NewValue(node, path.NewKeyPath("goodmap"))

	var badMap map[int]string

	err := val.Get(&badMap)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "must have string keys")
}

func TestValue_Get_Int8ToIntConversion(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("val"), int8(42))

	node := root.Get(path.NewKeyPath("val"))
	val := tree.NewValue(node, path.NewKeyPath("val"))

	var i int

	err := val.Get(&i)
	must.NoError(t, err)
	test.Eq(t, 42, i)
}

func TestValue_Get_Int16(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("val16"), int16(1000))

	node := root.Get(path.NewKeyPath("val16"))
	val := tree.NewValue(node, path.NewKeyPath("val16"))

	var i16 int16

	err := val.Get(&i16)
	must.NoError(t, err)
	test.Eq(t, int16(1000), i16)
}

func TestValue_Get_Int32(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("val32"), int32(100000))

	node := root.Get(path.NewKeyPath("val32"))
	val := tree.NewValue(node, path.NewKeyPath("val32"))

	var i32 int32

	err := val.Get(&i32)
	must.NoError(t, err)
	test.Eq(t, int32(100000), i32)
}

func TestValue_Get_Uint8(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("valu8"), uint8(200))

	node := root.Get(path.NewKeyPath("valu8"))
	val := tree.NewValue(node, path.NewKeyPath("valu8"))

	var u8 uint8

	err := val.Get(&u8)
	must.NoError(t, err)
	test.Eq(t, uint8(200), u8)
}

func TestValue_Get_Uint16(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("valu16"), uint16(50000))

	node := root.Get(path.NewKeyPath("valu16"))
	val := tree.NewValue(node, path.NewKeyPath("valu16"))

	var u16 uint16

	err := val.Get(&u16)
	must.NoError(t, err)
	test.Eq(t, uint16(50000), u16)
}

func TestValue_Get_Uint32(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("valu32"), uint32(3000000000))

	node := root.Get(path.NewKeyPath("valu32"))
	val := tree.NewValue(node, path.NewKeyPath("valu32"))

	var u32 uint32

	err := val.Get(&u32)
	must.NoError(t, err)
	test.Eq(t, uint32(3000000000), u32)
}

func TestValue_Get_DurationFromInt64(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("sec"), int64(5))

	node := root.Get(path.NewKeyPath("sec"))
	val := tree.NewValue(node, path.NewKeyPath("sec"))

	var d time.Duration

	err := val.Get(&d)
	must.NoError(t, err)
	test.Eq(t, 5*time.Second, d)
}

func TestValue_Get_DurationFromUint64(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("usec"), uint64(5))

	node := root.Get(path.NewKeyPath("usec"))
	val := tree.NewValue(node, path.NewKeyPath("usec"))

	var d time.Duration

	err := val.Get(&d)
	must.NoError(t, err)
	test.Eq(t, 5*time.Second, d)
}

func TestValue_Get_DurationFromFloat(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("floatsec"), 2.5)

	node := root.Get(path.NewKeyPath("floatsec"))
	val := tree.NewValue(node, path.NewKeyPath("floatsec"))

	var d time.Duration

	err := val.Get(&d)
	must.NoError(t, err)
	test.Eq(t, time.Duration(2500000000), d)
}

func TestValue_Get_StringFromBytes(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("bytes"), []byte("hello bytes"))

	node := root.Get(path.NewKeyPath("bytes"))
	val := tree.NewValue(node, path.NewKeyPath("bytes"))

	var s string

	err := val.Get(&s)
	must.NoError(t, err)
	test.Eq(t, "hello bytes", s)
}

func TestValue_Get_StringFromStringer(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("stringer"), testStringer{})

	node := root.Get(path.NewKeyPath("stringer"))
	val := tree.NewValue(node, path.NewKeyPath("stringer"))

	var s string

	err := val.Get(&s)
	must.NoError(t, err)
	test.Eq(t, "I am a Stringer", s)
}

func TestValue_Get_StringFromInt(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("int"), 123)

	node := root.Get(path.NewKeyPath("int"))
	val := tree.NewValue(node, path.NewKeyPath("int"))

	var s string

	err := val.Get(&s)
	must.NoError(t, err)
	test.Eq(t, "123", s)
}

func TestValue_Get_BoolFromZero(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("zero"), 0)

	node := root.Get(path.NewKeyPath("zero"))
	val := tree.NewValue(node, path.NewKeyPath("zero"))

	var b bool

	err := val.Get(&b)
	must.NoError(t, err)
	test.False(t, b)
}

func TestValue_Get_BoolFromOne(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("one"), 1)

	node := root.Get(path.NewKeyPath("one"))
	val := tree.NewValue(node, path.NewKeyPath("one"))

	var b bool

	err := val.Get(&b)
	must.NoError(t, err)
	test.True(t, b)
}

func TestValue_Get_BoolFromNegative(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("neg"), -5)

	node := root.Get(path.NewKeyPath("neg"))
	val := tree.NewValue(node, path.NewKeyPath("neg"))

	var b bool

	err := val.Get(&b)
	must.NoError(t, err)
	test.True(t, b)
}

func TestValue_Get_BoolFromUint(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("uintval"), uint(10))

	node := root.Get(path.NewKeyPath("uintval"))
	val := tree.NewValue(node, path.NewKeyPath("uintval"))

	var b bool

	err := val.Get(&b)
	must.NoError(t, err)
	test.True(t, b)
}

func TestValue_Get_NilSource(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("nilval"), nil)

	node := root.Get(path.NewKeyPath("nilval"))
	val := tree.NewValue(node, path.NewKeyPath("nilval"))

	var s string

	err := val.Get(&s)
	must.NoError(t, err)
	test.Eq(t, "", s)

	var i int

	err = val.Get(&i)
	must.NoError(t, err)
	test.Eq(t, 0, i)

	var p *int

	err = val.Get(&p)
	must.NoError(t, err)
	must.Nil(t, p)
}

func TestValue_Meta(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("test"), "value")

	node := root.Get(path.NewKeyPath("test"))
	must.NotNil(t, node)

	node.Source = "file.yaml"
	node.Revision = "42"

	val := tree.NewValue(node, path.NewKeyPath("test"))
	mi := val.Meta()
	test.Eq(t, path.NewKeyPath("test"), mi.Key)
	test.Eq(t, "file.yaml", mi.Source.Name)
	test.Eq(t, meta.UnknownSource, mi.Source.Type)
	test.Eq(t, "42", mi.Revision)
}

func TestValue_Get_Uint64ToInt64Overflow(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("large"), uint64(1<<63))

	node := root.Get(path.NewKeyPath("large"))
	val := tree.NewValue(node, path.NewKeyPath("large"))

	var i64 int64

	err := val.Get(&i64)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "overflow")
}

func TestValue_Get_UintToIntOverflow(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("large"), uint(1<<63))

	node := root.Get(path.NewKeyPath("large"))
	val := tree.NewValue(node, path.NewKeyPath("large"))

	var i int

	err := val.Get(&i)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "overflow")
}

func TestValue_Get_IntFromBool(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("booltrue"), true)

	node := root.Get(path.NewKeyPath("booltrue"))
	val := tree.NewValue(node, path.NewKeyPath("booltrue"))

	var got int

	err := val.Get(&got)
	must.NoError(t, err)
	test.Eq(t, 1, got)

	root.Set(path.NewKeyPath("boolfalse"), false)

	node = root.Get(path.NewKeyPath("boolfalse"))
	val = tree.NewValue(node, path.NewKeyPath("boolfalse"))

	err = val.Get(&got)
	must.NoError(t, err)
	test.Eq(t, 0, got)
}

func TestValue_Get_Int16Overflow(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("large"), 32768)

	node := root.Get(path.NewKeyPath("large"))
	val := tree.NewValue(node, path.NewKeyPath("large"))

	var i16 int16

	err := val.Get(&i16)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "overflow")

	root.Set(path.NewKeyPath("small"), -32769)

	node = root.Get(path.NewKeyPath("small"))
	val = tree.NewValue(node, path.NewKeyPath("small"))

	err = val.Get(&i16)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "overflow")
}

func TestValue_Get_Int32Overflow(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("large"), int64(2147483648))

	node := root.Get(path.NewKeyPath("large"))
	val := tree.NewValue(node, path.NewKeyPath("large"))

	var i32 int32

	err := val.Get(&i32)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "overflow")

	root.Set(path.NewKeyPath("small"), int64(-2147483649))

	node = root.Get(path.NewKeyPath("small"))
	val = tree.NewValue(node, path.NewKeyPath("small"))

	err = val.Get(&i32)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "overflow")
}

func TestValue_Get_UintFromNegativeInt(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("neg"), -5)

	node := root.Get(path.NewKeyPath("neg"))
	val := tree.NewValue(node, path.NewKeyPath("neg"))

	var u uint

	err := val.Get(&u)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "negative")
}

func TestValue_Get_UintFromNegativeFloat(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("neg"), -3.14)

	node := root.Get(path.NewKeyPath("neg"))
	val := tree.NewValue(node, path.NewKeyPath("neg"))

	var u uint

	err := val.Get(&u)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "negative")
}

func TestValue_Get_UintFromNegativeFloat32(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("neg"), float32(-3.14))

	node := root.Get(path.NewKeyPath("neg"))
	val := tree.NewValue(node, path.NewKeyPath("neg"))

	var u uint

	err := val.Get(&u)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "negative")
}

func TestValue_Get_Uint16Overflow(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("large"), 65536)

	node := root.Get(path.NewKeyPath("large"))
	val := tree.NewValue(node, path.NewKeyPath("large"))

	var u16 uint16

	err := val.Get(&u16)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "overflow")
}

func TestValue_Get_Uint32Overflow(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("large"), uint64(4294967296))

	node := root.Get(path.NewKeyPath("large"))
	val := tree.NewValue(node, path.NewKeyPath("large"))

	var u32 uint32

	err := val.Get(&u32)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "overflow")
}

func TestValue_Get_UintFromBool(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("booltrue"), true)

	node := root.Get(path.NewKeyPath("booltrue"))
	val := tree.NewValue(node, path.NewKeyPath("booltrue"))

	var got uint

	err := val.Get(&got)
	must.NoError(t, err)
	test.Eq(t, uint(1), got)

	root.Set(path.NewKeyPath("boolfalse"), false)

	node = root.Get(path.NewKeyPath("boolfalse"))
	val = tree.NewValue(node, path.NewKeyPath("boolfalse"))

	err = val.Get(&got)
	must.NoError(t, err)
	test.Eq(t, uint(0), got)
}

func TestValue_Get_UintFromInvalidString(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("invalid"), "abc")

	node := root.Get(path.NewKeyPath("invalid"))
	val := tree.NewValue(node, path.NewKeyPath("invalid"))

	var u uint

	err := val.Get(&u)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "convert")
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
			root.Set(path.NewKeyPath("val"), tt.src)

			node := root.Get(path.NewKeyPath("val"))
			val := tree.NewValue(node, path.NewKeyPath("val"))

			var got uint64

			err := val.Get(&got)
			must.NoError(t, err)
			test.Eq(t, tt.want, got)
		})
	}
}

func TestValue_Get_UintFromUnsupportedType(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("val"), []int{1, 2, 3})

	node := root.Get(path.NewKeyPath("val"))
	val := tree.NewValue(node, path.NewKeyPath("val"))

	var u uint

	err := val.Get(&u)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "convert to uint")
}

func TestValue_Get_Float32Overflow(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("huge"), 1e100)

	node := root.Get(path.NewKeyPath("huge"))
	val := tree.NewValue(node, path.NewKeyPath("huge"))

	var f32 float32

	err := val.Get(&f32)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "overflow")

	root.Set(path.NewKeyPath("negHuge"), -1e100)

	node = root.Get(path.NewKeyPath("negHuge"))
	val = tree.NewValue(node, path.NewKeyPath("negHuge"))

	err = val.Get(&f32)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "overflow")
}

func TestValue_Get_FloatFromUnsupportedType(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("unsupported"), []any{1, 2})

	node := root.Get(path.NewKeyPath("unsupported"))
	val := tree.NewValue(node, path.NewKeyPath("unsupported"))

	var f float64

	err := val.Get(&f)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "convert")
}

func TestValue_Get_BoolFromUnsupportedType(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("unsupported"), []any{1, 2})

	node := root.Get(path.NewKeyPath("unsupported"))
	val := tree.NewValue(node, path.NewKeyPath("unsupported"))

	var b bool

	err := val.Get(&b)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "convert")
}

func TestValue_Get_SliceFromNonSliceError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("notslice"), "string")

	node := root.Get(path.NewKeyPath("notslice"))
	val := tree.NewValue(node, path.NewKeyPath("notslice"))

	var s []int

	err := val.Get(&s)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "not a slice")
}

func TestValue_Get_MapKeyNotStringError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("badmap"), map[int]string{1: "foo"})

	node := root.Get(path.NewKeyPath("badmap"))
	val := tree.NewValue(node, path.NewKeyPath("badmap"))

	var m map[string]string

	err := val.Get(&m)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "key is not string")
}

func TestValue_Get_StructSourceNotMapError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("notmap"), "string")

	node := root.Get(path.NewKeyPath("notmap"))
	val := tree.NewValue(node, path.NewKeyPath("notmap"))

	type S struct {
		Field string `yaml:"field"`
	}

	var s S

	err := val.Get(&s)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "must be a map")
}

func TestValue_Get_StructMapKeyNotStringError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("badmap"), map[int]string{1: "foo"})

	node := root.Get(path.NewKeyPath("badmap"))
	val := tree.NewValue(node, path.NewKeyPath("badmap"))

	type S struct {
		Field string `yaml:"field"`
	}

	var s S

	err := val.Get(&s)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "must have string keys")
}

func TestValue_Get_StructFieldDecodeError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("struct/int"), "abc")

	node := root.Get(path.NewKeyPath("struct"))
	val := tree.NewValue(node, path.NewKeyPath("struct"))

	type S struct {
		Int int `yaml:"int"`
	}

	var s S

	err := val.Get(&s)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "convert")
}

func TestValue_Get_StructUnexportedField(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("struct/exported"), "hello")
	root.Set(path.NewKeyPath("struct/unexported"), "world")

	node := root.Get(path.NewKeyPath("struct"))
	val := tree.NewValue(node, path.NewKeyPath("struct"))

	type S struct {
		Exported   string `yaml:"exported"`
		unexported string
	}

	var s S

	err := val.Get(&s)
	must.NoError(t, err)
	test.Eq(t, "hello", s.Exported)
	test.Eq(t, "", s.unexported)
}

func TestValue_Get_StructYamlTagComma(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("struct/myfield"), "value")

	node := root.Get(path.NewKeyPath("struct"))
	val := tree.NewValue(node, path.NewKeyPath("struct"))

	type S struct {
		Field string `yaml:"myfield,omitempty"`
	}

	var s S

	err := val.Get(&s)
	must.NoError(t, err)
	test.Eq(t, "value", s.Field)
}

func TestValue_Get_StructMissingField(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("struct/present"), "hello")

	node := root.Get(path.NewKeyPath("struct"))
	val := tree.NewValue(node, path.NewKeyPath("struct"))

	type S struct {
		Present string `yaml:"present"`
		Missing string `yaml:"missing"`
	}

	var s S

	err := val.Get(&s)
	must.NoError(t, err)
	test.Eq(t, "hello", s.Present)
	test.Eq(t, "", s.Missing)
}

func TestValue_Get_DurationOverflowInt64(t *testing.T) {
	t.Parallel()

	maxSafeSeconds := math.MaxInt64 / int64(time.Second)
	root := tree.New()
	root.Set(path.NewKeyPath("large"), maxSafeSeconds+1)

	node := root.Get(path.NewKeyPath("large"))
	val := tree.NewValue(node, path.NewKeyPath("large"))

	var d time.Duration

	err := val.Get(&d)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "overflow")
}

func TestValue_Get_DurationOverflowUint64(t *testing.T) {
	t.Parallel()

	maxSafeUintSeconds := uint64(math.MaxInt64 / int64(time.Second))
	root := tree.New()
	root.Set(path.NewKeyPath("large"), maxSafeUintSeconds+1)

	node := root.Get(path.NewKeyPath("large"))
	val := tree.NewValue(node, path.NewKeyPath("large"))

	var d time.Duration

	err := val.Get(&d)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "overflow")
}

func TestValue_Get_UnsupportedDestinationType(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("val"), 123)

	node := root.Get(path.NewKeyPath("val"))
	val := tree.NewValue(node, path.NewKeyPath("val"))

	var c complex64

	err := val.Get(&c)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "unsupported")
}

func TestValue_Get_InterfaceImplemented(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("stringer"), testStringer{})

	node := root.Get(path.NewKeyPath("stringer"))
	val := tree.NewValue(node, path.NewKeyPath("stringer"))

	var iface fmt.Stringer

	err := val.Get(&iface)
	must.NoError(t, err)
	test.Eq(t, "I am a Stringer", iface.String())
}

func TestValue_Get_InterfaceEmpty(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("val"), 42)

	node := root.Get(path.NewKeyPath("val"))
	val := tree.NewValue(node, path.NewKeyPath("val"))

	var iface any

	err := val.Get(&iface)
	must.NoError(t, err)
	test.Eq(t, 42, iface)
}

func TestValue_Get_IntFromUint(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("val"), uint(42))

	node := root.Get(path.NewKeyPath("val"))
	val := tree.NewValue(node, path.NewKeyPath("val"))

	var i int

	err := val.Get(&i)
	must.NoError(t, err)
	test.Eq(t, 42, i)
}

func TestValue_Get_Int64FromUint64(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("val"), uint64(1234567890))

	node := root.Get(path.NewKeyPath("val"))
	val := tree.NewValue(node, path.NewKeyPath("val"))

	var i64 int64

	err := val.Get(&i64)
	must.NoError(t, err)
	test.Eq(t, int64(1234567890), i64)
}

func TestValue_Get_IntFromFloat(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("val"), 3.0)

	node := root.Get(path.NewKeyPath("val"))
	val := tree.NewValue(node, path.NewKeyPath("val"))

	var i int

	err := val.Get(&i)
	must.NoError(t, err)
	test.Eq(t, 3, i)
}

func TestValue_Get_UintFromFloat(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("val"), 5.0)

	node := root.Get(path.NewKeyPath("val"))
	val := tree.NewValue(node, path.NewKeyPath("val"))

	var u uint

	err := val.Get(&u)
	must.NoError(t, err)
	test.Eq(t, uint(5), u)
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
			root.Set(path.NewKeyPath("val"), tt.src)

			node := root.Get(path.NewKeyPath("val"))
			val := tree.NewValue(node, path.NewKeyPath("val"))

			var got int64

			err := val.Get(&got)
			must.NoError(t, err)
			test.Eq(t, tt.want, got)
		})
	}
}

func TestValue_Get_IntFromUnsupportedType(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("val"), []int{1, 2, 3})

	node := root.Get(path.NewKeyPath("val"))
	val := tree.NewValue(node, path.NewKeyPath("val"))

	var i int

	err := val.Get(&i)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "convert to int")
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
			root.Set(path.NewKeyPath("val"), tt.val)

			node := root.Get(path.NewKeyPath("val"))
			val := tree.NewValue(node, path.NewKeyPath("val"))

			var b bool

			err := val.Get(&b)
			must.NoError(t, err)
			test.Eq(t, tt.want, b)
		})
	}
}

func TestValue_Get_SliceElementConversionError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("slice"), []any{1, "abc", 3})

	node := root.Get(path.NewKeyPath("slice"))
	val := tree.NewValue(node, path.NewKeyPath("slice"))

	var s []int

	err := val.Get(&s)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "slice element")
}

func TestValue_Get_UnsupportedDestinationKinds(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("val"), 123)

	node := root.Get(path.NewKeyPath("val"))
	val := tree.NewValue(node, path.NewKeyPath("val"))

	var arr [1]int

	err := val.Get(&arr)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "unsupported")

	var ch chan int

	err = val.Get(&ch)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "unsupported")

	var fn func()

	err = val.Get(&fn)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "unsupported")

	var up unsafe.Pointer

	err = val.Get(&up)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "unsupported")

	var ui uintptr

	err = val.Get(&ui)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "unsupported")

	var c64 complex64

	err = val.Get(&c64)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "unsupported")

	var c128 complex128

	err = val.Get(&c128)
	must.Error(t, err)
	test.StrContains(t, err.Error(), "unsupported")
}
