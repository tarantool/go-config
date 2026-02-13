package config

import (
	"slices"

	"github.com/tarantool/go-config/keypath"
	"github.com/tarantool/go-config/tree"
)

// defaultMergerContext implements MergerContext.
type defaultMergerContext struct {
	collector    Collector
	parentOrders map[string][]string
}

func (ctx *defaultMergerContext) Collector() Collector {
	return ctx.collector
}

func (ctx *defaultMergerContext) RecordOrdering(parent keypath.KeyPath, child string) {
	if ctx.parentOrders == nil {
		return
	}

	parentKey := parent.String()

	keys := ctx.parentOrders[parentKey]
	if !slices.Contains(keys, child) {
		ctx.parentOrders[parentKey] = append(keys, child)
	}
}

func (ctx *defaultMergerContext) ApplyOrdering(root *tree.Node) error {
	if ctx.parentOrders == nil {
		return nil
	}

	for parentKey, orderedKeys := range ctx.parentOrders {
		var parentNode *tree.Node
		if parentKey == "" {
			parentNode = root
		} else {
			parentNode = root.Get(keypath.NewKeyPath(parentKey))
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

// DefaultMerger implements Merger with the standard merging logic.
type DefaultMerger struct{}

// CreateContext creates a new merger context for the given collector.
func (d *DefaultMerger) CreateContext(collector Collector) MergerContext {
	ctx := &defaultMergerContext{collector: collector, parentOrders: nil}
	if collector.KeepOrder() {
		ctx.parentOrders = make(map[string][]string)
	}

	return ctx
}

// MergeValue merges a single value into the tree using the default merging logic.
func (d *DefaultMerger) MergeValue(ctx MergerContext, root *tree.Node, keyPath keypath.KeyPath, value any) error {
	col := ctx.Collector()
	// Use internal mergeValue function.
	mergeValue(root, keyPath, value, col)
	// Record ordering if needed.
	if col.KeepOrder() && len(keyPath) > 0 {
		parent := keyPath.Parent()
		child := keyPath.Leaf()
		ctx.RecordOrdering(parent, child)
	}

	return nil
}

// Default is the default merger instance.
//
//nolint:gochecknoglobals
var Default = &DefaultMerger{}
