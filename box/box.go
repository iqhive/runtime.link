/*
Package box provides mechanisms for binary encoding and decoding of the "Binary Object eXchange" format.

BOX is a self-describing strongly-typed binary format, similar to encoding/gob, identified with an initial BOX control
sequence along with a binary configuration byte, followed by one or more messages within the stream, each message
begins with a header that defines a numbered 'box' in the subsequent payload format. Each 'box' acts as
a numerical field identifier, similar to the proto-number in protocol buffers. These boxes represent semantic
sub-components of a predefined data structure. These data structures can evolve over time as long as box numbers
are not reused for a different purpose then it was originally defined as.

	"BOX1" then stream of 'messages': [Binary][Object]0[Memory][EOM || u16[Memory] || u32[Memory]]

The encoding is flexible, as implementations can decider whether to optimise for speed and/or size. Messages
sent between the same system and implementation will typically be the fastest to encode and decode. BOX is
designed to be a suitable for use as a long-term storage format as well as on-the-wire network communication.

When rich schema information is included in messages, decoders should be able to derive information about the
data structure without prior knowledge of the schema. As such, the Schema Bit should always be enabled for
messages that are intended for long-term storage.

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
	BinaryLookup Binary = 0b10000000 // lookup a previously defined binary/object N in a 7bit ring buffer, where N is the remaining 7bits. if N is 0, this message has no body, instead, save this object as a new defined binary/object.
	BinaryTiming Binary = 0b01100000 // offset all times from the unix epoch in, 0=ns, 1=us, 2=ms, 3=s.
	BinaryBits32 Binary = 0b00010000 // 32bit pointers?
	BinaryColumn Binary = 0b00001000 // if 0, columns are stored row major, if 1, columns are stored column major.
	BinaryString Binary = 0b00000100 // if 0, strings are UTF-8, if 1, strings are UTF-16.
	BinarySchema Binary = 0b00000010 // schema types are included in the data?
	BinaryEndian Binary = 0b00000001 // big endian?
)

// NativeBinary returns the binary configuration for the current system.
func NativeBinary() Binary {
	var native Binary
	if reflect.TypeOf(0).Size() == 4 {
		native |= BinaryBits32
	}
	if binary.NativeEndian.Uint16([]byte{0x12, 0x34}) != uint16(0x3412) {
		native |= BinaryEndian
	}
	return native
}

/*
Object Byte

The three most-significant bits of an object byte are used to define the kind of the box, the remaining bits
are used to identify the box number. The end of the object is marked with a zero byte.

  - 0b11100000 - Kind Bits
  - 0b00011111 - Number of
*/
type Object byte

// Where N is the box number associated with the [Object] byte.
const (
	ObjectRepeat Object = 0x0 << 5 // 0 means EOF, > 0 means repeat the next object byte N times.
	ObjectStruct Object = 0x1 << 5 // 0 closes last struct, else open a new struct for box N.
	ObjectBytes1 Object = 0x2 << 5 // box N has 1 byte of data, if 0, then N is the next sequential box.
	ObjectBytes2 Object = 0x3 << 5 // box N has 2 bytes of data, if 0, then N is the next sequential box.
	ObjectBytes4 Object = 0x4 << 5 // box N has 4 bytes of data, if 0, then N is the next sequential box.
	ObjectBytes8 Object = 0x5 << 5 // box N has 8 bytes of data, if 0, then N is the next sequential box.
	ObjectMemory Object = 0x6 << 5 // 0 box N is a [metaMemory]bit pointer, into the memory buffer with a ([metaMemory]/2)bit byte length prefix.
	ObjectIgnore Object = 0x7 << 5 // ignore the next N object bytes, if 0, increment the sequential box number by 1.
)

// Schema Byte is included when [BinarySchema] is set and includes more specific
// type hints for each box.
//
// - 0b11110000 - Schema Bits
// - 0b00001111 - Number of bytes to read the UTF8 string hint for the box, if 7, then the next byte is the length instead, preceeds the [Object] byte.
type Schema byte

const (
	SchemaUnknown Schema = 0x0 << 4 // interpret value as raw bits.
	SchemaBoolean Schema = 0x1 << 4 // interpret value as boolean.
	SchemaNatural Schema = 0x2 << 4 // interpret value as natural number.
	SchemaInteger Schema = 0x3 << 4 // interpret value as integer.
	SchemaIEEE754 Schema = 0x4 << 4 // interpret value as an IEEE 754 floating point value.
	SchemaProgram Schema = 0x5 << 4 // interpret value as a function/program.
	SchemaColumns Schema = 0x6 << 4 // interpret value as a tensor/matrix.
	SchemaPointer Schema = 0x7 << 4 // interpret value as a pointer.
	SchemaMapping Schema = 0x8 << 4 // interpret struct with two fields as a mapping from the first to the second.
	SchemaOrdered Schema = 0x9 << 4 // interpret struct with a pointer field and 1, 2 length and capacity fields.
	SchemaChannel Schema = 0xA << 4 // interpret value a channel number.
	SchemaUnicode Schema = 0xB << 4 // interpret value a unicode string.
	SchemaElapsed Schema = 0xC << 4 // interpret value as a time duration measured in [BinaryTiming].
	SchemaInstant Schema = 0xD << 4 // interpret value as an instant in time, since the unix epoch, measured in [BinaryTiming].
	SchemaDynamic Schema = 0xE << 4 // interpret value as a enum/union/any type, the next byte is the ID of a previously defined schema where each defined box number is a possible value.
	SchemaDefined Schema = 0xF << 4 // interpret value as a named type, with a subsequent schema byte that defines the underlying type.
)
