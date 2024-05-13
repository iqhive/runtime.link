/*
Package rpc provides a way to expose closures over API implementation boundaries.
This enables 'function values' to be serialised to the database and called later,
they could used in payloads.

	// Callback is the rpc equivalent of
	// func() { fmt.Println("Hello World") }
	type Callback struct{}

	func (d Callback) LRPC() string { return "example.Callback" }

	func (d Callback) Call(ctx context.Context, svc Service, _ struct{}) (struct{}, error) {
		fmt.Println("Hello World")
		return struct{}{}, nil
	}

	type MyAPI struct {
		DoSomething func(context.Context, rpc.Void) error
	}

	type Service struct {
		RPC rpc.Transport
	}

	func NewService(API Service) MyAPI {
		rpc.HandleCall(API.RPC, API, Callback.Call)
		return MyAPI{
			DoSomething: API.doSomething,
		}
	}

	func (s Service) doSomething(ctx context.Context) error {
		fmt.Println("Hello World")
		return nil
	}


	func main() {
		var RPC = rpc.New()
		var service API = NewService(Service{RPC: RPC})

		service.DoSomething(context.TODO(), rpc.Call(Callback{}))
	}
*/
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"runtime.link/api/xray"
)

// Callback represents serialisable arguments along with a remote procedure
// that is responsible for performing an operation on them.
type Callback interface {

	// LRPC should return a globally (or at least within the scope of a distributed
	// system) unique string that identifies the remote procedure that is responsible
	// for performing an operation on this value. If the LRPC string contains spaces
	// then it is interpreted as a Go HTTP method and URL.
	//
	// For example: "POST https://example.com/request"
	LRPC() string
}

// Transport is required to
type Transport struct {
	// TODO make this implementable with an interface?
	reflect map[string]reflect.Type
	mapping map[string]func(ctx context.Context, self, args any) (any, error)
}

// New returns a new transport that can be used to register and call remote procedures.
func New() Transport {
	var RPC = Transport{
		reflect: make(map[string]reflect.Type),
		mapping: make(map[string]func(ctx context.Context, self, args any) (any, error)),
	}
	return RPC
}

// Compose returns a function that calls all the provided functions in order.
// It is a builtin function that does not need an implementation to be registered.
func Compose[A any](RPC Transport, functions ...Func[A, struct{}]) Func[A, struct{}] {
	return Call(compose[A](functions))
}

type isFunc[A, B, API any] interface {
	Callback

	Call(context.Context, API, A) (B, error)
}

// Func represents a JSON serialisable function value.
type Func[A, B any] map[struct{}]closure

// Interface returns the underlying [Callback], or nil
// if the [Callback] couldn't be deserialised.
func (fn Func[A, B]) Interface(t Transport) Callback {
	if fn == nil {
		return nil
	}
	cl := fn[struct{}{}]
	if cl.data != nil {
		return cl.data.(Callback)
	}
	if cl.lrpc == "rpc.compose" {
		var placeholder compose[any]
		if err := json.Unmarshal(cl.json, &placeholder); err != nil {
			return nil
		}
		return placeholder
	}
	rtype := t.reflect[cl.lrpc]
	val := reflect.New(rtype)
	if err := json.Unmarshal(cl.json, val.Interface()); err != nil {
		return nil
	}
	return val.Elem().Interface().(Callback)
}

// MarshalJSON implements the json.Marshaler interface.
func (fn Func[A, B]) MarshalJSON() ([]byte, error) {
	if fn == nil {
		return []byte("null"), nil
	}
	cl := fn[struct{}{}]
	if cl.data == nil && cl.json != nil {
		return json.Marshal(map[string]any{
			cl.lrpc: cl.json,
		})
	}
	return json.Marshal(map[string]any{
		cl.lrpc: cl.data,
	})
}

// UnmarshalJSON implements the json.Unmarshaler interface.
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

// Call will call the implementation of the function via the given transport.
func (r Func[A, B]) Call(ctx context.Context, t Transport, arg A) (B, error) {
	var zero B
	if r == nil {
		return zero, xray.Error(fmt.Errorf("rpc.Returns.Call: nil function call"))
	}
	if t.mapping == nil {
		return zero, xray.Error(fmt.Errorf("rpc.Returns.Call: nil transport"))
	}
	fn := r[struct{}{}]
	if fn.lrpc == "rpc.compose" {
		var array []json.RawMessage
		if err := json.Unmarshal(fn.json, &array); err != nil {
			return zero, err
		}
		for _, val := range array {
			var fn Func[A, B]
			if err := json.Unmarshal(val, &fn); err != nil {
				return zero, err
			}
			if _, err := fn.Call(ctx, t, arg); err != nil {
				return zero, err
			}
		}
		return zero, nil
	}
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

// Void is an alias for a function that accepts nothing and returns nothing.
type Void = Func[struct{}, struct{}]

type closure struct {
	lrpc string
	json json.RawMessage
	data any
}

// Call can be given a [Callback] and will return a JSON serialisable
// [Func] that can be called in future with its [Func.Call] method.
func Call[A, B, API any](fn isFunc[A, B, API]) Func[A, B] {
	return map[struct{}]closure{{}: {
		lrpc: fn.LRPC(),
		data: fn,
	}}
}

// HandleCall needs to be called on a transport to register the implementation of an [Callback]
// implementation.
func HandleCall[T Callback, A, B, API any](t Transport, api API, impl func(T, context.Context, API, A) (B, error)) {
	if t.mapping == nil {
		return
	}
	var zero T
	if t.reflect[zero.LRPC()] != nil {
		return // don't re-register
	}
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

type compose[A any] []Func[A, struct{}]

func (c compose[A]) LRPC() string { return "rpc.compose" }

func (c compose[A]) Call(ctx context.Context, RPC Transport, args A) (struct{}, error) {
	var zero struct{}
	for _, fn := range c {
		if _, err := fn.Call(ctx, RPC, args); err != nil {
			return zero, err
		}
	}
	return zero, nil
}
