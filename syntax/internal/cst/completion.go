package cst

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/kaptinlin/jsonschema"
	sitter "github.com/odvcencio/gotreesitter"
	yamlgrammar "github.com/odvcencio/gotreesitter/grammars/yaml"
	"github.com/tarantool/go-config/v2/internal/schemautil"
	"github.com/tarantool/go-config/v2/syntax/internal/path"
	"github.com/tarantool/go-config/v2/syntax/internal/schema"
	"go.yaml.in/yaml/v3"
)

// DefaultIndent is the indentation used when inserting a new block container.
const DefaultIndent = 2

// CompletionItem contains a plain-text YAML edit in the original source.
// Replace is single-line and uses byte columns.
// Type is empty when the schema type is unknown or ambiguous.
type CompletionItem struct {
	Label      string // Displayed property name or value, without YAML quoting.
	Type       string // JSON type; empty when unknown or ambiguous. Numeric values use "number".
	InsertText string // Plain text, including any required separators or indentation.
	Replace    Range  // Half-open, single-line range in the original UTF-8 source.
}

// CompletionContext describes a completion replacement and its surrounding YAML syntax.
// Its range and indentation refer to the original source.
type CompletionContext struct {
	Context

	Prefix   string       // Decoded token text before the cursor, used to filter candidates.
	Start    sitter.Point // Start of the edit in the original source.
	End      sitter.Point // End of the edit in the original source.
	Indent   int          // Byte column used to indent children of a newly inserted container.
	Before   string       // Required separator: space or newline with indentation.
	After    string       // Closing delimiter or separator required after the insertion.
	HasColon bool         // The edited key already has its colon.
	QuoteKey bool         // A flow value directly after the colon requires a quoted key.
}

// lastPairBefore finds the last mapping pair starting at or before the cursor.
// Its value may span the cursor line; comments do not count as pairs.
func lastPairBefore(mapping *sitter.Node, offset int) *sitter.Node {
	language := yamlgrammar.Language()

	var last *sitter.Node

	for index := range mapping.NamedChildCount() {
		child := mapping.NamedChild(index)
		if child.ChildByFieldName("key", language) != nil && int(child.StartByte()) <= offset {
			last = child
		}
	}

	return last
}

// CompletionAt returns property and scalar suggestions at the cursor.
// Unsupported contexts and values without finite candidates produce no suggestions.
// Invalid source coordinates return an error.
func CompletionAt(
	root *sitter.Node, source []byte, starts []int, compiled *schema.Schema, pos sitter.Point,
) ([]CompletionItem, error) {
	location, err := Locate(root, source, starts, pos)
	if err != nil {
		return nil, err
	}

	context, ok := completionEdit(source, starts, compiled, location, pos)
	if !ok {
		return nil, nil
	}

	if context.IsKey && context.Current != nil {
		colon := ChildOfType(context.Current, ":")

		context.HasColon = colon != nil
		// JSON-style flow pairs allow an adjacent value after a quoted key.
		// Keep the key quoted without changing its colon or value.
		context.QuoteKey = colon != nil && context.Flow && int(colon.EndByte()) < len(source) &&
			!strings.ContainsRune(" \t\r\n,}]", rune(source[colon.EndByte()]))
	}

	var items []CompletionItem

	if context.IsKey {
		// An empty block-key edit must not join a new pair to existing content.
		if !context.Flow && context.Start == context.End {
			start := starts[pos.Row] + int(pos.Column)

			suffix := bytes.TrimSpace(source[start:LineEnd(source, starts, pos.Row)])
			if len(suffix) > 0 && suffix[0] != '#' {
				return nil, nil
			}
		}

		items = keyCompletions(source, compiled, context)
	} else {
		items = valueCompletions(compiled, context)
	}

	// Both new values and replaced tokens must stay separated from a comment.
	end := starts[pos.Row] + int(context.End.Column)
	if end < LineEnd(source, starts, pos.Row) && source[end] == '#' {
		for index := range items {
			if !strings.HasSuffix(items[index].InsertText, " ") {
				items[index].InsertText += " "
			}
		}
	}

	return items, nil
}

// completionEdit selects the replacement range and separators at a located cursor.
func completionEdit(
	source []byte, starts []int, compiled *schema.Schema, location Location, pos sitter.Point,
) (CompletionContext, bool) {
	language := yamlgrammar.Language()

	var empty CompletionContext

	node, offset := location.Node, location.Offset

	// Leading whitespace can precede the first document's CST range.
	if node.Type(language) == "stream" {
		for index := range node.NamedChildCount() {
			child := node.NamedChild(index)
			if child.Type(language) == "document" {
				if offset < int(child.StartByte()) {
					node = ContentNode(child)
				}

				break
			}
		}
	}

	for current := node; current != nil; current = current.Parent() {
		if current.Type(language) == nodeComment && offset == int(current.StartByte()) {
			continue
		}

		if (current.Type(language) == nodeComment || current.Type(language) == "alias" ||
			current.Type(language) == "block_scalar" || current.Type(language) == "anchor") &&
			(location.Contains(current) || offset == int(current.EndByte())) {
			return empty, false
		}

		if IsScalar(current) && (location.Contains(current) || offset == int(current.EndByte())) {
			context := CompletionContext{
				Context:  NodeContext(source, compiled, current),
				Prefix:   "",
				Start:    pos,
				End:      pos,
				Indent:   0,
				Before:   "",
				After:    "",
				HasColon: false,
				QuoteKey: false,
			}
			if context.Separator != nil {
				context.Before = " "
			}

			context, ok := tokenContext(source, starts, context, current, pos, offset)

			return context, ok
		}
	}

	for current := node; current != nil; current = current.Parent() {
		switch current.Type(language) {
		case nodeBlockMappingPair, nodeFlowPair:
			colon := ChildOfType(current, ":")
			value := ContentNode(current.ChildByFieldName("value", language))
			key := current.ChildByFieldName("key", language)

			if key != nil && IsScalar(value) && pos.Row > value.EndPoint().Row &&
				pos.Column > current.StartPoint().Column &&
				!compiled.HasObjectSchema(FieldPath(source, NodeContext(source, compiled, current))) {
				return empty, false
			}

			//nolint:nestif // Keep the same-line value and its decoration checks together.
			if colon != nil && pos.Row == colon.EndPoint().Row && offset >= int(colon.EndByte()) {
				context := valueContext(source, compiled, current, pos, offset)
				if value == nil {
					if decorated := current.ChildByFieldName("value", language); decorated != nil &&
						offset < int(decorated.EndByte()) {
						return empty, false
					}

					return context, true
				}

				if IsScalar(value) && value.StartPoint().Row == pos.Row {
					context.IsKey = false

					context.Before, context.After = "", ""
					if offset == int(colon.EndByte()) {
						context.Before = " "
					}

					context, ok := tokenContext(source, starts, context, value, pos, offset)

					return context, ok
				}

				return empty, false
			}
		case "block_sequence_item":
			dash := ChildOfType(current, "-")
			if dash == nil || offset < int(dash.EndByte()) || pos.Column < dash.StartPoint().Column {
				continue
			}

			value := ContentNode(current)
			if pos.Row > dash.StartPoint().Row && (pos.Column == dash.StartPoint().Column ||
				IsScalar(value)) {
				return empty, false
			}

			if value == nil {
				if offset < int(current.EndByte()) {
					return empty, false
				}

				context := emptyCompletionContext(compiled, NodeContext(source, compiled, current).Path, pos)

				if offset == int(dash.EndByte()) {
					context.Before = " "
					context.Indent++
				}

				return context, true
			}
		case nodeFlowMapping, nodeFlowSequence:
			if offset > int(current.StartByte()) && offset < int(current.EndByte()) {
				context, ok := flowContext(source, starts, compiled, current, pos, offset)
				return context, ok
			}

			if offset >= int(current.EndByte()) && len(NodeContext(source, compiled, current).Path) == 0 {
				return empty, false
			}
		case nodeBlockMapping:
			//nolint:nestif // Indentation selects the current mapping or its pending child value.
			if pos.Column >= current.StartPoint().Column {
				mapping := current
				if pair := lastPairBefore(mapping, offset); pair != nil {
					key := pair.ChildByFieldName("key", language)

					value := ContentNode(pair.ChildByFieldName("value", language))
					if key != nil && pos.Column > pair.StartPoint().Column {
						if value == nil {
							return valueContext(source, compiled, pair, pos, offset), true
						}

						if value.Type(language) == nodeBlockMapping && offset < int(value.StartByte()) &&
							pos.Column >= value.StartPoint().Column {
							mapping = value
						}
					}
				}

				if pos.Column != mapping.StartPoint().Column {
					return empty, false
				}

				context := emptyCompletionContext(compiled, NodeContext(source, compiled, mapping).Path, pos)

				context.IsKey = true
				context.Mapping = mapping

				return context, true
			}
		case nodeError:
			scope, token := ErrorContext(source, compiled, current, NodeContext(source, compiled, current), pos, offset)

			context := CompletionContext{
				Context:  scope,
				Start:    pos,
				End:      pos,
				Indent:   int(pos.Column),
				Prefix:   "",
				Before:   "",
				After:    "",
				HasColon: false,
				QuoteKey: false,
			}
			if scope.Separator != nil {
				context.Before = " "
			}

			if token != nil {
				context, ok := tokenContext(source, starts, context, token, pos, offset)
				return context, ok
			}

			// Recovery may leave source bytes outside its recognized children.
			// A missing token is an insertion point only in separator whitespace.
			end := min(offset, int(current.StartByte()))

			var previous string

			for index := range current.ChildCount() {
				child := current.Child(index)
				if int(child.EndByte()) > offset {
					break
				}

				end = int(child.EndByte())
				if child.Type(language) != nodeComment {
					previous = child.Type(language)
				}
			}

			if strings.TrimSpace(string(source[end:offset])) != "" {
				return empty, false
			}

			// A missing flow value can be an object even when its braces have
			// not been typed yet, including an item in an unfinished sequence.
			if !context.IsKey && context.Flow && (previous == "[" || previous == "," || previous == ":") &&
				compiled.HasObjectSchema(context.Path) {
				context.IsKey = true

				context.Before += "{"

				context.After = "}"
			}

			return context, true
		}
	}

	if strings.TrimSpace(string(source[starts[pos.Row]:offset])) != "" {
		return empty, false
	}

	return emptyCompletionContext(compiled, nil, pos), true
}

// emptyCompletionContext creates a zero-width edit at pos using the schema role at path.
func emptyCompletionContext(compiled *schema.Schema, steps []path.Step, pos sitter.Point) CompletionContext {
	return CompletionContext{
		Context:  ContextAtPath(compiled, steps),
		Start:    pos,
		End:      pos,
		Indent:   int(pos.Column),
		Prefix:   "",
		Before:   "",
		After:    "",
		HasColon: false,
		QuoteKey: false,
	}
}

// valueContext selects the schema under a mapping key and supplies the space
// or newline needed to insert its value after the colon.
func valueContext(
	source []byte, compiled *schema.Schema, pair *sitter.Node, pos sitter.Point, offset int,
) CompletionContext {
	parent := NodeContext(source, compiled, pair)
	context := emptyCompletionContext(compiled, FieldPath(source, parent), pos)

	context.Flow = parent.Flow

	colon := ChildOfType(pair, ":")

	switch {
	case context.IsKey && context.Flow:
		context.Before, context.After = "{", "}"
		if colon != nil && offset == int(colon.EndByte()) {
			context.Before = " {"
		}
	case context.IsKey && colon != nil && pos.Row == colon.EndPoint().Row:
		context.Indent = int(pair.StartPoint().Column) + DefaultIndent
		context.Before = Newline(source) + strings.Repeat(" ", context.Indent)
	case colon != nil && offset == int(colon.EndByte()):
		context.Before = " "
	}

	return context
}

// tokenContext selects the editable span of a CST scalar or unfinished quote.
// It rejects multiline values; incomplete plain keys may use only their first line.
func tokenContext(
	source []byte, starts []int, context CompletionContext, node *sitter.Node, pos sitter.Point, offset int,
) (CompletionContext, bool) {
	language := yamlgrammar.Language()

	var empty CompletionContext

	if node.StartPoint().Row != pos.Row ||
		node.Type(language) == "block_scalar" || node.Type(language) == nodeComment {
		return empty, false
	}

	if node.EndPoint().Row != pos.Row && (!context.IsKey || node.Type(language) != "string_scalar") {
		return empty, false
	}

	end := int(node.EndByte())
	if node.Type(language) == "\"" || node.Type(language) == "'" {
		end = LineEnd(source, starts, pos.Row)
	}

	return editToken(source, starts, context, int(node.StartByte()), end, pos, offset)
}

// editToken decodes the prefix and builds a single-line range containing the cursor.
// It can absorb separator whitespace but cannot cross anchors or other
// content. Spaces inside quoted values remain significant for prefix filtering.
func editToken(
	source []byte, starts []int, context CompletionContext, start, end int, pos sitter.Point, offset int,
) (CompletionContext, bool) {
	var empty CompletionContext

	end = min(end, LineEnd(source, starts, pos.Row))

	if offset < start && strings.TrimSpace(string(source[offset:start])) != "" ||
		offset > end && strings.TrimSpace(string(source[end:offset])) != "" {
		return empty, false
	}

	// Decode only the token; separator whitespace belongs to the edit range,
	// while whitespace inside a quoted value remains part of the prefix.
	raw := string(source[start:end])
	prefix := string(source[start:max(start, min(offset, end))])

	if len(raw) > 0 && (raw[0] == '\'' || raw[0] == '"') {
		quote := raw[:1]

		if len(prefix) > 0 {
			prefix = prefix[1:]
		}

		if offset >= end && len(raw) > 1 && strings.HasSuffix(raw, quote) {
			prefix = strings.TrimSuffix(prefix, quote)
		}

		var decoded string

		err := yaml.Unmarshal([]byte(quote+prefix+quote), &decoded)
		if err != nil {
			return empty, false
		}

		prefix = decoded
	} else {
		prefix = strings.TrimRight(prefix, " \t")
	}

	start = min(start, offset)
	end = max(end, offset)
	context.Prefix = prefix
	//nolint:gosec // Bounded by syntax tree coordinates.
	context.Start = sitter.Point{Row: pos.Row, Column: uint32(start - starts[pos.Row])}
	//nolint:gosec // Bounded by syntax tree coordinates.
	context.End = sitter.Point{Row: pos.Row, Column: uint32(end - starts[pos.Row])}
	context.Indent = int(context.Start.Column)

	return context, true
}

// flowContext locates the comma-delimited slot containing the cursor.
// An occupied scalar is replaced; an occupied container or a position before
// an existing pair cannot accept another item without a separator.
func flowContext(
	source []byte, starts []int, compiled *schema.Schema, node *sitter.Node, pos sitter.Point, offset int,
) (CompletionContext, bool) {
	language := yamlgrammar.Language()

	var empty CompletionContext

	context := emptyCompletionContext(compiled, NodeContext(source, compiled, node).Path, pos)

	context.Flow = true
	context.Mapping = node

	// A slot extends between commas, including whitespace around its content.
	var occupied *sitter.Node

	index := 0

	for childIndex := range node.ChildCount() {
		child := node.Child(childIndex)
		if child.Type(language) == "," {
			if int(child.EndByte()) > offset {
				break
			}

			index++

			occupied = nil
		} else if child.IsNamed() && child.Type(language) != nodeComment {
			occupied = child
		}
	}

	//nolint:nestif // Flow pairs and bare keys share the same occupied slot.
	if node.Type(language) == nodeFlowMapping {
		context.IsKey = true

		if occupied != nil {
			if occupied.Type(language) == nodeFlowPair {
				if offset < int(occupied.StartByte()) {
					return empty, false
				}

				context = valueContext(source, compiled, occupied, pos, offset)
				occupied = occupied.ChildByFieldName("value", language)
			} else {
				context.Current = occupied
			}
		}
	} else {
		context.Path = append(context.Path, path.Index(index))
		context.IsKey = compiled.HasObjectSchema(context.Path)

		context.Mapping = nil
		if context.IsKey {
			context.Before, context.After = "{", "}"
		}
	}

	if content := ContentNode(occupied); content != nil {
		if IsScalar(content) {
			return tokenContext(source, starts, context, content, pos, offset)
		}

		return empty, false
	}

	return context, true
}

// keyCompletions enumerates explicit schema properties, filters existing keys
// and forbidden paths, and formats the key with its colon and optional default.
func keyCompletions(source []byte, compiled *schema.Schema, context CompletionContext) []CompletionItem {
	replace := Range{
		Start: Position{Line: context.Start.Row, ByteColumn: context.Start.Column},
		End:   Position{Line: context.End.Row, ByteColumn: context.End.Column},
	}

	used := ExistingKeys(source, compiled, context.Context)
	names := make(map[string]bool)

	for _, candidate := range compiled.Lookup(context.Path) {
		for _, variant := range schemautil.Variants(candidate) {
			if variant.Properties != nil {
				for name := range *variant.Properties {
					names[name] = true
				}
			}
		}
	}

	var items []CompletionItem

	for name := range names {
		steps := append(slices.Clone(context.Path), path.Property(name))
		if used[name] || !strings.HasPrefix(name, context.Prefix) || !compiled.AcceptsAt(steps, nil, false) {
			continue
		}

		schemas := compiled.Lookup(steps)
		item := CompletionItem{Label: name, Type: schema.Type(schemas), InsertText: "", Replace: replace}

		insert, ok := scalarText(name)

		if !ok {
			continue
		}

		if context.QuoteKey {
			insert = strconv.Quote(name)
		}

		if !context.HasColon {
			insert += propertyValue(source, compiled, context, item.Type, steps, schemas)
		}

		item.InsertText = context.Before + insert + context.After
		items = append(items, item)
	}

	return sortItems(items)
}

// propertyValue supplies separators, container delimiters, and the first valid default.
func propertyValue(
	source []byte, compiled *schema.Schema, context CompletionContext,
	itemType string, steps []path.Step, schemas []*jsonschema.Schema,
) string {
	if itemType == schema.TypeObject || itemType == schema.TypeArray {
		if !context.Flow {
			return ":" + Newline(source) + strings.Repeat(" ", context.Indent+DefaultIndent)
		}

		if itemType == schema.TypeArray {
			return ": []"
		}

		return ": {}"
	}

	for _, candidate := range schemas {
		for _, variant := range schemautil.Variants(candidate) {
			if variant.Default == nil || !compiled.AcceptsAt(steps, variant.Default, true) {
				continue
			}

			if text, ok := scalarText(variant.Default); ok {
				return ": " + text
			}
		}
	}

	return ": "
}

// valueCompletions enumerates scalar enums, constants, defaults, booleans, and
// null. It validates candidates and deduplicates their encoded YAML text so
// strings such as "true" remain distinct from boolean true.
func valueCompletions(compiled *schema.Schema, context CompletionContext) []CompletionItem {
	replace := Range{
		Start: Position{Line: context.Start.Row, ByteColumn: context.Start.Column},
		End:   Position{Line: context.End.Row, ByteColumn: context.End.Column},
	}

	schemas := compiled.Lookup(context.Path)
	byValue := make(map[string]CompletionItem)

	for _, candidate := range schemas {
		for _, variant := range schemautil.Variants(candidate) {
			values := slices.Clone(variant.Enum)
			if variant.Const != nil && variant.Const.IsSet {
				values = append(values, variant.Const.Value)
			}

			if variant.Default != nil {
				values = append(values, variant.Default)
			}

			if slices.Contains(variant.Type, "boolean") {
				values = append(values, false, true)
			}

			if slices.Contains(variant.Type, "null") {
				values = append(values, nil)
			}

			for _, value := range values {
				insert, ok := scalarText(value)
				if !ok || !compiled.AcceptsAt(context.Path, value, true) {
					continue
				}

				label := fmt.Sprint(value)
				if value == nil {
					label = "null"
				}

				if !strings.HasPrefix(label, context.Prefix) {
					continue
				}

				byValue[insert] = CompletionItem{
					Label: label, Type: scalarType(value),
					InsertText: context.Before + insert + context.After, Replace: replace,
				}
			}
		}
	}

	items := make([]CompletionItem, 0, len(byValue))
	for _, item := range byValue {
		items = append(items, item)
	}

	return sortItems(items)
}

// scalarType classifies supported JSON scalar values for CompletionItem.Type.
// Numeric values use "number"; compound and unsupported Go values return empty.
func scalarType(value any) string {
	switch value.(type) {
	case string:
		return schema.TypeString
	case bool:
		return "boolean"
	case nil:
		return "null"
	case json.Number, int, int64, uint64, float64, float32:
		return "number"
	default:
		return ""
	}
}

// scalarText encodes a single-line YAML scalar safe in block and flow styles.
// It preserves json.Number literals and quotes strings with flow delimiters or
// line breaks. Compound values cannot be inserted by scalar completion.
func scalarText(value any) (string, bool) {
	if scalarType(value) == "" {
		return "", false
	}

	if number, ok := value.(json.Number); ok {
		return number.String(), true
	}

	if text, ok := value.(string); ok && strings.ContainsAny(text, ",[]{}\r\n") {
		encoded, err := json.Marshal(text)
		return string(encoded), err == nil
	}

	encoded, err := yaml.Marshal(value)

	return strings.TrimSuffix(string(encoded), "\n"), err == nil
}

// sortItems orders candidates by label, then type, preserving distinct scalar types.
func sortItems(items []CompletionItem) []CompletionItem {
	slices.SortFunc(items, func(first, second CompletionItem) int {
		if order := cmp.Compare(first.Label, second.Label); order != 0 {
			return order
		}

		return cmp.Compare(first.Type, second.Type)
	})

	return items
}
