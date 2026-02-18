package config_test

import (
	"context"
	"testing"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
)

func TestConfig_Stat_ExistingKey(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"server": map[string]any{
			"port": 8080,
		},
	}
	col := collectors.NewMap(data).WithName("test")

	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	must.SliceEmpty(t, errs)

	meta, ok := cfg.Stat(config.NewKeyPath("server/port"))
	must.True(t, ok)
	test.Eq(t, "test", meta.Source.Name)
	test.Eq(t, config.UnknownSource, meta.Source.Type)
}

func TestConfig_Stat_NonExistentKey(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	must.SliceEmpty(t, errs)

	meta, ok := cfg.Stat(config.NewKeyPath("missing"))
	test.False(t, ok)
	test.Eq(t, "", meta.Source.Name)
	test.Eq(t, config.UnknownSource, meta.Source.Type)
}

func TestConfig_Stat_NestedKey(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": "value",
			},
		},
	}
	col := collectors.NewMap(data).WithName("nested")
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	must.SliceEmpty(t, errs)

	meta, ok := cfg.Stat(config.NewKeyPath("a/b/c"))
	must.True(t, ok)
	test.Eq(t, "nested", meta.Source.Name)
}

func TestConfig_Slice_Root(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"key": "value",
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	must.SliceEmpty(t, errs)

	sliced, err := cfg.Slice(config.NewKeyPath(""))
	must.NoError(t, err)

	var val string

	_, err = sliced.Get(config.NewKeyPath("key"), &val)
	must.NoError(t, err)
	test.Eq(t, "value", val)
}

func TestConfig_Slice_ValidPath(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"parent": map[string]any{
			"child": map[string]any{
				"grandchild": 42,
			},
		},
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	must.SliceEmpty(t, errs)

	sliced, err := cfg.Slice(config.NewKeyPath("parent/child"))
	must.NoError(t, err)

	var val int

	_, err = sliced.Get(config.NewKeyPath("grandchild"), &val)
	must.NoError(t, err)
	test.Eq(t, 42, val)
}

func TestConfig_Slice_NonExistentPath(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"foo": "bar",
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	must.SliceEmpty(t, errs)

	_, err := cfg.Slice(config.NewKeyPath("nonexistent"))
	test.Error(t, err)
}

func TestConfig_Walk_RootAllValues(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	data := map[string]any{
		"a": 1,
		"b": 2,
		"c": map[string]any{
			"d": 3,
		},
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	must.SliceEmpty(t, errs)

	ch, err := cfg.Walk(ctx, config.NewKeyPath(""), -1)
	must.NoError(t, err)

	count := 0
	for v := range ch {
		count++

		var dest any

		err := v.Get(&dest)
		must.NoError(t, err)
	}

	test.Eq(t, 3, count)
}

func TestConfig_Walk_WithDepthLimit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	data := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": 1,
			},
		},
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	must.SliceEmpty(t, errs)

	ch, err := cfg.Walk(ctx, config.NewKeyPath(""), 1)
	must.NoError(t, err)

	count := 0
	for range ch {
		count++
	}

	test.Eq(t, 0, count)
}

func TestConfig_Walk_FromSubPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	data := map[string]any{
		"parent": map[string]any{
			"child1": 1,
			"child2": 2,
		},
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	must.SliceEmpty(t, errs)

	ch, err := cfg.Walk(ctx, config.NewKeyPath("parent"), -1)
	must.NoError(t, err)

	count := 0
	for v := range ch {
		count++

		var dest any

		err := v.Get(&dest)
		must.NoError(t, err)
	}

	test.Eq(t, 2, count)
}

func TestConfig_String_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	must.SliceEmpty(t, errs)

	test.Eq(t, "", cfg.String())
}

func TestConfig_MarshalYAML_ReturnsNil(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	must.SliceEmpty(t, errs)

	bytes, err := cfg.MarshalYAML()
	must.NoError(t, err)
	test.Nil(t, bytes)
}

func TestMutableConfig_Set_NotImplemented(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable()
	must.SliceEmpty(t, errs)

	err := cfg.Set(config.NewKeyPath("key"), "value")
	must.NoError(t, err)
}

func TestMutableConfig_Merge_NotImplemented(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable()
	must.SliceEmpty(t, errs)

	err := cfg.Merge(&cfg.Config)
	must.NoError(t, err)
}

func TestMutableConfig_Update_NotImplemented(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable()
	must.SliceEmpty(t, errs)

	err := cfg.Update(&cfg.Config)
	must.NoError(t, err)
}

func TestMutableConfig_Delete_NotImplemented(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable()
	must.SliceEmpty(t, errs)

	deleted := cfg.Delete(config.NewKeyPath("key"))
	test.False(t, deleted)
}
