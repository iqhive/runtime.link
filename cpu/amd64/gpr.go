package amd64

type Register[T uint8 | uint16 | uint32 | uint64] uint8

type AnyRegister interface {
	Register[uint8] | Register[uint16] | Register[uint32] | Register[uint64]
}

func (Register[T]) canAddTo(Register[T]) {}

type Pointer[T uint8 | uint16 | uint32 | uint64] uint8

type AnyPointer interface {
	Pointer[uint8] | Pointer[uint16] | Pointer[uint32] | Pointer[uint64]
}

func (Pointer[T]) canAddTo(Register[T]) {}

type Imm8 uint8

func (Imm8) canAddTo(Register[uint8]) {}

type Imm16 uint16

func (Imm16) canAddTo(Register[uint16]) {}

type Imm32 uint32

func (Imm32) canAddTo(Register[uint32]) {}

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
