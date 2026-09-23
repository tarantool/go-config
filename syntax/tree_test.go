package syntax //nolint:testpackage // Tests share fixtures that inspect parser, schema, and CST state.

import (
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/tarantool/go-config/v2/syntax/internal/cst"
	"github.com/tarantool/go-config/v2/syntax/internal/schema"
)

func TestTreeLifetime(t *testing.T) {
	t.Parallel()

	parser := buildParser(t)
	first := parseSource(t, parser, "mode: prod")
	second := parseSource(t, parser, "mode: dev")

	compiled := first.schema
	if compiled == nil || compiled != second.schema {
		t.Fatal("trees from the same parser must share the compiled schema")
	}

	first.Close()
	first.Close()
	parser.Close()
	parser.Close()

	if first.schema != compiled || schema.Type(compiled.Lookup(nil)) != "object" {
		t.Fatal("closing a tree or parser invalidated its schema")
	}

	if got := string(first.Source()); got != "mode: prod" {
		t.Fatalf("closed tree lost its source: %q", got)
	}

	location, err := cst.Locate(second.cst.RootNode(), second.source, second.lineStarts, sitter.Point{Column: 7})
	if err != nil {
		t.Fatal(err)
	}

	if got := location.Node.Content(second.Source()); got != "dev" {
		t.Fatalf("closing another tree or the parser affected the live tree: %q", got)
	}
}

func TestAbsentTree(t *testing.T) {
	t.Parallel()

	var tree *Tree

	tree.Close()

	if tree.Source() != nil {
		t.Error("nil tree returned source")
	}
}
