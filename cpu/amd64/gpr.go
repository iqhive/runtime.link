package amd64

type Register[T uint8 | uint16 | uint32 | uint64] uint8

func (r Register[T]) AsPointer() Pointer[T] { return Pointer[T](r) }

type AnyRegister interface {
	Register[uint8] | Register[uint16] | Register[uint32] | Register[uint64]
}

func (Register[T]) canAddToRegister(Register[T]) {}
func (Register[T]) canAddToPointer(Pointer[T])   {}

type Pointer[T uint8 | uint16 | uint32 | uint64] uint8

type AnyPointer interface {
	Pointer[uint8] | Pointer[uint16] | Pointer[uint32] | Pointer[uint64]
}

func (Pointer[T]) canAddToRegister(Register[T]) {}

type Imm8 uint8

func (Imm8) canAddToRegister(Register[uint8]) {}
func (Imm8) canAddToPointer(Pointer[uint8])   {}

type Imm16 uint16

func (Imm16) canAddToRegister(Register[uint16]) {}
func (Imm16) canAddToPointer(Pointer[uint16])   {}

type Imm32 uint32

func (Imm32) canAddToRegister(Register[uint32]) {}
func (Imm32) canAddToPointer(Pointer[uint32])   {}

const (
	RAX Register[uint64] = iota
	RCX
	RDX
	RBX
	RSP
	RBP
	RSI
	RDI
)

const (
	EAX Register[uint32] = iota
	ECX
	EDX
	EBX
	ESP
	EBP
	ESI
	EDI
)

const (
	AX Register[uint16] = iota
	CX
	DX
	BX
	SP
	BP
	SI
	DI
)

const (
	AL Register[uint8] = iota
	CL
	DL
	BL
	AH
	CH
	DH
	BH
)

type canBeAddedToRegister[R any] interface {
	canAddToRegister(R)
}

type canBeAddedToPointer[P any] interface {
	canAddToPointer(P)
}
