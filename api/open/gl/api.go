package gl

import (
	"unsafe"

	"runtime.link/api"
)

type API struct {
	api.Specification

	V1
	V2
	V3
	V4
}

type Pointer unsafe.Pointer

type DebugFunc func(source DebugSource, dtype DebugType, ID uint32, severity DebugSeverity, length int, message *byte, userParam unsafe.Pointer)
type BinaryFormat uint32
