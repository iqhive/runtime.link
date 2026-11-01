package jit

import (
	"reflect"
	"runtime"
	"testing"
	"unsafe"
)

// A pin that is lost track of is a panic, in a finalizer, that takes down the program.
func TestPinnerReleasesEverythingItPinned(t *testing.T) {
	asm := Assembly{direct: true}
	for range 64 {
		p := asm.Pinner()
		p.Pin(Value{direct: reflect.ValueOf(new(int))})
		p.Pin(Value{direct: reflect.ValueOf(make([]byte, 16))})
		p.Pin(Value{direct: reflect.ValueOf(string(make([]byte, 16)))})
		p.Pin(Value{direct: reflect.ValueOf(unsafe.Pointer(new(int)))})
		p.Pin(Value{direct: reflect.ValueOf("literal")})
		p.Pin(Value{direct: reflect.ValueOf("")})
		p.Pin(Value{direct: reflect.ValueOf([]byte(nil))})
		p.Pin(Value{direct: reflect.ValueOf((*int)(nil))})
		p.Unpin()
	}
	runtime.GC()
	runtime.GC()
}

func TestPinnerHoldsPinsBetweenCalls(t *testing.T) {
	asm := Assembly{direct: true}
	p := asm.Pinner()
	defer p.Unpin()
	first, second := new(int), new(int)
	p.Pin(Value{direct: reflect.ValueOf(first)})
	p.Pin(Value{direct: reflect.ValueOf(second)})
	if p.pinner == (runtime.Pinner{}) {
		t.Errorf("the pins were made on a copy")
	}
}

func TestIsDirectComparesEachParameter(t *testing.T) {
	for _, c := range []struct {
		a, b reflect.Type
		want bool
	}{
		{reflect.TypeFor[func(int32) int32](), reflect.TypeFor[func(uint32) uint32](), true},
		{reflect.TypeFor[func(*int)](), reflect.TypeFor[func(uintptr)](), true},
		{reflect.TypeFor[func(float64)](), reflect.TypeFor[func(float64)](), true},
		{reflect.TypeFor[func(int8)](), reflect.TypeFor[func(string)](), false},
		{reflect.TypeFor[func(int8)](), reflect.TypeFor[func(int16)](), false},
		{reflect.TypeFor[func() int8](), reflect.TypeFor[func() int16](), false},
		{reflect.TypeFor[func(int64)](), reflect.TypeFor[func(float64)](), false},
		{reflect.TypeFor[func(int)](), reflect.TypeFor[func(int, int)](), false},
		{reflect.TypeFor[func(string)](), reflect.TypeFor[func(string)](), false},
	} {
		if got := isDirect(c.a, c.b); got != c.want {
			t.Errorf("isDirect(%s, %s) = %v", c.a, c.b, got)
		}
	}
}
