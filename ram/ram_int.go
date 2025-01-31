package ram

import "runtime.link/via"

type Int via.Any[IntProxy]

func (i Int) Int64() int64 { return via.Methods(i).Int64(via.Internal(i)) }
func (i *Int) Set(val Int) {
	*i = via.Methods(*i).Set(via.Internal(*i), val)
}

type IntProxy interface {
	via.API

	Int64(via.CachedState) int64
	Set(via.CachedState, Int) Int
	Add(via.CachedState, Int) Int
}

type goMemoryInt struct{}

func (goMemoryInt) Int64(cache via.CachedState) int64             { return via.Cached[goMemoryInt, int64](cache) }
func (goMemoryInt) SetInt64(cache via.CachedState, val int64) Int { return NewInt(val) }
func (goMemoryInt) Alive(cache via.CachedState) bool {
	return via.Cached[goMemoryInt, int64](cache) != 0
}
func (goMemoryInt) Add(cache via.CachedState, val Int) Int {
	return NewInt(via.Cached[goMemoryInt, int64](cache) + val.Int64())
}
func (goMemoryInt) Set(cache via.CachedState, val Int) Int {
	return NewInt(via.Cached[goMemoryInt, int64](cache) - val.Int64())
}

func NewInt[T ~int8 | ~int16 | ~int32 | ~int64 | ~int](val T) Int {
	return via.New[Int, IntProxy](goMemoryInt{}, via.NewCache[goMemoryInt](val))
}
