package collectors

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/tree"
)

// Struct reads configuration data from a Go struct using reflection.
//
// Field names come from the `config` struct tag, falling back to `yaml`, then
// the lowercased field name. "-" skips a field, "omitempty" skips zero values,
// and "inline" merges a struct/map field's keys into the enclosing map (an
// anonymous field without "inline" is nested under its lowercased type name).
// Unexported fields are ignored, except those promoted from an embedded struct.
//
// The struct is converted via [StructToMap] and flattened like the [Map]
// collector: nested structs become nested maps, slices and arrays become
// []any, []byte stays a byte slice, and maps are copied with keys stringified.
//
// Key order is preserved by default, like [YamlFormat]. If the value passed to
// [NewStruct] is not a struct (nor a pointer to one), Read yields no values.
type Struct struct {
	data       any
	name       string
	sourceType config.SourceType
	revision   config.RevisionType
	keepOrder  bool
}

// NewStruct creates a Struct collector for the given value, which must be a
// struct or a pointer to a struct. The source type defaults to
// config.UnknownSource and key order is preserved by default.
func NewStruct(data any) *Struct {
	return &Struct{
		data:       data,
		name:       "struct",
		sourceType: config.UnknownSource,
		revision:   "",
		keepOrder:  true,
	}
}

// WithName sets a custom name for the collector.
func (sc *Struct) WithName(name string) *Struct {
	sc.name = name
	return sc
}

// WithSourceType sets the source type for the collector.
func (sc *Struct) WithSourceType(source config.SourceType) *Struct {
	sc.sourceType = source
	return sc
}

// WithRevision sets the revision for the collector.
func (sc *Struct) WithRevision(rev config.RevisionType) *Struct {
	sc.revision = rev
	return sc
}

// WithKeepOrder sets whether the collector preserves key order.
func (sc *Struct) WithKeepOrder(keep bool) *Struct {
	sc.keepOrder = keep
	return sc
}

// Read implements the Collector interface.
func (sc *Struct) Read(ctx context.Context) <-chan config.Value {
	valueCh := make(chan config.Value)

	go func() {
		defer close(valueCh)

		data, err := StructToMap(sc.data)
		if err != nil {
			return
		}

		root := tree.New()
		flattenMapIntoTree(root, config.NewKeyPath(""), data, sc.keepOrder)
		walkTree(ctx, root, config.NewKeyPath(""), valueCh)
	}()

	return valueCh
}

// Name implements the Collector interface.
func (sc *Struct) Name() string {
	return sc.name
}

// Source implements the Collector interface.
func (sc *Struct) Source() config.SourceType {
	return sc.sourceType
}

// Revision implements the Collector interface.
func (sc *Struct) Revision() config.RevisionType {
	return sc.revision
}

// KeepOrder implements the Collector interface.
func (sc *Struct) KeepOrder() bool {
	return sc.keepOrder
}

// StructToMap converts a struct (or a pointer to one) into a map[string]any
// using the same field-naming, tag, and value-conversion rules as the
// [Struct] collector. It returns ErrNotStruct for any other input.
func StructToMap(v any) (map[string]any, error) {
	rval := derefValue(reflect.ValueOf(v))
	if !rval.IsValid() || rval.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%w: got %T", ErrNotStruct, v)
	}

	out := map[string]any{}
	structFieldsIntoMap(out, rval)

	return out, nil
}

// derefValue unwraps pointers and interfaces, yielding the zero Value on a nil.
func derefValue(rval reflect.Value) reflect.Value {
	for rval.IsValid() && (rval.Kind() == reflect.Pointer || rval.Kind() == reflect.Interface) {
		if rval.IsNil() {
			return reflect.Value{}
		}

		rval = rval.Elem()
	}

	return rval
}

// structFieldsIntoMap writes the struct's fields into out per the tag rules
// described on [StructToMap].
func structFieldsIntoMap(out map[string]any, rval reflect.Value) {
	rtype := rval.Type()

	for idx := range rtype.NumField() {
		sfield := rtype.Field(idx)
		// Skip unexported fields, but keep embedded ones: their promoted
		// exported fields are still reachable via reflection.
		if !sfield.IsExported() && !sfield.Anonymous {
			continue
		}

		name, opts := fieldTag(sfield)
		if name == "-" {
			continue
		}

		field := rval.Field(idx)
		if opts.has("omitempty") && field.IsZero() {
			continue
		}

		if opts.has("inline") && mergeInline(out, field) {
			continue
		}

		key := name
		if key == "" {
			key = strings.ToLower(sfield.Name)
		}

		out[key] = goValue(field)
	}
}

// mergeInline merges a struct- or map-valued field's keys into out, reporting
// whether it did. A non-struct/non-map field is left to normal handling
// (returns false); a nil one counts as an empty merge.
func mergeInline(out map[string]any, field reflect.Value) bool {
	deref := derefValue(field)
	if !deref.IsValid() {
		return true
	}

	switch {
	case deref.Kind() == reflect.Struct:
		structFieldsIntoMap(out, deref)
		return true
	case deref.Kind() == reflect.Map:
		maps.Copy(out, mapToAny(deref))
		return true
	default:
		return false
	}
}

// goValue converts a reflect.Value into a plain Go value: structs and maps
// become map[string]any (map keys stringified), slices/arrays become []any
// (except []byte, kept as-is), anything unreadable or nil becomes nil, and
// the rest is returned via Interface().
func goValue(rval reflect.Value) any {
	rval = derefValue(rval)
	if !rval.IsValid() {
		return nil
	}

	kind := rval.Kind()

	switch {
	case kind == reflect.Struct:
		out := map[string]any{}
		structFieldsIntoMap(out, rval)

		return out
	case kind == reflect.Map:
		return mapToAny(rval)
	case kind == reflect.Slice && rval.IsNil():
		return nil
	case kind == reflect.Slice && rval.Type().Elem().Kind() == reflect.Uint8:
		return rval.Bytes()
	case kind == reflect.Slice || kind == reflect.Array:
		return sliceToAny(rval)
	case !rval.CanInterface():
		// Reachable only for an unexported, embedded scalar field.
		return nil
	default:
		return rval.Interface()
	}
}

// sliceToAny converts each element of a slice or array value into a []any.
func sliceToAny(rval reflect.Value) []any {
	out := make([]any, rval.Len())

	for idx := range rval.Len() {
		out[idx] = goValue(rval.Index(idx))
	}

	return out
}

// mapToAny copies a map value into a map[string]any, stringifying keys that
// are not already strings via fmt.Sprint. A nil map yields a nil result.
func mapToAny(rval reflect.Value) map[string]any {
	if rval.IsNil() {
		return nil
	}

	out := make(map[string]any, rval.Len())

	iter := rval.MapRange()
	for iter.Next() {
		out[mapKeyString(iter.Key())] = goValue(iter.Value())
	}

	return out
}

// mapKeyString renders a map key as a string: string keys pass through, other
// keys go through fmt.Sprint, and an invalid (nil interface) key becomes
// "<nil>".
func mapKeyString(key reflect.Value) string {
	if key.Kind() == reflect.String {
		return key.String()
	}

	if deref := derefValue(key); deref.IsValid() {
		return fmt.Sprint(deref.Interface())
	}

	return "<nil>"
}

// tagOptions holds the comma-separated options of a struct tag (everything
// after the name).
type tagOptions []string

func (o tagOptions) has(opt string) bool {
	return slices.Contains(o, opt)
}

// fieldTag returns the configured name and options for a struct field,
// preferring the `config` tag over the `yaml` tag. An absent tag yields an
// empty name and no options.
func fieldTag(sfield reflect.StructField) (string, tagOptions) {
	raw, ok := sfield.Tag.Lookup("config")
	if !ok {
		raw, ok = sfield.Tag.Lookup("yaml")
	}

	if !ok {
		return "", nil
	}

	parts := strings.Split(raw, ",")

	return parts[0], tagOptions(parts[1:])
}
