//go:build !cgo

package call

import (
	"unsafe"
)

func jump_call(trampoline, fn, result unsafe.Pointer, args *unsafe.Pointer) {
	panic("cgo disabled")
}
