package data_test

import (
	"testing"
	"unicode/utf8"

	"runtime.link/api/data"
)

type Request struct {
	Bool    bool
	Int     int
	String  string
	UTF8    string
	Slice   []string
	Map     map[string]string
	Pointer *uintptr
}

func (req *Request) Validate() error {
	return data.Object(req).Reports(
		data.Exists(&req.Pointer, func(value *uintptr) error {
			return data.Number(value).Missing()
		}),
		data.Number(&req.Int).Missing(),
		data.String(&req.String).Missing(),
		data.String(&req.String).Exceeds(100),
		data.Sliced(&req.Slice).Exceeds(10),
		data.Mapped(&req.Map).Exceeds(10),
		data.Mapped(&req.Map).ForEach(func(value *string) error {
			return data.String(value).Exceeds(100)
		}),
		data.String(&req.UTF8).Invalid("utf8", utf8.ValidString),
	)
}

func TestData(t *testing.T) {
	var req = Request{
		Bool:   true,
		Int:    1,
		String: "hello",
		Slice:  []string{"a", "b"},
		Map:    map[string]string{"a": "b"},
	}
	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}
}
