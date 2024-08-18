/*
Package box provides mechanisms for binary encoding and decoding of the "Binary Object X" format.

BOX is a self-describing strongly-typed binary format, similar to encoding/gob, identified with an initial BOX control
sequence along with a binary configuration byte, followed by one or more messages within the stream, each message
begins with a object specificaton that defines a series of numbered 'boxes' in the subsequent payload format. Each
'box' acts as a numerical field identifier, similar to the proto-number in protocol buffers. These boxes represent
semantic sub-components of a predefined data structure. These data structures can evolve over time as long as box
numbers are not reused for a different purpose then it was originally defined for and still possibly interpreted as.

	"BOX1" then stream of 'messages': [Binary][Object]0[X]

The encoding is flexible, as implementations can decider whether to optimise for encoding/decoding speed and/or size.
Messages sent between the same system and implementation will typically be the fastest to encode and decode. BOX is
designed to be a suitable for use as a long-term storage format as well as on-the-wire network communication.

When rich schema information is included in messages, decoders should be able to derive information about the
data structure without prior knowledge of the schema. As such, the Schema Bit should always be enabled for
messages that are intended for long-term storage, so that data can be meaningfully inspected without access to
pre-defined schemas.

The intended media type for BOX data is "application/box", or "application/x-binary-object". The file extension
should be treated as ".box" although due to the magic "BOX" string, custom file extensions may be used to
better reflect the kind of data kept in the file.
*/
package box

import (
	"encoding/binary"
	"reflect"
)

/*
Binary Byte

The binary byte begins each BOX message. The binary byte is used to specify the encoding of the message.
*/
type Binary byte

const (
	// BinaryLookup the previously defined binary/object N in a 7bit ring buffer, where N is the remaining 7bits.
	// Implementations should maintain a [128]Object buffer for this purpose. Each Object written into the BOX
	// stream (in order of appearance) is also written into the ring buffer.
	BinaryLookup Binary = 0b10000000

	// BinaryTiming is the mask that identifies the timing bits of the [Binary] byte.
	// See [TimingUnits] for more information on possible values.
	BinaryTiming Binary = 0b01100000

	// BinaryMemory identifies the size of [ObjectMemory] pointers within X.
	BinaryMemory Binary = 0b00010000

	// BinaryColumn indicates whether tensors are stored in column major, by default they are stored in row major.
	BinaryColumn Binary = 0b00000100

	// BinarySchema indicates that a [Schema] byte follows each [Object] byte.
	BinarySchema Binary = 0b00000010

	// BinaryEndian indicates that big endian is used for numerical values rather than the default
	// little endian. This is only applicable to numerical values, not to strings, floats or other types.
	BinaryEndian Binary = 0b00000001 // big endian?
)

// TimingUnits used to specify the unit for instants of time and durations.
const (
	TimingUnits Binary = 0b01100000 // seconds
	TimingNanos Binary = 0b00000000 // nanoseconds
	TimingMicro Binary = 0b00100000 // microseconds
	TimingMilli Binary = 0b01000000 // milliseconds
)

// Memory Address Sizes
const (
	MemorySize1 Binary = 0b00110000 // 1 byte
	MemorySize2 Binary = 0b00010000 // 2 bytes
	MemorySize4 Binary = 0b00100000 // 4 bytes
	MemorySize8 Binary = 0b00000000 // 8 bytes
)

// NativeBinary returns the binary configuration for the current system.
func NativeBinary() Binary {
	var native Binary = MemorySize8
	if reflect.TypeOf(0).Size() == 4 {
		native |= MemorySize4
	}
	if binary.NativeEndian.Uint16([]byte{0x12, 0x34}) != uint16(0x3412) {
		native |= BinaryEndian
	}
	return native
}

/*
Object Byte

The three most-significant bits of an object byte are used to define the kind of the box, the remaining bits
are used as a numerical value. If the numerical value is >30, then the numerical value follows as a uint16
(offset by 30).

  - 0b11100000 - Kind Bits
  - 0b00011111 - Numerical Value
*/
type Object byte

// Where N is the numerical value associated with the [Object] byte.
const (
	// ObjectRepeat indicates that the next object byte should be repeated N times.
	// If N is 0, then this is a 0 byte and marks the end of the [Object] definition
	// and the beginning of the payload.
	ObjectRepeat Object = 0x0 << 5

	// ObjectStruct opens a new structure for box N, if N is 0, then this is an addressable value.
	ObjectStruct Object = 0x1 << 5

	ObjectBytes1 Object = 0x2 << 5 // box N has 1 byte of data, if 0, then this is an addressable value.
	ObjectBytes2 Object = 0x3 << 5 // box N has 2 bytes of data, if 0, then this is an addressable value.
	ObjectBytes4 Object = 0x4 << 5 // box N has 4 bytes of data, if 0, then this is an addressable value.
	ObjectBytes8 Object = 0x5 << 5 // box N has 8 bytes of data, if 0, then this is an addressable value.

	// ObjectMemory means box N is a Memory address of size [BinaryMemory]. The value at this address must begin with
	// a [Binary][Object] definition. If 0, then this is an addressable value.
	ObjectMemory Object = 0x6 << 5

	// ObjectIgnore means to ignore the next N object bytes, if 0, close the last struct.
	ObjectIgnore Object = 0x7 << 5
)

// Schema Byte is included when [BinarySchema] is set and includes more specific
// type hints for each box.
//
// - 0b11100000 - Schema Bits
// - 0b00011111 - Number of bytes to read the UTF8 string hint for the box, if 31, then a uint16 encoded length-30 follows.
type Schema byte

// byte schema
const (
	SchemaUnknown Schema = 0x1 << 4 // bytes
	SchemaBoolean Schema = 0x2 << 4 // interpret bytes as boolean.
	SchemaNatural Schema = 0x3 << 4 // interpret bytes as natural number (unsigned).
	SchemaInteger Schema = 0x4 << 4 // interpret bytes as an integer (signed).
	SchemaIEEE754 Schema = 0x5 << 4 // interpret bytes as an IEEE 754 floating point value.
	SchemaElapsed Schema = 0x6 << 4 // interpret bytes as a time duration measured in [BinaryTiming].
	SchemaInstant Schema = 0x7 << 4 // interpret bytes as an instant in time, since the unix epoch, measured in [BinaryTiming].
)

// structure schema
const (
	SchemaDefined Schema = 0x1 << 4 // interpret structure as a pre-defined struct/tuple.
	SchemaIndexed Schema = 0x2 << 4 // interpret structure as a map[uint]any
	SchemaMapping Schema = 0x3 << 4 // interpret structure as a map/dictionary entry with box 1 as the key and box 2 as the value.
	SchemaProgram Schema = 0x4 << 4 // interpret structure as a function/program specification with box 1 as the arguments, box 2 as the result, box 3 is name, box 4 is the web assembly.
	SchemaDynamic Schema = 0x5 << 4 // interpret structure as a enum/union/any type, each box number represents a possible value.
	SchemaChannel Schema = 0x6 << 4 // interpret structure as a channel send, send the box's value to the channel identified by the box number.
	SchemaPointer Schema = 0x7 << 4 // interpret structure as a 'fat' pointer, box 1 is the memory address, box 2 is the length, box 3 is the capacity.
)

// repeated schema
const (
	SchemaOrdered Schema = 0x1 << 4 // interpret repeated box as an ordered list.
	SchemaUnicode Schema = 0x2 << 4 // interpret bytes as a UTF-8 encoded string.
	SchemaVector2 Schema = 0x3 << 4 // interpret repeated box as a 2D vector / complex number.
	SchemaVector3 Schema = 0x4 << 4 // interpret repeated box as a 3D vector.
	SchemaVector4 Schema = 0x5 << 4 // interpret repeated box as a 4D vector.
	SchemaTabular Schema = 0x6 << 4 // interpret repeated box as a table/matrix.
	SchemaAppends Schema = 0x7 << 4 // interpret repeated box as a single value made up as the concatenation of each element.
)
