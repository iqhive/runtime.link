//go:build !(cgo && linux && !android)

package cgo

// OtherLibrary reports whether the program is built with a C library that the
// tables of the platform were not written from. Only linux has more than one
// in use, and which it is can only be told when a C compiler is there to ask.
const OtherLibrary = false
