package id

import (
	"database/sql/driver"
	"errors"
	"testing"
)

func TestErrInvalid(t *testing.T) {
	if !errors.Is(ErrInvalid, ErrInvalid) {
		t.Error("ErrInvalid must be errors.Is-able with itself")
	}
}

func TestWrap(t *testing.T) {
	if Wrap("x", nil) != nil {
		t.Error("Wrap with nil error must return nil")
	}
	err := Wrap("uuid v7", ErrInvalid)
	if err == nil {
		t.Fatal("Wrap returned nil")
	}
	if !errors.Is(err, ErrInvalid) {
		t.Error("Wrap must preserve the sentinel via errors.Is")
	}
}

func TestScanText(t *testing.T) {
	if s, err := ScanText(nil); err != nil || s != "" {
		t.Errorf("ScanText(nil) = %q, %v", s, err)
	}
	if s, err := ScanText("abc"); err != nil || s != "abc" {
		t.Errorf("ScanText(string) = %q, %v", s, err)
	}
	if s, err := ScanText([]byte("abc")); err != nil || s != "abc" {
		t.Errorf("ScanText([]byte) = %q, %v", s, err)
	}
	if _, err := ScanText(42); err == nil {
		t.Error("ScanText(int) should fail")
	}
}

func TestValue(t *testing.T) {
	v, err := Value("")
	if err != nil || v != nil {
		t.Errorf("Value(\"\") = %v, %v, want nil,nil", v, err)
	}
	v, err = Value("abc")
	if err != nil || v != "abc" {
		t.Errorf("Value(\"abc\") = %v, %v", v, err)
	}
	if _, ok := v.(driver.Value); !ok {
		t.Error("returned value is not a driver.Value")
	}
}

func TestMarshalJSON(t *testing.T) {
	b, err := MarshalJSON("abc")
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `"abc"` {
		t.Errorf("MarshalJSON = %s", b)
	}
	if _, err := MarshalJSON(""); err == nil {
		t.Error("MarshalJSON(\"\") should fail")
	}
}
