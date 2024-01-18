// Package mmm provides protections against double-free and memory leaks for manually managed memory and resources.
package mmm

import (
	"context"
	"reflect"
	"unsafe"
)

// IsPointer can be used to represent a dynamically-typed pointer to
// manually managed memory.
type IsPointer[T comparable] interface {
	// Pointer returns the raw value of the pointer.
	Pointer() T

	// Context returns the context that is associated with this pointer
	// when the returned context is cancelled or freed, the pointer will
	// be automatically freed, after which, Context will return nil.
	Context() Context

	// Free should be implemented by the wrapping type, it must free
	// the underlying resource and then call [MarkFree].
	Free()
}

type mmmPointer[T comparable] interface {
	IsPointer[T]
}

type IsPointerAlias[API any, Kind comparable] interface {
	~struct {
		pointer[Kind]
		API *API
	}
	IsPointer[Kind]

	getPointer() pointer[Kind]
}

// Pointer of unique type T belonging to the given API, using the given RAM allocator
// that is reponsible for freeing the pointer.
type Pointer[API any, T IsPointerAlias[API, Kind], Kind comparable] struct {
	pointer[Kind]
	API *API
}

// Make a new pointer of type T belonging to the given context, when the context is
// cancelled or the underlying cascade is freed, the returned pointer will be freed.
// The context.Context must be derived from a call to [ContextWithCascade].
func Make[API any, T IsPointerAlias[API, Kind], Kind comparable](ctx context.Context, api *API, raw Kind) T {
	var kind Kind
	var ptr pointer[Kind]
	ptr.raw = raw
	if ctx != nil {
		gc, ok := ctx.Value(cascadeKey{}).(*cascadeFree)
		if !ok {
			panic("mmm: context.Context not derived from mmm.ContextWithCascade")
		}
		var free = T.Free
		ptr.ctx = gc.get(reflect.TypeOf(kind), reflect.TypeOf(cascade[Kind]{})).(*cascade[Kind])
		ptr.gen = ptr.ctx.gen
		ptr.ctx.ptr = append(ptr.ctx.ptr, isPointer[Kind]{
			value: raw,
			data:  unsafe.Pointer(api),
			free:  *(*func(unsafePointer[Kind]))(unsafe.Pointer(&free)), // safe because they are the same shape.
		})
	}
	var val struct {
		pointer[Kind]
		API unsafe.Pointer
	}
	val.pointer = ptr
	val.API = unsafe.Pointer(api)
	return *(*T)(unsafe.Pointer(&val))
}

func MarkFree[API any, T IsPointerAlias[API, Kind], Kind comparable](src T) {
	var ptr = src.getPointer()
	ptr.ctx.remove(ptr.gen, ptr.raw)
}

// Move returns an updated copy of the pointer, with its context moved
// to 'ctx', any existing copies of the pointer are invalidated and the
// returned value will be kept alive until 'ctx' is cancelled or freed.
func Move[API any, T IsPointerAlias[API, Kind], Kind comparable](src T, ctx context.Context) T {
	var ptr = src.getPointer()
	ptr.ctx.remove(ptr.gen, ptr.raw)
	var ref = *(*unsafePointer[Kind])(unsafe.Pointer(&src))
	return Make[API, T](ctx, (*API)(ref.api), ptr.raw)
}

type pointer[Kind comparable] struct {
	raw Kind           // TODO remove this.
	gen uintptr        // TODO if ctx is nil, this should be a temporary [C/pinned] pointer to Kind?
	ctx *cascade[Kind] // cascade determines what will free this pointer, a context or another pointer?
}

func (ptr pointer[Kind]) getPointer() pointer[Kind] { return ptr }

// Context implements [IsPointer]
func (ptr pointer[Kind]) Context() Context {
	if ptr.ctx == nil {
		return nil
	}
	return ptr.ctx
}

func (ptr *pointer[Kind]) setPointer(to pointer[Kind]) {
	*ptr = to
}

// Pointer returns the raw value of the pointer.
func (ptr pointer[Kind]) Pointer() Kind {
	if ptr.ctx == nil {
		return ptr.raw
	}
	if ptr.gen != ptr.ctx.gen {
		panic("mmm: pointer has been freed")
	}
	return ptr.raw // TODO fix use after free, pointer should be loaded directly from the ctx.
}
