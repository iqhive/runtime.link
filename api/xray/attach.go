package xray

import (
	"context"
	"reflect"
	"sync/atomic"
	"time"
	"unsafe"
)

// sequence is a process-wide monotonic counter used to order calls in a flat
// trace by the order in which they began. Wall-clock timestamps are unreliable
// for this at sub-microsecond resolution (a parent call can appear to start
// after a child it synchronously invokes), so callers that build ordered
// traces stamp Seq at call-start instead.
var sequence atomic.Uint64

// Sequence returns the next value of the process-wide monotonic call counter.
// It is used to order calls in a flat trace; a caller should read it once, at
// the moment a call begins, so that a parent call is ordered ahead of any
// calls it makes.
func Sequence() uint64 {
	return sequence.Add(1)
}

// Attach xray interceptors to any function values within the given structure.
func Attach[T any](v *T) {
	attach(reflect.ValueOf(v), reflect.StructField{}, make(map[visit]bool))
}

// visit identifies a pointer followed by [Attach]. The type is part of it as a
// struct and its first field have the same address.
type visit struct {
	ptr   unsafe.Pointer
	rtype reflect.Type
}

// attach keeps track of the pointers it has followed, so that a cycle ends and
// a function reachable by two paths is only wrapped once.
func attach(rvalue reflect.Value, field reflect.StructField, seen map[visit]bool) {
	switch rvalue.Kind() {
	case reflect.Struct:
		for i := 0; i < rvalue.NumField(); i++ {
			attach(rvalue.Field(i), rvalue.Type().Field(i), seen)
		}
	case reflect.Pointer:
		at := visit{rvalue.UnsafePointer(), rvalue.Type()}
		if !rvalue.IsNil() && !seen[at] {
			seen[at] = true
			attach(rvalue.Elem(), field, seen)
		}
	case reflect.Func:
		if rvalue.CanSet() && !rvalue.IsNil() {
			// Snapshot the original function into a fresh, non-addressable
			// Value so it does not alias the struct field. Capturing rvalue
			// directly would call the replacement (this wrapper) instead of
			// the original, recursing forever.
			fn := reflect.ValueOf(rvalue.Interface())
			call := fn.Call
			if fn.Type().IsVariadic() {
				// A MakeFunc wrapper is given variadic arguments as a slice.
				call = fn.CallSlice
			}
			rvalue.Set(reflect.MakeFunc(rvalue.Type(), func(args []reflect.Value) (vals []reflect.Value) {
				// Stamp a monotonic sequence at call-start so a flat trace
				// can be ordered by the sequence in which calls were
				// initiated, ahead of any nested calls they make.
				seq := Sequence()
				start := time.Now()
				vals = call(args)
				if len(args) > 0 {
					ctx, ok := reflect.TypeAssert[context.Context](args[0])
					if ok {
						ContextAdd(ctx, Call{
							Name: field.Name,
							Tags: field.Tag,
							Func: fn,
							Args: args,
							Vals: vals,
							Time: start,
							Seq:  seq,
						})
					}
				}
				return vals
			}))
		}
	}
}

type Call struct {
	Name     string
	Tags     reflect.StructTag
	Func     reflect.Value
	Args     []reflect.Value
	ArgNames []string // Go parameter names aligned with Args; empty/short means unlabelled.
	Vals     []reflect.Value
	Time     time.Time
	Seq      uint64
}
