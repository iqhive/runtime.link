package grpc

import (
	"bytes"
	"testing"
)

func TestEncoderOmitsUnexportedFields(t *testing.T) {
	type private struct{ Secret string }
	type message struct {
		Before      string
		secret      string
		After       string
		unsupported chan int
		private
	}
	value := message{Before: "a", secret: "private-password", After: "z", unsupported: make(chan int), private: private{Secret: "embedded-secret"}}
	// Public fields keep their original field numbers (1 and 3). Skipping a
	// private field must not shift the schema or inflate enclosing lengths.
	one := []byte{0x0a, 6, 0x0a, 1, 'a', 0x1a, 1, 'z'}
	for _, tc := range []struct {
		name  string
		value any
		want  []byte
	}{
		{"struct", value, one},
		{"pointer", &value, one},
		{"slice", []message{value, value}, append(append([]byte{}, one...), one...)},
		{"map", map[string]message{"k": value}, []byte{0x0a, 11, 0x0a, 1, 'k', 0x12, 6, 0x0a, 1, 'a', 0x1a, 1, 'z'}},
		{"all private", struct{ secret string }{"private-password"}, []byte{0x0a, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := NewEncoder(&buf).Encode(tc.value); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(buf.Bytes(), tc.want) {
				t.Fatalf("encoded % x; want % x", buf.Bytes(), tc.want)
			}
		})
	}
}
