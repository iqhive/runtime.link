//go:build arm64

package arm64_test

import (
	"errors"
	"math"
	"os"
	"slices"
	"testing"

	"runtime.link/api"
	"runtime.link/api/cmdl"
	. "runtime.link/cpu/arm64"
)

var sysctl = api.Import[struct {
	api.Specification

	List func() []string `cmdl:"-a"`
}](cmdl.API, "sysctl", nil)

var (
	cssc bool
	mte  bool
)

func call[T any](t *testing.T, asm func(API) error, features ...*bool) T {
	t.Helper()
	for _, feature := range features {
		if !*feature {
			t.SkipNow()
		}
	}
	fn, err := Compile[T](asm)
	if err != nil {
		t.Fatal(err)
	}
	return fn
}

func fn_i_i(expected int64, t *testing.T, a int64, asm func(API) error, features ...*bool) {
	t.Helper()
	if result := call[func(int64) int64](t, asm, features...)(a); result != expected {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func fn_u_u(expected uint64, t *testing.T, a uint64, asm func(API) error, features ...*bool) {
	t.Helper()
	if result := call[func(uint64) uint64](t, asm, features...)(a); result != expected {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func fn_ui8_u(expected uint64, t *testing.T, a uint64, b int8, asm func(API) error, features ...*bool) {
	t.Helper()
	if result := call[func(uint64, int8) uint64](t, asm, features...)(a, b); result != expected {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func fn_uu_u(expected uint64, t *testing.T, a, b uint64, asm func(API) error, features ...*bool) {
	t.Helper()
	if result := call[func(uint64, uint64) uint64](t, asm, features...)(a, b); result != expected {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestMain(m *testing.M) {
	list := sysctl.List()
	if slices.Contains(list, "hw.optional.arm.FEAT_CSSC: 1") {
		cssc = true
	}
	if slices.Contains(list, "hw.optional.arm.FEAT_MTE: 1") {
		mte = true
	}
	os.Exit(m.Run())
}

const (
	MaxU = math.MaxUint64
	MaxI = math.MaxInt64
)

type T = testing.T

func Err(errs ...error) error { return errors.Join(errs...) }

func TestABS(t *T) { fn_i_i(1, t, -1, func(q API) error { return Err(q.ABS(0, 0), q.RET(30)) }, &cssc) }
func TestADC(t *T) {
	fn_uu_u(3, t, 1, 2, func(q API) error { return Err(q.CMPs(0, 0, 0, 0), q.ADC(0, 0, 1), q.RET(30)) })
}
func TestADCS(t *T) {
	fn_uu_u(1, t, MaxU, 1, func(q API) error {
		return Err(q.ADDS.Immediate(1, 0, 0), q.ADCS(0, 0, 1), q.CSET(0, CarrySet), q.RET(30))
	})
}
func TestADD(t *T) {
	fn_ui8_u(8, t, 5, 3, func(q API) error { return Err(q.ADD.ExtendedRegister(0, 0, 1, ExtendSignedByte, 0), q.RET(30)) })
	fn_i_i(8, t, 5, func(q API) error { return Err(q.ADD.Immediate(0, 0, 3), q.RET(30)) })
	fn_uu_u(5+(3<<2), t, 5, 3, func(q API) error { return Err(q.ADD.ShiftedRegister(0, 0, 1, ShiftLogicalLeft, 2), q.RET(30)) })
}
func TestADDG(t *T) {
	fn_uu_u(0xB23456789ABCD100, t, 0xA23456789ABCD000, 0, func(q API) error { return Err(q.ADDG(0, 0, 16, 1), q.RET(30)) }, &mte)
}

func TestADDS(t *T) {
	fn_ui8_u(1, t, MaxU, 1, func(q API) error {
		return Err(q.ADDS.ExtendedRegister(0, 0, 1, ExtendSignedByte, 0), q.CSET(0, CarrySet), q.RET(30))
	})
	fn_u_u(1, t, MaxU, func(q API) error { return Err(q.ADDS.Immediate(0, 0, 1), q.CSET(0, CarrySet), q.RET(30)) })
	fn_uu_u(1, t, MaxU, 1, func(q API) error {
		return Err(q.ADDS.ShiftedRegister(0, 0, 1, ShiftLogicalLeft, 0), q.CSET(0, CarrySet), q.RET(30))
	})
}
