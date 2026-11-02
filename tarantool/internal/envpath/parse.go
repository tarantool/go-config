package envpath

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
)

// Parse reads raw, the value of an environment variable, as a value of type
// t, by the rules of Tarantool's schema.fromenv
// (src/box/lua/config/utils/schema.lua):
//
//   - An empty value is unset: Parse returns nil, true, and so it does for a
//     JSON null where JSON is read.
//   - A string is raw as is.
//   - A number or an integer is parsed from decimal text, surrounding spaces
//     allowed; a "0x" prefix reads hex, and an integer also takes "0b" for
//     binary. Integral text gives an int64 (a uint64 above its range) and
//     other text a float64, the Go types the YAML collector gives the same
//     text.
//   - A boolean is "true" or "false" in any case, or "1" or "0".
//   - A value without a type is JSON, and only JSON.
//   - An array is a JSON array when raw starts with "[", is never a JSON
//     object, and is otherwise split on "," into items parsed by the items
//     type when that is a scalar (or a union of scalars) other than "any".
//   - A map is a JSON object when raw starts with "{", is never a JSON array,
//     and is otherwise "k=v,k2=v2" with values parsed by the values type
//     under the same condition as array items.
//   - A union takes the first variant that reads raw as something other than
//     raw itself, and otherwise the first that reads raw as raw: a number
//     variant beats a string one for "5", and the string one takes "info".
//   - A record never comes from one variable.
//
// ok is false when raw cannot be read as t. Tarantool fails to start then;
// the caller decides what to do with the raw string instead.
//
// Tarantool also parses a number or an integer with a duration or byte_size
// annotation ("30s", "1GiB") into a number. The JSON schema renders such a
// field as a union of the number type and a string with a pattern, so Parse
// reads "30s" as the string variant and keeps it, the way the YAML collector
// keeps it in a file; only plain numbers become numbers.
func (t *Type) Parse(raw string) (any, bool) {
	if raw == "" {
		return nil, true
	}

	switch t.Kind {
	case KindString:
		return raw, true
	case KindNumber:
		return parseNumber(raw)
	case KindInteger:
		return parseInteger(raw)
	case KindBoolean:
		return parseBoolean(raw)
	case KindAny:
		return decodeJSON(raw)
	case KindArray:
		return t.parseArray(raw)
	case KindMap:
		return t.parseMap(raw)
	case KindUnion:
		return t.parseUnion(raw)
	case KindNull, KindRecord:
		return nil, false
	}

	return nil, false
}

// simple reports whether a value of type t can be written in the simple
// "foo,bar" array or "k=v" map format: it is a scalar other than "any", or a
// union of such scalars. This is Tarantool's can_have_simple_format.
func (t *Type) simple() bool {
	switch t.Kind {
	case KindString, KindNumber, KindInteger, KindBoolean:
		return true
	case KindUnion:
		for _, variant := range t.Variants {
			if !variant.simple() {
				return false
			}
		}

		return len(t.Variants) > 0
	case KindAny, KindNull, KindArray, KindMap, KindRecord:
		return false
	}

	return false
}

// parseArray reads raw as an array. See [Type.Parse].
func (t *Type) parseArray(raw string) (any, bool) {
	switch {
	case strings.HasPrefix(raw, "["):
		return decodeJSON(raw)
	case strings.HasPrefix(raw, "{"), !t.Items.simple():
		return nil, false
	}

	parts := strings.Split(raw, ",")
	items := make([]any, 0, len(parts))

	for _, part := range parts {
		item, ok := t.Items.Parse(part)
		if !ok {
			return nil, false
		}

		items = append(items, item)
	}

	// An empty item is unset, as an empty variable is: Tarantool stores nil
	// in its place. Trailing ones vanish, but one in the middle leaves a hole
	// that makes the result no array at all.
	for len(items) > 0 && items[len(items)-1] == nil {
		items = items[:len(items)-1]
	}

	if slices.ContainsFunc(items, func(item any) bool { return item == nil }) {
		return nil, false
	}

	return items, true
}

// parseMap reads raw as a map. See [Type.Parse].
func (t *Type) parseMap(raw string) (any, bool) {
	switch {
	case strings.HasPrefix(raw, "{"):
		return decodeJSON(raw)
	case strings.HasPrefix(raw, "["), !t.Values.simple():
		return nil, false
	}

	values := map[string]any{}

	for pair := range strings.SplitSeq(raw, ",") {
		key, text, found := strings.Cut(pair, "=")
		if !found || key == "" {
			return nil, false
		}

		value, ok := t.Values.Parse(text)
		if !ok {
			return nil, false
		}

		// An empty value unsets the key, as an empty variable does.
		if value == nil {
			delete(values, key)

			continue
		}

		values[key] = value
	}

	return values, true
}

// parseUnion reads raw as a union, the way Tarantool's union_from_env does:
// a variant's value counts only if it satisfies the variant's enum, and the
// first one that differs from raw wins over a string variant that keeps raw.
func (t *Type) parseUnion(raw string) (any, bool) {
	var fallback any

	for _, variant := range t.Variants {
		value, ok := variant.Parse(raw)
		if !ok || value == nil || !variant.allows(value) {
			continue
		}

		if text, isString := value.(string); !isString || text != raw {
			return value, true
		}

		if fallback == nil {
			fallback = value
		}
	}

	if fallback != nil {
		return fallback, true
	}

	return nil, false
}

// allows reports whether value is one of the enum values of t, or t has no
// enum. Numbers compare by value whatever their Go type.
func (t *Type) allows(value any) bool {
	if len(t.Enum) == 0 {
		return true
	}

	number, isNumber := toFloat(value)

	for _, allowed := range t.Enum {
		if isNumber {
			if other, ok := toFloat(allowed); ok && other == number {
				return true
			}

			continue
		}

		if allowed == value {
			return true
		}
	}

	return false
}

// toFloat converts a number of any of the Go types a parsed value or a JSON
// schema enum holds to a float64.
func toFloat(value any) (float64, bool) {
	switch number := value.(type) {
	case int64:
		return float64(number), true
	case uint64:
		return float64(number), true
	case float64:
		return number, true
	default:
		return 0, false
	}
}

// parseNumber reads raw as a number, the way Lua's tonumber does.
func parseNumber(raw string) (any, bool) {
	text := strings.TrimSpace(raw)

	value, ok := parseIntegerText(text, false)
	if ok {
		return value, true
	}

	// strconv.ParseFloat also takes "inf", "nan", hex floats and digit
	// separators, which Tarantool does not.
	if !isDecimalFloat(text) {
		return nil, false
	}

	number, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil, false
	}

	return number, true
}

// parseInteger reads raw as an integer, the way Tarantool's tonumber64 does.
func parseInteger(raw string) (any, bool) {
	return parseIntegerText(strings.TrimSpace(raw), true)
}

// parseIntegerText reads text as an optionally signed integer: decimal, hex
// with a "0x" prefix, or binary with a "0b" prefix when binary is set. A
// leading zero does not make it octal.
func parseIntegerText(text string, binary bool) (any, bool) {
	body, negative := text, false

	switch {
	case strings.HasPrefix(body, "-"):
		body, negative = body[1:], true
	case strings.HasPrefix(body, "+"):
		body = body[1:]
	}

	base := 10

	if len(body) > len("0x") && body[0] == '0' {
		switch body[1] {
		case 'x', 'X':
			base, body = 16, body[2:]
		case 'b', 'B':
			if binary {
				base, body = 2, body[2:]
			}
		}
	}

	// With an explicit base ParseUint takes digits only: no sign, no
	// separators.
	magnitude, err := strconv.ParseUint(body, base, 64)
	if err != nil {
		return nil, false
	}

	switch {
	case !negative && magnitude <= math.MaxInt64:
		return int64(magnitude), true
	case !negative:
		return magnitude, true
	case magnitude <= math.MaxInt64:
		return -int64(magnitude), true
	case magnitude == -math.MinInt64:
		return int64(math.MinInt64), true
	default:
		return nil, false
	}
}

// isDecimalFloat reports whether text has only the characters of a decimal
// floating-point number and at least one digit.
func isDecimalFloat(text string) bool {
	digits := false

	for _, char := range text {
		switch {
		case char >= '0' && char <= '9':
			digits = true
		case strings.ContainsRune("+-.eE", char):
		default:
			return false
		}
	}

	return digits
}

// parseBoolean reads raw as a boolean: "true" or "false" in any case, or "1"
// or "0".
func parseBoolean(raw string) (any, bool) {
	switch {
	case strings.EqualFold(raw, "true") || raw == "1":
		return true, true
	case strings.EqualFold(raw, "false") || raw == "0":
		return false, true
	default:
		return nil, false
	}
}

// decodeJSON reads raw as a single JSON value. Numbers become an int64, a
// uint64 or a float64, as in [parseNumber]; a number out of the float64 range
// fails the decoding.
func decodeJSON(raw string) (any, bool) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()

	var value any

	err := decoder.Decode(&value)
	if err != nil {
		return nil, false
	}

	_, err = decoder.Token()
	if !errors.Is(err, io.EOF) {
		return nil, false
	}

	return normalizeJSON(value)
}

// normalizeJSON replaces the json.Number values in a decoded JSON value with
// Go numbers, in place.
func normalizeJSON(value any) (any, bool) {
	switch typed := value.(type) {
	case json.Number:
		return jsonNumber(typed)
	case []any:
		for i, item := range typed {
			normalized, ok := normalizeJSON(item)
			if !ok {
				return nil, false
			}

			typed[i] = normalized
		}

		return typed, true
	case map[string]any:
		for key, item := range typed {
			normalized, ok := normalizeJSON(item)
			if !ok {
				return nil, false
			}

			typed[key] = normalized
		}

		return typed, true
	default:
		return value, true
	}
}

// jsonNumber converts a JSON number to an int64 when it is integral and fits,
// a uint64 when it is integral and fits only that, and a float64 otherwise.
func jsonNumber(number json.Number) (any, bool) {
	text := number.String()

	integer, err := strconv.ParseInt(text, 10, 64)
	if err == nil {
		return integer, true
	}

	unsigned, err := strconv.ParseUint(text, 10, 64)
	if err == nil {
		return unsigned, true
	}

	float, err := number.Float64()
	if err != nil {
		return nil, false
	}

	return float, true
}
