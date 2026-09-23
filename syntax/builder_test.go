package syntax //nolint:testpackage // Tests share fixtures that inspect parser, schema, and CST state.

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildCanceledWhileResolvingSchema(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		cancel()

		_, err := io.WriteString(writer, `{"type":"object"}`)
		if err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)

	builder := NewBuilder().WithJSONSchema([]byte(`{"$ref":"` + server.URL + `/schema.json"}`))

	parser, err := builder.Build(ctx)
	if parser != nil {
		parser.Close()
		t.Fatal("Build returned a parser after cancellation while resolving a reference")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Build error = %v, want context.Canceled", err)
	}

	parser, err = builder.Build(t.Context())
	if err != nil {
		t.Fatalf("Build after cancellation: %v", err)
	}

	t.Cleanup(parser.Close)

	tree := parseSource(t, parser, "mode: prod")
	if tree.schema == nil {
		t.Fatal("rebuilt parser has no schema")
	}
}

func TestBuilderBuild(t *testing.T) {
	t.Parallel()

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

	property := (*parser.schema.Lookup(nil)[0].Properties)["name"]
	if property == nil || property.ResolvedRef == nil {
		t.Fatal("Build() did not resolve local schema reference")
	}
}

func TestBuilderBuildErrors(t *testing.T) {
	t.Parallel()

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

func TestBuilderBuildRejectsUnresolvedReferences(t *testing.T) {
	t.Parallel()

	for _, source := range []string{
		`{"$ref":"#/$defs/missing"}`,
		`{"properties":{"mode":{"$ref":"#/$defs/missing"}}}`,
		`{"properties":{"mode":{"$dynamicRef":"#missing"}}}`,
	} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()

			parser, err := NewBuilder().WithJSONSchema([]byte(source)).Build(context.Background())
			if parser != nil {
				parser.Close()
			}

			if !errors.Is(err, ErrUnresolvedReference) {
				t.Fatalf("Build() error = %v; want unresolved reference error", err)
			}
		})
	}
}

func TestAbsentBuilder(t *testing.T) {
	t.Parallel()

	var builder *Builder

	_, err := builder.Build(t.Context())
	if !errors.Is(err, ErrNoJSONSchema) {
		t.Errorf("nil Builder.Build = %v, want ErrNoJSONSchema", err)
	}
}
