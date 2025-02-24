package box

import (
	"bufio"
	gobinary "encoding/binary"
	"fmt"
	"io"
	"math"
	"reflect"

	"runtime.link/api/xray"
)

const (
	sizingMask = 0b11100000
)

// Decoder for decoding values in box format, if values
// are already stored as the system in-memory representation
// then they can be decoded very quickly.
type Decoder struct {
	enc *Encoder
	r   *bufio.Reader

	first bool

	system Binary
	ptr   []uintptr // For memory reference handling
	refs  map[uintptr]reflect.Value // For circular reference support
}

// NewDecoder returns a new [Decoder] that reads from the
// specified reader.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		r:     bufio.NewReader(r),
		first: true,
		refs:  make(map[uintptr]reflect.Value),
	}
}

// Decode reads the next value from the reader and tries to
// store it in the specified value.
func (dec *Decoder) Decode(val any) error {
	if dec.first {
		var magic [4]byte
		if _, err := io.ReadAtLeast(dec.r, magic[:], 4); err != nil {
			return err
		}
		if magic[0] != 'B' || magic[1] != 'O' || magic[2] != 'X' || magic[3] != '1' {
			return xray.New(fmt.Errorf("box: invalid magic header %v", magic))
		}
		dec.first = false
		dec.system = Binary(magic[3])
	}
	binary, err := dec.r.ReadByte()
	if err != nil {
		return xray.New(err)
	}
	object, err := dec.r.ReadBytes(0)
	if err != nil {
		return xray.New(err)
	}
	length, _, err := dec.length(Binary(binary), 1, object)
	if err != nil {
		return xray.New(err)
	}
	xvalue := make([]byte, length) // TODO reuse some sort of buffer.
	if _, err := io.ReadAtLeast(dec.r, xvalue, length); err != nil {
		return xray.New(err)
	}
	object = object[:len(object)-1]
	rvalue := reflect.ValueOf(val)
	if rvalue.Kind() != reflect.Ptr {
		return xray.New(fmt.Errorf("box: value must be a pointer"))
	}
	rvalue = rvalue.Elem()
	_, _, err = dec.slow(Binary(binary), object, rvalue, xvalue)
	return err
}

func (dec *Decoder) slow(binary Binary, object []byte, rvalue reflect.Value, xvalue []byte) (int, int, error) {
	if len(object) == 0 {
		return 0, 0, nil
	}

	obj := Object(object[0])
	args := int(obj & 0b00011111)
	if args == 31 {
		return 0, 0, fmt.Errorf("box: unsupported object schema")
	}

	var offset int
	var schemaType Schema
	if binary&BinarySchema != 0 {
		if len(object) < 2 {
			return 0, 0, fmt.Errorf("box: invalid schema")
		}
		schemaType = Schema(object[1])
		offset = 1 // Skip schema byte
	}

	// Handle schema type with best-effort decoding
	if schemaType != 0 {
		// Extract schema category (top 3 bits)
		schemaCat := schemaType & 0b11100000
		err := dec.handleSchema(schemaCat, rvalue)
		if err != nil {
			return 0, 0, xray.New(err)
		}
	}

	var size int

	switch obj & sizingMask {
	case ObjectRepeat:
		if args == 0 {
			return 0, 1, nil // End of object definition
		}
		n, consumed, err := dec.slow(binary, object[1+offset:], rvalue, xvalue)
		if err != nil {
			return 0, 0, xray.New(err)
		}
		if n == 0 {
			return 0, consumed + 1 + offset, nil
		}
		return n * args, consumed + 1 + offset, nil

	case ObjectStruct:
		var total, consumed int
		for i := 1 + offset; i < len(object); i += consumed + 1 + offset {
			if i >= len(object) || total >= len(xvalue) {
				break
			}
			n, c, err := dec.slow(binary, object[i:], rvalue, xvalue[total:])
			if err != nil {
				return 0, 0, xray.New(err)
			}
			if c == 0 {
				break
			}
			consumed = c
			total += n
		}
		return total, consumed + 1 + offset, nil

	case ObjectBytes1, ObjectBytes2, ObjectBytes4, ObjectBytes8:
		var size int
		switch obj & sizingMask {
		case ObjectBytes1:
			size = 1
		case ObjectBytes2:
			size = 2
		case ObjectBytes4:
			size = 4
		case ObjectBytes8:
			size = 8
		}

		// Set value based on type
		switch rvalue.Kind() {
		case reflect.Bool:
			if len(xvalue) < 1 {
				return 0, 0, fmt.Errorf("box: buffer too small for bool")
			}
			rvalue.SetBool(xvalue[0] != 0)
			return 1, 1 + offset, nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			var val int64
			switch size {
			case 1:
				val = int64(int8(xvalue[0]))
			case 2:
				if len(xvalue) < 2 {
					return 0, 0, fmt.Errorf("box: buffer too small for uint16")
				}
				if binary&BinaryEndian != 0 {
					val = int64(int16(gobinary.BigEndian.Uint16(xvalue)))
				} else {
					val = int64(int16(gobinary.LittleEndian.Uint16(xvalue)))
				}
			case 4:
				if len(xvalue) < 4 {
					return 0, 0, fmt.Errorf("box: buffer too small for uint32")
				}
				if binary&BinaryEndian != 0 {
					val = int64(int32(gobinary.BigEndian.Uint32(xvalue)))
				} else {
					val = int64(int32(gobinary.LittleEndian.Uint32(xvalue)))
				}
			case 8:
				if len(xvalue) < 8 {
					return 0, 0, fmt.Errorf("box: buffer too small for uint64")
				}
				if binary&BinaryEndian != 0 {
					val = int64(gobinary.BigEndian.Uint64(xvalue))
				} else {
					val = int64(gobinary.LittleEndian.Uint64(xvalue))
				}
			}
			rvalue.SetInt(val)
			return size, 1 + offset, nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			var val uint64
			switch size {
			case 1:
				val = uint64(xvalue[0])
			case 2:
				if len(xvalue) < 2 {
					return 0, 0, fmt.Errorf("box: buffer too small for uint16")
				}
				if binary&BinaryEndian != 0 {
					val = uint64(gobinary.BigEndian.Uint16(xvalue))
				} else {
					val = uint64(gobinary.LittleEndian.Uint16(xvalue))
				}
			case 4:
				if len(xvalue) < 4 {
					return 0, 0, fmt.Errorf("box: buffer too small for uint32")
				}
				if binary&BinaryEndian != 0 {
					val = uint64(gobinary.BigEndian.Uint32(xvalue))
				} else {
					val = uint64(gobinary.LittleEndian.Uint32(xvalue))
				}
			case 8:
				if len(xvalue) < 8 {
					return 0, 0, fmt.Errorf("box: buffer too small for uint64")
				}
				if binary&BinaryEndian != 0 {
					val = uint64(gobinary.BigEndian.Uint64(xvalue))
				} else {
					val = uint64(gobinary.LittleEndian.Uint64(xvalue))
				}
			}
			rvalue.SetUint(val)
			return size, 1 + offset, nil
		case reflect.Float32:
			if len(xvalue) < 4 {
				return 0, 0, fmt.Errorf("box: buffer too small for float32")
			}
			var bits uint32
			if binary&BinaryEndian != 0 {
				bits = gobinary.BigEndian.Uint32(xvalue)
			} else {
				bits = gobinary.LittleEndian.Uint32(xvalue)
			}
			rvalue.SetFloat(float64(math.Float32frombits(bits)))
			return size, 1 + offset, nil
		case reflect.Float64:
			if len(xvalue) < 8 {
				return 0, 0, fmt.Errorf("box: buffer too small for float64")
			}
			var bits uint64
			if binary&BinaryEndian != 0 {
				bits = gobinary.BigEndian.Uint64(xvalue)
			} else {
				bits = gobinary.LittleEndian.Uint64(xvalue)
			}
			rvalue.SetFloat(math.Float64frombits(bits))
			return size, 1 + offset, nil
		case reflect.Complex64:
			if len(xvalue) < 8 {
				return 0, 0, fmt.Errorf("box: buffer too small for complex64")
			}
			var real, imag float32
			if binary&BinaryEndian != 0 {
				real = math.Float32frombits(gobinary.BigEndian.Uint32(xvalue[:4]))
				imag = math.Float32frombits(gobinary.BigEndian.Uint32(xvalue[4:8]))
			} else {
				real = math.Float32frombits(gobinary.LittleEndian.Uint32(xvalue[:4]))
				imag = math.Float32frombits(gobinary.LittleEndian.Uint32(xvalue[4:8]))
			}
			rvalue.SetComplex(complex(float64(real), float64(imag)))
			return 8, 1 + offset, nil
		case reflect.Complex128:
			if len(xvalue) < 16 {
				return 0, 0, fmt.Errorf("box: buffer too small for complex128")
			}
			var real, imag float64
			if binary&BinaryEndian != 0 {
				real = math.Float64frombits(gobinary.BigEndian.Uint64(xvalue[:8]))
				imag = math.Float64frombits(gobinary.BigEndian.Uint64(xvalue[8:16]))
			} else {
				real = math.Float64frombits(gobinary.LittleEndian.Uint64(xvalue[:8]))
				imag = math.Float64frombits(gobinary.LittleEndian.Uint64(xvalue[8:16]))
			}
			rvalue.SetComplex(complex(real, imag))
			return 16, 1 + offset, nil
		case reflect.String:
			if len(xvalue) < size {
				return 0, 0, fmt.Errorf("box: buffer too small for string")
			}
			str := string(xvalue[:size])
			rvalue.SetString(str)
			return size, 1 + offset, nil
		case reflect.Array:
			if rvalue.Type().Elem().Kind() == reflect.Uint8 {
				if len(xvalue) < size {
					return 0, 0, fmt.Errorf("box: buffer too small for array")
				}
				reflect.Copy(rvalue, reflect.ValueOf(xvalue[:size]))
				return size, 1 + offset, nil
			}
			// Handle non-byte arrays
			for i := 0; i < rvalue.Len(); i++ {
				if len(xvalue) <= offset {
					break
				}
				n, _, err := dec.slow(binary, object[1+offset:], rvalue.Index(i), xvalue[offset:])
				if err != nil {
					return 0, 0, err
				}
				offset += n
			}
			return offset, 1 + offset, nil
		case reflect.Slice:
			if rvalue.Type().Elem().Kind() == reflect.Uint8 {
				if len(xvalue) < size {
					return 0, 0, fmt.Errorf("box: buffer too small for slice")
				}
				rvalue.SetBytes(append([]byte(nil), xvalue[:size]...))
				return size, 1 + offset, nil
			}
			// Handle non-byte slices
			var elements []reflect.Value
			var consumed int
			for i := 1 + offset; i < len(object); i += consumed + 1 + offset {
				if i >= len(object) {
					break
				}
				elem := reflect.New(rvalue.Type().Elem()).Elem()
				n, c, err := dec.slow(binary, object[i:], elem, xvalue[offset:])
				if err != nil {
					return 0, 0, err
				}
				if c == 0 {
					break
				}
				consumed = c
				offset += n
				elements = append(elements, elem)
			}
			slice := reflect.MakeSlice(rvalue.Type(), len(elements), len(elements))
			for i, elem := range elements {
				slice.Index(i).Set(elem)
			}
			rvalue.Set(slice)
			return offset, consumed + 1 + offset, nil
		case reflect.Struct:
			// Handle struct fields recursively
			for i := 0; i < rvalue.NumField(); i++ {
				field := rvalue.Field(i)
				if !field.CanSet() {
					continue
				}
				n, _, err := dec.slow(binary, object[1+offset:], field, xvalue[offset:])
				if err != nil {
					return 0, 0, err
				}
				offset += n
			}
			return offset, 1 + offset, nil
		case reflect.Map:
			// Initialize map if nil
			if rvalue.IsNil() {
				rvalue.Set(reflect.MakeMap(rvalue.Type()))
			}
			// Read key-value pairs
			for offset < len(xvalue) {
				key := reflect.New(rvalue.Type().Key()).Elem()
				if len(xvalue) <= offset {
					break
				}
				n, _, err := dec.slow(binary, object[1+offset:], key, xvalue[offset:])
				if err != nil {
					break
				}
				offset += n

				if len(xvalue) <= offset {
					break
				}
				value := reflect.New(rvalue.Type().Elem()).Elem()
				n, _, err = dec.slow(binary, object[1+offset:], value, xvalue[offset:])
				if err != nil {
					break
				}
				offset += n

				rvalue.SetMapIndex(key, value)
			}
			return offset, 1 + offset, nil
		default:
			return 0, 0, fmt.Errorf("box: unsupported type for fixed-size value: %v", rvalue.Kind())
		}
		return size, 1 + offset, nil

		case ObjectMemory:
		size = 0
		switch binary & BinaryMemory {
		case MemorySize1:
			size = 1
		case MemorySize2:
			size = 2
		case MemorySize4:
			size = 4
		case MemorySize8:
			size = 8
		}

		if len(xvalue) < size {
			return 0, 0, fmt.Errorf("box: buffer too small for memory reference")
		}

		// Read memory reference
		var addr uintptr
		switch size {
		case 1:
			addr = uintptr(xvalue[0])
		case 2:
			if binary&BinaryEndian != 0 {
				addr = uintptr(gobinary.BigEndian.Uint16(xvalue))
			} else {
				addr = uintptr(gobinary.LittleEndian.Uint16(xvalue))
			}
		case 4:
			if binary&BinaryEndian != 0 {
				addr = uintptr(gobinary.BigEndian.Uint32(xvalue))
			} else {
				addr = uintptr(gobinary.LittleEndian.Uint32(xvalue))
			}
		case 8:
			if binary&BinaryEndian != 0 {
				addr = uintptr(gobinary.BigEndian.Uint64(xvalue))
			} else {
				addr = uintptr(gobinary.LittleEndian.Uint64(xvalue))
			}
		}

		// Handle nil pointers
		if addr == 0 {
			if rvalue.Kind() == reflect.Ptr || rvalue.Kind() == reflect.Map {
				rvalue.Set(reflect.Zero(rvalue.Type()))
			}
			return size, 1 + offset, nil
		}

		// Handle circular references
		if ref, ok := dec.refs[addr]; ok {
			if ref.IsValid() {
				rvalue.Set(ref)
			}
			return size, 1 + offset, nil
		}

		// Create new reference
		switch rvalue.Kind() {
		case reflect.Ptr:
			if rvalue.IsNil() {
				rvalue.Set(reflect.New(rvalue.Type().Elem()))
			}
			dec.refs[addr] = rvalue
			// Decode the pointed-to value
			if n, c, err := dec.slow(binary, object[1+offset:], rvalue.Elem(), xvalue[size:]); err != nil {
				return 0, 0, err
			} else {
				return size + n, c + 1 + offset, nil
			}
		case reflect.Map:
			if rvalue.IsNil() {
				rvalue.Set(reflect.MakeMap(rvalue.Type()))
			}
			dec.refs[addr] = rvalue
			// Continue decoding map entries
			return size, 1 + offset, nil
		case reflect.String:
			// String references are handled separately
			return size, 1 + offset, nil
		default:
			// Best-effort decode for other types
			if rvalue.CanAddr() {
				dec.refs[addr] = rvalue.Addr()
			}
			return size, 1 + offset, nil
		}
		

		// Handle nil pointers
		if addr == 0 {
			if rvalue.Kind() == reflect.Ptr || rvalue.Kind() == reflect.Map {
				rvalue.Set(reflect.Zero(rvalue.Type()))
			}
			return size, 1 + offset, nil
		}

		// Handle circular references
		if ref, ok := dec.refs[addr]; ok {
			// Found existing reference
			if ref.Kind() == reflect.Ptr && rvalue.Kind() == reflect.Ptr {
				// For pointers, we need to point to the same target
				rvalue.Set(ref)
			} else {
				// For other types, we copy the value
				rvalue.Set(ref)
			}
			return size, 1 + offset, nil
		}

		// Create new reference
		switch rvalue.Kind() {
		case reflect.Ptr:
			if rvalue.IsNil() {
				rvalue.Set(reflect.New(rvalue.Type().Elem()))
			}
			// Store reference for circular reference support
			dec.refs[addr] = rvalue
		case reflect.Map:
			if rvalue.IsNil() {
				rvalue.Set(reflect.MakeMap(rvalue.Type()))
			}
			// Store reference for circular reference support
			dec.refs[addr] = rvalue
		case reflect.String:
			// String references are handled separately
			break
		default:
			// Best-effort decode for other types
			if rvalue.CanAddr() {
				dec.refs[addr] = rvalue.Addr()
			}
		}
		return size, 1 + offset, nil

	case ObjectIgnore:
		if args == 0 {
			return 0, 1 + offset, nil // Close struct
		}
		return args, 1 + offset, nil
	}

	return 0, 0, fmt.Errorf("box: invalid object type: %v", obj&sizingMask)
}

func (dec *Decoder) handleSchema(schema Schema, rvalue reflect.Value) error {
	// Allow interface types for all schemas for dynamic handling
	if rvalue.Kind() == reflect.Interface {
		return nil
	}

	// Extract schema category (top 3 bits) and group (next 3 bits)
	category := schema & 0b11100000
	group := (schema & 0b00011111) >> 4

	// Handle byte schemas (0x00)
	if category == 0 {
		switch group {
		case 1: // Unknown
			return nil
		case 2: // Boolean
			switch rvalue.Kind() {
			case reflect.Bool:
				return nil
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				return nil
			default:
				return fmt.Errorf("box: incompatible type for boolean: %v", rvalue.Kind())
			}
		case 3: // Natural
			switch rvalue.Kind() {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				return nil
			default:
				return fmt.Errorf("box: incompatible type for natural: %v", rvalue.Kind())
			}
		case 4: // Integer
			switch rvalue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return nil
			default:
				return fmt.Errorf("box: incompatible type for integer: %v", rvalue.Kind())
			}
		case 5: // IEEE754
			switch rvalue.Kind() {
			case reflect.Float32, reflect.Float64:
				return nil
			default:
				return fmt.Errorf("box: incompatible type for float: %v", rvalue.Kind())
			}
		case 6: // Elapsed
			switch rvalue.Kind() {
			case reflect.Int64:
				return nil
			default:
				return fmt.Errorf("box: incompatible type for duration: %v", rvalue.Kind())
			}
		case 7: // Instant
			switch rvalue.Kind() {
			case reflect.Int64:
				return nil
			default:
				return fmt.Errorf("box: incompatible type for instant: %v", rvalue.Kind())
			}
		}
		return fmt.Errorf("box: unsupported schema group: %v", group)
	}

	// Handle structure schemas (0x20)
	if category == 0x20 {
		switch group {
		case 1: // Sourced
			switch rvalue.Kind() {
			case reflect.Struct, reflect.Map:
				return nil
			default:
				return fmt.Errorf("box: incompatible type for struct: %v", rvalue.Kind())
			}
		case 2, 3: // Indexed, Mapping
			switch rvalue.Kind() {
			case reflect.Map, reflect.Struct:
				return nil
			default:
				return fmt.Errorf("box: incompatible type for map: %v", rvalue.Kind())
			}
		case 4: // Program
			switch rvalue.Kind() {
			case reflect.Struct:
				return nil
			default:
				return fmt.Errorf("box: incompatible type for program: %v", rvalue.Kind())
			}
		case 5: // Dynamic
			return nil
		case 6: // Channel
			if rvalue.Kind() != reflect.Chan {
				return fmt.Errorf("box: incompatible type for channel: %v", rvalue.Kind())
			}
			return nil
		case 7: // Pointer
			switch rvalue.Kind() {
			case reflect.Ptr:
				return nil
			default:
				return fmt.Errorf("box: incompatible type for pointer: %v", rvalue.Kind())
			}
		}
		return nil
	}

	// Handle repeated schemas (0x40)
	if category == 0x40 {
		switch group {
		case 1: // Ordered
			switch rvalue.Kind() {
			case reflect.Slice, reflect.Array, reflect.String:
				return nil
			default:
				return fmt.Errorf("box: incompatible type for ordered collection: %v", rvalue.Kind())
			}
		case 2: // Unicode
			switch rvalue.Kind() {
			case reflect.String, reflect.Slice, reflect.Array:
				return nil
			default:
				return fmt.Errorf("box: incompatible type for string: %v", rvalue.Kind())
			}
		case 3: // Reflect
			return nil
		case 4: // Complex
			switch rvalue.Kind() {
			case reflect.Complex64, reflect.Complex128:
				return nil
			default:
				return fmt.Errorf("box: incompatible type for complex: %v", rvalue.Kind())
			}
		case 5, 6: // VectorN, TensorN
			switch rvalue.Kind() {
			case reflect.Slice, reflect.Array:
				return nil
			default:
				return fmt.Errorf("box: incompatible type for vector/tensor: %v", rvalue.Kind())
			}
		case 7: // Numeric
			switch rvalue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
				reflect.Float32, reflect.Float64:
				return nil
			default:
				return fmt.Errorf("box: incompatible type for numeric: %v", rvalue.Kind())
			}
		}
		return nil
	}

	return nil
}

func (dec *Decoder) length(binary Binary, multiplier int, object []byte) (length int, n int, err error) {
	sizing := 0
	schema := binary&BinarySchema != 0
	memory := 0
	switch binary & BinaryMemory {
	case MemorySize1:
		memory = 1
	case MemorySize2:
		memory = 2
	case MemorySize4:
		memory = 4
	case MemorySize8:
		memory = 8
	}
	for i := 0; i < len(object); i++ {
		size := Object(object[i])
		args := int(size & 0b00011111)
		if args == 31 {
			return sizing, i, fmt.Errorf("box: unsupported object schema")
		}
		switch size & sizingMask {
		case ObjectRepeat:
			multiplier *= args
		case ObjectBytes1:
			sizing += 1 * multiplier
		case ObjectBytes2:
			sizing += 2 * multiplier
		case ObjectBytes4:
			sizing += 4 * multiplier
		case ObjectBytes8:
			sizing += 8 * multiplier
		case ObjectStruct:
			if i+1 >= len(object) {
				return sizing, i, fmt.Errorf("box: invalid object schema")
			}
			length, n, err := dec.length(binary, 1, object[i+1:])
			if err != nil {
				return sizing, i, err
			}
			sizing += length * multiplier
			i += n
		case ObjectIgnore:
			if args == 0 {
				return sizing, i + 1, nil
			}
			sizing += args * multiplier
		case ObjectMemory:
			sizing += memory * multiplier
		}
		if multiplier > 1 {
			multiplier = 1
		}
		if schema {
			i++
		}
	}
	return sizing, len(object), nil
}
