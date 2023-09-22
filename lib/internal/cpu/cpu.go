package cpu

import (
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"unsafe"
)

type Error int8

func (err Error) Error() string {
	return fmt.Sprintf("cpu: %d", int8(err))
}

/*
	Helpful links:

	https://go.googlesource.com/go/+/refs/heads/master/src/cmd/compile/abi-internal.md
*/

// Program can be converted into any Go function at runtime. The virtual machine [Instruction]
// is geared for writing calling convention converters, although it is turing complete and can
// be used for general purpose computation (with limited access to the system).
type Program struct {
	Call unsafe.Pointer // unsafe pointer to the function that will be called by the [Call] instruction.
	Size int            // stack/frame size, zero if unknown.
	Text []Instruction
	Pins []Location

	Data []uint
	Func []func(Register) Register

	Dump func() // debug dumper

	mutx sync.Mutex
	ptrs []pins
}

// Add the given instructions to the program.
func (program *Program) Add(ins ...Instruction) {
	program.Text = append(program.Text, ins...)
}

// Pin the given pointer to the program.
func (program *Program) Pin(reg ...Location) {
	program.Pins = append(program.Pins, reg...)
}

type pins struct {
	free bool
	pins runtime.Pinner
}

func (p *Program) pin(ptr any) func() {
	p.mutx.Lock()
	defer p.mutx.Unlock()
	for i := range p.ptrs {
		if p.ptrs[i].free {
			p.ptrs[i].pins.Pin(ptr)
			p.ptrs[i].free = false
			return func() {
				p.mutx.Lock()
				defer p.mutx.Unlock()
				p.ptrs[i].pins.Unpin()
				p.ptrs[i].free = true
			}
		}
	}
	var pin runtime.Pinner
	pin.Pin(ptr)
	p.ptrs = append(p.ptrs, pins{
		free: false,
		pins: pin,
	})
	i := len(p.ptrs) - 1
	return func() {
		p.mutx.Lock()
		defer p.mutx.Unlock()
		p.ptrs[i].pins.Unpin()
		p.ptrs[i].free = true
	}
}

func Prepare()
func Restore()
func PushFunc()
func CallFunc()
func GrowStack()

// MakeFunc is the safest way to convert a Program into a Go function (still very unsafe!)
// as it garuntees that the Program will have access to the full set of registers. The
// resulting call-overhead is the highest. This can be optimized by calling the variants
// of MakeFunc that leverage a more restricted set of registers.
func (p *Program) MakeFunc(rtype reflect.Type) reflect.Value {
	r0, x0 := 0, 0
	for _, op := range p.Text {
		mode, data := op.Decode()
		switch mode {
		case Load, Move:
			data := Location(data)
			if data < X0 {
				r0 = max(r0, int(data))
			} else {
				x0 = max(x0, int(data-X0))
			}
		}
	}
	// we've determined how many registers are needed by the function
	// so that we can then switch to the implementation that uses the
	// least number of registers.
	if r0 <= 1 && x0 <= 0 {
		call := p.callFast
		if len(p.Pins) > 0 {
			call = p.pinCallFast
		}
		return reflect.NewAt(rtype, reflect.ValueOf(&call).UnsafePointer()).Elem()
	}
	call := p.callArch
	if len(p.Pins) > 0 {
		call = p.pinCallArch
	}
	return reflect.NewAt(rtype, reflect.ValueOf(&call).UnsafePointer()).Elem()
}
