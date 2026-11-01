package sodium_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/sql/std/sodium"
)

func TestSocketOperationCodesAreDistinct(t *testing.T) {
	rtype := reflect.TypeFor[sodium.Socket]()
	seen := make(map[string]string)
	for i := range rtype.NumField() {
		field := rtype.Field(i)
		tag, ok := field.Tag.Lookup("xyz")
		if !ok {
			continue
		}
		code, _, _ := strings.Cut(tag, "(")
		if other, ok := seen[code]; ok {
			t.Errorf("%s and %s share the operation code %s", other, field.Name, code)
		}
		seen[code] = field.Name
	}
}
