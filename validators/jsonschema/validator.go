package jsonschema

import (
	"fmt"
	"io"

	"github.com/kaptinlin/jsonschema"

	"github.com/tarantool/go-config/tree"
	"github.com/tarantool/go-config/validator"
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

	result := v.schema.Validate(data)
	if result.IsValid() {
		return nil
	}

	ve := validationErrors{root: root, errors: nil}
	ve.collectErrorsFromPath(result, "")

	return ve.All()
}

// SchemaType returns JSONSchema string.
func (v *Validator) SchemaType() string {
	return "json-schema"
}
