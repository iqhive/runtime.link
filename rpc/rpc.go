/*
Package rpc provides a mechanism for calling functions remotely, ie. functions with receivers that are
not immediately available to the caller, or closures with internal state that is not immediately available
to the caller.

This package enables a consistent pattern to serialise 'function closures' to persistent storage so they
can be called on a subsequent run of the program, on a different machine, or even in a different
runtime environment.

	// HelloWorld is a function, depending on a runtime that can be called remotely.
	type HelloWorld struct{
		Suffix string
	}

	func (d HelloWorld) Func(API Runtime) (string, func(context.Context) error) {
		// the string returned is a unique identifier for the function,
		// it is used to register the function with the RPC Transport.
		//
		return "example.HelloWorld", func(ctx context.Context) error {
			API.Out.Write([]byte("Hello World" + d.Suffix + "\n"))
			return nil
		}
	}

	type MyAPI struct {
		DoSomething func(context.Context, rpc.Any[func(context.Context) error]) error
	}

	type Runtime struct {
		RPC rpc.Transport
		Out io.Writer
	}

	func NewAPI(I Runtime) MyAPI {
		I.RPC.Register(I)
		return MyAPI{
			DoSomething: API.doSomething,
		}
	}

	func (API Service) doSomething(ctx context.Context, something rpc.Any[func()]) error {
		return something.Call(API.RPC)(ctx)
	}

	func main() {
		var RPC = rpc.New(
			HelloWorld.Func,
		)
		var service API = NewService(
			Service{RPC: RPC, Out: os.Stdout},
		)
		service.DoSomething(context.TODO(), rpc.Call(HelloWorld{
			Suffix: "!"
		}))
	}
*/
package rpc

import (
	"encoding/json"
	"reflect"
	"sync"
)

// Transport routes function calls to the registered functions.
type Transport struct {
	values map[reflect.Type]reflect.Value
	byname map[string]reflect.Type
}

type entry struct {
	ident string
	rtype reflect.Type
}

var global = make(map[reflect.Type][]entry)
var mutex sync.RWMutex

// New creates a new Transport instance and registers the provided functions and runtimes for use.
func New(functions ...any) Transport {
	var RPC = Transport{
		values: make(map[reflect.Type]reflect.Value),
		byname: make(map[string]reflect.Type),
	}
	for _, value := range functions {
		RPC.Register(value)
	}
	return RPC
}

// Register registers a function or runtime with the Transport.
func (t Transport) Register(value any) {
	rtype := reflect.TypeOf(value)
	rvalue := reflect.ValueOf(value)
	var e entry
	if rtype.Kind() == reflect.Func && rtype.NumIn() == 2 && rtype.NumOut() > 0 && rtype.Out(0).Kind() == reflect.String {
		call := rvalue.Call([]reflect.Value{reflect.Zero(rtype.In(0)), reflect.Zero(rtype.In(1))})[0].String()
		t.byname[call] = rtype
		e.ident = call
		e.rtype = rtype.In(0)
		mutex.Lock()
		defer mutex.Unlock()
		global[rtype.Out(1)] = append(global[rtype.Out(1)], e)
	}
	t.values[rtype] = rvalue
}

// Any is a function that can be called remotely, T should be a func type, otherwise it is considered to be
// a function of the type func(context.Context) T
type Any[T any] struct {
	call string
	args json.RawMessage
}

func (fn Any[T]) TypesJSON() []reflect.Type {
	mutex.RLock()
	defer mutex.RUnlock()
	var types []reflect.Type
	for _, fn := range global[reflect.TypeFor[T]()] {
		types = append(types, reflect.StructOf([]reflect.StructField{
			{
				Name: "Call",
				Type: reflect.TypeOf(""),
				Tag:  `json:"call" const:"` + reflect.StructTag(fn.ident) + `"`,
			},
			{
				Name: "Args",
				Type: fn.rtype,
				Tag:  `json:"args,omitzero"`,
			},
		}))
	}
	return types
}

// Call returns the underlying function to call, using the specified RPC Transport to determine where the
// function is registered and how to call it.
func (fn Any[T]) Call(RPC Transport) T {
	if fn.call == "" {
		return [1]T{}[0]
	}
	rtype, ok := RPC.byname[fn.call]
	if !ok || rtype.Kind() != reflect.Func || rtype.NumIn() != 2 || rtype.NumOut() != 2 || rtype.Out(0).Kind() != reflect.String || rtype.Out(1) != reflect.TypeFor[T]() {
		return [1]T{}[0]
	}
	value, ok := RPC.values[rtype]
	if !ok {
		return [1]T{}[0]
	}
	runtime, ok := RPC.values[rtype.In(1)]
	if !ok {
		return [1]T{}[0]
	}
	var recv = reflect.New(rtype.In(0))
	json.Unmarshal(fn.args, recv.Interface())
	return value.Call([]reflect.Value{
		recv,
		runtime,
	})[1].Interface().(T)
}

// MarshalJSON implements the json.Marshaler interface for Any[T].
func (fn Any[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Call string          `json:"call"`
		Args json.RawMessage `json:"args"`
	}{
		Call: fn.call,
		Args: fn.args,
	})
}

// UnmarshalJSON implements the json.Unmarshaler interface for Any[T].
func (fn *Any[T]) UnmarshalJSON(data []byte) error {
	var aux struct {
		Call string          `json:"call"`
		Args json.RawMessage `json:"args"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	fn.call = aux.Call
	fn.args = aux.Args
	return nil
}

// Function that depends on a specific runtime.
type Function[T any, API any] interface {
	Func(API) (string, T)
}

// Call returns the Any[T] from a Function[T, API] value, so that it can be
// used as a generic function closure parameter.
func Call[T any, API any](fn Function[T, API]) Any[T] {
	name, _ := fn.Func([1]API{}[0])
	args, _ := json.Marshal(fn)
	return Any[T]{
		call: name,
		args: args,
	}
}
