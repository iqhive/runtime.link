package gl

import (
	"unsafe"

	"github.com/iqhive/runtime.link/api"
)

// API is OpenGL, with each function in the version that added it. The
// functions are linked with api/call, to the OpenGL library by its name or
// through the lookup of whatever made the context, and one that the library
// does not have reports that when it is called:
//
//	GL := api.Import[gl.API](call.API, "libGL.so.1", call.Options{})
type API struct {
	api.Specification

	V1
	V2
	V3
	V4
}

type Pointer unsafe.Pointer

// DebugFunc is a GLDEBUGPROC, which GL calls with a message. Each parameter is
// as wide as what GL passes for it, as C could not call it otherwise: length
// is a GLsizei, which is an int in C and so an int32 here.
type DebugFunc func(source DebugSource, dtype DebugType, ID uint32, severity DebugSeverity, length int32, message *byte, userParam unsafe.Pointer)
type BinaryFormat uint32
