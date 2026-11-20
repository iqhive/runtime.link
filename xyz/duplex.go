package xyz

import (
	"encoding/json"
	"fmt"
	"strings"
)

type addable interface {
	int8 | int16 | int32 | int64 | uint8 | uint16 | uint32 | uint64 | float32 | float64 | complex64 | complex128
}

// maxDuplex is the longest Duplex number there is. A Duplex number of n
// numbers holds 2^(n-1) values, so one of a longer array would hold more
// values than there is room for: [Duplex.Values] refuses it, and
// [json.Unmarshal] refuses to make one.
const maxDuplex = 32

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

// UnmarshalJSON refuses a JSON array of more than [maxDuplex] numbers.
func (d *Duplex[T]) UnmarshalJSON(data []byte) error {
	var numbers []T
	if err := json.Unmarshal(data, &numbers); err != nil {
		return err
	}
	if len(numbers) > maxDuplex {
		return fmt.Errorf("a Duplex number of %d numbers holds too many values to give, at most %d", len(numbers), maxDuplex)
	}
	*d = numbers
	return nil
}

// Values returns each value represented by the Duplex number.
// ie
//   - a ± b returns a+b,a-b
//   - a ± b ± c returns a+b+c, a+b-c, a-b+c, a-b-c
//
// A Duplex number of n numbers holds 2^(n-1) values, so one of more than
// [maxDuplex] numbers is refused.
func (d Duplex[T]) Values() []T {
	if len(d) == 0 {
		return nil
	}
	if len(d) > maxDuplex {
		panic(fmt.Sprintf("a Duplex number of %d numbers holds too many values to give, at most %d", len(d), maxDuplex))
	}
	// The sign of each number after the first is one bit of the index of the
	// value it makes, counted from the last number, so the values are given
	// by counting the indexes.
	values := make([]T, 1<<(len(d)-1))
	for i := range values {
		val := d[0]
		for j := 1; j < len(d); j++ {
			if i>>(len(d)-1-j)&1 == 0 {
				val += d[j]
			} else {
				val -= d[j]
			}
		}
		values[i] = val
	}
	return values
}
