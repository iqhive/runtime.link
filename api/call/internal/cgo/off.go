//go:build !(linux && amd64) && !(darwin && arm64) && !(android && arm64)

package cgo

import "io"

var Constants constants
var Types types

func Dump(w io.Writer) {}

import "io"

func Dump(w io.Writer) {}
