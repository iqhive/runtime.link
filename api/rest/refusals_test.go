package rest_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
)

type decimalNumbers struct {
	api.Specification

	Price func(context.Context, float64) (string, error)    `rest:"GET /price/{amount=%v}"`
	Small func(context.Context, float32) (string, error)    `rest:"GET /small?n=%v"`
	Many  func(context.Context, []float64) (string, error)  `rest:"GET /many?n=%v"`
	Wave  func(context.Context, complex128) (string, error) `rest:"GET /wave/{z=%v}"`
	Text  func(context.Context, float64) (string, error)    `rest:"POST(text/plain) /text"`
	JSON  func(context.Context, float64) (string, error)    `rest:"POST /json"`
}

// TestNumbersAreFinite checks that a float in a path, a query or a text body is
// a finite number. NaN passes a check such as amount < 0 || amount > 100, and no
// JSON body could have sent it, nor an infinity. Every other way strconv has of
// writing a number is read as it always was.
func TestNumbersAreFinite(t *testing.T) {
	var calls atomic.Int32
	h, err := rest.Handler(nil, decimalNumbers{
		Price: func(_ context.Context, amount float64) (string, error) {
			calls.Add(1)
			if amount < 0 || amount > 100 {
				return "", errors.New("out of range")
			}
			return fmt.Sprint(amount), nil
		},
		Small: func(_ context.Context, n float32) (string, error) { calls.Add(1); return fmt.Sprint(n), nil },
		Many:  func(_ context.Context, n []float64) (string, error) { calls.Add(1); return fmt.Sprint(n), nil },
		Wave:  func(_ context.Context, z complex128) (string, error) { calls.Add(1); return fmt.Sprint(z), nil },
		Text:  func(_ context.Context, n float64) (string, error) { calls.Add(1); return fmt.Sprint(n), nil },
		JSON:  func(_ context.Context, n float64) (string, error) { calls.Add(1); return fmt.Sprint(n), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	refused := []string{
		"NaN", "nan", "Inf", "inf", "-Inf", "%2BInf", "infinity", "-Infinity",
		"0x10", "1e999", "-1e999", "1e", "e1", ".", "--1", "1.5x",
	}
	for _, text := range refused {
		for _, target := range []string{"/price/" + text, "/small?n=" + text, "/many?n=1&n=" + text} {
			before := calls.Load()
			if rec := serve(t, h, "GET", target, ""); rec.Code != 400 || calls.Load() != before {
				t.Errorf("GET %s: %d %q, want 400 before the function is called", target, rec.Code, rec.Body)
			}
		}
	}
	for _, c := range []struct{ target, want string }{
		{"/price/0", "0"},
		{"/price/1", "1"},
		{"/price/1.5", "1.5"},
		{"/price/%2B2", "2"},
		{"/price/1e1", "10"},
		{"/price/1E-3", "0.001"},
		{"/price/.5", "0.5"},
		{"/price/5.", "5"},
		{"/price/007", "7"},
		{"/price/0x1p4", "16"},
		{"/price/1_0", "10"},
		{"/small?n=0X1P4", "16"},
		{"/small?n=-1.25", "-1.25"},
		{"/small?n=1e2", "100"},
		{"/many?n=1&n=2.5", "[1 2.5]"},
		{"/wave/1%2B2i", "(1+2i)"},
		{"/wave/(1-2i)", "(1-2i)"},
		{"/wave/3", "(3+0i)"},
	} {
		if rec := serve(t, h, "GET", c.target, ""); rec.Code != 200 || rec.Body.String() != c.want {
			t.Errorf("GET %s: %d %q, want %q", c.target, rec.Code, rec.Body, c.want)
		}
	}
	for _, text := range []string{"NaN", "Inf", "1%2BInfi", "NaN%2B1i"} {
		if rec := serve(t, h, "GET", "/wave/"+text, ""); rec.Code != 400 {
			t.Errorf("GET /wave/%s: %d %q, want 400", text, rec.Code, rec.Body)
		}
	}
	for _, c := range []struct {
		body string
		code int
		want string
	}{
		{"1.5", 200, "1.5"},
		{" 2.5\n", 200, "2.5"},
		{"NaN", 400, ""},
		{"+Inf", 400, ""},
		{"0x1p4", 200, "16"},
		{"1_0", 200, "10"},
	} {
		rec := serve(t, h, "POST", "/text", c.body, "Content-Type", "text/plain")
		if rec.Code != c.code || c.code == 200 && rec.Body.String() != c.want {
			t.Errorf("POST /text %q: %d %q, want %d %q", c.body, rec.Code, rec.Body, c.code, c.want)
		}
	}
	// A JSON body has had these rules all along.
	if rec := serve(t, h, "POST", "/json", "1e1"); rec.Code != 200 || rec.Body.String() != "10" {
		t.Errorf("POST /json 1e1: %d %q", rec.Code, rec.Body)
	}
	if rec := serve(t, h, "POST", "/json", "NaN"); rec.Code != 400 {
		t.Errorf("POST /json NaN: %d %q", rec.Code, rec.Body)
	}
}

type failingStatus int

func (s failingStatus) Error() string   { return "it failed" }
func (s failingStatus) StatusHTTP() int { return int(s) }

type typedFailure struct {
	Status int `json:"status"`
}

func (f typedFailure) Error() string           { return "it failed with a body" }
func (f typedFailure) StatusHTTP() int         { return f.Status }
func (f typedFailure) ContentTypeHTTP() string { return "application/json" }

type failing struct {
	api.Specification

	Fail  func(context.Context, int) error `rest:"POST /fail/{status=%v}"`
	Typed func(context.Context, int) error `rest:"POST /typed/{status=%v}"`
}

// TestAnErrorIsSentWithAStatusThatEndsAResponse checks the status of an error
// as the status of a result is checked. An informational one was followed by
// net/http's own 200, which told the client the call had worked, and one that
// is no status at all was a panic that dropped the connection.
func TestAnErrorIsSentWithAStatusThatEndsAResponse(t *testing.T) {
	impl := failing{
		Fail:  func(_ context.Context, status int) error { return failingStatus(status) },
		Typed: func(_ context.Context, status int) error { return typedFailure{status} },
	}
	h, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	for status, want := range map[int]int{
		400: 400, 404: 404, 418: 418, 500: 500, 503: 503, 599: 599, 999: 999,
		// A status that ends a response is the error's to choose.
		200: 200, 302: 302,
		0: 500, 1: 500, 42: 500, 99: 500, 100: 500, 101: 500, 150: 500, 199: 500, 1000: 500, -1: 500, -404: 500,
	} {
		rec := serve(t, h, "POST", fmt.Sprint("/fail/", status), "")
		if rec.Code != want || !strings.Contains(rec.Body.String(), "it failed") {
			t.Errorf("an error with the status %d: %d %q, want %d", status, rec.Code, rec.Body, want)
		}
		rec = serve(t, h, "POST", fmt.Sprint("/typed/", status), "")
		if rec.Code != want || !strings.Contains(rec.Body.String(), `"status"`) || rec.Header().Get("Content-Type") != "application/json" {
			t.Errorf("an error with a body and the status %d: %d %q, want %d", status, rec.Code, rec.Body, want)
		}
	}

	srv := httptest.NewServer(h)
	defer srv.Close()
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	client := api.Import[failing](rest.API, srv.URL, srv.Client())
	for _, status := range []int{418, 100, 150, 199, 1000, -1, 42, 0} {
		err := client.Fail(context.Background(), status)
		if err == nil || !strings.Contains(err.Error(), "it failed") {
			t.Errorf("an error with the status %d reached the client as %v", status, err)
		}
	}
}

type labelled struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
}

type xmlResults struct {
	api.Specification

	Counts   func(context.Context) (map[string]int, error)      `rest:"POST /counts"`
	Labelled func(context.Context) (labelled, error)            `rest:"POST /labelled"`
	List     func(context.Context) ([]labelled, error)          `rest:"POST /list"`
	Blob     func(context.Context) ([]byte, error)              `rest:"POST /blob"`
	Thing    func(context.Context) (encodedThing, error)        `rest:"POST /thing"`
	Things   func(context.Context) ([]*encodedThing, error)     `rest:"POST /things"`
	Word     func(context.Context) (string, error)              `rest:"POST /word"`
	Any      func(context.Context, string) (any, error)         `rest:"POST /any/{kind=%v}"`
	Pair     func(context.Context) (map[string]int, int, error) `rest:"POST /pair counts,total"`
	Sums     func(context.Context) (int, []byte, error)         `rest:"POST /sums total,raw"`
	Tree     func(context.Context) (treeNode, error)            `rest:"POST /tree"`
	Nest     func(context.Context) (nestedMaps, error)          `rest:"POST /nest"`
}

// treeNode and nestedMaps hold themselves, which has no end for whoever walks a
// type to find out what it is made of.
type treeNode struct {
	Name string
	Kids []treeNode
	Up   *treeNode
}

type nestedMaps []map[string]nestedMaps

// TestXMLIsOnlyOfferedForWhatItCanWrite checks that application/xml is not on
// offer for a result that encoding/xml has no way to write, a map above all. It
// was chosen for every result, and the encoder then failed with a 500: now a
// client that takes something else as well is sent that, and one that takes
// only XML is answered as any client that takes nothing on offer is.
func TestXMLIsOnlyOfferedForWhatItCanWrite(t *testing.T) {
	var calls atomic.Int32
	called := func() { calls.Add(1) }
	h, err := rest.Handler(nil, xmlResults{
		Counts:   func(context.Context) (map[string]int, error) { called(); return map[string]int{"a": 1}, nil },
		Labelled: func(context.Context) (labelled, error) { called(); return labelled{Name: "x"}, nil },
		List:     func(context.Context) ([]labelled, error) { called(); return nil, nil },
		Blob:     func(context.Context) ([]byte, error) { called(); return []byte("bytes"), nil },
		Thing:    func(context.Context) (encodedThing, error) { called(); return encodedThing{Name: "x"}, nil },
		Things: func(context.Context) ([]*encodedThing, error) {
			called()
			return []*encodedThing{{Name: "x"}, {Name: "y"}}, nil
		},
		Word: func(context.Context) (string, error) { called(); return "word", nil },
		Any: func(_ context.Context, kind string) (any, error) {
			called()
			switch kind {
			case "map":
				return map[string]int{"a": 1}, nil
			case "nil":
				return nil, nil
			}
			return encodedThing{Name: kind}, nil
		},
		Pair: func(context.Context) (map[string]int, int, error) { called(); return map[string]int{"a": 1}, 1, nil },
		Sums: func(context.Context) (int, []byte, error) { called(); return 3, []byte("raw"), nil },
		Tree: func(context.Context) (treeNode, error) {
			called()
			return treeNode{Name: "root", Kids: []treeNode{{Name: "kid"}}}, nil
		},
		Nest: func(context.Context) (nestedMaps, error) { called(); return nestedMaps{{"a": nil}}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		target, accept string
		code, calls    int
		body           string
	}{
		{"/counts", "application/xml", 415, 1, ""},
		{"/counts", "application/xml;q=0.9, text/xml", 415, 1, ""},
		{"/labelled", "application/xml", 415, 1, ""},
		{"/list", "application/xml", 415, 1, ""},
		{"/blob", "application/xml", 415, 1, ""},
		{"/pair", "application/xml", 415, 1, ""},
		// A client that takes something else as well is sent that.
		{"/counts", "application/xml, application/json;q=0.5", 200, 1, "{\n\t\"a\": 1\n}"},
		{"/counts", "application/xml, */*;q=0.1", 200, 1, "{\n\t\"a\": 1\n}"},
		{"/counts", "*/*", 200, 1, "{\n\t\"a\": 1\n}"},
		{"/counts", "application/*", 200, 1, "{\n\t\"a\": 1\n}"},
		// What XML can write is still on offer.
		{"/thing", "application/xml", 200, 1, "<encodedThing><Name>x</Name></encodedThing>"},
		{"/things", "application/xml", 200, 1, "<encodedThing><Name>x</Name></encodedThing><encodedThing><Name>y</Name></encodedThing>"},
		{"/word", "application/xml", 200, 1, "<string>word</string>"},
		{"/sums", "application/xml", 200, 1, "<Sums><total>3</total><raw>raw</raw></Sums>"},
		{"/tree", "application/xml", 200, 1, "<treeNode><Name>root</Name><Kids><Name>kid</Name></Kids></treeNode>"},
		{"/nest", "application/xml", 415, 1, ""},
		// What an interface holds is only known once the function returns.
		{"/any/thing", "application/xml", 200, 1, "<encodedThing><Name>thing</Name></encodedThing>"},
		{"/any/nil", "application/xml", 200, 1, ""},
		{"/any/map", "application/xml", 415, 1, ""},
		{"/any/map", "application/xml, application/json;q=0.5", 200, 1, "{\n\t\"a\": 1\n}"},
	} {
		before := calls.Load()
		rec := serve(t, h, "POST", c.target, "", "Accept", c.accept)
		if rec.Code != c.code || int(calls.Load()-before) != c.calls || c.code == 200 && rec.Body.String() != c.body {
			t.Errorf("POST %s taking %q: %d %q after %d calls, want %d %q after %d",
				c.target, c.accept, rec.Code, rec.Body, calls.Load()-before, c.code, c.body, c.calls)
		}
	}
}
