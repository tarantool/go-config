//nolint:forbidigo
package config_test

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/tree"
)

// Example_validatingMerger demonstrates a custom merger that validates
// configuration values before merging them into the tree.
//
//nolint:err113
func Example_validatingMerger() {
	// Create a validating merger that enforces constraints.
	validator := &validatingMerger{
		rules: map[string]func(any) error{
			"port": func(v any) error {
				port, ok := v.(int)
				switch {
				case !ok:
					return errors.New("port must be an integer")
				case port < 1 || port > 65535:
					return fmt.Errorf("port must be between 1 and 65535, got %d", port)
				}

				return nil
			},
			"timeout": func(v any) error {
				timeout, ok := v.(int)
				switch {
				case !ok:
					return errors.New("timeout must be an integer")
				case timeout < 0:
					return fmt.Errorf("timeout must be non-negative, got %d", timeout)
				}

				return nil
			},
		},
	}

	// Valid configuration.
	validData := map[string]any{
		"server": map[string]any{
			"port":    8080,
			"timeout": 30,
		},
	}

	builder := config.NewBuilder()

	builder = builder.WithMerger(validator)
	builder = builder.AddCollector(collectors.NewMap(validData).WithName("valid"))

	cfg, errs := builder.Build()

	if errs != nil {
		log.Printf("Errors: %v", errs)
	} else {
		var port int

		_, err := cfg.Get(config.NewKeyPath("server/port"), &port)
		if err != nil {
			fmt.Printf("Failed to Get 'server/port': %s\n", err)
		} else {
			fmt.Printf("Valid port: %d\n", port)
		}
	}

	// Invalid configuration.
	invalidData := map[string]any{
		"server": map[string]any{
			"port":    99999, // Invalid port.
			"timeout": 30,
		},
	}

	builder = config.NewBuilder()
	builder = builder.WithMerger(validator)
	builder = builder.AddCollector(collectors.NewMap(invalidData).WithName("invalid"))

	_, errs = builder.Build()

	if errs != nil {
		fmt.Printf("Validation failed: %v\n", strings.Contains(errs[0].Error(), "port must be between"))
	}

	// Output:
	// Valid port: 8080
	// Validation failed: true
}

// validatingMerger validates values against rules before merging.
type validatingMerger struct {
	rules map[string]func(any) error
}

type validatingContext struct {
	collector    config.Collector
	merger       *validatingMerger
	parentOrders map[string][]string
}

func (v *validatingMerger) CreateContext(col config.Collector) config.MergerContext {
	ctx := &validatingContext{
		collector:    col,
		merger:       v,
		parentOrders: nil,
	}
	if col.KeepOrder() {
		ctx.parentOrders = make(map[string][]string)
	}

	return ctx
}

func (v *validatingMerger) MergeValue(
	ctx config.MergerContext,
	root *tree.Node,
	keyPath config.KeyPath,
	value any,
) error {
	// Check if we have a validation rule for the last key in the path.
	if len(keyPath) > 0 {
		lastKey := keyPath[len(keyPath)-1]
		if rule, ok := v.rules[lastKey]; ok {
			err := rule(value)
			if err != nil {
				return fmt.Errorf("validation failed for %s: %w", keyPath, err)
			}
		}
	}

	// Delegate to default merger after validation.
	return config.Default.MergeValue(ctx, root, keyPath, value)
}

func (vc *validatingContext) Collector() config.Collector {
	return vc.collector
}

func (vc *validatingContext) RecordOrdering(parent config.KeyPath, child string) {
	if vc.parentOrders == nil {
		return
	}

	parentKey := parent.String()
	if !slices.Contains(vc.parentOrders[parentKey], child) {
		vc.parentOrders[parentKey] = append(vc.parentOrders[parentKey], child)
	}
}

func (vc *validatingContext) ApplyOrdering(root *tree.Node) error {
	if vc.parentOrders == nil {
		return nil
	}

	for parentPath, order := range vc.parentOrders {
		var node *tree.Node
		if parentPath == "" {
			node = root
		} else {
			node = root.Get(config.NewKeyPath(parentPath))
		}

		if node != nil {
			_ = node.ReorderChildren(order)
		}
	}

	return nil
}

// Example_transformingMerger demonstrates a custom merger that transforms
// values based on their path or content before merging.
func Example_transformingMerger() {
	// Create a transformer that normalizes string values.
	transformer := &transformingMerger{
		transforms: map[string]func(any) any{
			"name": func(v any) any {
				if s, ok := v.(string); ok {
					return strings.TrimSpace(strings.ToLower(s))
				}

				return v
			},
			"email": func(v any) any {
				if s, ok := v.(string); ok {
					return strings.TrimSpace(strings.ToLower(s))
				}

				return v
			},
		},
	}

	data := map[string]any{
		"user": map[string]any{
			"name":  "  John Doe  ",
			"email": "  JOHN.DOE@EXAMPLE.COM  ",
			"age":   30,
		},
	}

	builder := config.NewBuilder()

	builder = builder.WithMerger(transformer)
	builder = builder.AddCollector(collectors.NewMap(data).WithName("user-data"))

	cfg, _ := builder.Build()

	var name, email string

	_, _ = cfg.Get(config.NewKeyPath("user/name"), &name)
	_, _ = cfg.Get(config.NewKeyPath("user/email"), &email)

	fmt.Printf("Name: %s\n", name)
	fmt.Printf("Email: %s\n", email)

	// Output:
	// Name: john doe
	// Email: john.doe@example.com
}

// transformingMerger transforms values before merging.
type transformingMerger struct {
	transforms map[string]func(any) any
}

type transformingContext struct {
	collector    config.Collector
	merger       *transformingMerger
	parentOrders map[string][]string
}

func (t *transformingMerger) CreateContext(col config.Collector) config.MergerContext {
	ctx := &transformingContext{
		collector:    col,
		merger:       t,
		parentOrders: nil,
	}
	if col.KeepOrder() {
		ctx.parentOrders = make(map[string][]string)
	}

	return ctx
}

func (t *transformingMerger) MergeValue(
	ctx config.MergerContext,
	root *tree.Node,
	keyPath config.KeyPath,
	value any,
) error {
	// Transform value if we have a rule for this key.
	transformedValue := value

	if len(keyPath) > 0 {
		lastKey := keyPath[len(keyPath)-1]
		if transform, ok := t.transforms[lastKey]; ok {
			transformedValue = transform(value)
		}
	}

	// Delegate to default merger with transformed value.
	return config.Default.MergeValue(ctx, root, keyPath, transformedValue)
}

func (tc *transformingContext) Collector() config.Collector {
	return tc.collector
}

func (tc *transformingContext) RecordOrdering(parent config.KeyPath, child string) {
	if tc.parentOrders == nil {
		return
	}

	parentKey := parent.String()
	if !slices.Contains(tc.parentOrders[parentKey], child) {
		tc.parentOrders[parentKey] = append(tc.parentOrders[parentKey], child)
	}
}

func (tc *transformingContext) ApplyOrdering(root *tree.Node) error {
	if tc.parentOrders == nil {
		return nil
	}

	for parentPath, order := range tc.parentOrders {
		var node *tree.Node
		if parentPath == "" {
			node = root
		} else {
			node = root.Get(config.NewKeyPath(parentPath))
		}

		if node != nil {
			_ = node.ReorderChildren(order)
		}
	}

	return nil
}

// Example_loggingMerger demonstrates a custom merger that logs all merge operations
// for auditing and debugging purposes.
func Example_loggingMerger() {
	// Create a logging merger that tracks operations.
	logger := &loggingMerger{
		prefix: "[CONFIG]",
	}

	data := map[string]any{
		"database": map[string]any{
			"host": "localhost",
			"port": 5432,
		},
	}

	builder := config.NewBuilder()

	builder = builder.WithMerger(logger)
	builder = builder.AddCollector(collectors.NewMap(data).WithName("db-config"))

	cfg, _ := builder.Build()

	var host string

	_, _ = cfg.Get(config.NewKeyPath("database/host"), &host)
	fmt.Printf("Host: %s\n", host)

	// Unordered output:
	// [CONFIG] Merging database/host = localhost (from db-config)
	// [CONFIG] Merging database/port = 5432 (from db-config)
	// Host: localhost
}

// loggingMerger logs all merge operations.
type loggingMerger struct {
	prefix string
}

type loggingContext struct {
	collector    config.Collector
	merger       *loggingMerger
	parentOrders map[string][]string
}

func (l *loggingMerger) CreateContext(col config.Collector) config.MergerContext {
	ctx := &loggingContext{
		collector:    col,
		merger:       l,
		parentOrders: nil,
	}
	if col.KeepOrder() {
		ctx.parentOrders = make(map[string][]string)
	}

	return ctx
}

func (l *loggingMerger) MergeValue(
	ctx config.MergerContext,
	root *tree.Node,
	keyPath config.KeyPath,
	value any,
) error {
	// Log the merge operation.
	fmt.Printf("%s Merging %s = %v (from %s)\n",
		l.prefix, keyPath, value, ctx.Collector().Name())

	// Delegate to default merger.
	return config.Default.MergeValue(ctx, root, keyPath, value)
}

func (lc *loggingContext) Collector() config.Collector {
	return lc.collector
}

func (lc *loggingContext) RecordOrdering(parent config.KeyPath, child string) {
	if lc.parentOrders == nil {
		return
	}

	parentKey := parent.String()
	if !slices.Contains(lc.parentOrders[parentKey], child) {
		lc.parentOrders[parentKey] = append(lc.parentOrders[parentKey], child)
	}
}

func (lc *loggingContext) ApplyOrdering(root *tree.Node) error {
	if lc.parentOrders == nil {
		return nil
	}

	for parentPath, order := range lc.parentOrders {
		var node *tree.Node
		if parentPath == "" {
			node = root
		} else {
			node = root.Get(config.NewKeyPath(parentPath))
		}

		if node != nil {
			_ = node.ReorderChildren(order)
		}
	}

	return nil
}

// Example_sourceBasedMerger demonstrates a custom merger that applies
// different merging strategies based on the collector source.
func Example_sourceBasedMerger() {
	// Create a merger that only accepts values from specific sources.
	sourceMerger := &sourceBasedMerger{
		allowedSources: map[string]bool{
			"production": true,
			"staging":    true,
		},
	}

	prodData := map[string]any{
		"api": map[string]any{
			"key": "prod-key-123",
		},
	}

	devData := map[string]any{
		"api": map[string]any{
			"key": "dev-key-456",
		},
	}

	builder := config.NewBuilder()

	builder = builder.WithMerger(sourceMerger)
	builder = builder.AddCollector(collectors.NewMap(prodData).WithName("production"))
	builder = builder.AddCollector(collectors.NewMap(devData).WithName("development"))

	cfg, _ := builder.Build()

	var apiKey string

	_, _ = cfg.Get(config.NewKeyPath("api/key"), &apiKey)

	// Only production data was merged.
	fmt.Printf("API Key: %s\n", apiKey)

	// Output:
	// API Key: prod-key-123
}

// sourceBasedMerger filters values based on collector source.
type sourceBasedMerger struct {
	allowedSources map[string]bool
}

type sourceBasedContext struct {
	collector    config.Collector
	merger       *sourceBasedMerger
	parentOrders map[string][]string
}

func (s *sourceBasedMerger) CreateContext(col config.Collector) config.MergerContext {
	ctx := &sourceBasedContext{
		collector:    col,
		merger:       s,
		parentOrders: nil,
	}
	if col.KeepOrder() {
		ctx.parentOrders = make(map[string][]string)
	}

	return ctx
}

func (s *sourceBasedMerger) MergeValue(
	ctx config.MergerContext,
	root *tree.Node,
	keyPath config.KeyPath,
	value any,
) error {
	// Check if this collector's source is allowed.
	collectorName := ctx.Collector().Name()
	if !s.allowedSources[collectorName] {
		// Skip merging for disallowed sources.
		return nil
	}

	// Delegate to default merger for allowed sources.
	return config.Default.MergeValue(ctx, root, keyPath, value)
}

func (sc *sourceBasedContext) Collector() config.Collector {
	return sc.collector
}

func (sc *sourceBasedContext) RecordOrdering(parent config.KeyPath, child string) {
	if sc.parentOrders == nil {
		return
	}

	parentKey := parent.String()
	if !slices.Contains(sc.parentOrders[parentKey], child) {
		sc.parentOrders[parentKey] = append(sc.parentOrders[parentKey], child)
	}
}

func (sc *sourceBasedContext) ApplyOrdering(root *tree.Node) error {
	if sc.parentOrders == nil {
		return nil
	}

	for parentPath, order := range sc.parentOrders {
		var node *tree.Node
		if parentPath == "" {
			node = root
		} else {
			node = root.Get(config.NewKeyPath(parentPath))
		}

		if node != nil {
			_ = node.ReorderChildren(order)
		}
	}

	return nil
}
