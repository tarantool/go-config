package config_test

import (
	"fmt"
	"strings"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/tree"
	"github.com/tarantool/go-config/validator"
	"github.com/tarantool/go-config/validators/jsonschema"
)

// Example_validation demonstrates JSON Schema validation of configuration.
func Example_validation() {
	// Define a simple JSON schema for server configuration.
	schema := []byte(`{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"server": {
				"type": "object",
				"properties": {
					"port": {
						"type": "integer",
						"minimum": 1024,
						"maximum": 65535
					},
					"host": {
						"type": "string",
						"pattern": "^[a-zA-Z0-9.-]+$"
					}
				},
				"required": ["port", "host"]
			}
		},
		"additionalProperties": false
	}`)

	// Create a JSON Schema validator.
	validator, err := jsonschema.New(schema)
	if err != nil {
		fmt.Printf("Failed to create validator: %v\n", err)
		return
	}

	// Valid configuration.
	validData := map[string]any{
		"server": map[string]any{
			"port": 8080,
			"host": "localhost",
		},
	}

	builder := config.NewBuilder()

	builder = builder.WithValidator(validator)
	builder = builder.AddCollector(collectors.NewMap(validData).WithName("valid"))

	cfg, errs := builder.Build()
	if len(errs) > 0 {
		fmt.Printf("Validation errors: %v\n", errs)
	} else {
		var port int

		var host string

		_, _ = cfg.Get(config.NewKeyPath("server/port"), &port)
		_, _ = cfg.Get(config.NewKeyPath("server/host"), &host)
		fmt.Printf("Valid configuration: host=%s port=%d\n", host, port)
	}

	// Invalid configuration (port out of range).
	invalidData := map[string]any{
		"server": map[string]any{
			"port": 80,
			"host": "localhost",
		},
	}

	builder = config.NewBuilder()
	builder = builder.WithValidator(validator)
	builder = builder.AddCollector(collectors.NewMap(invalidData).WithName("invalid"))

	_, errs = builder.Build()
	if len(errs) > 0 {
		fmt.Printf("Validation failed: %v\n", len(errs) > 0)
	}

	// Output:
	// Valid configuration: host=localhost port=8080
	// Validation failed: true
}

// requiredFieldValidator is a custom validator that checks for required fields.
type requiredFieldValidator struct{}

func (v *requiredFieldValidator) Validate(root *tree.Node) []validator.ValidationError {
	var errors []validator.ValidationError
	// Check that the root has a "service" field.
	if root.Get(config.NewKeyPath("service")) == nil {
		errors = append(errors, validator.ValidationError{
			Path:    config.NewKeyPath("service"),
			Range:   validator.NewEmptyRange(),
			Code:    "required",
			Message: "service configuration is required",
		})
	}

	return errors
}

func (v *requiredFieldValidator) SchemaType() string {
	return "custom"
}

// Example_customValidator demonstrates a custom validator that enforces business rules.
func Example_customValidator() {
	val := &requiredFieldValidator{}

	// Configuration missing required field.
	data := map[string]any{
		"port": 8080,
	}

	builder := config.NewBuilder()

	builder = builder.WithValidator(val)
	builder = builder.AddCollector(collectors.NewMap(data).WithName("config"))

	_, errs := builder.Build()
	if len(errs) > 0 {
		fmt.Printf("Missing service: %v\n", strings.Contains(errs[0].Error(), "service"))
	}

	// Configuration with required field.
	dataWithService := map[string]any{
		"service": map[string]any{
			"name": "api",
		},
		"port": 8080,
	}

	builder = config.NewBuilder()
	builder = builder.WithValidator(val)
	builder = builder.AddCollector(collectors.NewMap(dataWithService).WithName("config"))

	cfg, errs := builder.Build()
	if len(errs) > 0 {
		fmt.Printf("Unexpected errors: %v\n", errs)
	} else {
		var serviceName string

		_, _ = cfg.Get(config.NewKeyPath("service/name"), &serviceName)
		fmt.Printf("Service name: %s\n", serviceName)
	}

	// Output:
	// Missing service: true
	// Service name: api
}
