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
	schema   *jsonschema.Schema
	compiler *jsonschema.Compiler
}

// New creates a validator from schema bytes.
func New(schemaData []byte) (*Validator, error) {
	compiler := jsonschema.NewCompiler()

	schema, err := compiler.Compile(schemaData)
	if err != nil {
		return nil, fmt.Errorf("failed to compile JSON schema: %w", err)
	}

	return &Validator{schema: schema, compiler: compiler}, nil
}

// NewFromReader creates a validator from an io.Reader.
func NewFromReader(r io.Reader) (*Validator, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema: %w", err)
	}

	return New(data)
}

// Validate implements validator.Validator.
func (v *Validator) Validate(root *tree.Node) []validator.ValidationError {
	data := tree.ToAny(root)

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
