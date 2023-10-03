/*
Package link provides shared library linker for runtime.link (WORK-IN-PROGRESS).

# Link Tags

This package defines a library tag format for representing symbols
along with their type. This string always starts with comma seperated
symbol names, in order of preference. Next up is a space, followed by
the type of the symbol. The type either begins with 'func' for function
types, or the name of the standard C type. Similarly to Go, the return
type is placed after the parameter list.

	abs func(int)int // simple C function, no pointer semantics.

Assertions can be added to type identifiers to document pointer ownership,
error handling and memory safety assertions to make when interacting with
the function.

		fread func(&void[@3],size_t*@1,size_t,&FILE)size_t<@3; ferror(@4)
		           ^     ^         ^                      ^    ^
	             │     │         │                      │    └── error details.
	             │     |         |                      |
	             │     |         |                      └── error condition.
	             │     |         |
	             │     |         └── must equal the underlying value
	             │     |             sizeof the 1st argument (whatever void is).
	             │     |
	             │     └── memory capacity must be greater than the 3rd
	             │         argument.
	             |
	             └── fread borrows this buffer for the duration of the call.

Any '@n' component inside a tag may be substituted with a standard C
constant name or a 32bit integer literal.

# Ownership Assertions

These assertions are used to document the ownership semantics for
pointer types, their presence signals that the value is a pointer.

  - '$' memory ownership is sold to the reciever of the value,
    the receiver becomes responsible for freeing it and
    can do so immediately.
  - '&' the receiver borrows this pointer and will not keep a
    reference to it beyond the the lifetime of the
    function call. If specified on a return value, the
    receiver must copy the value.
  - '#' the value pointed to by this pointer is immutable can
    be preceded by one of the other ownership assertions
  - '~' the value is a static pointer, neither the receiver
    nor the sender may free it.
  - '+' the receiver borrows this pointer so that they can
    initialize it. The existing value will be overwritten.

# Safety Assertions

These assertions are used to document memory safety semantics. 'n'
in '@n' refers to an unsigned integer that refers to the Nth type
identifier in the library tag. 'x' refers to a nested assertion.

  - '[x]' the underlying memory capacity of the pointer must be
    greater (and not equal) to '@n', the '@' symbol can be
    omitted to refer to the literal integer value 'n'.
  - '!x' the value must not meet the assertion 'x'.
  - '|@n' the underlying memory capacity must overlap with
    the memory buffer of '@n'.
  - '*@n'	the value must equal the underlying value sizeof the
    value pointed to by '@n',
  - '^@n' the value points within the memory buffer of '@n',
    therefore the lifetime of this value must match the
    lifetime of '@n'.
  - '?@n' the value should be validated as a printf-style
    varar list, with the format parameter being '@n'.
  - ':@n' the value's points to a value that matches the type
    of the value pointed to by '@n'.
  - '>@n' must be greater than @n
  - '<@n' must be less than @n
  - '>=@n' must be greater than or equal to @n
  - '<=@n' must be less than or equal to @n
  - '=@n' must equal @n

# Failure Handling

When Safety Assertions are placed on the return value of a function
a semicolon can be used to indicate what to do when the assertion
fails. The following options are available:

  - ';' refer to the specified symbol for information about why
    this assertion failed.

# Macros

When a library tag is added to a Go func field, it conveys the standard
representation of that field. Some macros are supported for mapping
the Go function signature to the library's one. These macros are purely used for
convenience and do not change the semantics of the standard function signature
(semantics of a tag with macros remain constant when the macros are removed).

  - '-' this parameter should be ignored because it is a
    redundant parameter or can be inferred from an assertion.
  - '%v' the vth function argument is mapped against this
    parameter. Standard printf formatting rules apply
    as if each argument in the function was passed to
    the fmt.Sprintf function. Only %v and %[n]v verbs
    are supported.

# Structures

A struct is identified by an slice of library tags.

Structs passed across language boundaries must have their fields
tagged.

	type MyStruct {
		Name string `ffi:"name &char"`
	}

# Deep Copies

By default, values are deep-copied between languages. In order to
avoid copies for larger values and to keep reference semantics,
special types from xyz and txt can be used to preserve
foreign ownership and memory safety information, ie [txt.Pointer] and
certain [xyz] types. Which need to be manually freed. Struct fields
can be accessed directly this way by specifying getter and setter
functions. These types are safe to pass back and forth between
languages (although may panic when misused).
*/
package link
