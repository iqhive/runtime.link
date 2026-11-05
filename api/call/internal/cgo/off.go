//go:build !(linux && amd64) && !(darwin && arm64) && !(android && arm64)

package cgo

var Constants constants
var Types types
