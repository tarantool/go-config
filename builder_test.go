package config_test

import (
	"testing"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
)

func TestConfigBuilder_PortOverride(t *testing.T) {
	t.Parallel()

	// Create two map collectors with overlapping keys.
	map1 := map[string]any{
		"server": map[string]any{
			"port":    8080,
			"timeout": "30s",
		},
	}
	map2 := map[string]any{
		"server": map[string]any{
			"port": 9090, // Overrides port.
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
	must.SliceEmpty(t, errs)

	// Check that port is overridden by later collector.
	var port int

	meta, err := cfg.Get(config.NewKeyPath("server/port"), &port)
	must.NoError(t, err)
	test.Eq(t, 9090, port)
	test.Eq(t, "map2", meta.Source.Name)
}

func TestConfigBuilder_TimeoutFromFirstCollector(t *testing.T) {
	t.Parallel()

	// Create two map collectors with overlapping keys.
	map1 := map[string]any{
		"server": map[string]any{
			"port":    8080,
			"timeout": "30s",
		},
	}
	map2 := map[string]any{
		"server": map[string]any{
			"port": 9090, // Overrides port.
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
	must.SliceEmpty(t, errs)

	// Check timeout from first collector.
	var timeout string

	meta, err := cfg.Get(config.NewKeyPath("server/timeout"), &timeout)
	must.NoError(t, err)
	test.Eq(t, "30s", timeout)
	test.Eq(t, "map1", meta.Source.Name)
}

func TestConfigBuilder_LogLevelFromSecondCollector(t *testing.T) {
	t.Parallel()

	// Create two map collectors with overlapping keys.
	map1 := map[string]any{
		"server": map[string]any{
			"port":    8080,
			"timeout": "30s",
		},
	}
	map2 := map[string]any{
		"server": map[string]any{
			"port": 9090, // Overrides port.
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
	must.SliceEmpty(t, errs)

	// Check log level from second collector.
	var level string

	meta, err := cfg.Get(config.NewKeyPath("log/level"), &level)
	must.NoError(t, err)
	test.Eq(t, "debug", level)
	test.Eq(t, "map2", meta.Source.Name)
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
	must.SliceEmpty(t, errs)

	val, ok := cfg.Lookup(config.NewKeyPath("foo"))
	must.True(t, ok)

	var s string
	must.NoError(t, val.Get(&s))
	test.Eq(t, "bar", s)
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
	must.SliceEmpty(t, errs)

	// Non-existent key.
	_, ok := cfg.Lookup(config.NewKeyPath("nonexistent"))
	must.False(t, ok)
}
