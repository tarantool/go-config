package jsonschema

import (
	"fmt"
	"io"
	"slices"

	"github.com/kaptinlin/jsonschema"

	"github.com/tarantool/go-config/v2/keypath"
	"github.com/tarantool/go-config/v2/tree"
	"github.com/tarantool/go-config/v2/validator"
)

// Validator validates configuration against JSON Schema.
type Validator struct {
	schema     *jsonschema.Schema
	compiler   *jsonschema.Compiler
	nullCoerce NullCoercion
}

// Option configures a Validator.
type Option func(*Validator)

// WithNullCoercion sets the scalar-null coercion policy for this validator,
// overriding DefaultNullCoercion. See NullCoercion for details.
func WithNullCoercion(policy NullCoercion) Option {
	return func(v *Validator) {
		v.nullCoerce = policy
	}
}

// New creates a validator from schema bytes.
func New(schemaData []byte, opts ...Option) (*Validator, error) {
	compiler := jsonschema.NewCompiler()

	schema, err := compiler.Compile(schemaData)
	if err != nil {
		return nil, fmt.Errorf("failed to compile JSON schema: %w", err)
	}

	v := &Validator{schema: schema, compiler: compiler, nullCoerce: DefaultNullCoercion}
	for _, opt := range opts {
		opt(v)
	}

	return v, nil
}

// NewFromReader creates a validator from an io.Reader.
func NewFromReader(r io.Reader, opts ...Option) (*Validator, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema: %w", err)
	}

	return New(data, opts...)
}

// Validate implements validator.Validator.
func (v *Validator) Validate(root *tree.Node) []validator.ValidationError {
	data := tree.ToAny(root)

	data = coerceNulls(data, v.schema, v.nullCoerce)
	data = coerceScalars(data, v.schema)

	result := v.schema.Validate(data)
	if result.IsValid() {
		return nil
	}

	ve := validationErrors{root: root, errors: nil}
	ve.collectErrorsFromPath(result, "")

	return dropSummaries(ve.All())
}

// isSummary reports whether keyword applies a subschema to part of the
// instance and only reports that it failed; the failure itself is reported
// where it happened.
func isSummary(keyword string) bool {
	switch keyword {
	case "$ref", "$dynamicRef", "allOf", "properties", "patternProperties", "additionalProperties",
		"dependentSchemas", "items", "prefixItems", "contains", "unevaluatedProperties", "unevaluatedItems",
		"then", "else":
		return true
	default:
		return false
	}
}

// dropSummaries removes the errors of summary keywords that another error
// explains: any error below their path, or one of another keyword at it. One
// bad value is then reported once and not again at every map above it, and a
// summary nothing explains is kept.
func dropSummaries(errs []validator.ValidationError) []validator.ValidationError {
	kept := make([]validator.ValidationError, 0, len(errs))

	for _, err := range errs {
		if !isSummary(err.Code) || !explained(err.Path, errs) {
			kept = append(kept, err)
		}
	}

	return kept
}

func explained(path keypath.KeyPath, errs []validator.ValidationError) bool {
	for _, other := range errs {
		if len(other.Path) < len(path) || !slices.Equal(other.Path[:len(path)], path) {
			continue
		}

		if len(other.Path) > len(path) || !isSummary(other.Code) {
			return true
		}
	}

	return false
}

// SchemaType returns JSONSchema string.
func (v *Validator) SchemaType() string {
	return "json-schema"
}
