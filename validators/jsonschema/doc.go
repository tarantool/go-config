// Package jsonschema provides JSON Schema validation for go-config.
//
// This package implements the config.Validator interface using the
// kaptinlin/jsonschema library, supporting JSON Schema draft-2020-12
// and earlier drafts.
//
// # Basic Usage
//
//	schema := []byte(`{
//	    "type": "object",
//	    "properties": {
//	        "port": {"type": "integer", "minimum": 1, "maximum": 65535}
//	    },
//	    "required": ["port"]
//	}`)
//
//	validator, err := jsonschema.New(schema)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	cfg, errs := config.NewBuilder().
//	    AddCollector(myCollector).
//	    WithValidator(validator).
//	    Build()
//
// # Using WithJSONSchema Convenience Method
//
//	schemaFile, _ := os.Open("schema.json")
//	defer schemaFile.Close()
//
//	builder, err := config.NewBuilder().
//	    AddCollector(myCollector).
//	    WithJSONSchema(schemaFile)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	cfg, errs := builder.Build()
//
// # Error Handling
//
// Validation errors are returned as []config.ValidationError, each containing:
//   - Path: The KeyPath to the invalid field
//   - Code: Machine-readable error code (e.g., "type", "required", "minimum")
//   - Message: Human-readable error description
//   - Range: Source position (when position tracking is available)
package jsonschema
