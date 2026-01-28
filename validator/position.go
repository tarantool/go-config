package validator

// Position describes a position in source file (for LSP integration).
// Currently placeholder - will be populated when position tracking is implemented.
type Position struct {
	Line   int // Line number (1-based), 0 if unknown.
	Column int // Column number (1-based), 0 if unknown.
}
