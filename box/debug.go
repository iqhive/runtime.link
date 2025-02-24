package box

import (
	"fmt"
	"strings"
)

// DumpBytes returns a human-readable representation of box-encoded bytes
func DumpBytes(data []byte) string {
	if len(data) == 0 {
		return "empty box"
	}

	var b strings.Builder
	b.WriteString("BOX[\n")

	// Check magic bytes
	if len(data) >= 4 && string(data[:4]) == "BOX1" {
		b.WriteString("  Magic: BOX1\n")
		data = data[4:]
	}

	// Parse binary flags
	if len(data) > 0 {
		binary := Binary(data[0])
		b.WriteString(fmt.Sprintf("  Binary: %08b\n", binary))
		b.WriteString(fmt.Sprintf("    Endian: %v\n", binary&BinaryEndian != 0))
		b.WriteString(fmt.Sprintf("    Schema: %v\n", binary&BinarySchema != 0))
		b.WriteString(fmt.Sprintf("    Memory: %v\n", binary&BinaryMemory))
		data = data[1:]
	}

	// Parse objects
	b.WriteString("  Objects:\n")
	indent := "    "
	for i := 0; i < len(data); i++ {
		obj := Object(data[i])
		category := obj & sizingMask
		args := obj & 0b00011111

		switch category {
		case ObjectRepeat:
			b.WriteString(fmt.Sprintf("%sRepeat %d times\n", indent, args))
		case ObjectBytes1:
			b.WriteString(fmt.Sprintf("%sBytes1 (size=%d)\n", indent, args))
		case ObjectBytes2:
			b.WriteString(fmt.Sprintf("%sBytes2 (size=%d)\n", indent, args))
		case ObjectBytes4:
			b.WriteString(fmt.Sprintf("%sBytes4 (size=%d)\n", indent, args))
		case ObjectBytes8:
			b.WriteString(fmt.Sprintf("%sBytes8 (size=%d)\n", indent, args))
		case ObjectStruct:
			b.WriteString(fmt.Sprintf("%sStruct (fields=%d)\n", indent, args))
			indent += "  "
		case ObjectIgnore:
			if args == 0 {
				b.WriteString(fmt.Sprintf("%sEnd Structure\n", indent))
				if len(indent) > 4 {
					indent = indent[:len(indent)-2]
				}
			} else {
				b.WriteString(fmt.Sprintf("%sIgnore %d bytes\n", indent, args))
			}
		case ObjectMemory:
			b.WriteString(fmt.Sprintf("%sMemory Reference (size=%d)\n", indent, args))
		}
	}

	b.WriteString("]\n")
	return b.String()
}
