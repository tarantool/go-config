package schema

import (
	"maps"
	"slices"
	"strings"

	"github.com/kaptinlin/jsonschema"
)

// Metadata describes a field or array item without prescribing a display format.
// Descriptions are copied verbatim from JSON Schema. Child properties and array
// items are not included. Composition groups retain their branch associations;
// AllOf also contains metadata from references and overlapping property patterns.
// Empty AnyOf and OneOf entries retain permitted alternatives without exposed
// metadata; they may still constrain values.
// The returned values are independent of the compiled schema and may be modified.
type Metadata struct {
	Title       string
	Description string
	Types       []string
	Enum        []any
	Const       any
	HasConst    bool // Distinguishes an explicit null constant from an absent constant.
	Default     any  // Nil for an absent default or default: null.
	Deprecated  bool
	AllOf       []Metadata
	AnyOf       []Metadata
	OneOf       []Metadata
}

func (m Metadata) hasAnnotations() bool {
	return m.Title != "" || m.Description != "" || len(m.Types) > 0 ||
		len(m.Enum) > 0 || m.HasConst || m.Default != nil || m.Deprecated
}

func (m Metadata) empty() bool {
	if m.hasAnnotations() {
		return false
	}

	for _, group := range [][]Metadata{m.AllOf, m.AnyOf, m.OneOf} {
		for _, branch := range group {
			if !branch.empty() {
				return false
			}
		}
	}

	return true
}

// annotations copies metadata owned by this schema without visiting children.
func annotations(current *jsonschema.Schema) Metadata {
	var metadata Metadata

	if current.Title != nil && strings.TrimSpace(*current.Title) != "" {
		metadata.Title = *current.Title
	}

	if current.Description != nil && strings.TrimSpace(*current.Description) != "" {
		metadata.Description = *current.Description
	}

	metadata.Types = slices.Clone(current.Type)
	metadata.Enum = slices.Clone(current.Enum)

	for index, value := range metadata.Enum {
		metadata.Enum[index] = cloneValue(value)
	}

	if current.Const != nil && current.Const.IsSet {
		metadata.Const, metadata.HasConst = cloneValue(current.Const.Value), true
	}

	metadata.Default = cloneValue(current.Default)

	metadata.Deprecated = current.Deprecated != nil && *current.Deprecated

	return metadata
}

// cloneValue copies JSON objects and arrays without converting numeric types.
func cloneValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		result := maps.Clone(value)
		for key, child := range result {
			result[key] = cloneValue(child)
		}

		return result
	case []any:
		result := slices.Clone(value)
		for index, child := range result {
			result[index] = cloneValue(child)
		}

		return result
	default:
		return value
	}
}
