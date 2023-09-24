/*
Package jit provides a mechanism for the generation and execution of machine-code at runtime.

# Tag Format

The 'jit' tag format is used to specify how arguments are converted to machine code on a
bit-by-bit level. Each resulting byte is space-seperated, and each byte can either be a
literal hex value (starting with '0x'), or a bit sequence. The bit sequence can contain
substitutions based on the tagged function's arguments 'a', 'b', 'c' or 'd'.

  - [a000]: 'a' is the 1st argument and will be written here wrapped as a 3bit
    value (the number of zeros determines the number of bits written).

  - [a>n]: 'a' is the 1st argument and a 1 will be written here if the value
    is greater than n, otherwise a 0 will be written. n can be any integer.
*/
package jit

import (
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"runtime.link/std"
)

// Architecture should be embedded inside of an InstructionSet.
type Architecture struct{}

type Code[InstructionSet Assembly] struct {
	Assemble InstructionSet
	program  program
}

type program struct {
	code []byte
	done bool
}

func (arch Architecture) hasArchitecture() {}

func (c *Code[T]) get() *program {
	return &c.program
}

type Program interface {
	get() *program
}

type Assembly interface {
	hasArchitecture()
}

type Symbol struct {
	program *program
	offset  uint
}

// SymbolAt returns a new symbol referring to the current position in
// the machine code.
func SymbolAt(program Program) Symbol {
	return Symbol{
		program: program.get(),
		offset:  uint(len(program.get().code)),
	}
}

func (c *Code[T]) Compile() error {
	// FIXME, it may be possible to use Go allocator (ie. make([]byte))
	// and just set the memory to be executable. In order to do this on
	// linux, the memory in question will need to be aligned to a page
	// boundary. This means we can use GC to free the memory when no
	// longer in-use.
	code := c.program.code
	//fmt.Printf("%x\n", code)
	exec, err := syscall.Mmap(
		-1,
		0,
		len(code),
		syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC, syscall.MAP_PRIVATE|syscall.MAP_ANONYMOUS,
	)
	if err != nil {
		return err
	}
	copy(exec, code)
	c.program.code = exec
	c.program.done = true
	//fmt.Priantf("%x\n", code)
	return nil
}

func Make[T any](sym Symbol) T {
	return MakeFunc(reflect.TypeOf([0]T{}).Elem(), sym).Interface().(T)
}

// MakeFunc creates a Go function of the given type that executes the
// given machine code. This function is very unsafe, as it enables
// arbitrary code execution.
func MakeFunc(rtype reflect.Type, sym Symbol) reflect.Value {
	//fmt.Printf("%x\n", sym.program.code)
	ptr := &sym.program.code[sym.offset]
	ref := unsafe.Pointer(&ptr)
	//println(ptr)
	return reflect.NewAt(rtype, unsafe.Pointer(&ref)).Elem()
}

// New returns a new machine code compiler of the given type.
func New[T Assembly]() *Code[T] {
	var compiler Code[T]
	link(&compiler.program, std.StructureOf(&compiler.Assemble))
	return &compiler
}

func link(program *program, structure std.Structure) {
	for _, fn := range structure.Functions {
		tag := fn.Tags.Get("jit")
		fn.Make(func(args []reflect.Value) []reflect.Value {
			for _, pattern := range strings.Split(tag, " ") {
				if strings.HasPrefix(pattern, "0x") {
					b, err := strconv.ParseInt(pattern[2:], 16, 64)
					if err != nil {
						panic(err)
					}
					program.code = append(program.code, byte(b))
					continue
				}
				var (
					name rune
					bits uint8

					check int = -1
					more  bool
					less  bool
					same  bool

					n int = 7 // current bit index
					b byte
				)
				for i, char := range pattern {
					if bits > 0 {
						switch char {
						case 'a', 'b', 'c', 'd':
							name = char
						case ']':
							if bits == 0 {
								panic("invalid jit tag")
							}
							var arg = args[name-'a'].Uint()
							if bits > 1 || check == -1 {
								if bits > 1 {
									bits--
								}
								arg = arg % (1 << uint(bits))
								for i := int(bits) - 1; i >= 0; i-- {
									if arg&(1<<uint(i)) != 0 {
										b = b | (1 << n)
									}
									//fmt.Printf("%b %v %s %s %v\n", b, n, pattern, string(name), arg)
									n--
								}
							} else {
								literal, err := strconv.ParseUint(pattern[check:i], 10, 64)
								if err != nil {
									panic(err)
								}
								if more && arg > literal {
									b = b | (1 << n)
								}
								if less && arg < literal {
									b = b | (1 << n)
								}
								if same && arg == literal {
									b = b | (1 << n)
								}
								n--
							}
							bits = 0
							check = -1
							more = false
							less = false
							same = false
						case '0':
							if more || less || same || check != -1 {
								panic("invalid jit tag")
							}
							bits++
						case '<':
							if bits > 1 {
								panic("invalid jit tag")
							}
							less = true
						case '>':
							if bits > 1 {
								panic("invalid jit tag")
							}
							more = true
						case '=':
							if bits > 1 {
								panic("invalid jit tag")
							}
							same = true
						case '1', '2', '3', '4', '5', '6', '7', '8', '9':
							if bits > 1 {
								panic("invalid jit tag")
							}
							if check == -1 {
								check = i
							}
						default:
							panic("invalid jit tag")
						}
						continue
					}

					switch char {
					case '0':
						b = b & ^(1 << n)
						n--
					case '1':
						b = b | (1 << n)
						n--
					case '[':
						bits++
					default:
						if bits == 0 {
							panic("invalid jit tag")
						}
					}
					if n < 0 {
						panic("invalid jit tag")
					}
				}
				if n != -1 {
					panic(pattern + " has too few bits")
				}
				program.code = append(program.code, b)
				b = 0
			}
			return nil
		})
	}
	for _, namespace := range structure.Namespace {
		link(program, namespace)
	}
}
