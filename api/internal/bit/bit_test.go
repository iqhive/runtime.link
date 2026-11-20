package bit_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/iqhive/runtime.link/api/internal/bit"
)

func BenchmarkIO(b *testing.B) {
	buf := bytes.NewBuffer(nil)
	var buffer io.Writer = buf
	if os.Getenv("RANDOM") != "" {
		buffer = io.Discard
	}
	for b.Loop() {
		var data = []byte{0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde}
		buffer.Write(data)
		buf.Reset()
	}
}

type BitWriter struct {
	buf bytes.Buffer
}

func (w *BitWriter) WriteBits(array bit.Array) (int, error) {
	n, err := w.buf.Write(array.Bytes())
	return n * 8, err
}

type Discard struct{}

func (Discard) WriteBits(array bit.Array) (int, error) {
	return array.Len(), nil
}

// More than 16 bytes do not fit in the buffer of an Array, so they are
// written 16 at a time: all of them, and only as many bits as there are
// bytes.
func TestWriteMoreThanSixteenBytes(t *testing.T) {
	w := new(BitWriter)
	var data []byte
	for i := range 64 {
		data = append(data, byte(i))
	}
	if n, err := bit.WriteBytes(w, data...); err != nil {
		t.Fatal(err)
	} else if n != len(data)*8 {
		t.Errorf("wrote %d bits, want %d", n, len(data)*8)
	}
	if !bytes.Equal(w.buf.Bytes(), data) {
		t.Errorf("wrote %v, want %v", w.buf.Bytes(), data)
	}

	w.buf.Reset()
	const text = "0123456789abcdefghij"
	if n, err := bit.WriteString(w, text); err != nil {
		t.Fatal(err)
	} else if n != len(text)*8 {
		t.Errorf("wrote %d bits, want %d", n, len(text)*8)
	}
	if w.buf.String() != text {
		t.Errorf("wrote %q, want %q", w.buf.String(), text)
	}
}

func BenchmarkBit(b *testing.B) {
	var buf bit.Writer = new(BitWriter)
	if os.Getenv("RANDOM") != "" {
		buf = Discard{}
	}
	for b.Loop() {
		for range 4 {
			bit.WriteBytes(buf, 0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde)
		}
		buf.(*BitWriter).buf.Reset()
	}
}
