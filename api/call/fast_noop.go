//go:build !(amd64 || arm64)

package call

import (
	"iter"
	"reflect"
)

//go:nosplit
func fast_call() {}

// Fast is experiemental and unsafe.
func Fast(fn any, arguments iter.Seq[reflect.Value]) iter.Seq[Value] {
	return nil
}
