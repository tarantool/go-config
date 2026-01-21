package omap_test

import (
	"testing"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"

	"github.com/tarantool/go-config/internal/omap"
	"github.com/tarantool/go-config/internal/testutil"
)

func TestOrderedMap_Set_Get_single(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("a", 1)

	val, ok := m.Get("a")
	must.True(t, ok)
	test.Eq(t, 1, val)
}

func TestOrderedMap_Set_Get_multiple(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	val, ok := m.Get("a")
	must.True(t, ok)
	test.Eq(t, 1, val)

	val, ok = m.Get("b")
	must.True(t, ok)
	test.Eq(t, 2, val)

	val, ok = m.Get("c")
	must.True(t, ok)
	test.Eq(t, 3, val)
}

func TestOrderedMap_Set_Get_missing(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	_, ok := m.Get("missing")
	test.False(t, ok)
}

func TestOrderedMap_Set_Overwrite_value(t *testing.T) {
	t.Parallel()

	m := omap.New[string, string]()
	m.Set("key", "first")

	val, ok := m.Get("key")
	must.True(t, ok)
	test.Eq(t, "first", val)

	m.Set("key", "second")

	val, ok = m.Get("key")
	must.True(t, ok)
	test.Eq(t, "second", val)
}

func TestOrderedMap_Set_Overwrite_order(t *testing.T) {
	t.Parallel()

	m := omap.New[string, string]()
	m.Set("key", "first")
	m.Set("key", "second")

	testutil.TestIterSeqCompare(t, []string{"key"}, m.Keys())
}

func TestOrderedMap_Delete_existing(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	ok := m.Delete("b")
	must.True(t, ok)
	testutil.TestIterSeqLen(t, 2, m.Keys())

	_, ok = m.Get("b")
	test.False(t, ok)

	// Order should be a, c.
	testutil.TestIterSeqCompare(t, []string{"a", "c"}, m.Keys())
}

func TestOrderedMap_Delete_missing(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	ok := m.Delete("missing")
	test.False(t, ok)
}

func TestOrderedMap_Len_initial(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	test.Length(t, 0, m)
}

func TestOrderedMap_Len_afterSet(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("a", 1)
	test.Length(t, 1, m)
}

func TestOrderedMap_Len_afterMultipleSet(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	test.Length(t, 2, m)
}

func TestOrderedMap_Len_afterDelete(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Delete("a")
	test.Length(t, 1, m)
}

func TestOrderedMap_Len_afterClear(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Clear()
	test.Length(t, 0, m)
}

func TestOrderedMap_Keys_empty(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	testutil.TestIterSeqEmpty(t, m.Keys())
}

func TestOrderedMap_Keys_single(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("x", 10)
	testutil.TestIterSeqCompare(t, []string{"x"}, m.Keys())
}

func TestOrderedMap_Keys_multiple(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("x", 10)
	m.Set("y", 20)
	m.Set("z", 30)

	testutil.TestIterSeqCompare(t, []string{"x", "y", "z"}, m.Keys())
}

func TestOrderedMap_Values_empty(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	testutil.TestIterSeqEmpty(t, m.Values())
}

func TestOrderedMap_Values_single(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("a", 100)
	testutil.TestIterSeqCompare(t, []int{100}, m.Values())
}

func TestOrderedMap_Values_multiple(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("a", 100)
	m.Set("b", 200)
	m.Set("c", 300)

	testutil.TestIterSeqCompare(t, []int{100, 200, 300}, m.Values())
}

func TestOrderedMap_Items_empty(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	testutil.TestIterSeq2Empty(t, m.Items())
}

func TestOrderedMap_Items_single(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("first", 1)

	expected := []testutil.TestIterSeq2Pair[string, int]{
		{Key: "first", Value: 1},
	}
	testutil.TestIterSeq2Compare(t, expected, m.Items())
}

func TestOrderedMap_Items_multiple(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("first", 1)
	m.Set("second", 2)

	expected := []testutil.TestIterSeq2Pair[string, int]{
		{Key: "first", Value: 1}, {Key: "second", Value: 2},
	}
	testutil.TestIterSeq2Compare(t, expected, m.Items())
}

func TestOrderedMap_Clear_empties(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)

	m.Clear()
	test.Length(t, 0, m)
	testutil.TestIterSeqEmpty(t, m.Keys())
	testutil.TestIterSeqEmpty(t, m.Values())
}

func TestOrderedMap_Clear_canSetAgain(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)

	m.Clear()
	m.Set("c", 3)
	test.Length(t, 1, m)
	testutil.TestIterSeqCompare(t, []string{"c"}, m.Keys())
}

func TestOrderedMap_NewWithCapacity_initial(t *testing.T) {
	t.Parallel()

	m := omap.NewWithCapacity[string, int](10)
	test.Length(t, 0, m)
}

func TestOrderedMap_NewWithCapacity_afterSet(t *testing.T) {
	t.Parallel()

	m := omap.NewWithCapacity[string, int](10)
	m.Set("a", 1)
	m.Set("b", 2)
	test.Length(t, 2, m)
}

func TestOrderedMap_NewWithCapacity_zero(t *testing.T) {
	t.Parallel()

	m := omap.NewWithCapacity[string, int](0)
	test.Length(t, 0, m)
	m.Set("a", 1)
	test.Length(t, 1, m)
}

func TestOrderedMap_OrderPreservation_SetExisting(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("z", 1)
	m.Set("y", 2)
	m.Set("x", 3)

	// Insert existing key, order unchanged.
	m.Set("y", 99)

	testutil.TestIterSeqCompare(t, []string{"z", "y", "x"}, m.Keys())

	val, ok := m.Get("y")
	must.True(t, ok)
	test.Eq(t, 99, val)
}

func TestOrderedMap_OrderPreservation_DeleteMiddle(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("z", 1)
	m.Set("y", 2)
	m.Set("x", 3)

	m.Delete("y")
	testutil.TestIterSeqCompare(t, []string{"z", "x"}, m.Keys())
}

func TestOrderedMap_OrderPreservation_AddNew(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("z", 1)
	m.Set("y", 2)
	m.Set("x", 3)

	m.Delete("y")
	m.Set("w", 4)
	testutil.TestIterSeqCompare(t, []string{"z", "x", "w"}, m.Keys())
}

func TestOrderedMap_OrderPreservation_DeleteFirst(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("z", 1)
	m.Set("y", 2)
	m.Set("x", 3)

	m.Delete("z")
	testutil.TestIterSeqCompare(t, []string{"y", "x"}, m.Keys())
}

func TestOrderedMap_OrderPreservation_DeleteLast(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("z", 1)
	m.Set("y", 2)
	m.Set("x", 3)

	m.Delete("x")
	testutil.TestIterSeqCompare(t, []string{"z", "y"}, m.Keys())
}

func TestOrderedMap_OrderPreservation_DeleteAllThenAdd(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("z", 1)
	m.Set("y", 2)
	m.Set("x", 3)

	m.Delete("z")
	m.Delete("y")
	m.Delete("x")
	testutil.TestIterSeqEmpty(t, m.Keys())

	m.Set("a", 4)
	m.Set("b", 5)
	m.Set("c", 6)
	testutil.TestIterSeqCompare(t, []string{"a", "b", "c"}, m.Keys())
}

func TestOrderedMap_Has_true(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("a", 1)

	must.True(t, m.Has("a"))
}

func TestOrderedMap_Has_false(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	m.Set("a", 1)

	test.False(t, m.Has("b"))
}

func TestOrderedMap_EmptyMapBehavior_GetMissing(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	_, ok := m.Get("anything")
	test.False(t, ok)
}

func TestOrderedMap_EmptyMapBehavior_HasMissing(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	test.False(t, m.Has("anything"))
}

func TestOrderedMap_EmptyMapBehavior_DeleteMissing(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	test.False(t, m.Delete("anything"))
}

func TestOrderedMap_EmptyMapBehavior_Keys(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	testutil.TestIterSeqEmpty(t, m.Keys())
}

func TestOrderedMap_EmptyMapBehavior_Values(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	testutil.TestIterSeqEmpty(t, m.Values())
}

func TestOrderedMap_EmptyMapBehavior_Items(t *testing.T) {
	t.Parallel()

	m := omap.New[string, int]()
	testutil.TestIterSeq2Empty(t, m.Items())
}
