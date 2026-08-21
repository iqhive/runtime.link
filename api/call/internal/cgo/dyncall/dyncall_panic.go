//go:build !cgo

package dyncall

import "unsafe"

const (
	Void             = 'v'
	Bool             = 'B'
	Char             = 'c'
	UnsignedChar     = 'C'
	Short            = 's'
	UnsignedShort    = 'S'
	Int              = 'i'
	Uint             = 'I'
	Long             = 'j'
	UnsignedLong     = 'J'
	LongLong         = 'l'
	UnsignedLongLong = 'L'
	Float            = 'f'
	Double           = 'd'
	Pointer          = 'p'
	String           = 'z'
	Aggregate        = 'a'
)

type Signature struct {
	Args    []rune
	Returns rune
}

type Callback struct{}

type CallbackHandler func(*Callback, *Args, unsafe.Pointer) rune

func NewCallback(sig Signature, handler CallbackHandler) *Callback {
	panic("dyncall: CGO is disabled")
}

func (callback *Callback) Free() {}

type Args struct{}

func (args *Args) Bool() int32                     { panic("dyncall: CGO is disabled") }
func (args *Args) Char() int8                      { panic("dyncall: CGO is disabled") }
func (args *Args) Short() int16                    { panic("dyncall: CGO is disabled") }
func (args *Args) Int() int32                      { panic("dyncall: CGO is disabled") }
func (args *Args) Long() int                       { panic("dyncall: CGO is disabled") }
func (args *Args) LongLong() int64                 { panic("dyncall: CGO is disabled") }
func (args *Args) UnsignedChar() uint8             { panic("dyncall: CGO is disabled") }
func (args *Args) UnsignedShort() uint16           { panic("dyncall: CGO is disabled") }
func (args *Args) UnsignedInt() uint32             { panic("dyncall: CGO is disabled") }
func (args *Args) UnsignedLong() uint              { panic("dyncall: CGO is disabled") }
func (args *Args) UnsignedLongLong() uint64        { panic("dyncall: CGO is disabled") }
func (args *Args) Float() float32                  { panic("dyncall: CGO is disabled") }
func (args *Args) Double() float64                 { panic("dyncall: CGO is disabled") }
func (args *Args) Pointer() unsafe.Pointer         { panic("dyncall: CGO is disabled") }

type VM struct{}

func NewVM(size int) *VM {
	panic("dyncall: CGO is disabled")
}

func (vm *VM) Reset()                                    {}
func (vm *VM) Free()                                     {}
func (vm *VM) PushBool(value bool)                       { panic("dyncall: CGO is disabled") }
func (vm *VM) PushChar(value int8)                       { panic("dyncall: CGO is disabled") }
func (vm *VM) PushShort(value int16)                     { panic("dyncall: CGO is disabled") }
func (vm *VM) PushSignedInt(value int32)                 { panic("dyncall: CGO is disabled") }
func (vm *VM) PushSignedLong(value int)                  { panic("dyncall: CGO is disabled") }
func (vm *VM) PushSignedLongLong(value int64)            { panic("dyncall: CGO is disabled") }
func (vm *VM) PushFloat(value float32)                   { panic("dyncall: CGO is disabled") }
func (vm *VM) PushDouble(value float64)                  { panic("dyncall: CGO is disabled") }
func (vm *VM) PushPointer(value unsafe.Pointer)          { panic("dyncall: CGO is disabled") }
func (vm *VM) Call(address unsafe.Pointer)               { panic("dyncall: CGO is disabled") }
func (vm *VM) CallBool(address unsafe.Pointer) bool      { panic("dyncall: CGO is disabled") }
func (vm *VM) CallChar(address unsafe.Pointer) int8      { panic("dyncall: CGO is disabled") }
func (vm *VM) CallShort(address unsafe.Pointer) int16    { panic("dyncall: CGO is disabled") }
func (vm *VM) CallInt(address unsafe.Pointer) int32      { panic("dyncall: CGO is disabled") }
func (vm *VM) CallLong(address unsafe.Pointer) int       { panic("dyncall: CGO is disabled") }
func (vm *VM) CallLongLong(address unsafe.Pointer) int64  { panic("dyncall: CGO is disabled") }
func (vm *VM) CallFloat(address unsafe.Pointer) float32  { panic("dyncall: CGO is disabled") }
func (vm *VM) CallDouble(address unsafe.Pointer) float64 { panic("dyncall: CGO is disabled") }
func (vm *VM) CallPointer(address unsafe.Pointer) unsafe.Pointer {
	panic("dyncall: CGO is disabled")
}
