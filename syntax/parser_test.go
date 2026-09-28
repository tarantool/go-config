package syntax //nolint:testpackage // Tests share fixtures that inspect parser, schema, and CST state.

import (
	"bytes"
	"context"
	"errors"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	sitter "github.com/odvcencio/gotreesitter"
	yamlgrammar "github.com/odvcencio/gotreesitter/grammars/yaml"
	"github.com/tarantool/go-config/v2/syntax/internal/cst"
)

func TestParserParse(t *testing.T) {
	t.Parallel()

	language := yamlgrammar.Language()

	parser, err := NewBuilder().WithJSONSchema([]byte(`{"type":"object"}`)).Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer parser.Close()

	// Preserve multibyte source text independently of the caller's buffer.
	original := "name: " + strings.Repeat("🚀", 1025) + "\n"
	source := []byte(original)

	doc, err := parser.Parse(context.Background(), source)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	defer doc.Close()

	if doc.cst.RootNode().HasError() {
		t.Fatal("valid YAML has syntax errors")
	}

	if got := doc.cst.RootNode().SExpr(language); !strings.Contains(got, "block_mapping_pair") {
		t.Fatalf("valid YAML was not parsed as a mapping: %s", got)
	}

	if end := doc.cst.RootNode().EndByte(); int(end) != len(source) {
		t.Fatalf("parsed %d bytes, want %d", end, len(source))
	}

	source[0] = '!'

	if !bytes.Equal(doc.Source(), []byte(original)) {
		t.Fatal("document source changed with caller's buffer")
	}

	copyOfSource := doc.Source()

	copyOfSource[0] = '!'

	if string(doc.Source()) != original {
		t.Fatal("Source exposed the owned source buffer")
	}

	incomplete, err := parser.Parse(context.Background(), []byte("name: [\n"))
	if err != nil {
		t.Fatalf("Parse(incomplete YAML) error = %v", err)
	}
	defer incomplete.Close()

	if !incomplete.cst.RootNode().HasError() {
		t.Fatalf("incomplete YAML has no recovery error: %s", incomplete.cst.RootNode().SExpr(language))
	}
}

func TestParserParseCanceled(t *testing.T) {
	t.Parallel()

	parser, err := NewBuilder().WithJSONSchema([]byte(`{}`)).Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer parser.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = parser.Parse(ctx, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Parse() error = %v, want context.Canceled", err)
	}
}

func TestParserParseCanceledDuringParsing(t *testing.T) {
	t.Parallel()

	parser, err := NewBuilder().WithJSONSchema([]byte("{}")).Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(parser.Close)

	// Build the input before starting the deadline so cancellation occurs
	// while handling a document rather than while preparing the test.
	source := []byte(strings.Repeat("- item\n", 100_000))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	partial, err := parser.Parse(ctx, source)
	if partial != nil {
		partial.Close()
		t.Fatal("Parse() returned a partial tree after cancellation")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Parse() error = %v, want context.DeadlineExceeded", err)
	}

	tree, err := parser.Parse(context.Background(), []byte("mode: prod"))
	if err != nil {
		t.Fatalf("Parse() after cancellation: %v", err)
	}
	defer tree.Close()

	if tree.cst.RootNode().HasError() {
		t.Fatal("parser retained state from the canceled document")
	}
}

func TestParserParseStoppedEarly(t *testing.T) {
	t.Parallel()

	parser, err := NewBuilder().WithJSONSchema([]byte(`{}`)).Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(parser.Close)

	parser.yaml.SetParseWorkLimits(sitter.ParseWorkLimits{IterationLimit: 1})

	partial, err := parser.Parse(t.Context(), []byte("mode: prod"))
	if partial != nil {
		partial.Close()
		t.Fatal("Parse() returned a partial tree after reaching a parser limit")
	}

	if !errors.Is(err, sitter.ErrParseStoppedEarly) {
		t.Fatalf("Parse() error = %v, want ErrParseStoppedEarly", err)
	}

	parser.yaml.SetParseWorkLimits(sitter.ParseWorkLimits{})

	tree, err := parser.Parse(t.Context(), []byte("mode: dev"))
	if err != nil {
		t.Fatalf("Parse() after reaching a parser limit: %v", err)
	}
	defer tree.Close()

	if tree.cst.RootNode().HasError() {
		t.Fatal("parser retained state from the incomplete parse")
	}
}

func TestParserParseAfterClose(t *testing.T) {
	t.Parallel()

	parser, err := NewBuilder().WithJSONSchema([]byte(`{}`)).Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	parser.Close()

	_, err = parser.Parse(context.Background(), nil)
	if !errors.Is(err, ErrClosedParser) {
		t.Fatalf("Parse() error = %v, want ErrClosedParser", err)
	}
}

//nolint:paralleltest // This regression test changes the process-wide GOMAXPROCS setting.
func TestParserParseAfterCompletedContextCancellation(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)

	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })

	parser, err := NewBuilder().WithJSONSchema([]byte("{}")).Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(parser.Close)

	// gotreesitter checks cancellation at parse entry, including short inputs.
	source := []byte("mode: dev")

	// Repeat to exercise callbacks scheduled near parse completion.
	for attempt := range 64 {
		ctx, cancel := context.WithCancel(context.Background())
		tree, err := parser.Parse(ctx, []byte("mode: prod"))

		cancel()

		if err != nil {
			t.Fatalf("Parse() before cancellation, attempt %d: %v", attempt, err)
		}

		tree.Close()

		// Allow a pending cancellation callback to run before parser reuse.
		runtime.Gosched()

		next, err := parser.Parse(context.Background(), source)
		if err != nil {
			t.Fatalf("Parse() after previous context cancellation, attempt %d: %v", attempt, err)
		}

		next.Close()
	}
}

func TestParserConcurrentParse(t *testing.T) {
	t.Parallel()

	language := yamlgrammar.Language()

	parser, err := NewBuilder().WithJSONSchema([]byte(`{}`)).Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(parser.Close)

	for index := range 32 {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			t.Parallel()

			source := []byte(strings.Repeat("- item\n", index+1))

			tree, err := parser.Parse(t.Context(), source)
			if err != nil {
				t.Fatal(err)
			}
			defer tree.Close()

			if tree.cst.RootNode().HasError() || !bytes.Equal(tree.Source(), source) {
				t.Fatal("concurrent Parse returned an invalid tree or another request's source")
			}

			sequence := cst.ContentNode(tree.cst.RootNode().NamedChild(0))
			if sequence == nil || sequence.Type(language) != "block_sequence" ||
				sequence.NamedChildCount() != index+1 {
				t.Fatalf("Parse returned an unexpected sequence: %s", tree.cst.RootNode().SExpr(language))
			}
		})
	}
}

func TestParserConcurrentClose(t *testing.T) {
	t.Parallel()

	parser, err := NewBuilder().WithJSONSchema([]byte(`{}`)).Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(parser.Close)

	source := []byte("mode: prod")

	original, err := parser.Parse(t.Context(), source)
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()

	start := make(chan struct{})

	var workers sync.WaitGroup

	for range 16 {
		workers.Go(func() {
			<-start

			tree, err := parser.Parse(t.Context(), source)
			if errors.Is(err, ErrClosedParser) {
				return
			}

			if err != nil {
				t.Errorf("Parse concurrent with Close: %v", err)
				return
			}

			defer tree.Close()

			if tree.cst.RootNode().HasError() || tree.cst.RootNode().Text(source) != string(source) {
				t.Error("Close invalidated a concurrently returned tree")
			}
		})
	}

	for range 4 {
		workers.Go(func() {
			<-start
			parser.Close()
		})
	}

	close(start)
	workers.Wait()

	_, err = parser.Parse(t.Context(), source)
	if !errors.Is(err, ErrClosedParser) {
		t.Fatalf("Parse after concurrent Close = %v, want ErrClosedParser", err)
	}

	if original.cst.RootNode().HasError() || original.cst.RootNode().Text(source) != string(source) {
		t.Fatal("Close invalidated a previously returned tree")
	}
}

func TestAbsentParser(t *testing.T) {
	t.Parallel()

	var parser *Parser

	parser.Close()

	tree, err := parser.Parse(t.Context(), nil)
	if !errors.Is(err, ErrClosedParser) || tree != nil {
		t.Errorf("nil Parser.Parse = %v, %v; want nil, ErrClosedParser", tree, err)
	}
}

func buildParser(t *testing.T) *Parser {
	t.Helper()

	parser, err := NewBuilder().WithJSONSchema([]byte(`{"type":"object"}`)).Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(parser.Close)

	return parser
}

func parseSource(t *testing.T, parser *Parser, source string) *Tree {
	t.Helper()

	tree, err := parser.Parse(t.Context(), []byte(source))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(tree.Close)

	return tree
}
