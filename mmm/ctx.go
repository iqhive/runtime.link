package mmm

import (
	"context"
	"reflect"
	"runtime"
	"sync"
	"unsafe"
)

// Context with a [Free] method that can be used to free resources
// that were manually allocated using the context.
type Context interface {
	context.Context

	// Free any pointers or resources that were manually allocated
	// not safe to call concurrently with any live [Pointer] values.
	Free()

	// Defer adds a function to be called when the context is freed.
	Defer(func())
}

var contexts sync.Pool

// NewContext returns a [context.Context], where any [Pointer]s created using this context
// will be freed when the returned context's [Context.Free] method is called. If the parent
// context has a nil [Context.Done] channel, then the returned context will be cancelled
// when [Context.Free] is called. A [Context] should not be used after it has been freed,
// as it may become recycled as a new context and using it may cause [Pointer]s to be
// unexpectedly freed, although the context is safe to use as a [context.Context] for child
// goroutines, it shouldn't be used for allocations across multiple goroutines.
func NewContext(ctx context.Context) Context {
	cas, ok := contexts.Get().(*cascadeFree)
	if !ok {
		cas = new(cascadeFree)
		cas.int.Context = cas
		cas.uintptr.Context = cas
		cas.unsafe.Context = cas
		cas.Context = ctx
		if ctx.Done() == nil && ctx != context.Background() {
			cas.done = make(chan struct{})
		}
		runtime.SetFinalizer(cas, func(cas *cascadeFree) { cas.free() })
	} else {
		cas.Context = ctx
		if ctx.Done() == nil && ctx != context.Background() {
			cas.done = make(chan struct{})
		}
	}
	return cas
}

type ContextWith[API any] interface {
	Context

	API() *API
}

type apiWrappedContext[API any] struct {
	*cascadeFree
}

func (cas apiWrappedContext[API]) API() *API {
	return cas.api.(*API)
}

func NewContextWith[T any](ctx context.Context, api *T) ContextWith[T] {
	cas := NewContext(ctx).(*cascadeFree)
	cas.api = api
	return apiWrappedContext[T]{cas}
}

type isCascade interface {
	setContext(context.Context)
	Free()
	Defer(func())
}

// contextWithFree is designed so that it can be reused and placed in a [sync.Pool]
// in order to reduce allocations.
type contextWithFree struct {
	cas *cascadeFree
	gen uintptr
}

type cascadeFree struct {
	context.Context

	api any
	gen uintptr // generation counter

	int     cascade[int]
	uintptr cascade[uintptr]
	unsafe  cascade[unsafe.Pointer]

	slow map[reflect.Type]isCascade

	done chan struct{}

	defers []func()
}

func (cas *cascadeFree) Done() <-chan struct{} {
	if ch := cas.Context.Done(); ch != nil {
		return ch // prefer parent context's done channel.
	}
	return cas.done
}

// Defer adds a function to be called when the context is freed.
func (cas *cascadeFree) Defer(fn func()) {
	cas.defers = append(cas.defers, fn)
}

func (cas *cascadeFree) Err() error {
	if cas.done == nil {
		return cas.Context.Err()
	}
	select {
	case _, ok := <-cas.done:
		if !ok {
			return context.Canceled
		}
		return nil
	default:
		return nil
	}
}

func (cas *cascadeFree) Value(key any) any {
	if (key == cascadeKey{}) {
		return cas
	}
	return cas.Context.Value(key)
}

type cascadeKey struct{}

func (cas *cascadeFree) get(kind, generic reflect.Type) any {
	switch kind {
	case reflect.TypeOf(int(0)):
		return &cas.int
	case reflect.TypeOf(uintptr(0)):
		return &cas.uintptr
	case reflect.TypeOf(unsafe.Pointer(nil)):
		return &cas.unsafe
	default:
		if cas.slow == nil {
			cas.slow = make(map[reflect.Type]isCascade)
		}
		var ptr, ok = cas.slow[kind]
		if !ok {
			ptr = reflect.New(generic).Interface().(isCascade)
			ptr.setContext(cas)
			cas.slow[kind] = ptr
		}
		return ptr
	}
}

func (cas *cascadeFree) Free() {
	for _, fn := range cas.defers {
		fn()
	}
	cas.defers = cas.defers[:0]
	cas.free()
	if cas.done == nil {
		contexts.Put(cas)
	}
}

func (cas *cascadeFree) free() {
	for _, fn := range cas.defers {
		fn()
	}
	cas.int.Free()
	cas.uintptr.Free()
	cas.unsafe.Free()
	for _, ptr := range cas.slow {
		ptr.Free()
	}
	cas.slow = nil
	if cas.done != nil {
		close(cas.done)
	}
}

// cascade is a collection of pointers that share a context, that is to say that they
// should all be freed when the context is cancelled.
type cascade[T comparable] struct {
	context.Context

	gen uintptr
	gcc uintptr
	ptr []isPointer[T] // zero-allocation fast path.
	fns []func()
}

type unsafePointer[T comparable] struct {
	ptr pointer[T]
	api unsafe.Pointer
}

type isPointer[T comparable] struct {
	value T

	data unsafe.Pointer
	free func(unsafePointer[T])
}

func (cas *cascade[T]) Defer(fn func()) {
	cas.fns = append(cas.fns, fn)
}

// remove walks backwards because it is more likely
// that a recently created object is being moved
// than an older one.
func (cas *cascade[T]) remove(gen uintptr, raw T) {
	if cas.gcc > gen {
		return
	}
	for i := len(cas.ptr) - 1; i >= 0; i-- {
		if cas.ptr[i].value == raw {
			cas.ptr[i] = isPointer[T]{}
			return
		}
	}
}

func (cas *cascade[T]) setContext(ctx context.Context) {
	cas.Context = ctx
}

// Free all the pointers in the cascade.
func (cas *cascade[T]) Free() {
	if len(cas.ptr) == 0 {
		return
	}
	cas.gcc++
	for _, fn := range cas.fns {
		fn()
	}
	cas.fns = cas.fns[:0]
	for _, ptr := range cas.ptr {
		if ptr.data != nil {
			ptr.free(unsafePointer[T]{ptr: pointer[T]{raw: ptr.value, ctx: cas, gen: cas.gen}, api: ptr.data})
		}
	}
	cas.ptr = cas.ptr[:0]
	cas.gen++
}
