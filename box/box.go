/*
Package box provides mechanisms for binary encoding and decoding of the "Binary Object eXchange" format.
*/
package box

type big bool // endianness

type sys byte

const (
	metaUpdate sys = 0b11000000 // version
	metaTicker sys = 0b00110000 // utc bits
	metaEndian sys = 0b00001000 // big endian?
	metaSchema sys = 0b00000100 // schema is included in the data
	metaString sys = 0b00000010 // string names are included in the schema
	metaEpochs sys = 0b00000001 // the next byte is the epoch to use (if not the unix one).
)

type utc byte

const (
	timeNanos utc = 0 // nanoseconds
	timeMicro utc = 1 // microseconds
	timeMilli utc = 2 // milliseconds
	timeUnits utc = 3 // seconds
)

type box byte

// box byte, 1-30 slots, 31 means slot number is 16 bit, 0 means assign the next available box.
// 3bit kind (5bit box)
const (
	kindStatic box = 0x0 << 5 // 0 means EOF, > 0 means channel number
	kindLookup box = 0x1 << 5 // 0 closes last struct
	kindStruct box = 0x2 << 5
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
	typePadding uno = 0xF << 4
)
