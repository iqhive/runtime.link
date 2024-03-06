// Package rpc provides a way to expose closures over API implementation boundaries.
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

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
	reflect map[string]reflect.Type
	mapping map[string]func(ctx context.Context, self, args any) (any, error)
}

func New() Transport {
	return Transport{
		reflect: make(map[string]reflect.Type),
		mapping: make(map[string]func(ctx context.Context, self, args any) (any, error)),
	}
}

type isFunc[A, B, API any] interface {
	Type

	Call(context.Context, API, A) (B, error)
}

type Func[A, B any] map[struct{}]closure

func (fn Func[A, B]) Interface(t Transport) any {
	if fn == nil {
		return nil
	}
	cl := fn[struct{}{}]
	if cl.data != nil {
		return cl.data
	}
	rtype := t.reflect[cl.lrpc]
	val := reflect.New(rtype)
	if err := json.Unmarshal(cl.json, val.Interface()); err != nil {
		return err
	}
	return val.Elem().Interface()
}

func (fn Func[A, B]) MarshalJSON() ([]byte, error) {
	if fn == nil {
		return []byte("null"), nil
	}
	cl := fn[struct{}{}]
	if cl.data == nil && cl.json != nil {
		return json.Marshal(map[string]any{
			cl.lrpc: cl.data,
		})
	}
	return json.Marshal(map[string]any{
		cl.lrpc: cl.data,
	})
}

func (fn *Func[A, B]) UnmarshalJSON(data []byte) error {
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
			json: val,
		}
		return nil
	}
	return nil
}

func (r Func[A, B]) Call(ctx context.Context, t Transport, arg A) (B, error) {
	var zero B
	if r == nil {
		return zero, xray.Error(fmt.Errorf("rpc.Returns.Call: nil function call"))
	}
	if t.mapping == nil {
		return zero, xray.Error(fmt.Errorf("rpc.Returns.Call: nil transport"))
	}
	fn := r[struct{}{}]
	ret, err := t.mapping[fn.lrpc](ctx, r.Interface(t), arg)
	if err != nil {
		return zero, err
	}
	val, ok := ret.(B)
	if !ok {
		return zero, fmt.Errorf("unexpected return type: %T", ret)
	}
	return val, nil
}

type Void = Func[struct{}, struct{}]

type closure struct {
	lrpc string
	json json.RawMessage
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
	t.reflect[zero.LRPC()] = reflect.TypeOf(zero)
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
