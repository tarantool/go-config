package config_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/tree"
)

// countingMerger counts the number of MergeValue calls.
type countingMerger struct {
	count int
}

type countingContext struct {
	collector    config.Collector
	merger       *countingMerger
	parentOrders map[string][]string
}

func (c *countingMerger) CreateContext(col config.Collector) config.MergerContext {
	ctx := &countingContext{
		collector:    col,
		merger:       c,
		parentOrders: nil,
	}
	if col.KeepOrder() {
		ctx.parentOrders = make(map[string][]string)
	}

	return ctx
}

func (c *countingMerger) MergeValue(ctx config.MergerContext, root *tree.Node, path config.KeyPath, value any) error {
	c.count++
	// Delegate to default merger.
	return config.Default.MergeValue(ctx, root, path, value)
}

func (c *countingContext) Collector() config.Collector { return c.collector }

func (c *countingContext) RecordOrdering(parent config.KeyPath, child string) {
	if c.parentOrders == nil {
		return
	}

	parentKey := parent.String()

	keys := c.parentOrders[parentKey]
	if !slices.Contains(keys, child) {
		c.parentOrders[parentKey] = append(keys, child)
	}
}

func (c *countingContext) ApplyOrdering(root *tree.Node) error {
	if c.parentOrders == nil {
		return nil
	}

	for parentKey, orderedKeys := range c.parentOrders {
		var parentNode *tree.Node
		if parentKey == "" {
			parentNode = root
		} else {
			parentNode = root.Get(config.NewKeyPath(parentKey))
		}

		if parentNode == nil {
			continue
		}

		if parentNode.OrderSet() {
			continue
		}

		_ = parentNode.ReorderChildren(orderedKeys)
		parentNode.SetOrderSet(true)
	}

	return nil
}

func TestCustomMerger(t *testing.T) {
	t.Parallel()

	merger := &countingMerger{count: 0}

	builder := config.NewBuilder()

	builder = builder.WithMerger(merger)

	col := collectors.NewMap(map[string]any{
		"port": 8080,
		"host": "localhost",
	}).WithName("test")

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	require.Empty(t, errs)

	// Verify merger was called for each key.
	assert.Equal(t, 2, merger.count)

	// Verify merging actually worked.
	var port int

	_, err := cfg.Get(config.NewKeyPath("port"), &port)
	require.NoError(t, err)
	assert.Equal(t, 8080, port)

	var host string

	_, err = cfg.Get(config.NewKeyPath("host"), &host)
	require.NoError(t, err)
	assert.Equal(t, "localhost", host)
}

// selectiveMerger only merges specific paths.
type selectiveMerger struct {
	allowedPrefix string
}

type selectiveContext struct {
	collector    config.Collector
	merger       *selectiveMerger
	parentOrders map[string][]string
}

func (s *selectiveMerger) CreateContext(col config.Collector) config.MergerContext {
	ctx := &selectiveContext{
		collector:    col,
		merger:       s,
		parentOrders: nil,
	}
	if col.KeepOrder() {
		ctx.parentOrders = make(map[string][]string)
	}

	return ctx
}

func (s *selectiveMerger) MergeValue(ctx config.MergerContext, root *tree.Node, path config.KeyPath, value any) error {
	// Only merge paths under the allowed prefix.
	if path.String() != s.allowedPrefix && !path.Match(config.NewKeyPath(s.allowedPrefix+"/*")) {
		// Skip merging.
		return nil
	}

	return config.Default.MergeValue(ctx, root, path, value)
}

func (c *selectiveContext) Collector() config.Collector { return c.collector }

func (c *selectiveContext) RecordOrdering(parent config.KeyPath, child string) {
	if c.parentOrders == nil {
		return
	}

	parentKey := parent.String()

	keys := c.parentOrders[parentKey]
	if !slices.Contains(keys, child) {
		c.parentOrders[parentKey] = append(keys, child)
	}
}

func (c *selectiveContext) ApplyOrdering(root *tree.Node) error {
	if c.parentOrders == nil {
		return nil
	}

	for parentKey, orderedKeys := range c.parentOrders {
		var parentNode *tree.Node
		if parentKey == "" {
			parentNode = root
		} else {
			parentNode = root.Get(config.NewKeyPath(parentKey))
		}

		if parentNode == nil {
			continue
		}

		if parentNode.OrderSet() {
			continue
		}

		_ = parentNode.ReorderChildren(orderedKeys)
		parentNode.SetOrderSet(true)
	}

	return nil
}

func TestSelectiveMerger(t *testing.T) {
	t.Parallel()

	merger := &selectiveMerger{allowedPrefix: "server"}

	builder := config.NewBuilder()

	builder = builder.WithMerger(merger)

	col := collectors.NewMap(map[string]any{
		"server": map[string]any{
			"port": 8080,
			"host": "localhost",
		},
		"client": map[string]any{
			"timeout": "5s",
		},
	}).WithName("test")

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	require.Empty(t, errs)

	// Server keys should be present.
	var port int

	_, err := cfg.Get(config.NewKeyPath("server/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, 8080, port)

	// Client keys should be absent (skipped by merger).
	_, ok := cfg.Lookup(config.NewKeyPath("client/timeout"))
	assert.False(t, ok)
}
