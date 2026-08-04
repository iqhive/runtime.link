package bit

import (
	"unsafe"
)

type Array struct {
	buffer complex128
	length int
	memory unsafe.Pointer
}

func (arr Array) Len() int { return arr.length }

func (arr Array) Bytes() []byte {
	if arr.memory != nil {
		return unsafe.Slice((*byte)(arr.memory), arr.length/8)
	}
	return (*[16]byte)(unsafe.Pointer(&arr))[0 : arr.length/8]
}

type Writer interface {
	WriteBits(Array) (int, error)
}

func WriteBytes(w Writer, data ...byte) (int, error) {
	if len(data) > 16 {
		return writeBytesSlow(w, data)
	}
	var array Array
	array.length = len(data) * 8
	copy((*[16]byte)(unsafe.Pointer(&array.buffer))[:], data)
	return w.WriteBits(array)
}

func writeBytesSlow(w Writer, data []byte) (int, error) {
	var n int
	for len(data) > 0 {
		var arr Array
		copy((*[16]byte)(unsafe.Pointer(&arr))[:], data[:min(16, len(data))])
		add, err := w.WriteBits(arr)
		n += add
		if err != nil {
			return n, err
		}
		data = data[min(16, len(data)):]
	}
	return n, nil
}

func WriteString(w Writer, data string) (int, error) {
	if len(data) > 16 {
		var n int
		for len(data) > 0 {
			var arr Array
			arr.length = len(data) * 8
			copy((*[16]byte)(unsafe.Pointer(&arr))[:], data)
			add, err := w.WriteBits(arr)
			n += add
			if err != nil {
				return n, err
			}
			data = data[min(16, len(data)):]
		}
		return n, nil
	}
	var array Array
	array.length = len(data) * 8
	copy((*[16]byte)(unsafe.Pointer(&array))[:], data)
	return w.WriteBits(array)
}

type Reader interface {
	ReadBits(int) (Array, error)
}

type StreamWriter struct {
	w   Writer
	buf [1024]byte
	n   int
}

func NewStreamWriter(w Writer) StreamWriter {
	return StreamWriter{w: w}
}

func (s *StreamWriter) WriteBits(array Array) (int, error) {
	dat := array.Bytes()
	if s.n+len(dat) > len(s.buf) {
		if err := s.Flush(); err != nil {
			return 0, err
		}
	}
	if len(dat) > len(s.buf) {
		return s.w.WriteBits(array)
	}
	s.n += copy(s.buf[s.n:], dat)
	return array.Len(), nil
}

func (s *StreamWriter) Flush() error {
	if s.n > 0 {
		if _, err := WriteBytes(s.w, s.buf[:s.n]...); err != nil {
			return err
		}
		s.n = 0
	}
	return nil
}
