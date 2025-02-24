/*
Package box provides binary object serialization utilities for runtime.link.
It implements efficient serialization and deserialization of Go types while
maintaining type safety and memory efficiency.

# Example Usage

Basic serialization:

	type Message struct {
		ID   int64
		Text string
	}

	msg := Message{ID: 1, Text: "Hello"}
	var buf bytes.Buffer
	
	// Serialize
	if err := box.Write(&buf, msg); err != nil {
		return err
	}

	// Deserialize
	var decoded Message
	if err := box.Read(&buf, &decoded); err != nil {
		return err
	}

# Features

The box package supports:
  - Type-safe serialization
  - Memory-efficient encoding/decoding
  - Support for custom type serialization
  - Integration with runtime.link type system

# Custom Type Serialization

Types can implement custom serialization:

	type CustomType struct {
		data []byte
	}

	func (c *CustomType) MarshalBinary() ([]byte, error) {
		return c.data, nil
	}

	func (c *CustomType) UnmarshalBinary(data []byte) error {
		c.data = make([]byte, len(data))
		copy(c.data, data)
		return nil
	}
*/
package box
