package xyz_test

import (
	"errors"
	"testing"

	"github.com/iqhive/runtime.link/xyz"
)

type byCaseType xyz.Tagged[any, struct {
	Text   xyz.Case[byCaseType, string]
	Number xyz.Case[byCaseType, float64]
	Fail   xyz.Case[byCaseType, error]
	Ptr    xyz.Case[byCaseType, *int]
	Lit    byCaseType `txt:"a"`
}]

var _ xyz.Validator = byCaseType{}

// Validate does not check its value, and reports false for every one, as it
// did in 840f71d.
func TestValidateReportsFalse(t *testing.T) {
	var v byCaseType
	for _, val := range []any{"x", "a", 1.5, errors.New("x"), new(int), (*int)(nil), nil, 1, true} {
		if v.Validate(val) {
			t.Errorf("%#v reported valid", val)
		}
	}
}
