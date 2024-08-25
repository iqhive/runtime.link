package box

import (
	"bufio"
	"fmt"
	"io"
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
}

// NewDecoder returns a new [Decoder] that reads from the
// specified reader.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: bufio.NewReader(r), first: true}
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
