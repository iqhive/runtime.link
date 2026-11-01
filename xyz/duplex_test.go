package xyz_test

import (
	"encoding/json"
	"fmt"
	"slices"
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
