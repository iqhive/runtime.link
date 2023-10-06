/*
Package ref provides means for representing reference values, ie. foreign keys and other external IDs.

# Reference Values

Programming languages such as Go often have internal pointer types that can be used to represent a
reference to a value located at another location in memory. This package aims to provide a similar
way to represent references to values that are located outside of the current process, aka. foreign
keys or external IDs. This is common when interacting with APIs, where they will often return an
ID value that represents a reference to a value that is located in a database or another persistent
storage system.
*/
package ref

import (
	"fmt"
)

// Any is a string reference that can be used to reference any type of value. The value
// of this reference will be meaningful to the creator of it. Care must be taken to ensure
// that readers and writes use unique and distinguishable prefixes for their own references
// so that do not conflict with each other.
type Any string

// For denotes a reference to a value of type T from the specified API, using the specified
// underlying reference type. This value can be used as a named type so that methods can be
// attached to it. Methods should use functions from the specified API to perform operations
// on the referenced value. [api.Linker]s will take care of filling in the API value.
type For[API any, T any, Ref comparable] struct {
	_   [0]*T
	Ref Ref
	API *API
}

type isRef[API any, T any, Ref comparable] interface {
	~struct {
		_   [0]*T
		Ref Ref
		API *API
	}
}

// New returns a new Ptr to T using the specified API and reference value.
// Only the first type parameter should be specified, the others should be
// inferred.
func New[ID isRef[API, T, Ref], T any, API any, Ref comparable](api *API, ref Ref) ID {
	return ID(For[API, T, Ref]{API: api, Ref: ref})
}

// Our denotes a relationship between an external and internal identifier, where both
// are strings. The external identifier may have a specific prefix format, used to
// differentiate it from identifiers of a different type, which is determined by the
// value of this relationship which may contain format verbs. The format must be
// bijective across [fmt.Sprintf] and [fmt.Sscanf], ie. it must be possible to transform
// the internal representation into the external representation and vice versa. If the
// format is not specified, then "%v" is used.
type Our[External ~string, Internal any] string

// New transforms the internal representation of the identifier into the external
// representation.
func (m Our[External, Internal]) New(internal Internal) External {
	if m == "" {
		m = "%v"
	}
	return External(fmt.Sprintf(string(m), internal))
}

// Get transforms the external representation of the identifier into the internal
// representation and a boolean flag indicating whether the transformation was
// successful.
func (m Our[External, Internal]) Get(external External) (Internal, bool) {
	var internal Internal
	_, err := fmt.Sscanf(string(external), string(m), &internal)
	return internal, err == nil
}
