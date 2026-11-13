//go:build cgo

package dll

import (
	"strings"
	"testing"
)

func TestSymbolOfASystemLibrary(t *testing.T) {
	table, err := Open("kernel32.dll")
	if err != nil {
		t.Fatal(err)
	}
	address, err := Sym(table, "GetCurrentProcessId")
	if err != nil || address == nil {
		t.Errorf("GetCurrentProcessId = %v, %v", address, err)
	}
	if _, err := Sym(table, "definitely_not_a_symbol"); err == nil || !strings.Contains(err.Error(), "definitely_not_a_symbol") {
		t.Errorf("error = %v", err)
	}
}

func TestFailedOpenExplainsItself(t *testing.T) {
	_, err := Open("does_not_exist.dll")
	if err == nil || !strings.Contains(err.Error(), "does_not_exist.dll") {
		t.Errorf("error = %v", err)
	}
}
