package data_test

import (
	"errors"
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/iqhive/runtime.link/api/data"
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
		var missing *data.ErrMissing
		errors.As(err, &missing)
		fmt.Println(missing.FieldName())
		t.Fatal(err)
	}
}

type privateFields struct {
	Name  string
	cache int
}

func (req *privateFields) Validate() error {
	return data.Object(req).Reports(data.String(&req.Name).Missing())
}

func TestReportingStructWithUnexportedField(t *testing.T) {
	err := (&privateFields{cache: 1}).Validate()
	var missing *data.ErrMissing
	if !errors.As(err, &missing) {
		t.Fatalf("got %v", err)
	}
	if missing.FieldName() != "Name" {
		t.Errorf("FieldName() = %q", missing.FieldName())
	}
}

type unexportedTarget struct {
	name string
}

func TestReportingUnexportedFieldItself(t *testing.T) {
	var req unexportedTarget
	err := data.Object(&req).Reports(data.String(&req.name).Missing())
	var missing *data.ErrMissing
	if !errors.As(err, &missing) || missing.FieldName() != "name" {
		t.Fatalf("got %v", err)
	}
}

type leaf struct {
	Value string
}

func (l *leaf) Validate() error {
	return data.Object(l).Reports(data.String(&l.Value).Missing())
}

type branch struct {
	Hidden int
	Leaf   leaf
	hidden int
}

func (b *branch) Validate() error {
	return data.Object(b).Reports(b.Leaf.Validate())
}

type tree struct {
	Branch branch
}

func (o *tree) Validate() error {
	return data.Object(o).Reports(o.Branch.Validate())
}

func TestNestedReportsNameTheWholePath(t *testing.T) {
	var missing *data.ErrMissing

	err := (&branch{}).Validate()
	if !errors.As(err, &missing) {
		t.Fatalf("got %v", err)
	}
	if missing.FieldName() != "Leaf.Value" || err.Error() != "please provide 'Leaf.Value'" {
		t.Errorf("FieldName() = %q, Error() = %q", missing.FieldName(), err)
	}

	err = (&tree{}).Validate()
	if !errors.As(err, &missing) {
		t.Fatalf("got %v", err)
	}
	if missing.FieldName() != "Branch.Leaf.Value" {
		t.Errorf("FieldName() = %q", missing.FieldName())
	}
	_ = branch{}.hidden
}

func TestFieldNameInEveryMessage(t *testing.T) {
	type inner struct {
		Text string
	}
	type outer struct {
		Inner inner
	}
	validate := func(v *outer, check error) error {
		return data.Object(v).Reports(data.Object(&v.Inner).Reports(check))
	}
	v := outer{Inner: inner{Text: "too long"}}
	tests := []struct {
		err  error
		want string
	}{
		{data.String(&v.Inner.Text).Exceeds(1), "please limit 'Inner.Text' to less than 1 bytes"},
		{data.String(&v.Inner.Text).Invalid("name", func(string) bool { return false }, "hint"), "please ensure 'Inner.Text' is valid\n(name: hint)"},
	}
	for _, tt := range tests {
		if got := validate(&v, tt.err).Error(); got != tt.want {
			t.Errorf("got %q, want %q", got, tt.want)
		}
	}
}

func TestReportOnANonStructObject(t *testing.T) {
	var value string
	err := data.Object(&value).Reports(data.String(&value).Missing())
	type holder struct {
		Value string
	}
	var h holder
	nested := data.Object(&h.Value).Reports(data.String(&h.Value).Missing())
	if outer := data.Object(&h).Reports(nested); err == nil || outer == nil {
		t.Fatalf("got %v, %v", err, outer)
	}
}
