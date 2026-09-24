package syntax

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestParserParse(t *testing.T) {
	parser, err := NewBuilder().WithJSONSchema([]byte(`{"type":"object"}`)).Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer parser.Close()

	source := []byte("name: alice\n")
	doc, err := parser.Parse(context.Background(), source)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	defer doc.Close()
	if doc.cst.RootNode().HasError() {
		t.Fatal("valid YAML has syntax errors")
	}
	if got := doc.cst.RootNode().String(); !strings.Contains(got, "block_mapping_pair") {
		t.Fatalf("valid YAML was not parsed as a mapping: %s", got)
	}
	source[0] = '!'
	if !bytes.Equal(doc.Source(), []byte("name: alice\n")) {
		t.Fatal("document source changed with caller's buffer")
	}

	incomplete, err := parser.Parse(context.Background(), []byte("name: [\n"))
	if err != nil {
		t.Fatalf("Parse(incomplete YAML) error = %v", err)
	}
	defer incomplete.Close()
	if !incomplete.cst.RootNode().HasError() {
		t.Fatalf("incomplete YAML has no recovery error: %s", incomplete.cst.RootNode().String())
	}
}

func TestParserParseCanceled(t *testing.T) {
	parser, err := NewBuilder().WithJSONSchema([]byte(`{}`)).Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer parser.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := parser.Parse(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Parse() error = %v, want context.Canceled", err)
	}
}

func TestParserParseAfterClose(t *testing.T) {
	parser, err := NewBuilder().WithJSONSchema([]byte(`{}`)).Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	parser.Close()
	if _, err := parser.Parse(context.Background(), nil); !errors.Is(err, ErrClosedParser) {
		t.Fatalf("Parse() error = %v, want ErrClosedParser", err)
	}
}
