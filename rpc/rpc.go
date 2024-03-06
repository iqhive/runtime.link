// Package rpc provides a way to expose closures over API implementation boundaries.
package rpc

import (
	"context"
	"encoding/json"
	"fmt"

	"runtime.link/api/xray"
)

// Type represents serialisable arguments along with a remote procedure
// that is responsible for performing an operation on them.
type Type interface {

	// LRPC should return a globally (or at least within the scope of a distributed
	// system) unique string that identifies the remote procedure that is responsible
	// for performing an operation on this value. If the LRPC string contains spaces
	// then it is interpreted as a Go HTTP method and URL.
	//
	// For example: "POST https://example.com/request"
	LRPC() string
}

type Transport struct {
	// TODO make this implementable with an interface?

	mapping map[string]func(ctx context.Context, self, args any) (any, error)
}

func New() Transport {
	return Transport{
		mapping: make(map[string]func(ctx context.Context, self, args any) (any, error)),
	}
}

type isFunc[A, B, API any] interface {
	Type

	Call(context.Context, API, A) (B, error)
}

type Func[T any, V any] map[struct{}]closure

func (fn Func[T, V]) MarshalJSON() ([]byte, error) {
	var structure = make(map[string]json.RawMessage)
	cl := fn[struct{}{}]
	body, err := json.Marshal(cl.data)
	if err != nil {
		return nil, err
	}
	structure[cl.lrpc] = body
	return json.Marshal(structure)
}

func (fn *Func[T, V]) UnmarshalJSON(data []byte) error {
	var structure = make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &structure); err != nil {
		return err
	}
	for key, val := range structure {
		if *fn == nil {
			*fn = make(map[struct{}]closure)
		}
		(*fn)[struct{}{}] = closure{
			lrpc: key,
			data: val,
		}
		return nil
	}
	return nil
}

func (r Func[T, V]) Call(ctx context.Context, t Transport, arg V) (T, error) {
	var zero T
	if r == nil {
		return zero, xray.Error(fmt.Errorf("rpc.Returns.Call: nil function call"))
	}
	fn := r[struct{}{}]
	ret, err := t.mapping[fn.lrpc](ctx, fn.data, arg)
	if err != nil {
		return zero, err
	}
	val, ok := ret.(T)
	if !ok {
		return zero, fmt.Errorf("unexpected return type: %T", ret)
	}
	return val, nil
}

type Void = Func[struct{}, struct{}]

type closure struct {
	lrpc string
	data any
}

func Call[A, B, API any](fn isFunc[A, B, API]) Func[A, B] {
	return map[struct{}]closure{{}: {
		lrpc: fn.LRPC(),
		data: fn,
	}}
}

func HandleCall[T Type, A, B, API any](t Transport, api API, impl func(T, context.Context, API, A) (B, error)) {
	if t.mapping == nil {
		return
	}
	var zero T
	t.mapping[zero.LRPC()] = func(ctx context.Context, self, args any) (any, error) {
		this, ok := self.(T)
		if !ok {
			return nil, xray.Error(fmt.Errorf("unexpected self type: %T", self))
		}
		val, ok := args.(A)
		if !ok {
			return nil, xray.Error(fmt.Errorf("unexpected argument type: %T", args))
		}
		return impl(this, ctx, api, val)
	}
}
