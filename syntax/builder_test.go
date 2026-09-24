package syntax

import (
	"context"
	"errors"
	"testing"
)

func TestBuilderBuild(t *testing.T) {
	schema := []byte(`{
		"$defs": {"entry": {"type": "string"}},
		"properties": {"name": {"$ref": "#/$defs/entry"}}
	}`)
	builder := NewBuilder().WithJSONSchema(schema)
	schema[0] = '!'

	parser, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	defer parser.Close()

	if parser.schema == nil || parser.yaml == nil {
		t.Fatal("Build() did not configure both parsers")
	}

	property := (*parser.schema.Properties)["name"]
	if property == nil || property.ResolvedRef == nil {
		t.Fatal("Build() did not resolve local schema reference")
	}
}

func TestBuilderBuildErrors(t *testing.T) {
	_, err := NewBuilder().Build(context.Background())
	if !errors.Is(err, ErrNoJSONSchema) {
		t.Fatalf("Build() error = %v, want ErrNoJSONSchema", err)
	}

	_, err = NewBuilder().WithJSONSchema([]byte(`{invalid`)).Build(context.Background())
	if err == nil {
		t.Fatal("Build() accepted malformed JSON Schema")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = NewBuilder().WithJSONSchema([]byte(`{}`)).Build(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Build() error = %v, want context.Canceled", err)
	}
}
