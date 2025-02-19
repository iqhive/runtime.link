//go:build cgo && !(arm64 || amd64)

package call

import (
	"unsafe"

	"runtime.link/api/call/internal/cgo/dyncall"
)

func jump_call(trampoline, fn, result unsafe.Pointer, args *unsafe.Pointer) {
	returns := (*Returns[struct{}])(fn)
	dyncall.Slow(unsafe.Pointer(fn), unsafe.Pointer(&result), unsafe.Slice(args, returns.count)...)
}
