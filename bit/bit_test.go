package bit_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"runtime.link/bit"
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
