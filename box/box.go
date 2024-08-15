/*
Package box provides mechanisms for binary encoding and decoding of the "Binary Object X" format.

BOX is a self-describing binary format, similar to encoding/gob, identified with an initial BOX control
sequence along with a binary configuration byte, followed by one or more messages within the stream, each message
begins with a header that defines a numbered 'box' in the subsequent payload format. Each 'box' acts as
a numerical field identifier, similar to the proto-number in protocol buffers. These boxes represent semantic
sub-components of a predefined data structure. These data structures can evolve over time as long as box numbers
are not reused for a different purpose then it was originally defined as.

"BOX1" then stream of [Binary](if Binary.NotifyBits then [NotifyBits Stream])[Object][0][X](if Binary.MemoryBits then [MemoryBits Length][Memory])...

The encoding is flexible, as implementations can decider whether to optimise for speed and/or size. Messages
sent between the same system and implementation will typically be the fastest to encode and decode. BOX is
designed to be a suitable as a long-term storage format as well as being useful for network communication.

When rich schema information is included in messages, decoders should be able to derive information about the
data structure without prior knowledge of the schema. As such, the Schema Bit should always be enabled for files.

# Binary Byte

  - 0b10000000 - Lookup previous header N, where N is the remaining seven bits in the binary byte.
  - 0b01100000 - Notify Bits stream N (if not 0b00) with the contents of the message, where N is the next 'Notify Bits'&2 number of bytes.
  - 0b00000100 -
  - 0b00000010 - Schema Bit (if 1, object bytes are suffixed with a schema byte).
  - 0b00000001 - Endian Bit (if 1, big endian, if 0, little endian).

# Object Byte

The three most-significant bits of an object byte are used to define the kind of the box, the remaining bits
are used to identify the box number. The end of the object is marked with a zero byte.

  - 0b11100000 - Kind Bits
  - 0b00011111 - Box Number Bits, if 0, ignore, if 31, the box number overflows either to the schema byte or else the following uint16.

# Schema Byte

The schema byte is used to define the type of the box, the schema byte is only present if the Schema Bit is set in the configuration byte.

  - 0b11110000 - Type Bits
  - 0b00001111 - String Name Lengrh, the number of subsequent bytes (after the extended box number) for the string name of the field.

# Payload

The payload is the data that is being encoded, the payload is encoded based on the structure defined by the object bytes. The start of the
payload is always preceded by a zero byte.

# Memory

If the Memory Bit is set in the configuration byte, then the payload is followed by a uint16 length and that number of bytes as extra memory.
Memory data is used to store values that are referenced by pointers in the payload.

# Kind Bits

Where N is the box number associated with the object byte.

  - 0x0 << 7 - notify, when box number is 0, means end-of-header, otherwise identifies a channel number N.
  - 0x1 << 7 - lookup, 0 means the next object byte is a pointer, decode it from the message's memory.
  - 0x2 << 7 - struct, defines a new sub-structure in box N, with an isolated box-number address space.
  - 0x3 << 7 - bytes1, 1 byte of data in box N.
  - 0x4 << 7 - bytes2, 2 bytes of data in box N.
  - 0x5 << 7 - bytes4, 4 bytes of data in box N.
  - 0x6 << 7 - bytes8, 8 bytes of data in box N.
  - 0x7 << 7 - repeat, repeat the next object byte N times, where N is the box number belonging to the object byte.

# Type Bits

  - 0x0 << 4 // interpret value as raw bits.
  - 0x1 << 4 // interpret value as boolean
  - 0x2 << 4 // interpret value as natural number
  - 0x3 << 4 // interpret value as integer
  - 0x4 << 4 // interpret value as an IEEE 754 floating point value
  - 0x5 << 4 // interpret value as web assembly, or if struct with single pointer to a string, interpret as a function URI.
  - 0x6 << 4 // interpret value as a matrix
  - 0x7 << 4 // interpret value as a pointer
  - 0x8 << 4 // interpret struct with two fields as a mapping from the first to the second
  - 0x9 << 4 // interpret struct with a pointer field and 1, 2 length and capacity fields.
  - 0xA << 4 // interpret value a channel number.
  - 0xB << 4 // interpret value a unicode string
  - 0xC << 4 // interpret value a time value offset from the [metaEpoch]
  - 0xD << 4 // interpret value as a dynamic type, with a [typePointer] field in box 1 and a [typePointer] field in box 2.
  - 0xE << 4 // interpret value as a defined type.
  - 0xF << 4 // interpret value as padding.
*/
package box

type big bool // endianness

type sys byte

const (
	metaLookup sys = 0b10000000 // lookup previous header N, where N is the remaining number of bits.
	metaNotify sys = 0b01100000 // notify bits [not]
	metaMemory sys = 0b00011000 // buffer bits [buf]
	metaString sys = 0b00000100 // string names are included for each box number?
	metaSchema sys = 0b00000010 // schema types are included in the data?
	metaEndian sys = 0b00000001 // big endian?
)

type buf byte

const (
	size0 buf = 0b00 // no buffer
	size1 buf = 0b01 // 8 bit len
	size2 buf = 0b00 // 16 bit len
	size4 buf = 0b00 // 32 bit len
)

type box byte

// box byte, 1-30 slots, 31 means slot number is 16 bit, 0 means assign the next available box.
// 3bit kind (5bit box)
const (
	kindRepeat box = 0x0 << 5 // 0 means EOF, > 0 means repeat the next header byte N times.
	kindStruct box = 0x1 << 5 // 0 closes last struct, else open a new struct for box N.
	kindBytes1 box = 0x2 << 5 // box N has 1 byte of data.
	kindBytes2 box = 0x3 << 5 // box N has 2 bytes of data.
	kindBytes4 box = 0x4 << 5 // box N has 4 bytes of data.
	kindBytes8 box = 0x5 << 5 // box N has 8 bytes of data.
	kindAddr16 box = 0x6 << 5 // box N is a 16bit pointer into the memory buffer with a 2 byte length prefix.
	kindAddr32 box = 0x7 << 5 // box N is a 32bit pointer into the memory buffer with a 4 byte length prefix.
)

type vry byte

const ()

type uno byte

// uno byte
// 4bit type
const (
	typeUnknown uno = 0x0 << 4 // interpret value as raw bits.
	typeBoolean uno = 0x1 << 4 // interpret value as boolean
	typeNatural uno = 0x2 << 4 // interpret value as natural number
	typeInteger uno = 0x3 << 4 // interpret value as integer
	typeIEEE754 uno = 0x4 << 4 // interpret value as an IEEE 754 floating point value
	typeProgram uno = 0x5 << 4 // interpret value as web assembly, or if struct with single pointer to a string, interpret as a function URI.
	typeColumns uno = 0x6 << 4 // interpret value as a matrix
	typePointer uno = 0x7 << 4 // interpret value as a pointer
	typeMapping uno = 0x8 << 4 // interpret struct with two fields as a mapping from the first to the second
	typeOrdered uno = 0x9 << 4 // interpret struct with a pointer field and 1, 2 length and capacity fields.
	typeChannel uno = 0xA << 4 // interpret value a channel number.
	typeUnicode uno = 0xB << 4 // interpret value a unicode string
	typeElapsed uno = 0xC << 4 // interpret value a time value offset from the [metaEpoch]
	typeDynamic uno = 0xD << 4 // interpret value as a dynamic type, with a [typePointer] field in box 1 and a [typePointer] field in box 2.
	typeDefined uno = 0xE << 4 // interpret value as a defined type.
	typePadding uno = 0xF << 4 // interpret value as padding.
)
