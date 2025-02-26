package box

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestBox(t *testing.T) {
	type Something struct {
		Number int
		Bool   bool
	}
	var (
		s = Something{Number: 42, Bool: true}
	)
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(s); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Encoded bytes:\n%v", buf.Bytes())
	buf = bytes.Buffer{}
	enc = NewEncoder(&buf)
	if err := enc.Encode(s); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Encoded bytes:\n%v", buf.Bytes())
	var raw = buf.Bytes()
	var decoded Something
	if err := NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != s {
		t.Fatalf("decoded value does not match original: %v != %v (%v)", decoded, s, DumpBytes(raw))
	}
}

func TestStrings(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode("hello"); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Encoded bytes:\n%v", buf.Bytes())

	buf = bytes.Buffer{}
	var Strings struct {
		A string
		B string
	}
	Strings.A = "hello"
	enc = NewEncoder(&buf)
	if err := enc.Encode(Strings); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Encoded bytes:\n%v", buf.Bytes())
}

func TestDecoderTypes(t *testing.T) {
	t.Run("Basic Types", func(t *testing.T) {
		tests := []struct {
			name  string
			value interface{}
		}{
			{"bool", true},
			{"int", int(42)},
			{"int8", int8(8)},
			{"int16", int16(16)},
			{"int32", int32(32)},
			{"int64", int64(64)},
			{"uint", uint(42)},
			{"uint8", uint8(8)},
			{"uint16", uint16(16)},
			{"uint32", uint32(32)},
			{"uint64", uint64(64)},
			{"float32", float32(3.14)},
			{"float64", float64(3.14159)},
			{"string", "hello"},
			{"complex64", complex64(1 + 2i)},
			{"complex128", complex128(1 + 2i)},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var buf bytes.Buffer
				enc := NewEncoder(&buf)
				if err := enc.Encode(tt.value); err != nil {
					t.Fatalf("encode failed: %v", err)
				}

				fmt.Println("Encoded bytes:", buf.Bytes())

				var decoded interface{}
				switch tt.value.(type) {
				case bool:
					var v bool
					decoded = &v
				case int:
					var v int
					decoded = &v
				case int8:
					var v int8
					decoded = &v
				case int16:
					var v int16
					decoded = &v
				case int32:
					var v int32
					decoded = &v
				case int64:
					var v int64
					decoded = &v
				case uint:
					var v uint
					decoded = &v
				case uint8:
					var v uint8
					decoded = &v
				case uint16:
					var v uint16
					decoded = &v
				case uint32:
					var v uint32
					decoded = &v
				case uint64:
					var v uint64
					decoded = &v
				case float32:
					var v float32
					decoded = &v
				case float64:
					var v float64
					decoded = &v
				case string:
					var v string
					decoded = &v
				case complex64:
					var v complex64
					decoded = &v
				case complex128:
					var v complex128
					decoded = &v
				}

				if err := NewDecoder(&buf).Decode(decoded); err != nil {
					t.Fatalf("decode failed: %v", err)
				}

				got := reflect.ValueOf(decoded).Elem().Interface()
				if got != tt.value {
					t.Errorf("got %v, want %v", got, tt.value)
				}
			})
		}
	})

	t.Run("Collections", func(t *testing.T) {
		tests := []struct {
			name  string
			value interface{}
		}{
			{"slice", []int{1, 2, 3}},
			{"array", [3]string{"a", "b", "c"}},
			{"map", map[string]int{"a": 1, "b": 2}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var buf bytes.Buffer
				enc := NewEncoder(&buf)
				if err := enc.Encode(tt.value); err != nil {
					t.Fatalf("encode failed: %v", err)
				}

				var decoded interface{}
				switch tt.value.(type) {
				case []int:
					var v []int
					decoded = &v
				case [3]string:
					var v [3]string
					decoded = &v
				case map[string]int:
					var v map[string]int
					decoded = &v
				}

				if err := NewDecoder(&buf).Decode(decoded); err != nil {
					t.Fatalf("decode failed: %v", err)
				}

				if !reflect.DeepEqual(reflect.ValueOf(decoded).Elem().Interface(), tt.value) {
					t.Errorf("got %v, want %v", decoded, tt.value)
				}
			})
		}
	})
}

func TestDecoderSchema(t *testing.T) {
	t.Run("Schema Validation", func(t *testing.T) {
		tests := []struct {
			name     string
			schema   Schema
			value    interface{}
			wantErr  bool
			errMatch string
		}{
			{"bool-valid", SchemaBoolean, true, false, ""},
			{"bool-invalid", SchemaBoolean, 42, true, "incompatible type"},
			{"natural-valid", SchemaNatural, uint(42), false, ""},
			{"natural-invalid", SchemaNatural, -42, true, "incompatible type"},
			{"integer-valid", SchemaInteger, int(-42), false, ""},
			{"float-valid", SchemaIEEE754, float64(3.14), false, ""},
			{"float-invalid", SchemaIEEE754, "not a float", true, "incompatible type"},
			{"time-valid", SchemaElapsed, int64(42), false, ""},
			{"time-invalid", SchemaElapsed, "not a time", true, "incompatible type"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var buf bytes.Buffer
				enc := NewEncoder(&buf)
				if err := enc.Encode(tt.value); err != nil {
					t.Fatalf("encode failed: %v", err)
				}

				var decoded interface{}
				switch tt.value.(type) {
				case bool:
					var v bool
					decoded = &v
				case uint:
					var v uint
					decoded = &v
				case int:
					var v int
					decoded = &v
				case float64:
					var v float64
					decoded = &v
				case int64:
					var v int64
					decoded = &v
				case string:
					var v string
					decoded = &v
				}

				err := NewDecoder(&buf).Decode(decoded)
				if (err != nil) != tt.wantErr {
					t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
				}
				if err != nil && tt.errMatch != "" && !strings.Contains(err.Error(), tt.errMatch) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMatch)
				}
			})
		}
	})
}

func TestDecoderMemory(t *testing.T) {
	t.Run("Circular References", func(t *testing.T) {
		type Node struct {
			Value int
			Next  *Node
		}

		// Create a circular linked list
		n1 := &Node{Value: 1}
		n2 := &Node{Value: 2}
		n3 := &Node{Value: 3}
		n1.Next = n2
		n2.Next = n3
		n3.Next = n1 // Create cycle

		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		if err := enc.Encode(n1); err != nil {
			t.Fatalf("encode failed: %v", err)
		}

		var decoded Node
		if err := NewDecoder(&buf).Decode(&decoded); err != nil {
			t.Fatalf("decode failed: %v", err)
		}

		// Verify circular reference
		if decoded.Value != 1 {
			t.Errorf("got value %d, want 1", decoded.Value)
		}
		if decoded.Next.Value != 2 {
			t.Errorf("got next value %d, want 2", decoded.Next.Value)
		}
		if decoded.Next.Next.Value != 3 {
			t.Errorf("got next next value %d, want 3", decoded.Next.Next.Value)
		}
		if decoded.Next.Next.Next != &decoded {
			t.Error("circular reference not preserved")
		}
	})

	t.Run("Nil Pointers", func(t *testing.T) {
		type Data struct {
			Ptr *int
			Map map[string]int
		}

		original := Data{
			Ptr: nil,
			Map: nil,
		}

		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		if err := enc.Encode(original); err != nil {
			t.Fatalf("encode failed: %v", err)
		}

		var decoded Data
		if err := NewDecoder(&buf).Decode(&decoded); err != nil {
			t.Fatalf("decode failed: %v", err)
		}

		if decoded.Ptr != nil {
			t.Error("nil pointer not preserved")
		}
		if decoded.Map != nil {
			t.Error("nil map not preserved")
		}
	})
}

func TestDecoderEdgeCases(t *testing.T) {
	t.Run("Empty Values", func(t *testing.T) {
		tests := []struct {
			name  string
			value interface{}
		}{
			{"empty-string", ""},
			{"empty-slice", []int{}},
			{"empty-map", map[string]int{}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var buf bytes.Buffer
				enc := NewEncoder(&buf)
				if err := enc.Encode(tt.value); err != nil {
					t.Fatalf("encode failed: %v", err)
				}

				var decoded interface{}
				switch tt.value.(type) {
				case string:
					var v string
					decoded = &v
				case []int:
					var v []int
					decoded = &v
				case map[string]int:
					var v map[string]int
					decoded = &v
				}

				if err := NewDecoder(&buf).Decode(decoded); err != nil {
					t.Fatalf("decode failed: %v", err)
				}

				if !reflect.DeepEqual(reflect.ValueOf(decoded).Elem().Interface(), tt.value) {
					t.Errorf("got %v, want %v", decoded, tt.value)
				}
			})
		}
	})

	t.Run("Invalid Input", func(t *testing.T) {
		tests := []struct {
			name    string
			input   []byte
			wantErr string
		}{
			{"empty", []byte{}, ""},
			{"invalid-schema", []byte{0xFF}, "invalid schema"},
			{"truncated", []byte{0x01}, "invalid schema"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var decoded interface{}
				err := NewDecoder(bytes.NewReader(tt.input)).Decode(&decoded)
				if err == nil && tt.wantErr != "" {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				}
				if err != nil && tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
			})
		}
	})
}
