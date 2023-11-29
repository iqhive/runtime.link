package api

import "runtime.link/xyz"

// Error can be used to specify an enumerated set of error
// values that can be returned by an API endpoint. It behaves
// like a [xyz.Switch] that implements [error].
type Error[T any] struct {
	errorMethods[T]
}

type errorMethods[T any] xyz.Switch[error, T]

func (e errorMethods[T]) Error() string {
	err, ok := e.Get()
	if !ok {
		return "nil"
	}
	return err.Error()
}
