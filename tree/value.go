package tree

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/tarantool/go-config/keypath"
	"github.com/tarantool/go-config/meta"
	"github.com/tarantool/go-config/value"
)

// valueImpl is the internal implementation of the value.Value interface.
type valueImpl struct {
	node    *Node
	keyPath keypath.KeyPath
	source  meta.SourceInfo
	rev     meta.RevisionType
}

// NewValue creates a new value.Value from a tree node and its key path.
// The source information and revision are extracted from the node.
func NewValue(node *Node, keyPath keypath.KeyPath) value.Value {
	var source meta.SourceInfo
	if node.Source != "" {
		source = meta.SourceInfo{
			Name: node.Source,
			Type: meta.UnknownSource,
		}
	}

	rev := meta.RevisionType(node.Revision)

	return &valueImpl{
		node:    node,
		keyPath: keyPath,
		source:  source,
		rev:     rev,
	}
}

// Get implements value.Value.Get.
func (v *valueImpl) Get(dest any) error {
	// Ensure dest is a pointer.
	destVal := reflect.ValueOf(dest)
	if destVal.Kind() != reflect.Ptr || destVal.IsNil() {
		return ErrDestinationMustBePointer
	}

	// Convert the node to a generic value.
	raw := nodeToValue(v.node)
	// Decode raw into dest using reflection.
	return decode(raw, destVal.Elem())
}

// Meta implements value.Value.Meta.
func (v *valueImpl) Meta() meta.Info {
	return meta.Info{
		Key:      v.keyPath,
		Source:   v.source,
		Revision: v.rev,
	}
}

// nodeToValue converts a tree node into a generic Go value.
// If the node is a leaf (no children), returns node.Value (which may be a slice, map, or primitive).
// Otherwise, builds a map[string]any from its children.
func nodeToValue(node *Node) any {
	if node.IsLeaf() {
		return node.Value
	}

	// Build map from children.
	children := node.Children()
	keys := node.ChildrenKeys()

	m := make(map[string]any, len(children))
	for i, child := range children {
		m[keys[i]] = nodeToValue(child)
	}

	return m
}

// decode decodes a generic value into a reflect.Value destination.
func decode(src any, dst reflect.Value) error {
	// Handle nil source.
	if src == nil {
		// Set zero value.
		dst.Set(reflect.Zero(dst.Type()))
		return nil
	}

	// Convert source to reflect.Value.
	srcVal := reflect.ValueOf(src)
	// If types are directly assignable, assign.
	if srcVal.Type().AssignableTo(dst.Type()) {
		dst.Set(srcVal)
		return nil
	}

	// Check for time.Duration special case.
	if dst.Type() == reflect.TypeFor[time.Duration]() {
		return decodeDuration(src, dst)
	}

	// Perform type conversion based on destination kind.
	switch dst.Kind() {
	case reflect.Bool:
		return decodeBool(src, dst)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return decodeInt(src, dst)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return decodeUint(src, dst)
	case reflect.Float32, reflect.Float64:
		return decodeFloat(src, dst)
	case reflect.String:
		return decodeString(src, dst)
	case reflect.Slice:
		return decodeSlice(src, dst)
	case reflect.Map:
		return decodeMap(src, dst)
	case reflect.Struct:
		return decodeStruct(src, dst)
	case reflect.Ptr:
		return decodePtr(src, dst)
	case reflect.Interface:
		// Assign directly if src type implements the interface.
		if srcVal.Type().Implements(dst.Type()) {
			dst.Set(srcVal)
			return nil
		}

		// Otherwise, set as empty interface.
		dst.Set(srcVal)

		return nil
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64,
		reflect.Complex128, reflect.Array, reflect.Chan,
		reflect.Func, reflect.UnsafePointer:
		// These kinds are unsupported.
		return fmt.Errorf("%w: %v", ErrUnsupportedDestinationType, dst.Kind())
	default:
		return fmt.Errorf("%w: %v", ErrUnsupportedDestinationType, dst.Kind())
	}
}

const (
	maxUint64SafeSeconds = uint64(math.MaxInt64 / int64(time.Second))
	maxInt64SafeSeconds  = math.MaxInt64 / int64(time.Second)
)

func safeConvertUintToSecDuration(uintSeconds uint64) (time.Duration, error) {
	// Check if uintSeconds seconds would overflow when converted to nanoseconds (Duration).
	if uintSeconds > maxUint64SafeSeconds {
		return 0, fmt.Errorf("%w: %d", ErrOverflow, uintSeconds)
	}

	return time.Duration(uintSeconds) * time.Second, nil
}

func safeConvertIntToSecDuration(intSeconds int64) (time.Duration, error) {
	// Check if intSeconds seconds would overflow when converted to nanoseconds (Duration).
	if intSeconds > maxInt64SafeSeconds {
		return 0, fmt.Errorf("%w: %d", ErrOverflow, intSeconds)
	}

	return time.Duration(intSeconds) * time.Second, nil
}

// decodeDuration converts src to time.Duration.
func decodeDuration(src any, dst reflect.Value) error {
	switch typedSrc := src.(type) {
	case string:
		d, err := time.ParseDuration(typedSrc)
		if err != nil {
			return fmt.Errorf("%w %q: %w", ErrParseDuration, typedSrc, err)
		}

		dst.Set(reflect.ValueOf(d))

		return nil
	case int, int8, int16, int32, int64:
		// Treat as seconds, convert to nanoseconds.
		val := reflect.ValueOf(typedSrc).Int()

		ns, err := safeConvertIntToSecDuration(val)
		if err != nil {
			return err
		}

		dst.Set(reflect.ValueOf(ns))

		return nil
	case uint, uint8, uint16, uint32, uint64:
		// Treat as seconds, convert to nanoseconds.
		val := reflect.ValueOf(typedSrc).Uint()

		ns, err := safeConvertUintToSecDuration(val)
		if err != nil {
			return err
		}

		dst.Set(reflect.ValueOf(ns))

		return nil
	case float32, float64:
		// Treat as seconds, convert to nanoseconds.
		val := reflect.ValueOf(typedSrc).Float()
		ns := time.Duration(val * float64(time.Second))
		dst.Set(reflect.ValueOf(ns))

		return nil
	default:
		return fmt.Errorf("%w: %T", ErrConvertToDuration, src)
	}
}

// decodeBool converts src to bool.
func decodeBool(src any, dst reflect.Value) error {
	switch typedSrc := src.(type) {
	case bool:
		dst.SetBool(typedSrc)
		return nil
	case string:
		b, err := strconv.ParseBool(typedSrc)
		if err != nil {
			return fmt.Errorf("%w %q: %w", ErrConvertToBool, typedSrc, err)
		}

		dst.SetBool(b)

		return nil
	case int, int8, int16, int32, int64:
		// Treat non-zero as true.
		dst.SetBool(reflect.ValueOf(typedSrc).Int() != 0)
		return nil
	case uint, uint8, uint16, uint32, uint64:
		// Treat non-zero as true.
		dst.SetBool(reflect.ValueOf(typedSrc).Uint() != 0)
		return nil
	default:
		return fmt.Errorf("%w: %T", ErrConvertToBool, src)
	}
}

func safeUintToInt64(u uint) (int64, error) {
	if uint64(u) > math.MaxInt64 {
		return 0, fmt.Errorf("%w: %d", ErrOverflow, u)
	}

	return int64(u), nil
}

func safeUint64ToInt64(uk uint64) (int64, error) {
	if uk > uint64(math.MaxInt) {
		return 0, fmt.Errorf("%w: %d", ErrOverflow, uk)
	}

	return int64(uk), nil
}

// decodeInt converts src to signed integer.
func decodeInt(src any, dst reflect.Value) error {
	var result int64

	switch typedSrc := src.(type) {
	case int:
		result = int64(typedSrc)
	case int8:
		result = int64(typedSrc)
	case int16:
		result = int64(typedSrc)
	case int32:
		result = int64(typedSrc)
	case int64:
		result = typedSrc
	case uint:
		var err error

		result, err = safeUintToInt64(typedSrc)
		if err != nil {
			return err
		}
	case uint8:
		result = int64(typedSrc)
	case uint16:
		result = int64(typedSrc)
	case uint32:
		result = int64(typedSrc)
	case uint64:
		var err error

		result, err = safeUint64ToInt64(typedSrc)
		if err != nil {
			return err
		}
	case float32:
		result = int64(typedSrc)
	case float64:
		result = int64(typedSrc)
	case string:
		i, err := strconv.ParseInt(typedSrc, 10, 64)
		if err != nil {
			return fmt.Errorf("%w %q: %w", ErrConvertToInt, typedSrc, err)
		}

		result = i
	case bool:
		if typedSrc {
			result = 1
		} else {
			result = 0
		}
	default:
		return fmt.Errorf("%w: %T", ErrConvertToInt, src)
	}

	// Check for overflow of the specific destination type.
	switch dst.Kind() {
	case reflect.Int8:
		if result < -1<<7 || result >= 1<<7 {
			return fmt.Errorf("%w int8: %d", ErrOverflow, result)
		}
	case reflect.Int16:
		if result < -1<<15 || result >= 1<<15 {
			return fmt.Errorf("%w int16: %d", ErrOverflow, result)
		}
	case reflect.Int32:
		if result < -1<<31 || result >= 1<<31 {
			return fmt.Errorf("%w int32: %d", ErrOverflow, result)
		}
	case reflect.Int64:
		// No overflow check needed.
	case reflect.Int:
		// Platform dependent; assume int64.
	case reflect.Invalid, reflect.Bool, reflect.Uint, reflect.Uint8,
		reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64,
		reflect.Complex128, reflect.Array, reflect.Chan, reflect.Func,
		reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice,
		reflect.String, reflect.Struct, reflect.UnsafePointer:
		// Unreachable because decodeInt is only called for signed integer kinds.
	}

	dst.SetInt(result)

	return nil
}

// decodeUint converts src to unsigned integer.
func decodeUint(src any, dst reflect.Value) error {
	var result uint64

	switch typedSrcValue := src.(type) {
	case uint:
		result = uint64(typedSrcValue)
	case uint8:
		result = uint64(typedSrcValue)
	case uint16:
		result = uint64(typedSrcValue)
	case uint32:
		result = uint64(typedSrcValue)
	case uint64:
		result = typedSrcValue
	case int, int8, int16, int32, int64:
		val := reflect.ValueOf(typedSrcValue).Int()
		if val < 0 {
			return fmt.Errorf("%w: %d", ErrNegativeToUnsigned, val)
		}

		result = uint64(val)
	case float32:
		if typedSrcValue < 0 {
			return fmt.Errorf("%w: %v", ErrNegativeToUnsigned, typedSrcValue)
		}

		result = uint64(typedSrcValue)
	case float64:
		if typedSrcValue < 0 {
			return fmt.Errorf("%w: %v", ErrNegativeToUnsigned, typedSrcValue)
		}

		result = uint64(typedSrcValue)
	case string:
		u, err := strconv.ParseUint(typedSrcValue, 10, 64)
		if err != nil {
			return fmt.Errorf("%w %q: %w", ErrConvertToUint, typedSrcValue, err)
		}

		result = u
	case bool:
		if typedSrcValue {
			result = 1
		} else {
			result = 0
		}
	default:
		return fmt.Errorf("%w: %T", ErrConvertToUint, src)
	}

	// Overflow check.
	switch dst.Kind() {
	case reflect.Uint8:
		if result >= 1<<8 {
			return fmt.Errorf("%w uint8: %d", ErrOverflow, result)
		}
	case reflect.Uint16:
		if result >= 1<<16 {
			return fmt.Errorf("%w uint16: %d", ErrOverflow, result)
		}
	case reflect.Uint32:
		if result >= 1<<32 {
			return fmt.Errorf("%w uint32: %d", ErrOverflow, result)
		}
	case reflect.Uint64:
		// No overflow check needed.
	case reflect.Uint:
		// Platform dependent; assume 64-bit.
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8,
		reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64,
		reflect.Complex128, reflect.Array, reflect.Chan, reflect.Func,
		reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice,
		reflect.String, reflect.Struct, reflect.UnsafePointer:
		// Unreachable because decodeUint is only called for unsigned integer kinds.
	}

	dst.SetUint(result)

	return nil
}

// decodeFloat converts src to float.
func decodeFloat(src any, dst reflect.Value) error {
	var result float64

	switch typedSrcValue := src.(type) {
	case float32:
		result = float64(typedSrcValue)
	case float64:
		result = typedSrcValue
	case int, int8, int16, int32, int64:
		result = float64(reflect.ValueOf(typedSrcValue).Int())
	case uint, uint8, uint16, uint32, uint64:
		result = float64(reflect.ValueOf(typedSrcValue).Uint())
	case string:
		f, err := strconv.ParseFloat(typedSrcValue, 64)
		if err != nil {
			return fmt.Errorf("%w %q: %w", ErrConvertToFloat, typedSrcValue, err)
		}

		result = f
	default:
		return fmt.Errorf("%w: %T", ErrConvertToFloat, src)
	}

	// Overflow check for float32.
	if dst.Kind() == reflect.Float32 {
		// Naive overflow check; not exhaustive.
		if result > 3.4e38 || result < -3.4e38 {
			return fmt.Errorf("%w float32: %g", ErrOverflow, result)
		}
	}

	dst.SetFloat(result)

	return nil
}

// decodeString converts src to string.
func decodeString(src any, dst reflect.Value) error {
	switch typedSrcValue := src.(type) {
	case string:
		dst.SetString(typedSrcValue)
	case []byte:
		dst.SetString(string(typedSrcValue))
	case fmt.Stringer:
		dst.SetString(typedSrcValue.String())
	default:
		// Use fmt.Sprint as fallback.
		dst.SetString(fmt.Sprint(typedSrcValue))
	}

	return nil
}

// decodeSlice converts src to slice.
func decodeSlice(src any, dst reflect.Value) error {
	srcVal := reflect.ValueOf(src)
	if srcVal.Kind() != reflect.Slice && srcVal.Kind() != reflect.Array {
		return fmt.Errorf("%w: %T", ErrSourceNotSliceOrArray, src)
	}

	length := srcVal.Len()

	slice := reflect.MakeSlice(dst.Type(), length, length)
	for i := range length {
		elem := srcVal.Index(i).Interface()

		err := decode(elem, slice.Index(i))
		if err != nil {
			return fmt.Errorf("slice element [%d]: %w", i, err)
		}
	}

	dst.Set(slice)

	return nil
}

// decodeMap converts src to map.
func decodeMap(src any, dst reflect.Value) error {
	srcVal := reflect.ValueOf(src)
	if srcVal.Kind() != reflect.Map {
		return fmt.Errorf("%w: %T", ErrSourceNotMap, src)
	}

	// Destination map must have string keys (as per configuration keys).
	if dst.Type().Key().Kind() != reflect.String {
		return fmt.Errorf("%w: %v", ErrDestinationMapStringKeys, dst.Type().Key())
	}

	// Create a new map.
	mapType := dst.Type()
	newMap := reflect.MakeMapWithSize(mapType, srcVal.Len())

	iter := srcVal.MapRange()
	for iter.Next() {
		key := iter.Key()
		if key.Kind() != reflect.String {
			return fmt.Errorf("%w: %v", ErrSourceMapKeyNotString, key.Kind())
		}

		// Create a new value of the map's element type.
		elem := reflect.New(mapType.Elem()).Elem()

		err := decode(iter.Value().Interface(), elem)
		if err != nil {
			return fmt.Errorf("map key %q: %w", key.String(), err)
		}

		newMap.SetMapIndex(key, elem)
	}

	dst.Set(newMap)

	return nil
}

// decodeStruct converts src to struct.
func decodeStruct(src any, dst reflect.Value) error {
	srcVal := reflect.ValueOf(src)
	// Source must be a map[string]any.
	if srcVal.Kind() != reflect.Map {
		return fmt.Errorf("%w: %T", ErrSourceForStructMustBeMap, src)
	}

	if srcVal.Type().Key().Kind() != reflect.String {
		return ErrSourceMapMustHaveStringKeys
	}

	// Iterate over struct fields.
	dstType := dst.Type()
	for i := range dstType.NumField() {
		field := dstType.Field(i)
		// Skip unexported fields.
		if !field.IsExported() {
			continue
		}

		// Get yaml tag.
		tag := field.Tag.Get("yaml")
		if tag == "" {
			// Fallback to field name.
			tag = field.Name
		} else {
			// Handle yaml tag options like "omitempty".
			if comma := strings.Index(tag, ","); comma != -1 {
				tag = tag[:comma]
			}
		}

		// Look up key in source map.
		key := reflect.ValueOf(tag)

		val := srcVal.MapIndex(key)
		if !val.IsValid() {
			// Field not found; leave zero value.
			continue
		}

		// Decode into field.
		err := decode(val.Interface(), dst.Field(i))
		if err != nil {
			return fmt.Errorf("field %q: %w", field.Name, err)
		}
	}

	return nil
}

// decodePtr converts src to pointer.
func decodePtr(src any, dst reflect.Value) error {
	// If dst is nil, allocate a new value.
	if dst.IsNil() {
		dst.Set(reflect.New(dst.Type().Elem()))
	}

	// Dereference and decode.
	return decode(src, dst.Elem())
}
