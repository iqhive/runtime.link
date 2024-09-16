package xyz

import (
	"fmt"
	"strings"
)

type addable interface {
	int8 | int16 | int32 | int64 | uint8 | uint16 | uint32 | uint64 | float32 | float64 | complex64 | complex128
}

// Duplex numbers (a±b±c...)
type Duplex[T addable] []T

// String returns the string representation of the Duplex number.
func (d Duplex[T]) String() string {
	var s strings.Builder
	for i, v := range d {
		if i > 0 {
			s.WriteString("±")
		}
		s.WriteString(fmt.Sprint(v))
	}
	return s.String()
}

// Values returns each value represented by the Duplex number.
// ie
//   - a ± b returns a+b,a-b
//   - a ± b ± c returns a+b+c, a+b-c, a-b+c, a-b-c
func (d Duplex[T]) Values() []T {
	var values []T
	var branch func(idx int, val T)
	branch = func(idx int, val T) {
		if idx == len(d) {
			values = append(values, val)
			return
		}
		branch(idx+1, val+d[idx])
		branch(idx+1, val-d[idx])
	}
	branch(1, d[0])
	return values
}
