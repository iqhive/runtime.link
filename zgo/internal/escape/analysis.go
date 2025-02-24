package escape

// Kind represents the type of escape analysis result
type Kind uint8

const (
	// NoEscape indicates the value does not escape its current scope
	NoEscape Kind = iota
	// StackEscape indicates the value escapes to the heap but remains within the same stack
	StackEscape
	// GoroutineEscape indicates the value escapes to another goroutine
	GoroutineEscape
	// HeapEscape indicates the value escapes to the heap
	HeapEscape
)

// Info contains escape analysis information for a value
type Info struct {
	Kind Kind
	// Reason contains a description of why the value escapes
	Reason string
}
