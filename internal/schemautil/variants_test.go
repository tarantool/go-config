package schemautil_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/kaptinlin/jsonschema"
	"github.com/tarantool/go-config/v2/internal/schemautil"
)

func TestVariantsGraph(t *testing.T) {
	t.Parallel()

	root := &jsonschema.Schema{}
	reference := &jsonschema.Schema{ResolvedRef: root}
	dynamic := &jsonschema.Schema{ResolvedRef: reference}
	shared := &jsonschema.Schema{}
	intersection := &jsonschema.Schema{AnyOf: []*jsonschema.Schema{shared}}
	alternative := &jsonschema.Schema{OneOf: []*jsonschema.Schema{shared}}
	exclusive := &jsonschema.Schema{}

	root.ResolvedRef = reference
	root.ResolvedDynamicRef = dynamic
	root.AllOf = []*jsonschema.Schema{intersection}
	root.AnyOf = []*jsonschema.Schema{alternative, nil}
	root.OneOf = []*jsonschema.Schema{exclusive, root}

	want := []*jsonschema.Schema{root, reference, dynamic, intersection, shared, alternative, exclusive}
	if got := schemautil.Variants(root); !slices.Equal(got, want) {
		t.Fatalf("Variants = %p, want %p in traversal order without repeated nodes", got, want)
	}

	if got := schemautil.Variants(nil); len(got) != 0 {
		t.Fatalf("Variants(nil) = %v, want no variants", got)
	}
}

func TestBranchesPreservesSource(t *testing.T) {
	t.Parallel()

	first, second, third := &jsonschema.Schema{}, &jsonschema.Schema{}, &jsonschema.Schema{}
	// Extra capacity detects accidental append into the source's backing array.
	backing := []*jsonschema.Schema{first, nil, nil}
	root := &jsonschema.Schema{
		AllOf: backing[:1], AnyOf: []*jsonschema.Schema{second}, OneOf: []*jsonschema.Schema{third},
		ResolvedRef: &jsonschema.Schema{}, ResolvedDynamicRef: &jsonschema.Schema{},
	}

	branches := schemautil.Branches(root)
	if !slices.Equal(branches, []*jsonschema.Schema{first, second, third}) {
		t.Fatalf("Branches = %p, want only allOf, anyOf and oneOf in order", branches)
	}

	branches[0] = nil

	if !slices.Equal(backing, []*jsonschema.Schema{first, nil, nil}) {
		t.Fatal("Branches modified the source slice or exposed its backing array")
	}
}

func TestReferenceDialects(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		dialect   string
		overrides bool
	}{
		{"http://json-schema.org/draft-04/schema#", true},
		{"http://json-schema.org/draft-06/schema#", true},
		{"http://json-schema.org/draft-07/schema#", true},
		{"https://json-schema.org/draft/2019-09/schema", false},
		{"https://json-schema.org/draft/2020-12/schema", false},
	} {
		t.Run(test.dialect, func(t *testing.T) {
			t.Parallel()

			source := strings.ReplaceAll(`{
				"$schema":"DIALECT", "$ref":"#/definitions/value",
				"allOf":[{"type":"boolean"}],
				"definitions":{"value":{"type":"string"}}
			}`, "DIALECT", test.dialect)

			compiled, err := jsonschema.NewCompiler().Compile([]byte(source))
			if err != nil {
				t.Fatal(err)
			}

			if got := schemautil.RefOverridesSiblings(compiled); got != test.overrides {
				t.Errorf("RefOverridesSiblings = %v, want %v", got, test.overrides)
			}

			want := []*jsonschema.Schema{compiled.ResolvedRef}
			if !test.overrides {
				want = []*jsonschema.Schema{compiled, compiled.ResolvedRef, compiled.AllOf[0]}
			}

			if got := schemautil.Variants(compiled); !slices.Equal(got, want) {
				t.Errorf("Variants = %p, want %p", got, want)
			}

			if schemautil.RefOverridesSiblings(compiled.ResolvedRef) {
				t.Error("schema without a reference must preserve its own keywords")
			}
		})
	}
}
