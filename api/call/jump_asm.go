//go:build cgo && (arm64 || amd64)

package call

import "unsafe"

//go:noescape
func jump_call(trampoline uintptr, fn unsafe.Pointer, result unsafe.Pointer, args *unsafe.Pointer)
