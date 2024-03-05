// Package rpc provides a way to expose closures over API implementation boundaries.
package rpc

import (
	"context"
	"fmt"

	"runtime.link/api/xray"
	"runtime.link/xyz"
)

// TODO transport arguments here shouldn't be any? I think they need to be decodable
// values so that the underlying encoding can be handled by the transport layer.

type Transport map[string]func(ctx context.Context, self, args any) (any, error)

type Call xyz.Extern[Call, any]

type validatable interface {
	valid() bool
}

// Valid returns true if the rpc function can be called (it is not nil).
func Valid(v validatable) bool {
	return v.valid()
}

func HandleCall[Self any](t Transport, extension xyz.Case[Call, Self], impl func(ctx context.Context, self Self) error) {
	key, err := extension.Key()
	if err != nil {
		panic(fmt.Sprintf("rpc.HandleCall for %T: %v", extension, err))
	}
	if t == nil {
		return
	}
	t[key] = func(ctx context.Context, self, args any) (any, error) {
		this, ok := self.(Self)
		if !ok {
			return nil, xray.Error(fmt.Errorf("unexpected self type: %T", self))
		}
		return nil, impl(ctx, this)
	}
}

func (fn Call) valid() bool { return fn != Call{} }

func (fn Call) Call(ctx context.Context, rpc Transport) error {
	if rpc == nil {
		return xray.Error(fmt.Errorf("rpc.Call: nil transport"))
	}
	pair, err := fn.MarshalPair()
	if err != nil {
		return xray.Error(err)
	}
	key, val := pair.Split()
	if _, err := rpc[key](ctx, val, nil); err != nil {
		return xray.Error(err)
	}
	return nil
}

type Func[Args any] xyz.Extern[Func[Args], any]

func HandleFunc[Self, Args any](t Transport, extension xyz.Case[Func[Args], Self], impl func(context.Context, Self, Args) error) {
	key, err := extension.Key()
	if err != nil {
		panic(fmt.Sprintf("rpc.HandleCall for %T: %v", extension, err))
	}
	if t == nil {
		return
	}
	t[key] = func(ctx context.Context, self, args any) (any, error) {
		this, ok := self.(Self)
		if !ok {
			return nil, xray.Error(fmt.Errorf("unexpected self type: %T", self))
		}
		val, ok := args.(Args)
		if !ok {
			return nil, xray.Error(fmt.Errorf("unexpected argument type: %T", args))
		}
		return nil, impl(ctx, this, val)
	}
}

func (fn Func[T]) valid() bool { return fn != Func[T]{} }

func (fn Func[T]) Call(ctx context.Context, rpc Transport, arg T) error {
	if rpc == nil {
		return xray.Error(fmt.Errorf("rpc.Call: nil transport"))
	}
	pair, err := fn.MarshalPair()
	if err != nil {
		return xray.Error(err)
	}
	key, val := pair.Split()
	if _, err := rpc[key](ctx, val, arg); err != nil {
		return xray.Error(err)
	}
	return nil
}

type Returns[T any, Args any] xyz.Extern[Returns[T, Args], any]

func HandleReturns[V any, Self, Args any](t Transport, extension xyz.Case[Returns[V, Args], Self], impl func(context.Context, Self, Args) (V, error)) {
	key, err := extension.Key()
	if err != nil {
		panic(fmt.Sprintf("rpc.HandleCall for %T: %v", extension, err))
	}
	if t == nil {
		return
	}
	t[key] = func(ctx context.Context, self, args any) (any, error) {
		this, ok := self.(Self)
		if !ok {
			return nil, xray.Error(fmt.Errorf("unexpected self type: %T", self))
		}
		val, ok := args.(Args)
		if !ok {
			return nil, xray.Error(fmt.Errorf("unexpected argument type: %T", args))
		}
		return impl(ctx, this, val)
	}
}

func (fn Returns[T, Args]) valid() bool { return fn != Returns[T, Args]{} }

func (fn Returns[T, Args]) Call(ctx context.Context, rpc Transport, arg Args) (T, error) {
	var zero T
	if rpc == nil {
		return zero, xray.Error(fmt.Errorf("rpc.Call: nil transport"))
	}
	pair, err := fn.MarshalPair()
	if err != nil {
		return zero, xray.Error(err)
	}
	key, val := pair.Split()
	ret, err := rpc[key](ctx, val, arg)
	if err != nil {
		return zero, xray.Error(err)
	}
	zero, ok := ret.(T)
	if !ok {
		return zero, fmt.Errorf("unexpected return type: %T", ret)
	}
	return zero, nil
}
