package cgo

import (
	"math"
	"testing"
)

func TestGeneratedFloatConstantsAreExact(t *testing.T) {
	if Constants.CHAR_BIT == 0 {
		t.Skip("no generated constants for this platform")
	}
	for _, c := range []struct {
		name      string
		got, want float64
	}{
		{"FLT_MIN", Constants.FLT_MIN, float64(math.Float32frombits(0x00800000))},
		{"DBL_MIN", Constants.DBL_MIN, math.Float64frombits(0x0010000000000000)},
		{"FLT_TRUE_MIN", Constants.FLT_TRUE_MIN, math.SmallestNonzeroFloat32},
		{"DBL_TRUE_MIN", Constants.DBL_TRUE_MIN, math.SmallestNonzeroFloat64},
		{"FLT_EPSILON", Constants.FLT_EPSILON, math.Ldexp(1, -23)},
		{"DBL_EPSILON", Constants.DBL_EPSILON, math.Ldexp(1, -52)},
	} {
		if c.got != c.want {
			t.Errorf("%s = %g, want %g", c.name, c.got, c.want)
		}
	}
}

func TestDefaultEnvironmentHasNoExceptionsRaised(t *testing.T) {
	if Constants.CHAR_BIT != 0 && Constants.FE_DFL_ENV != 0 {
		t.Errorf("FE_DFL_ENV = %d", Constants.FE_DFL_ENV)
	}
}
