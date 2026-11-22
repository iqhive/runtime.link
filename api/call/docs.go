/*
Package call provides shared library linker for runtime.link (WORK-IN-PROGRESS).

# Link Tags

This package defines a library tag format for representing symbols
along with their type. This string always starts with comma seperated
symbol names. Each library is searched for each of the names, in the
order that they are given, and the last symbol that is found is the one
that is used. Next up is a space, followed by the type of the symbol.
The type either begins with 'func' for function types, or the name of
the standard C type, which a Go function calls as a C function with no
parameters and no result. Similarly to Go, the return type is placed
after the parameter list. Anything after a ',' or ')' that follows the
type is ignored.

	abs func(int)int // simple C function, no pointer semantics.

Assertions can be added to type identifiers to document pointer ownership,
error handling and memory safety assertions to make when interacting with
the function.

		fread func(&void[@3],size_t*@1,size_t,&FILE)size_t=@3; ferror(@4)
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
constant name, such as SIZE_MAX, DBL_MAX or NULL, or a 32bit integer
literal.

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

A Go string passed for a '$' or '~' parameter is copied into memory
from malloc, which C then owns: with '$' it is for C to free, and with
'~' it is never freed. A Go slice cannot be passed for either, as it is
Go's memory, and neither can a Go string for '+', as it can't be written
to. Memory from C can be passed as an [unsafe.Pointer]. The malloc is
that of the C runtime that the Go program is built with. On windows each
C runtime has a heap of its own, so there a '$' string is only for a
library that frees with the same one, which for mingw-w64 is the UCRT.

A pointer passed to C is pinned where it is for the duration of the
call, which the runtime cannot do for one that points into a
goroutine's stack, as a stack that grows moves it: an argument must
point to the Go heap or to memory from C, never into a stack frame.

A Go string for any other parameter is passed as a copy with a NUL after
it, which lasts until the call returns, so nothing that C writes to it
reaches the Go string. Only for a parameter marked '#', which C does
not write to, is a string that already ends in a NUL passed as it is.

# Safety Assertions

These assertions are used to document memory safety semantics. 'n'
in '@n' refers to an unsigned integer that refers to the Nth type
identifier in the library tag, the first being '@1'. 'x' refers to a
nested assertion.

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
  - '>=@n' must be greater than or equal to @n, which can also be
    written as '>@n='.
  - '<=@n' must be less than or equal to @n, which can also be
    written as '<@n='.
  - '=@n' must equal @n

A type takes one assertion, except for the result of the function,
which may be followed by a second one, as in 'int>0<5', where both are
checked of it. For a Go slice or string, '@n' is its length, and for a
pointer to an array it is the length of the array. A comparison that
is written on such a parameter is of the same number. Any other
pointer is compared by its address. The capacity of a slice or the
array behind a pointer is counted in elements, and a pointer to one
value has room for one. A nil pointer has no array and room for
nothing, so it is 0 for all of these: '!=NULL' and '>0' keep NULL
from C, and NULL does not pass with a size. NULL with a size of 0
from '@n' does pass, also where more room than the size is asked for,
as '[@n]' does.

Integers and floating point values are compared by value, whatever
their types, and a NaN is not greater than, less than or equal to
anything. A number that is an argument is compared as C is passed
it, which is after it is converted to the type of its parameter: a Go
int of -1 that is passed as a size_t is SIZE_MAX, whether an assertion
is written on it or refers to it with '@n'.

The smallest and the largest long double, LDBL_MIN, LDBL_TRUE_MIN and
LDBL_MAX, are only constants where a long double is a double, as it is
for macOS on arm64. A wider one has limits that no Go number can be
compared with, so elsewhere they are unknown constants.

# Failure Handling

The assertions about memory, '[x]', '*@n', '|@n', '~@n' and '^@n', are
checked on every call, and one that fails on an argument stops the
function from being called. The failure is returned as the error of a
Go function with an error result, and otherwise it panics. A NULL
pointer has no memory, so it overlaps nothing. The memory of a Go
string is what C is passed: the string itself when it is passed as it
is, and otherwise a copy, which is a byte longer for its NUL.

The other comparisons, such as '=@n' or '>0', are checked on the
arguments of a call, where one that fails stops the function from
being called, and on the return value of a Go function with an error
result, where a failure is returned as the error. When the return
value has no assertion, the error is always nil. A comparison on a '-'
parameter says what to infer it from, so it is not itself checked.

An assertion that cannot be checked is refused when the tag is linked:
a tag that promises a check that cannot be made is not trusted to have
made it. That includes an '@n' that is out of range, an unknown
constant, and the capacity of a value that has no known capacity, such
as an [unsafe.Pointer]. It includes an assertion in the type of a
function that C calls, as no call sees what C passes to that function
or what it returns. The comparison on a return value is refused for
the same reasons when the Go function has no error result, although it
is not checked then. Only '?@n' and ':@n' are left out, as they
document intent that is not checked.

When Safety Assertions are placed on the return value of a function
a semicolon can be used to indicate what to do when the assertion
fails. The following options are available:

  - ';' refer to the specified symbol for information about why
    this assertion failed. For a Go function with an error result,
    it is called with the given arguments and its int result is
    included in the error. It's looked up along with the function,
    so it is not used by [Make], and it's left out of the error when
    it cannot be found. It is called on the thread that the
    function was called on, as C keeps what explains a failure for
    each thread, the way that it keeps errno.

# Variadic Functions

The first of the variable arguments of a C function is marked with
'...' after its type, in place of an assertion, and it and every
parameter after it are passed the way that the platform passes variable
arguments. That is not always the way that it passes the named ones,
macOS on arm64 being one that differs, so a tag that leaves the '...'
out only works on some platforms.

	printf func(&char,int...,double)int // printf("%d %f", 1, 2.0)

The type of a function pointer ends with its result, so that is where
the '...' goes when it is the first of the variable arguments. After
the result of the function itself, '...' marks no parameter, and the
function is called as one without variable arguments.

	curl_easy_setopt func(&void,int,func(&char,size_t,size_t,&void)size_t...)int

The tag lists the variable arguments of one way of calling the
function, so each way of calling it needs a tag of its own. A 'float'
among them is passed as a double, as C does, so a Go function that C
calls with variable arguments takes it as a float64. On macOS on arm64,
a Go function cannot be given to C as a function with variable
arguments.

# Callbacks

A Go function passed as an argument is given to C as a function pointer
that is valid until the call returns, unless the argument is marked with
'$' or '~', which leaves it valid forever. Use [Back] for a function
that C keeps hold of and that can be released, and pass the Back
itself, not a pointer to it. A Go function whose type does not match
the tag, in the size and in the kind of each value, or that C has no
way to call, can only be nil, which is passed as NULL. A function
pointer that C hands to the Go function is a parameter of it that is a
Go func, to call, or a Back, to pass on, and its type has to match the
tag in the same way, except that it may leave out a result that it has
no use for. No Go type matches a long double.

# Platforms

A tag is read with the C types and constants of the platform that the
program runs on, which are known for linux and darwin on amd64 and
arm64, for android on arm64, and for windows on amd64 with mingw-w64.
Those of linux are glibc's. Every call is made with cgo, so in a
program built without it, [Make] and linking return an error.

# Libraries

Linking takes the library to link to, which is the path or the name of
one, or a list of them separated by spaces, to look for each symbol in.
What is given is tried as one library first, so a path may have spaces
of its own when it is the only one. Without it, the library is the one
that the structure names for the platform with a [To] field, and it is
an error if none of them can be opened. A namespace of the structure
with a [To] field of its own is looked for in the libraries that it
names as well as in those around it, where a symbol that those have
is still the one that is used. A function of the structure that has no
call tag is not linked, and nothing is reported for it: it is left as
it was, which is nil unless something else has implemented it, and a
nil function panics when it is called.

# Macros

When a library tag is added to a Go func field, it conveys the standard
representation of that field. Some macros are supported for mapping
the Go function signature to the library's one. These macros are purely used for
convenience and do not change the semantics of the standard function signature
(semantics of a tag with macros remain constant when the macros are removed).

  - '-' this parameter should be ignored because it is a
    redundant parameter or can be inferred from an assertion.
    The Go function may have an argument in the position of
    each '-' parameter, which is ignored, or none for them at
    all. Go arguments after those for the last parameter are
    ignored. A bool is true when what it is inferred from
    isn't zero.
*/
package call
