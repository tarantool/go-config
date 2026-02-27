package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
)

func TestConfigBuilder_PortOverride(t *testing.T) {
	t.Parallel()

	map1 := map[string]any{
		"server": map[string]any{
			"port":    8080,
			"timeout": "30s",
		},
	}
	map2 := map[string]any{
		"server": map[string]any{
			"port": 9090,
		},
		"log": map[string]any{
			"level": "debug",
		},
	}

	col1 := collectors.NewMap(map1).WithName("map1")
	col2 := collectors.NewMap(map2).WithName("map2")

	builder := config.NewBuilder()

	builder = builder.AddCollector(col1)
	builder = builder.AddCollector(col2)

	cfg, errs := builder.Build()
	require.Empty(t, errs)

	var port int

	meta, err := cfg.Get(config.NewKeyPath("server/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, 9090, port)
	assert.Equal(t, "map2", meta.Source.Name)
}

func TestConfigBuilder_TimeoutFromFirstCollector(t *testing.T) {
	t.Parallel()

	map1 := map[string]any{
		"server": map[string]any{
			"port":    8080,
			"timeout": "30s",
		},
	}
	map2 := map[string]any{
		"server": map[string]any{
			"port": 9090,
		},
		"log": map[string]any{
			"level": "debug",
		},
	}

	col1 := collectors.NewMap(map1).WithName("map1")
	col2 := collectors.NewMap(map2).WithName("map2")

	builder := config.NewBuilder()

	builder = builder.AddCollector(col1)
	builder = builder.AddCollector(col2)

	cfg, errs := builder.Build()
	require.Empty(t, errs)

	var timeout string

	meta, err := cfg.Get(config.NewKeyPath("server/timeout"), &timeout)
	require.NoError(t, err)
	assert.Equal(t, "30s", timeout)
	assert.Equal(t, "map1", meta.Source.Name)
}

func TestConfigBuilder_LogLevelFromSecondCollector(t *testing.T) {
	t.Parallel()

	map1 := map[string]any{
		"server": map[string]any{
			"port":    8080,
			"timeout": "30s",
		},
	}
	map2 := map[string]any{
		"server": map[string]any{
			"port": 9090,
		},
		"log": map[string]any{
			"level": "debug",
		},
	}

	col1 := collectors.NewMap(map1).WithName("map1")
	col2 := collectors.NewMap(map2).WithName("map2")

	builder := config.NewBuilder()

	builder = builder.AddCollector(col1)
	builder = builder.AddCollector(col2)

	cfg, errs := builder.Build()
	require.Empty(t, errs)

	var level string

	meta, err := cfg.Get(config.NewKeyPath("log/level"), &level)
	require.NoError(t, err)
	assert.Equal(t, "debug", level)
	assert.Equal(t, "map2", meta.Source.Name)
}

func TestConfigBuilder_Lookup_ExistingKey(t *testing.T) {
	t.Parallel()

	m := map[string]any{
		"foo": "bar",
	}
	col := collectors.NewMap(m)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	require.Empty(t, errs)

	val, ok := cfg.Lookup(config.NewKeyPath("foo"))
	require.True(t, ok)

	var s string
	require.NoError(t, val.Get(&s))
	assert.Equal(t, "bar", s)
}

func TestConfigBuilder_Lookup_NonExistentKey(t *testing.T) {
	t.Parallel()

	m := map[string]any{
		"foo": "bar",
	}
	col := collectors.NewMap(m)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	require.Empty(t, errs)

	_, ok := cfg.Lookup(config.NewKeyPath("nonexistent"))
	assert.False(t, ok)
}
