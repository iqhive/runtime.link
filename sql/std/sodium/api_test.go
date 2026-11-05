package sodium_test

import (
	"reflect"
	"testing"

	"github.com/iqhive/runtime.link/sql/std/sodium"
)

// The operation codes are on the wire, so they stay as they were published,
// even where Delete and Update share one.
func TestSocketOperationCodesArePublished(t *testing.T) {
	want := map[string]string{
		"Read":   "0",
		"Search": "1(1,2)",
		"Output": "2(1,2,3)",
		"Delete": "3(1,2)",
		"Insert": "4(1,2,3,4,5)",
		"Update": "3(1,2,3)",
		"Manage": "6(1)",
	}
	rtype := reflect.TypeFor[sodium.Socket]()
	got := make(map[string]string)
	for i := range rtype.NumField() {
		field := rtype.Field(i)
		if tag, ok := field.Tag.Lookup("xyz"); ok {
			got[field.Name] = tag
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got operation codes %v, want %v", got, want)
	}
}
