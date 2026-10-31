package grpc_test

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/api/grpc"
)

// consumeBytes checks the framing as a protobuf reader would, independently of
// the encoder's size calculations.
func consumeBytes(t *testing.T, data *[]byte, tag byte) []byte {
	t.Helper()
	if len(*data) == 0 || (*data)[0] != tag {
		t.Fatalf("expected tag %x, got %x", tag, *data)
	}
	size, n := binary.Uvarint((*data)[1:])
	if n <= 0 {
		t.Fatal("invalid length varint")
	}
	rest := (*data)[1+n:]
	if size > uint64(len(rest)) {
		t.Fatalf("length %d exceeds available %d bytes", size, len(rest))
	}
	value := rest[:int(size)]
	*data = rest[int(size):]
	return value
}

func TestEncoderMapFraming(t *testing.T) {
	for _, values := range []map[string]string{
		nil,
		{},
		{"a": "b"},
		{"a": "b", "longer": strings.Repeat("x", 200)},
	} {
		for _, nested := range []bool{false, true} {
			var input any = values
			if nested {
				input = struct{ Values map[string]string }{values}
			}
			var buf bytes.Buffer
			if err := grpc.NewEncoder(&buf).Encode(input); err != nil {
				t.Fatal(err)
			}
			data := buf.Bytes()
			if nested {
				body := consumeBytes(t, &data, 0x0a)
				if len(data) != 0 {
					t.Fatalf("unexpected bytes after enclosing message: %x", data)
				}
				data = body
			}
			got := make(map[string]string)
			for len(data) > 0 {
				entry := consumeBytes(t, &data, 0x0a)
				key := string(consumeBytes(t, &entry, 0x0a))
				value := string(consumeBytes(t, &entry, 0x12))
				if len(entry) != 0 {
					t.Fatalf("unexpected bytes after map entry: %x", entry)
				}
				if _, duplicate := got[key]; duplicate {
					t.Fatalf("duplicate key %q", key)
				}
				got[key] = value
			}
			if len(values) == 0 && len(got) == 0 {
				continue
			}
			if !reflect.DeepEqual(got, values) {
				t.Fatalf("got %v, want %v", got, values)
			}
		}
	}
}
