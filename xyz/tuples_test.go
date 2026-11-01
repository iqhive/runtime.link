package xyz_test

import (
	"encoding/json"
	"testing"

	"github.com/iqhive/runtime.link/xyz"
)

func TestNullLeavesTuplesAlone(t *testing.T) {
	var s struct {
		P xyz.Pair[int, int]           `json:"p"`
		T xyz.Trio[int, int, int]      `json:"t"`
		Q xyz.Quad[int, int, int, int] `json:"q"`
	}
	s.P, s.T, s.Q = xyz.NewPair(1, 2), xyz.NewTrio(1, 2, 3), xyz.NewQuad(1, 2, 3, 4)
	if err := json.Unmarshal([]byte(`{"p":null,"t":null,"q":null}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.P != xyz.NewPair(1, 2) || s.T != xyz.NewTrio(1, 2, 3) || s.Q != xyz.NewQuad(1, 2, 3, 4) {
		t.Errorf("null changed the values: %+v", s)
	}
}

func TestTuplesRoundTrip(t *testing.T) {
	pair := xyz.NewPair(1, "x")
	trio := xyz.NewTrio(1, "x", 2.5)
	quad := xyz.NewQuad(1, "x", 2.5, true)
	nested := xyz.NewPair(xyz.NewPair(1, 2), []int{3})
	check := func(wire string, in, out any, equal func() bool) {
		t.Helper()
		b, err := json.Marshal(in)
		if err != nil || string(b) != wire {
			t.Fatalf("got %s, %v, want %s", b, err, wire)
		}
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("%s: %v", b, err)
		}
		if !equal() {
			t.Errorf("%s decoded to %v", b, out)
		}
	}
	var p xyz.Pair[int, string]
	check(`[1,"x"]`, pair, &p, func() bool { return p == pair })
	var r xyz.Trio[int, string, float64]
	check(`[1,"x",2.5]`, trio, &r, func() bool { return r == trio })
	var q xyz.Quad[int, string, float64, bool]
	check(`[1,"x",2.5,true]`, quad, &q, func() bool { return q == quad })
	var n xyz.Pair[xyz.Pair[int, int], []int]
	check(`[[1,2],[3]]`, nested, &n, func() bool { return n.X == nested.X && len(n.Y) == 1 && n.Y[0] == 3 })
}

func TestTuplesOfTheWrongShapeAreAnError(t *testing.T) {
	for _, input := range []string{`[1]`, `[]`, `5`, `"x"`, `{}`, `[1,"x"]`} {
		var p xyz.Pair[int, int]
		if err := json.Unmarshal([]byte(input), &p); err == nil {
			t.Errorf("%s decoded to %v", input, p)
		}
	}
	for _, input := range []string{`[1,2]`, `5`} {
		var r xyz.Trio[int, int, int]
		if err := json.Unmarshal([]byte(input), &r); err == nil {
			t.Errorf("%s decoded to %v", input, r)
		}
	}
	for _, input := range []string{`[1,2,3]`, `5`} {
		var q xyz.Quad[int, int, int, int]
		if err := json.Unmarshal([]byte(input), &q); err == nil {
			t.Errorf("%s decoded to %v", input, q)
		}
	}
}

func TestTupleAccessors(t *testing.T) {
	if x, y := xyz.NewPair(1, "x").Get(); x != 1 || y != "x" {
		t.Errorf("pair Get: %v %v", x, y)
	}
	if x, y := xyz.NewPair(1, "x").Split(); x != 1 || y != "x" {
		t.Errorf("pair Split: %v %v", x, y)
	}
	if x, y, z := xyz.NewTrio(1, "x", 2.5).Get(); x != 1 || y != "x" || z != 2.5 {
		t.Errorf("trio Get: %v %v %v", x, y, z)
	}
	if x, y, z := xyz.NewTrio(1, "x", 2.5).Split(); x != 1 || y != "x" || z != 2.5 {
		t.Errorf("trio Split: %v %v %v", x, y, z)
	}
	if a, b, c, d := xyz.NewQuad(1, "x", 2.5, true).Get(); a != 1 || b != "x" || c != 2.5 || !d {
		t.Errorf("quad Get: %v %v %v %v", a, b, c, d)
	}
	if a, b, c, d := xyz.NewQuad(1, "x", 2.5, true).Split(); a != 1 || b != "x" || c != 2.5 || !d {
		t.Errorf("quad Split: %v %v %v %v", a, b, c, d)
	}
}
