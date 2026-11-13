package rest

import (
	"context"
	"encoding/xml"
	"reflect"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api"
)

type (
	xmlNamed       struct{ Name string }
	xmlWithMap     struct{ Labels map[string]string }
	xmlWithNested  struct{ Inner xmlWithMap }
	xmlWithPointer struct{ Inner *xmlWithMap }
	xmlWithList    struct{ Inner []xmlWithMap }
	xmlWithFunc    struct{ Do func() }
	xmlWithChan    struct{ Events chan int }
	xmlWithComplex struct{ Z complex128 }
	xmlWithSkipped struct {
		Labels map[string]string `xml:"-"`
	}
	xmlWithHidden struct {
		Labels map[string]string `json:"-"`
	}
	xmlWithUnexported struct {
		Name   string
		labels map[string]string
	}
	xmlWithAny       struct{ Value any }
	xmlWithAnonymous struct{ Items []struct{ N int } }
	xmlWithBytes     struct{ Raw []byte }
	xmlWithTime      struct{ At time.Time }
	xmlWithXMLName   struct {
		XMLName xml.Name `xml:"thing"`
		N       int
	}
	xmlEmbedding struct {
		xmlNamed
		N int
	}
	xmlEmbeddingMap struct {
		xmlWithMap
		N int
	}
	xmlRecursive struct {
		Name string
		Next *xmlRecursive
		Kids []xmlRecursive
	}
	xmlRecursiveMap struct {
		Next   *xmlRecursiveMap
		Labels map[string]string
	}
	xmlBytes []byte
	xmlNames []string
	xmlMaps  []map[string]int

	// Types that hold themselves without a struct in between.
	xmlLoop            []xmlLoop
	xmlPointerLoop     *xmlPointerLoop
	xmlLoopOfMaps      []map[string]xmlLoopOfMaps
	xmlWithLoop        struct{ Loop xmlLoop }
	xmlWithPointerLoop struct{ Pointer xmlPointerLoop }
)

// TestXMLWritesAgreesWithEncodingXML checks what is said of a type against what
// encoding/xml does with a value of it, as that is the claim: a type it says
// can be written is one a client is offered application/xml for.
func TestXMLWritesAgreesWithEncodingXML(t *testing.T) {
	labels := map[string]string{"a": "b"}
	for _, value := range []any{
		// What encoding/xml writes.
		"text", 1, uint8(1), 1.5, true, time.Minute, time.Unix(0, 0).UTC(),
		xmlNamed{Name: "x"}, &xmlNamed{Name: "x"}, []xmlNamed{{Name: "x"}}, []*xmlNamed{{Name: "x"}}, [1]xmlNamed{},
		[]string{"a", "b"}, [2]int{1, 2}, xmlNames{"a"}, []time.Time{time.Unix(0, 0).UTC()},
		xmlBytes("raw"), xmlWithSkipped{Labels: labels}, xmlWithUnexported{Name: "x", labels: labels},
		xmlWithAny{Value: 1}, xmlWithAnonymous{Items: []struct{ N int }{{1}}}, xmlWithBytes{Raw: []byte("raw")},
		xmlWithTime{}, xmlWithXMLName{N: 1}, xmlEmbedding{N: 1}, xmlRecursive{Next: &xmlRecursive{}, Kids: []xmlRecursive{{}}},
		struct {
			XMLName xml.Name
			N       int
		}{XMLName: xml.Name{Local: "named"}, N: 1},
		xmlLoop{{}, {{}}}, xmlWithLoop{Loop: xmlLoop{{}}},
		// What it does not.
		xmlLoopOfMaps{{"a": nil}},
		labels, &labels, []map[string]int{{"a": 1}}, xmlMaps{{"a": 1}}, []byte("raw"), [2]byte{1, 2},
		xmlWithMap{Labels: labels}, xmlWithMap{}, &xmlWithMap{}, []xmlWithMap{{}}, xmlWithNested{}, xmlWithList{Inner: []xmlWithMap{{}}},
		xmlWithPointer{Inner: &xmlWithMap{}}, xmlWithFunc{}, xmlWithChan{}, xmlWithComplex{}, xmlWithHidden{Labels: labels},
		xmlEmbeddingMap{}, xmlRecursiveMap{Next: &xmlRecursiveMap{}},
		struct{ N int }{1}, []struct{ N int }{{1}}, complex(1, 2), make(chan int), func() {},
	} {
		_, err := marshalXML(value)
		if got := xmlWrites(reflect.TypeOf(value), false, make(map[reflect.Type]bool)); got != (err == nil) {
			t.Errorf("xmlWrites(%T) = %v, and encoding/xml says %v", value, got, err)
		}
	}
	// Inside an element that is named for it, as results with rules are, a
	// value needs no name of its own.
	for _, value := range []any{struct{ N int }{1}, []byte("raw"), struct{ Labels map[string]string }{labels}, labels} {
		_, err := marshalXML(xmlElement{name: "named", value: value})
		if got := xmlWrites(reflect.TypeOf(value), true, make(map[reflect.Type]bool)); got != (err == nil) {
			t.Errorf("xmlWrites(%T) inside a named element = %v, and encoding/xml says %v", value, got, err)
		}
	}
}

// TestXMLWritesWhatMayBeLeftOut checks the two cases that only a value decides,
// which are taken for what can be written: a client is refused what can never
// be sent, not what sometimes cannot.
func TestXMLWritesWhatMayBeLeftOut(t *testing.T) {
	type optional struct {
		Labels map[string]string `xml:",omitempty"`
	}
	type held struct{ Value any }
	for _, rtype := range []reflect.Type{reflect.TypeFor[optional](), reflect.TypeFor[held](), reflect.TypeFor[[]any]()} {
		if !xmlWrites(rtype, false, make(map[reflect.Type]bool)) {
			t.Errorf("xmlWrites(%v) = false", rtype)
		}
	}
	if _, err := marshalXML(optional{}); err != nil {
		t.Errorf("an empty map that is left out: %v", err)
	}
	if _, err := marshalXML(held{}); err != nil {
		t.Errorf("an interface that holds nothing: %v", err)
	}
}

// TestXMLIsNotOfferedForAPointerToItself checks the one kind of type that
// encoding/xml is not asked about here, as it does not return from looking at a
// field of one. A type that is a pointer to itself is refused, and a request
// for it as XML is answered at all.
func TestXMLIsNotOfferedForAPointerToItself(t *testing.T) {
	for _, rtype := range []reflect.Type{
		reflect.TypeFor[xmlPointerLoop](),
		reflect.TypeFor[*xmlPointerLoop](),
		reflect.TypeFor[xmlWithPointerLoop](),
		reflect.TypeFor[[]xmlWithPointerLoop](),
	} {
		if xmlWrites(rtype, false, make(map[reflect.Type]bool)) {
			t.Errorf("xmlWrites(%v) = true", rtype)
		}
	}
	// A struct that points at itself is not such a type, however often.
	type node struct {
		Left, Right *node
		Up          **node
	}
	if !xmlWrites(reflect.TypeFor[node](), false, make(map[reflect.Type]bool)) {
		t.Error("xmlWrites of a struct with pointers to itself = false")
	}
}

type sampledPaths struct {
	api.Specification

	Get    func(context.Context, string) (string, error)  `rest:"GET /things/{id=%v}"`
	GetPtr func(context.Context, *string) (string, error) `rest:"GET /ptr/{id=%v}"`
	Search func(context.Context, sampleBody) error        `rest:"GET /search"`
}

type sampleBody struct {
	Name string `json:"name"`
}

// TestSampleShowsACallThatCouldNotBeSent checks that the picture of a call an
// example made is still drawn when its path value is empty. The client refuses
// to send such a call, but the example called the function some other way, and
// what it called it with is what the page is to show.
func TestSampleShowsACallThatCouldNotBeSent(t *testing.T) {
	functions := make(map[string]api.Function)
	for _, fn := range api.StructureOf(&sampledPaths{}).Functions {
		functions[fn.Name] = fn
	}
	var nothing *string
	for _, c := range []struct {
		name string
		arg  any
		url  string
		req  string
	}{
		{"Get", "", "GET /things/", ""},
		{"Get", "5", "GET /things/5", ""},
		{"GetPtr", nothing, "GET /ptr/{id}", ""},
		// A GET with an argument for the body is sent with one, and shown so.
		{"Search", sampleBody{Name: "x"}, "GET /search", "{\n\t\"name\": \"x\"\n}"},
	} {
		url, req, _, err := sample(functions[c.name], []reflect.Value{reflect.ValueOf(c.arg)}, nil)
		if err != nil || url != c.url || string(req) != c.req {
			t.Errorf("sample of %s(%v) = %q with the body %q, %v; want %q with %q", c.name, c.arg, url, req, err, c.url, c.req)
		}
	}
}
