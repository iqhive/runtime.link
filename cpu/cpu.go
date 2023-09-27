/*
Package cpu enables you to compile and link to machine code at runtime.

# Tag Format

The 'cpu' tag format is used to specify how arguments are converted to machine code on a
bit-by-bit level. Each output byte is space-seperated, and each byte can either be a
literal hex value (starting with '0x'), or a bit sequence. The bit sequence can contain
substitutions based on the tagged function's arguments 'a', 'b', 'c' or 'd'.

  - a: will be replaced with the Nth bit of 'a' where N is the number
    of occurences of 'a' so far in the tag.

  - [a000]: 'a' is the 1st argument and will be written here wrapped as a 3bit
    value (the number of zeros determines the number of bits written).

  - [a>n]: 'a' is the 1st argument and a 1 will be written here if the value
    of 'a' is greater than n, otherwise a 0 will be written. n can be any integer.
*/
package cpu

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"unsafe"

	"runtime.link/qnq"
)

// Architecture should be embedded inside of an instruction set structure.
type Architecture struct {
	qnq.Host
}

// Program being written with a specific instruction set.
type Program[Assembly hasIntructionSet] struct {
	Assemble *Assembly
	program  *program
}

type program struct {
	arch string
	code []byte
	done bool
}

func (arch Architecture) hasArchitecture() {}

type hasIntructionSet interface {
	hasArchitecture()
}

// Symbol is a reference to location within a program. It can be used
// as an entrypoint, or to make a Go function value at runtime.
type Symbol struct {
	program *program
	offset  uint
}

// Symbol returns a symbol pointing to the current location in the
// program.
func (src *Program[T]) Symbol() Symbol {
	return Symbol{
		program: src.program,
		offset:  uint(len(src.program.code)),
	}
}

// Compile the program into executable memory and bake in any symbol
// offsets.
func (src *Program[T]) Compile() error {
	//fmt.Printf("%x\n", src.program.code)
	//fmt.Printf("%b\n", src.program.code)
	return src.compile()
}

// Make a Go function value from the given symbol.
func Make[Function any](sym Symbol) Function {
	return MakeFunc(reflect.TypeOf([0]Function{}).Elem(), sym).Interface().(Function)
}

// MakeFunc creates a Go function of the given type that executes the
// given machine code. This function is very unsafe, as it enables
// arbitrary code execution.
func MakeFunc(rtype reflect.Type, sym Symbol) reflect.Value {
	if rtype.Kind() != reflect.Func {
		panic("cpu: MakeFunc called with non-func type")
	}
	if sym.program.arch != runtime.GOARCH {
		panic(fmt.Sprintf("cpu: cannot make function for '%s' on '%s'", sym.program.arch, runtime.GOARCH))
	}
	//fmt.Printf("%x\n", sym.program.code)
	ptr := &sym.program.code[sym.offset]
	ref := unsafe.Pointer(&ptr)
	//println(ptr)
	return reflect.NewAt(rtype, unsafe.Pointer(&ref)).Elem()
}

// NewProgram returns a new program using the given instruction set.
func NewProgram[Assembly hasIntructionSet]() Program[Assembly] {
	var src Program[Assembly]
	src.program = new(program)
	src.Assemble = new(Assembly)
	structure := qnq.StructureOf(&src.Assemble)
	link(src.program, structure, strings.Contains(structure.Host.Get("cpu"), ",reverse"))
	src.program.arch, _, _ = strings.Cut(structure.Host.Get("cpu"), ",")
	return src
}

func link(program *program, structure qnq.Structure, reverse bool) {
	for _, fn := range structure.Functions {
		tag := fn.Tags.Get("cpu")
		fn.Make(func(args []reflect.Value) []reflect.Value {
			var (
				name rune
				bits uint8
			)
			var words = strings.Split(tag, " ")
			if reverse {
				for i, j := 0, len(words)-1; i < j; i, j = i+1, j-1 {
					words[i], words[j] = words[j], words[i]
				}
			}
			for _, word := range words {
				if strings.HasPrefix(word, "0x") {
					b, err := strconv.ParseUint(word[2:], 16, 8)
					if err != nil {
						panic(err)
					}
					program.code = append(program.code, byte(b))
					continue
				}
				var (
					sub bool

					check int = -1
					more  bool
					less  bool
					same  bool

					n int = 7 // current bit index
					b byte
				)
				for i, char := range word {
					if bits > 0 && sub {
						switch char {
						case 'a', 'b', 'c', 'd':
							name = char
						case ']':
							if bits == 0 {
								panic("invalid cpu tag")
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
								literal, err := strconv.ParseUint(word[check:i], 10, 64)
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
							sub = false
							bits = 0
							check = -1
							more = false
							less = false
							same = false
						case '0':
							if more || less || same || check != -1 {
								panic("invalid cpu tag")
							}
							bits++
						case '<':
							if bits > 1 {
								panic("invalid cpu tag")
							}
							less = true
						case '>':
							if bits > 1 {
								panic("invalid cpu tag")
							}
							more = true
						case '=':
							if bits > 1 {
								panic("invalid cpu tag")
							}
							same = true
						case '1', '2', '3', '4', '5', '6', '7', '8', '9':
							if bits > 1 {
								panic("invalid cpu tag")
							}
							if check == -1 {
								check = i
							}
						default:
							panic("invalid cpu tag")
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
					case 'a', 'b', 'c', 'd':
						size := uint8(strings.Count(tag, string(char))) - 1
						if name != char {
							bits = 0
						}
						name = char
						var arg = args[name-'a'].Uint()
						arg = arg % (1 << uint(size))
						if arg&(1<<uint(size-bits)) != 0 {
							b = b | (1 << n)
						}
						bits++
						n--
					case '[':
						bits = 1
						sub = true
					default:
						if bits == 0 {
							panic("invalid cpu tag")
						}
					}
				}
				if n != -1 {
					panic(word + " has too few bits")
				}
				program.code = append(program.code, b)
				b = 0
			}
			return nil
		})
	}
	for _, namespace := range structure.Namespace {
		link(program, namespace, reverse)
	}
}
