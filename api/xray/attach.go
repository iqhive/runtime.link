package xray

import (
	"context"
	"reflect"
	"sync/atomic"
	"time"
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
	attach(reflect.ValueOf(v), reflect.StructField{})
}

func attach(rvalue reflect.Value, field reflect.StructField) {
	switch rvalue.Kind() {
	case reflect.Struct:
		for i := 0; i < rvalue.NumField(); i++ {
			attach(rvalue.Field(i), rvalue.Type().Field(i))
		}
	case reflect.Pointer:
		if !rvalue.IsNil() {
			attach(rvalue.Elem(), field)
		}
	case reflect.Func:
		if rvalue.CanSet() && !rvalue.IsNil() {
			// Snapshot the original function into a fresh, non-addressable
			// Value so it does not alias the struct field. Capturing rvalue
			// directly would call the replacement (this wrapper) instead of
			// the original, recursing forever.
			fn := reflect.ValueOf(rvalue.Interface())
			rvalue.Set(reflect.MakeFunc(rvalue.Type(), func(args []reflect.Value) (vals []reflect.Value) {
				// Stamp a monotonic sequence at call-start so a flat trace
				// can be ordered by the sequence in which calls were
				// initiated, ahead of any nested calls they make.
				seq := Sequence()
				start := time.Now()
				vals = fn.Call(args)
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
	Name string
	Tags reflect.StructTag
	Func reflect.Value
	Args []reflect.Value
	Vals []reflect.Value
	Time time.Time
	Seq  uint64
}
