package call_test

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"

	"runtime.link/api/call"
)

var counter atomic.Int64

func DoSomething(a, b int) int64 {
	return counter.Add(1)
}

func TestFast(t *testing.T) {
	args := func(yield func(reflect.Value) bool) {
		yield(reflect.ValueOf(1))
		yield(reflect.ValueOf(2))
	}
	for arg := range call.Fast(DoSomething, args) {
		fmt.Println(arg.Int64())
	}
	for arg := range call.Fast(DoSomething, args) {
		fmt.Println(arg.Int64())
	}
	for arg := range call.Fast(DoSomething, args) {
		fmt.Println(arg.Int64())
	}
}

func BenchmarkFast(t *testing.B) {
	args := func(yield func(reflect.Value) bool) {
		yield(reflect.ValueOf(1))
		yield(reflect.ValueOf(2))
	}
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		for arg := range call.Fast(DoSomething, args) {
			_ = arg
		}
	}
}

func BenchmarkSlow(t *testing.B) {
	for i := 0; i < t.N; i++ {
		reflect.ValueOf(DoSomething).Call([]reflect.Value{reflect.ValueOf(1), reflect.ValueOf(2)})
	}
}
