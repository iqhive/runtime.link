/*
Package box provides mechanisms for binary encoding and decoding of the "Binary Object eXchange" format.

BOX is a self-describing binary format, similar to encoding/gob, identified with an initial BOX control
sequence along with a configuration byte, followed by one or more messages within a stream, each message
begins with a header that defines each encoded 'box' in the subsequent payload format. Each 'box' acts as
a numerical field identifier, similar to the proto-number in protocol buffers. These boxes represent semantic
parts of a predefined data structure. These data structures can evolve over time as long as they do not reuse
the same box for new fields.

"BOX" then [Configuration Byte][Header]0[Payload] if 'Buffer Bit' then [u16 length] and [Buffer]...

Encoding is flexible and encoding implementations can decider whether to optimise for speed and/or size.

# Configuration Byte

  - 0b11100000 - Version Bits, use 0b00 for version 1.0 of the BOX format.
  - 0b00011000 - Duration Bits, use 0b00 for nanoseconds, 0b01 for microseconds, 0b10 for milliseconds, 0b11 for seconds.
  - 0b00000100 - Buffer Bit, use 0b1 if the message contains a buffer.
  - 0b00000010 - Schema Bit, use 0b1 if additional type annotation are included in each Header.
  - 0b00000001 - Endian Bit, use 0b1 for big endian, or 0b0 for little endian.

# Header Byte

The three most-significant bits of the header byte are used to define the kind of the box, the remaining bits
are used to identify the box number. The end of the header is marked with a zero byte.

  - 0b11100000 - Kind Bits
  - 0b00011111 - Box Number Bits, if 0, ignore, if 31, the box number overflows either to the schema byte or else the following uint16.

# Schema Byte

The schema byte is used to define the type of the box, the schema byte is only present if the Schema Bit is set in the configuration byte.

  - 0b11110000 - Type Bits
  - 0b00001111 - Box Number Overflow Bits

# Payload

The payload is the data that is being encoded, the payload is encoded based on the structure defined by the header bytes. The start of the
payload is always preceded by a zero byte.

# Buffer

If the Buffer Bit is set in the configuration byte, then the payload is followed by a uint16 length and that number of bytes as buffer data.
Buffer data may contain nested messages (without the 'BOX' control sequence) or arbitary bytes such that numerical values may be interpreted
as pointers that refer to an offset location within the bytes of the message's buffer.

# Kind Bits

Where N is the box number associated with the header byte.

  - 0b00000000 - notify, when box number is 0, means end-of-header, otherwise identifies a channel number N.
  - 0b00100000 - lookup, 0 means the next header byte is a pointer, otherwise replace with previously defined header N.
  - 0b01000000 - struct, defines a new sub-structure in box N, with an isolated box-number address space.
  - 0b01100000 - bytes1, 1 byte of data in box N.
  - 0b10000000 - bytes2, 2 bytes of data in box N.
  - 0b10100000 - bytes4, 4 bytes of data in box N.
  - 0b11000000 - bytes8, 8 bytes of data in box N.
  - 0b11100000 - repeat, repeat the next header byte N times, where N is the box number belonging to the header byte.
*/
package box

type big bool // endianness

type sys byte

const (
	metaEndian sys = 0b00000001 // big endian?
	metaSchema sys = 0b00000010 // schema is included in the data
	metaBuffer sys = 0b00000100 // buffer follows payload
	metaTicker sys = 0b00011000 // duration type bits
	metaUpdate sys = 0b11100000 // version bits
)

type utc byte

const (
	timeNanos utc = 0b00 // nanoseconds
	timeMicro utc = 0b01 // microseconds
	timeMilli utc = 0b10 // milliseconds
	timeUnits utc = 0b11 // seconds
)

type box byte

// box byte, 1-30 slots, 31 means slot number is 16 bit, 0 means assign the next available box.
// 3bit kind (5bit box)
const (
	kindNotify box = 0x0 << 5 // 0 means EOF, > 0 means channel number
	kindLookup box = 0x1 << 5 // 0 means next byte is pointer, > 0 means replace with previously defined header.
	kindStruct box = 0x2 << 5 // 0 closes last struct, else open a new struct for box N.
	kindBytes1 box = 0x3 << 5
	kindBytes2 box = 0x4 << 5
	kindBytes4 box = 0x5 << 5
	kindBytes8 box = 0x6 << 5
	kindRepeat box = 0x7 << 5
)

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
