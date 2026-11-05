/*
Package bin enables you to represent binary formats.

# Bin Tag

The 'bin' tag format is used to specify how arguments are converted to binary on a
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
package bin

import (
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/iqhive/runtime.link/api"
)

// Format should be embedded inside of an binary format.
type Format interface {
	format()
}

// Writer for a specific binary format.
type Writer[Encoder Format] struct {
	Encoder *Encoder
	code    *[]byte
}

func (src *Writer[T]) Bytes() []byte {
	return *src.code
}

// NewWriter returns a new writer using the given assembly format.
func NewWriter[Encoder Format]() Writer[Encoder] {
	var src Writer[Encoder]
	src.Encoder = new(Encoder)
	src.code = new([]byte)
	structure := api.StructureOf(&src.Encoder)
	link(src.code, structure, isReversed(reflect.TypeFor[Encoder]()))
	return src
}

// isReversed looks at the tag of the embedded [Format], which is what names
// the format, it isn't a host so [api.Structure.Host] doesn't include it.
func isReversed(encoder reflect.Type) bool {
	format, ok := encoder.FieldByName("Format")
	return ok && strings.Contains(format.Tag.Get("bin"), ",reverse")
}

// integer returns an argument as bits. Register
// numbers are the usual reason for it to be signed.
func integer(arg reflect.Value) uint64 {
	if arg.CanInt() {
		return uint64(arg.Int())
	}
	return arg.Uint()
}

// width returns the number of bits of the given argument
// that a tag spells out letter by letter, outside of any '[]'.
func width(tag string, name rune) uint8 {
	var (
		count  uint8
		inside bool
	)
	for _, char := range tag {
		switch char {
		case '[':
			inside = true
		case ']':
			inside = false
		case name:
			if !inside {
				count++
			}
		}
	}
	return count
}

func link(code *[]byte, structure api.Structure, reverse bool) {
	for _, fn := range structure.Functions {
		tag := fn.Tags.Get("bin")
		fn.Make(func(args []reflect.Value) []reflect.Value {
			var (
				name rune
				bits uint8
				seen [4]uint8 // bits written so far of each argument.
			)
			var encoded []byte
			for _, word := range strings.Fields(tag) {
				if strings.HasPrefix(word, "0x") {
					b, err := strconv.ParseUint(word[2:], 16, 8)
					if err != nil {
						panic(err)
					}
					encoded = append(encoded, byte(b))
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
								panic("invalid jit tag")
							}
							var arg = integer(args[name-'a'])
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
							if more || less || same {
								if check == -1 {
									check = i
								}
								continue
							}
							if check != -1 {
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
					case 'a', 'b', 'c', 'd':
						size := width(tag, char)
						if integer(args[char-'a'])&(1<<uint(size-1-seen[char-'a'])) != 0 {
							b = b | (1 << n)
						}
						seen[char-'a']++
						n--
					case '[':
						bits = 1
						sub = true
					default:
						if bits == 0 {
							panic("invalid jit tag")
						}
					}
				}
				if n != -1 {
					panic(word + " has too few bits")
				}
				encoded = append(encoded, b)
				b = 0
			}
			// the bytes are reversed and not the words, as an
			// argument can be spelled out across more than one word.
			if reverse {
				slices.Reverse(encoded)
			}
			*code = append(*code, encoded...)
			return nil
		})
	}
	for _, namespace := range structure.Namespace {
		link(code, namespace, reverse)
	}
}
