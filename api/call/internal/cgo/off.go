//go:build !(linux && (amd64 || arm64)) && !(darwin && (amd64 || arm64)) && !(windows && amd64)

package cgo

// A platform without a table of its own knows nothing of C: see [types.Known].
var Constants constants
var Types types
