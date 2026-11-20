package xyz_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/xyz"
)

func TestDuplex(t *testing.T) {
	var a = xyz.Duplex[float64]{5, 2, 1}
	fmt.Println(a)
	fmt.Println(a.Values())
}

func TestDuplexWithoutValues(t *testing.T) {
	var d xyz.Duplex[float64]
	if got := d.Values(); len(got) != 0 {
		t.Errorf("got %v", got)
	}
	if got := d.String(); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestDuplexValues(t *testing.T) {
	tests := []struct {
		number xyz.Duplex[int32]
		values []int32
		text   string
	}{
		{xyz.Duplex[int32]{5}, []int32{5}, "5"},
		{xyz.Duplex[int32]{5, 2}, []int32{7, 3}, "5±2"},
		{xyz.Duplex[int32]{5, 2, 1}, []int32{8, 6, 4, 2}, "5±2±1"},
		{xyz.Duplex[int32]{5, 2, 1, 3}, []int32{11, 5, 9, 3, 7, 1, 5, -1}, "5±2±1±3"},
	}
	for _, tt := range tests {
		if got := tt.number.Values(); !slices.Equal(got, tt.values) {
			t.Errorf("%v gave %v, want %v", tt.number, got, tt.values)
		}
		if got := tt.number.String(); got != tt.text {
			t.Errorf("got %q, want %q", got, tt.text)
		}
	}
}

func TestDuplexRoundTripsThroughJSON(t *testing.T) {
	in := xyz.Duplex[float64]{5, 2.5}
	b, err := json.Marshal(in)
	if err != nil || string(b) != `[5,2.5]` {
		t.Fatalf("got %s, %v", b, err)
	}
	var out xyz.Duplex[float64]
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(out, in) {
		t.Errorf("got %v", out)
	}
}

// A Duplex number of n numbers holds 2^(n-1) values, so one made of an array
// of any length would hold more values than there is room for: the array is
// refused, and one that is made anyway is refused by Values.
func TestDuplexRefusesAlongerNumberThanItHolds(t *testing.T) {
	over := []byte("[" + strings.Repeat("1,", 32) + "1]")
	var d xyz.Duplex[float64]
	if err := json.Unmarshal(over, &d); err == nil {
		t.Error("a Duplex number of 33 numbers was made, want the refusal")
	}
	// One of as many numbers as there is room for is made as it always was.
	if err := json.Unmarshal([]byte("["+strings.Repeat("1,", 31)+"1]"), &d); err != nil {
		t.Errorf("a Duplex number of 32 numbers was refused: %v", err)
	}

	long := make(xyz.Duplex[float64], 33)
	defer func() {
		if recover() == nil {
			t.Error("the values of 33 numbers were given, want the refusal")
		}
	}()
	long.Values()
}
