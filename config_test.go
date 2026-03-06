package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	meta, ok := cfg.Stat(config.NewKeyPath("server/port"))
	require.True(t, ok)
	assert.Equal(t, "test", meta.Source.Name)
	assert.Equal(t, config.UnknownSource, meta.Source.Type)
}

func TestConfig_Stat_NonExistentKey(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	meta, ok := cfg.Stat(config.NewKeyPath("missing"))
	assert.False(t, ok)
	assert.Empty(t, meta.Source.Name)
	assert.Equal(t, config.UnknownSource, meta.Source.Type)
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

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	meta, ok := cfg.Stat(config.NewKeyPath("a/b/c"))
	require.True(t, ok)
	assert.Equal(t, "nested", meta.Source.Name)
}

func TestConfig_Slice_Root(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"key": "value",
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	sliced, err := cfg.Slice(config.NewKeyPath(""))
	require.NoError(t, err)

	var val string

	_, err = sliced.Get(config.NewKeyPath("key"), &val)
	require.NoError(t, err)
	assert.Equal(t, "value", val)
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

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	sliced, err := cfg.Slice(config.NewKeyPath("parent/child"))
	require.NoError(t, err)

	var val int

	_, err = sliced.Get(config.NewKeyPath("grandchild"), &val)
	require.NoError(t, err)
	assert.Equal(t, 42, val)
}

func TestConfig_Slice_NonExistentPath(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"foo": "bar",
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	_, err := cfg.Slice(config.NewKeyPath("nonexistent"))
	assert.Error(t, err)
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

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	ch, err := cfg.Walk(ctx, config.NewKeyPath(""), -1)
	require.NoError(t, err)

	count := 0
	for v := range ch {
		count++

		var dest any

		err := v.Get(&dest)
		require.NoError(t, err)
	}

	assert.Equal(t, 3, count)
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

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	ch, err := cfg.Walk(ctx, config.NewKeyPath(""), 1)
	require.NoError(t, err)

	count := 0
	for range ch {
		count++
	}

	assert.Equal(t, 0, count)
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

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	ch, err := cfg.Walk(ctx, config.NewKeyPath("parent"), -1)
	require.NoError(t, err)

	count := 0
	for v := range ch {
		count++

		var dest any

		err := v.Get(&dest)
		require.NoError(t, err)
	}

	assert.Equal(t, 2, count)
}

func TestConfig_String_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	assert.Empty(t, cfg.String())
}

func TestConfig_MarshalYAML_ReturnsNil(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	bytes, err := cfg.MarshalYAML()
	require.NoError(t, err)
	assert.Nil(t, bytes)
}

func TestMutableConfig_Set(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "original"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	err := cfg.Set(config.NewKeyPath("key"), "updated")
	require.NoError(t, err)

	var val string

	meta, err := cfg.Get(config.NewKeyPath("key"), &val)
	require.NoError(t, err)
	assert.Equal(t, "updated", val)
	assert.Equal(t, "modified", meta.Source.Name)
	assert.Equal(t, "1", string(meta.Revision))
}

func TestMutableConfig_Set_RevisionIncrement(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "v1"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	err := cfg.Set(config.NewKeyPath("key"), "v2")
	require.NoError(t, err)

	meta, _ := cfg.Stat(config.NewKeyPath("key"))
	assert.Equal(t, "1", string(meta.Revision))

	err = cfg.Set(config.NewKeyPath("key"), "v3")
	require.NoError(t, err)

	meta, _ = cfg.Stat(config.NewKeyPath("key"))
	assert.Equal(t, "2", string(meta.Revision))
}

func TestMutableConfig_Set_NewKey(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	err := cfg.Set(config.NewKeyPath("newkey"), "value")
	require.NoError(t, err)

	var val string

	_, err = cfg.Get(config.NewKeyPath("newkey"), &val)
	require.NoError(t, err)
	assert.Equal(t, "value", val)
}

func TestMutableConfig_Set_ValidationFailure_Reverts(t *testing.T) {
	t.Parallel()

	v := &valueChangeValidator{}
	data := map[string]any{"key": "original"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.WithValidator(v)
	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	err := cfg.Set(config.NewKeyPath("key"), "bad")
	require.Error(t, err)

	// Value should be reverted to original.
	var val string

	_, err = cfg.Get(config.NewKeyPath("key"), &val)
	require.NoError(t, err)
	assert.Equal(t, "original", val)
}

func TestMutableConfig_Set_ValidationFailure_RevertsNewPath(t *testing.T) {
	t.Parallel()

	v := &maxKeysValidator{maxKeys: 1}
	data := map[string]any{"key": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.WithValidator(v)
	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	err := cfg.Set(config.NewKeyPath("secondkey"), "value")
	require.Error(t, err)

	// New key should not exist after revert.
	_, ok := cfg.Lookup(config.NewKeyPath("secondkey"))
	assert.False(t, ok)
}

func TestMutableConfig_Merge(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	err := cfg.Merge(&cfg.Config)
	require.NoError(t, err)
}

func TestMutableConfig_Merge_Metadata(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	otherData := map[string]any{"key": "newval", "extra": "added"}
	col2 := collectors.NewMap(otherData)
	builder2 := config.NewBuilder()

	builder2 = builder2.AddCollector(col2)

	otherCfg, errs2 := builder2.Build(t.Context())
	require.Empty(t, errs2)

	err := cfg.Merge(&otherCfg)
	require.NoError(t, err)

	meta, ok := cfg.Stat(config.NewKeyPath("key"))
	assert.True(t, ok)
	assert.Equal(t, "modified", meta.Source.Name)

	meta, ok = cfg.Stat(config.NewKeyPath("extra"))
	assert.True(t, ok)
	assert.Equal(t, "modified", meta.Source.Name)
}

func TestMutableConfig_Merge_ValidationFailure_Reverts(t *testing.T) {
	t.Parallel()

	v := &maxKeysValidator{maxKeys: 1}
	data := map[string]any{"key": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.WithValidator(v)
	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	otherData := map[string]any{"newkey": "newvalue"}
	col2 := collectors.NewMap(otherData)
	builder2 := config.NewBuilder()

	builder2 = builder2.AddCollector(col2)

	otherCfg, errs2 := builder2.Build(t.Context())
	require.Empty(t, errs2)

	err := cfg.Merge(&otherCfg)
	require.Error(t, err)

	// newkey should not exist after revert.
	_, ok := cfg.Lookup(config.NewKeyPath("newkey"))
	assert.False(t, ok)
}

func TestMutableConfig_Update(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	err := cfg.Update(&cfg.Config)
	require.NoError(t, err)
}

func TestMutableConfig_Update_Metadata(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	otherData := map[string]any{"key": "updated"}
	col2 := collectors.NewMap(otherData)
	builder2 := config.NewBuilder()

	builder2 = builder2.AddCollector(col2)

	otherCfg, errs2 := builder2.Build(t.Context())
	require.Empty(t, errs2)

	err := cfg.Update(&otherCfg)
	require.NoError(t, err)

	meta, ok := cfg.Stat(config.NewKeyPath("key"))
	assert.True(t, ok)
	assert.Equal(t, "modified", meta.Source.Name)
}

func TestMutableConfig_Update_ValidationFailure_Reverts(t *testing.T) {
	t.Parallel()

	v := &valueChangeValidator{}
	data := map[string]any{"key": "original"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.WithValidator(v)
	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	otherData := map[string]any{"key": "newvalue"}
	col2 := collectors.NewMap(otherData)
	builder2 := config.NewBuilder()

	builder2 = builder2.AddCollector(col2)

	otherCfg, errs2 := builder2.Build(t.Context())
	require.Empty(t, errs2)

	err := cfg.Update(&otherCfg)
	require.Error(t, err)

	// Value should be reverted to original.
	var val string

	_, err = cfg.Get(config.NewKeyPath("key"), &val)
	require.NoError(t, err)
	assert.Equal(t, "original", val)
}

func TestMutableConfig_Delete_ExistingKey(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "value", "other": "data"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	deleted := cfg.Delete(config.NewKeyPath("key"))
	assert.True(t, deleted)

	_, ok := cfg.Lookup(config.NewKeyPath("key"))
	assert.False(t, ok)

	// Other key should still exist.
	_, ok = cfg.Lookup(config.NewKeyPath("other"))
	assert.True(t, ok)
}

func TestMutableConfig_Delete_MissingKey(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	deleted := cfg.Delete(config.NewKeyPath("nonexistent"))
	assert.False(t, deleted)
}

func TestMutableConfig_Delete_EmptyPath(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	deleted := cfg.Delete(config.NewKeyPath(""))
	assert.False(t, deleted)
}

func TestMutableConfig_Delete_NestedKey(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"server": map[string]any{
			"host": "localhost",
			"port": 8080,
		},
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	deleted := cfg.Delete(config.NewKeyPath("server/host"))
	assert.True(t, deleted)

	_, ok := cfg.Lookup(config.NewKeyPath("server/host"))
	assert.False(t, ok)

	// server/port should still exist.
	var port int

	_, err := cfg.Get(config.NewKeyPath("server/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, 8080, port)
}

func TestMutableConfig_Delete_ValidationFailure_Reverts(t *testing.T) {
	t.Parallel()

	// minKeysValidator requires at least 2 keys. Start with 2 keys (build passes).
	// Deleting one would drop to 1 key, failing validation, so tree should be restored.
	v := &minKeysValidator{minKeys: 2}
	data := map[string]any{"key": "value", "other": "data"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.WithValidator(v)
	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	deleted := cfg.Delete(config.NewKeyPath("key"))
	assert.False(t, deleted)

	// Key should still exist after revert.
	_, ok := cfg.Lookup(config.NewKeyPath("key"))
	assert.True(t, ok)
}
