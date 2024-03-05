// Package rpc provides a way to expose closures over API implementation boundaries.
package rpc

import (
	"context"
	"fmt"

	"runtime.link/api/xray"
	"runtime.link/xyz"
)

type Transport map[string]func(ctx context.Context, self, args any) (any, error)

type Call xyz.Extern[Call, any]

func HandleCall[T xyz.TypeOf[Call]](t Transport, extension T, impl func(context.Context) error) {
	key, err := extension.Key()
	if err != nil {
		panic(fmt.Sprintf("rpc.HandleCall for %T: %v", extension, err))
	}
	t[key] = func(ctx context.Context, self, args any) (any, error) {
		return nil, impl(ctx)
	}
}

func (fn Call) Call(ctx context.Context, rpc Transport) error {
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

func HandleFunc[T xyz.TypeOf[Func[Args]], Args any](t Transport, extension T, impl func(context.Context, Args) error) {
	key, err := extension.Key()
	if err != nil {
		panic(fmt.Sprintf("rpc.HandleCall for %T: %v", extension, err))
	}
	t[key] = func(ctx context.Context, self, args any) (any, error) {
		val, ok := args.(Args)
		if !ok {
			return nil, xray.Error(fmt.Errorf("unexpected argument type: %T", args))
		}
		return nil, impl(ctx, val)
	}
}

func (fn Func[T]) Call(ctx context.Context, rpc Transport, arg T) error {
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

func HandleReturns[V any, T xyz.TypeOf[Returns[V, Args]], Args any](t Transport, extension T, impl func(context.Context, Args) (V, error)) {
	key, err := extension.Key()
	if err != nil {
		panic(fmt.Sprintf("rpc.HandleCall for %T: %v", extension, err))
	}
	t[key] = func(ctx context.Context, self, args any) (any, error) {
		val, ok := args.(Args)
		if !ok {
			return nil, xray.Error(fmt.Errorf("unexpected argument type: %T", args))
		}
		return impl(ctx, val)
	}
}

func (fn Returns[T, Args]) Call(ctx context.Context, rpc Transport, arg Args) (T, error) {
	var zero T
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
