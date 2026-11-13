//go:build !((linux && (amd64 || arm64)) || (darwin && (amd64 || arm64)) || (windows && amd64))

package ffi

// Const returns an empty string, as it does for a constant that is not
// supported: there is no table of the C constants of this platform. The tables
// of another would be the wrong numbers, and without one the package would not
// build here at all, where a tag can still be parsed.
func Const(name string) string { return "" }
