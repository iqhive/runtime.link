package box

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"reflect"
	"unsafe"

	"runtime.link/api/xray"
	"runtime.link/ram"
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
func NewDecoder(r ram.Reader) *Decoder {
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
	rtype := reflect.TypeOf(val)
	value := reflect.ValueOf(val)
	if rtype.Kind() != reflect.Ptr {
		return xray.New(fmt.Errorf("box: cannot decode into non-pointer type %v", rtype))
	}
	rtype = rtype.Elem()
	var memory bytes.Buffer
	dec.enc = NewEncoder(&memory)
	specific, err := dec.enc.basic(1, rtype, reflect.Value{})
	if err != nil {
		return xray.New(err)
	}
	header, err := dec.r.ReadBytes(0)
	if err != nil {
		return err
	}
	header = header[:len(header)-1]
	if len(specific.then) == 0 && bytes.Equal(header, memory.Bytes()) {
		_, err := io.ReadAtLeast(dec.r, unsafe.Slice((*byte)(value.UnsafePointer()), rtype.Size()), int(rtype.Size()))
		return err
	}
	return xray.New(fmt.Errorf("box: slow decoding for %s not implemented yet", rtype))
}
