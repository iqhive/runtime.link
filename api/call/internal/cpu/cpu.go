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

// MakeFunc converts a Program into a Go function. The Program sees arguments in,
// and leaves results in, the registers that Go's internal ABI assigns to them.
// A value that the ABI would put on the stack is out of reach of a Program.
//
// The function is real, built by [reflect.MakeFunc], as a Program interpreter
// can't take the place of a function of another type: Go's calling convention has
// the caller reserve the space for the callee to spill its register arguments.
func (p *Program) MakeFunc(rtype reflect.Type) reflect.Value {
	var (
		args = make([]assignment, rtype.NumIn())
		rets = make([]assignment, rtype.NumOut())
		r, x int
	)
	for i := range args {
		args[i] = assign(rtype.In(i), &r, &x)
	}
	r, x = 0, 0
	for i := range rets {
		rets[i] = assign(rtype.Out(i), &r, &x)
	}
	return reflect.MakeFunc(rtype, func(values []reflect.Value) []reflect.Value {
		var reg RegistersArch
		for i, value := range values {
			args[i].load(&reg, value)
		}
		var pins runtime.Pinner
		defer pins.Unpin()
		for _, pin := range p.Pins {
			pins.Pin(general(&reg, int(pin)).UnsafePointer())
		}
		out := p.run(reg, &pins)
		results := make([]reflect.Value, len(rets))
		for i := range rets {
			results[i] = rets[i].store(&out, rtype.Out(i))
		}
		return results
	})
}
