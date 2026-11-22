package rest_test

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/big"
	"mime"
	"mime/multipart"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
	"github.com/iqhive/runtime.link/xyz"
)

// same is a function that returns what it was given, so that a test sees what
// arrived.
func same[T any](_ context.Context, value T) (T, error) { return value, nil }

// tenant is a name that cannot be written when it has a slash in it, or when it
// is not set.
type tenant string

func (t tenant) MarshalText() ([]byte, error) {
	switch {
	case t == "":
		return nil, errors.New("a tenant has a name")
	case strings.Contains(string(t), "/"):
		return nil, errors.New("a tenant has no slash in its name")
	}
	return []byte(t), nil
}

func (t *tenant) UnmarshalText(text []byte) error {
	*t = tenant(text)
	return nil
}

type shape xyz.Tagged[any, struct {
	Circle xyz.Case[shape, struct{ Radius int }] `json:"circle"`
	Name   xyz.Case[shape, string]               `json:"name"`
}]

var shapes = xyz.AccessorFor(shape.Values)

type unwritable struct {
	api.Specification

	List  func(context.Context, tenant) (string, error)            `rest:"GET /things?owner=%v"`
	Get   func(context.Context, tenant) (string, error)            `rest:"GET /things/{owner=%v}"`
	Many  func(context.Context, []tenant) (string, error)          `rest:"GET /many?owner=%v"`
	Named func(context.Context, map[tenant]string) (string, error) `rest:"GET /named?%v"`
	Near  func(context.Context, shape) (string, error)             `rest:"GET /near?shape=%v"`
}

// TestClientFailsACallWhoseValueCannotBeWritten checks that a value its own
// MarshalText refuses is an error to the caller, and that nothing is sent: sent
// empty, as it was, the server takes the parameter for left out and the function
// runs without it.
func TestClientFailsACallWhoseValueCannotBeWritten(t *testing.T) {
	var (
		mu    sync.Mutex
		calls []string
	)
	called := func(with string) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, with)
		return with, nil
	}
	client, log := linked(t, unwritable{
		List: func(_ context.Context, owner tenant) (string, error) { return called("List " + string(owner)) },
		Get:  func(_ context.Context, owner tenant) (string, error) { return called("Get " + string(owner)) },
		Many: func(_ context.Context, owners []tenant) (string, error) {
			return called(fmt.Sprintf("Many %q", owners))
		},
		Named: func(_ context.Context, names map[tenant]string) (string, error) {
			return called(fmt.Sprint("Named ", names))
		},
		Near: func(_ context.Context, near shape) (string, error) { return called("Near") },
	})
	ctx := context.Background()
	refused := map[string]func() error{
		"a query value":        func() error { _, err := client.List(ctx, "a/b"); return err },
		"a path value":         func() error { _, err := client.Get(ctx, "a/b"); return err },
		"an element of a list": func() error { _, err := client.Many(ctx, []tenant{"a", "b/c"}); return err },
		"the name of a query":  func() error { _, err := client.Named(ctx, map[tenant]string{"a/b": "x"}); return err },
		"a value with no text": func() error { _, err := client.Near(ctx, shapes.Name.As("bob")); return err },
		"an unset path value":  func() error { _, err := client.Get(ctx, ""); return err },
	}
	for name, call := range refused {
		if err := call(); err == nil {
			t.Errorf("%s that cannot be written: no error", name)
		}
	}
	if sent := log.all(); len(sent) != 0 || len(calls) != 0 {
		t.Fatalf("calls that could not be written were sent: %v, and ran %q", sent, calls)
	}

	// What can be written is, and a value that is not set is left out, as it
	// always was: left out is what it is.
	if got, err := client.List(ctx, "a"); err != nil || got != "List a" {
		t.Errorf("List(a) = %q, %v", got, err)
	}
	if got, err := client.List(ctx, ""); err != nil || got != "List " || log.last(t).query != "" {
		t.Errorf("List of no tenant = %q, %v, sent %q", got, err, log.last(t).query)
	}
	if got, err := client.Many(ctx, []tenant{"", "a"}); err != nil || got != `Many ["" "a"]` || log.last(t).query != "owner=&owner=a" {
		t.Errorf("Many of an unset tenant and a = %q, %v, sent %q", got, err, log.last(t).query)
	}
}

type sparse struct {
	api.Specification

	Nums  func(context.Context, []*int) (string, error)    `rest:"GET /nums?n=%v"`
	Names func(context.Context, []*string) (string, error) `rest:"GET /names?n=%v"`
	Plain func(context.Context, []int) (string, error)     `rest:"GET /plain?n=%v"`
	Path  func(context.Context, []*int) (string, error)    `rest:"GET /nums/{n=%v}"`
}

func showPointers[T any](_ context.Context, values []*T) (string, error) {
	said := make([]string, len(values))
	for i, value := range values {
		said[i] = "nil"
		if value != nil {
			said[i] = fmt.Sprint(*value)
		}
	}
	return strings.Join(said, ","), nil
}

// TestClientKeepsTheNilsOfAQueryList checks that a nil in a list of pointers is
// sent as the empty element the server reads as nil. Left out, the elements
// after it arrived at another index.
func TestClientKeepsTheNilsOfAQueryList(t *testing.T) {
	client, log := linked(t, sparse{
		Nums:  showPointers[int],
		Names: showPointers[string],
		Plain: func(_ context.Context, values []int) (string, error) { return fmt.Sprint(values), nil },
		Path:  showPointers[int],
	})
	ctx := context.Background()
	three, name := 3, "a"

	got, err := client.Nums(ctx, []*int{nil, &three, nil})
	if err != nil || got != "nil,3,nil" || log.last(t).query != "n=&n=3&n=" {
		t.Errorf("Nums([nil 3 nil]) sent %q: %q, %v", log.last(t).query, got, err)
	}
	got, err = client.Nums(ctx, []*int{nil})
	if err != nil || got != "nil" {
		t.Errorf("Nums([nil]) sent %q: %q, %v", log.last(t).query, got, err)
	}
	got, err = client.Names(ctx, []*string{&name, nil})
	if err != nil || got != "a,nil" {
		t.Errorf("Names([a nil]) sent %q: %q, %v", log.last(t).query, got, err)
	}
	// A path kept them already, and what has no nils is sent as it was.
	got, err = client.Path(ctx, []*int{nil, &three})
	if err != nil || got != "nil,3" || log.last(t).path != "/nums/,3" {
		t.Errorf("Path([nil 3]) sent %q: %q, %v", log.last(t).path, got, err)
	}
	got, err = client.Nums(ctx, []*int{&three, &three})
	if err != nil || got != "3,3" || log.last(t).query != "n=3&n=3" {
		t.Errorf("Nums([3 3]) sent %q: %q, %v", log.last(t).query, got, err)
	}
	got, err = client.Plain(ctx, []int{0, 1})
	if err != nil || got != "[0 1]" || log.last(t).query != "n=0&n=1" {
		t.Errorf("Plain([0 1]) sent %q: %q, %v", log.last(t).query, got, err)
	}
}

// fromPointer writes itself with methods of the pointer receiver only, as
// *big.Int does, and has a field that would be written in their place.
type fromPointer struct {
	Field string
}

func (p *fromPointer) MarshalText() ([]byte, error) { return []byte("text:" + p.Field), nil }

func (p *fromPointer) UnmarshalText(text []byte) error {
	field, ok := strings.CutPrefix(string(text), "text:")
	if !ok {
		return fmt.Errorf("%q is not what MarshalText wrote", text)
	}
	p.Field = field
	return nil
}

// writeOnly writes itself from the pointer receiver and has nothing to read
// that form with, so it is read by its fields.
type writeOnly struct {
	Field string
}

func (w *writeOnly) MarshalJSON() ([]byte, error) { return []byte(`"written:` + w.Field + `"`), nil }

type pointers struct {
	api.Specification

	Written func(context.Context, *writeOnly) (string, error) `rest:"POST /written"`

	Body  func(context.Context, *big.Int) (string, error)           `rest:"POST /n"`
	Path  func(context.Context, *big.Int) (string, error)           `rest:"GET /n/{n=%v}"`
	Query func(context.Context, *big.Int) (string, error)           `rest:"GET /n?n=%v"`
	Text  func(context.Context, *big.Int) (string, error)           `rest:"POST(text/plain) /n/text"`
	Both  func(context.Context, *big.Int, *big.Int) (string, error) `rest:"POST /n/both"`

	OwnBody func(context.Context, *fromPointer) (string, error) `rest:"POST /own"`
	OwnPath func(context.Context, *fromPointer) (string, error) `rest:"GET /own/{p=%v}"`
	OwnXML  func(context.Context, *fromPointer) (string, error) `rest:"POST(application/xml) /own/xml"`

	Plain func(context.Context, *pathKey) (string, error) `rest:"POST /plain"`
}

// TestClientSendsAPointerThatWritesItself checks that an argument whose type
// writes itself from the pointer receiver is sent in that form. The client used
// to follow the pointer first, and sent the fields of what it points to: {} for
// a *big.Int, which the server refuses.
func TestClientSendsAPointerThatWritesItself(t *testing.T) {
	number := func(_ context.Context, n *big.Int) (string, error) { return n.String(), nil }
	field := func(_ context.Context, p *fromPointer) (string, error) { return p.Field, nil }
	client, log := linked(t, pointers{
		Body: number, Path: number, Query: number, Text: number,
		Both:    func(_ context.Context, a, b *big.Int) (string, error) { return a.String() + "+" + b.String(), nil },
		OwnBody: field, OwnPath: field, OwnXML: field,
		Plain:   func(_ context.Context, key *pathKey) (string, error) { return key.ID, nil },
		Written: func(_ context.Context, w *writeOnly) (string, error) { return w.Field, nil },
	})
	ctx := context.Background()
	for name, call := range map[string]func(context.Context, *big.Int) (string, error){
		"body": client.Body, "path": client.Path, "query": client.Query, "text/plain body": client.Text,
	} {
		if got, err := call(ctx, big.NewInt(12345)); err != nil || got != "12345" {
			last := log.last(t)
			t.Errorf("a *big.Int in a %s (sent %s?%s %q): %q, %v", name, last.path, last.query, last.body, got, err)
		}
	}
	if got, err := client.Both(ctx, big.NewInt(1), big.NewInt(2)); err != nil || got != "1+2" {
		t.Errorf("two of them in a body (sent %q): %q, %v", log.last(t).body, got, err)
	}
	for name, call := range map[string]func(context.Context, *fromPointer) (string, error){
		"body": client.OwnBody, "path": client.OwnPath, "XML body": client.OwnXML,
	} {
		if got, err := call(ctx, &fromPointer{Field: "f"}); err != nil || got != "f" {
			last := log.last(t)
			t.Errorf("a pointer with its own text in a %s (sent %s %q): %q, %v", name, last.path, last.body, got, err)
		}
	}
	// A pointer to a struct with no form of its own is sent as its fields, as
	// it was, and so is one with a form that it cannot read back.
	if got, err := client.Written(ctx, &writeOnly{Field: "f"}); err != nil || got != "f" {
		t.Errorf("a pointer that only writes itself (sent %q): %q, %v", log.last(t).body, got, err)
	}
	if got, err := client.Plain(ctx, &pathKey{ID: "k"}); err != nil || got != "k" || !strings.Contains(log.last(t).body, `"id": "k"`) {
		t.Errorf("a pointer to a plain struct (sent %q): %q, %v", log.last(t).body, got, err)
	}
}

// label is a string with a String method, as many named types have for logs.
type label string

func (l label) String() string { return "label(" + string(l) + ")" }

// level is a number with a String method, as stringer makes for an enum.
type level int

func (l level) String() string { return [...]string{"low", "high"}[l] }

// colour prints its name and reads it back, so its text is the name.
type colour string

func (c colour) String() string { return "the colour " + string(c) }

func (c *colour) Scan(state fmt.ScanState, _ rune) error {
	var words []string
	for range 3 {
		word, err := state.Token(true, nil)
		if err != nil {
			return err
		}
		words = append(words, string(word))
	}
	if words[0] != "the" || words[1] != "colour" {
		return fmt.Errorf("%q is not what String prints", words)
	}
	*c = colour(words[2])
	return nil
}

type printed struct {
	api.Specification

	Body  func(context.Context, label) (label, error)            `rest:"POST /label"`
	Path  func(context.Context, label) (label, error)            `rest:"GET /labels/{label=%v}"`
	Query func(context.Context, label) (label, error)            `rest:"GET /labels?label=%v"`
	Text  func(context.Context, label) (label, error)            `rest:"POST(text/plain) /label/text"`
	Lines func(context.Context, []label) ([]label, error)        `rest:"POST(text/plain) /label/lines"`
	Named func(context.Context, map[label]label) (string, error) `rest:"GET /named?%v"`

	LevelBody  func(context.Context, level) (level, error)     `rest:"POST /level"`
	LevelPath  func(context.Context, level) (level, error)     `rest:"GET /levels/{level=%v}"`
	LevelQuery func(context.Context, []level) ([]level, error) `rest:"GET /levels?level=%v"`
	LevelText  func(context.Context, level) (level, error)     `rest:"POST(text/plain) /level/text"`

	Colour     func(context.Context, colour) (colour, error) `rest:"GET /colours?colour=%v"`
	ColourText func(context.Context, colour) (colour, error) `rest:"POST(text/plain) /colours"`

	Wait     func(context.Context, time.Duration) (time.Duration, error) `rest:"GET /wait?for=%v"`
	WaitText func(context.Context, time.Duration) (time.Duration, error) `rest:"POST(text/plain) /wait"`
}

// TestClientSendsAValueWithAStringMethodByKind checks that a string or a number
// whose type has a String method arrives as the value it is, whichever way it
// travels. The client wrote what String prints in a path, a query and a
// text/plain body, which the server reads by kind: a string arrived as another
// string, with no error, and a number was refused.
func TestClientSendsAValueWithAStringMethodByKind(t *testing.T) {
	client, log := linked(t, printed{
		Body: same[label], Path: same[label], Query: same[label], Text: same[label], Lines: same[[]label],
		Named: func(_ context.Context, names map[label]label) (string, error) {
			return fmt.Sprintf("%q", map[string]string{"x": string(names["x"])}), nil
		},
		LevelBody: same[level], LevelPath: same[level], LevelQuery: same[[]level], LevelText: same[level],
		Colour: same[colour], ColourText: same[colour],
		Wait: same[time.Duration], WaitText: same[time.Duration],
	})
	ctx := context.Background()
	sent := func() string {
		last := log.last(t)
		return fmt.Sprintf("%s?%s %q", last.path, last.query, last.body)
	}
	for name, call := range map[string]func(context.Context, label) (label, error){
		"JSON body": client.Body, "path": client.Path, "query": client.Query, "text/plain body": client.Text,
	} {
		if got, err := call(ctx, "x y"); err != nil || got != "x y" {
			t.Errorf("a string in a %s (sent %s): the function got %q, %v", name, sent(), string(got), err)
		}
	}
	if got, err := client.Lines(ctx, []label{"a", "b"}); err != nil || !reflect.DeepEqual(got, []label{"a", "b"}) {
		t.Errorf("a list of strings in a text/plain body (sent %s): %q, %v", sent(), got, err)
	}
	if got, err := client.Named(ctx, map[label]label{"x": "y"}); err != nil || got != `map["x":"y"]` {
		t.Errorf("a map of strings in a query (sent %s): %s, %v", sent(), got, err)
	}
	for name, call := range map[string]func(context.Context, level) (level, error){
		"JSON body": client.LevelBody, "path": client.LevelPath, "text/plain body": client.LevelText,
	} {
		if got, err := call(ctx, 1); err != nil || got != 1 {
			t.Errorf("a number in a %s (sent %s): the function got %d, %v", name, sent(), int(got), err)
		}
	}
	if got, err := client.LevelQuery(ctx, []level{0, 1}); err != nil || !reflect.DeepEqual(got, []level{0, 1}) || log.last(t).query != "level=0&level=1" {
		t.Errorf("a list of numbers in a query (sent %s): %v, %v", sent(), got, err)
	}

	// A type that reads the text of its String method is sent that text, as
	// it was: the two are a pair. So is a time.Duration, which the server
	// reads as it prints.
	if got, err := client.Colour(ctx, "red"); err != nil || got != "red" || log.last(t).query != "colour=the+colour+red" {
		t.Errorf("a string that reads what it prints, in a query (sent %s): %q, %v", sent(), string(got), err)
	}
	if got, err := client.ColourText(ctx, "red"); err != nil || got != "red" || log.last(t).body != "the colour red" {
		t.Errorf("a string that reads what it prints, in a body (sent %s): %q, %v", sent(), string(got), err)
	}
	if got, err := client.Wait(ctx, 1500*time.Millisecond); err != nil || got != 1500*time.Millisecond || log.last(t).query != "for=1.5s" {
		t.Errorf("a time.Duration in a query (sent %s): %v, %v", sent(), got, err)
	}
	if got, err := client.WaitText(ctx, 1500*time.Millisecond); err != nil || got != 1500*time.Millisecond || log.last(t).body != "1.5s" {
		t.Errorf("a time.Duration in a body (sent %s): %v, %v", sent(), got, err)
	}
}

type note struct {
	Title string `json:"title"`
	Stars int    `json:"stars"`
	Draft string `json:"-"`
}

type textBodies struct {
	api.Specification

	Note   func(context.Context, note) (note, error)                     `rest:"POST(text/plain) /note"`
	Maybe  func(context.Context, *note) (string, error)                  `rest:"POST(text/plain) /note/maybe"`
	Notes  func(context.Context, []note) ([]note, error)                 `rest:"POST(text/plain) /notes"`
	Labels func(context.Context, map[string]int) (map[string]int, error) `rest:"POST(text/plain) /labels"`
	Pair   func(context.Context, [2]int) ([2]int, error)                 `rest:"POST(text/plain) /pair"`
	Rows   func(context.Context, [][]string) ([][]string, error)         `rest:"POST(text/plain) /rows"`

	Held      func(context.Context, xyz.Maybe[string]) (string, error) `rest:"POST(text/plain) /held"`
	HeldQuery func(context.Context, xyz.Maybe[string]) (string, error) `rest:"GET /held?v=%v"`
}

func newTextBodies() textBodies {
	held := func(_ context.Context, value xyz.Maybe[string]) (string, error) {
		text, _ := value.Get()
		return text, nil
	}
	return textBodies{
		Note: same[note],
		Maybe: func(_ context.Context, n *note) (string, error) {
			if n == nil {
				return "nil", nil
			}
			return n.Title, nil
		},
		Notes: same[[]note], Labels: same[map[string]int], Pair: same[[2]int], Rows: same[[][]string],
		Held: held, HeldQuery: held,
	}
}

// TestTextBodyOfWhatHasNoTextForm checks that a struct, a map and an array are
// read from a text/plain body the way the client writes them, as JSON. The
// server refused them, so such a function could not be called.
func TestTextBodyOfWhatHasNoTextForm(t *testing.T) {
	impl := newTextBodies()
	client, log := linked(t, impl)
	ctx := context.Background()

	got, err := client.Note(ctx, note{Title: "a", Stars: 5, Draft: "hidden"})
	if sent := log.last(t).body; sent != `{"title":"a","stars":5}` {
		t.Fatalf("a struct was written as %q", sent)
	}
	if err != nil || got != (note{Title: "a", Stars: 5}) {
		t.Errorf("a struct: %+v, %v", got, err)
	}
	if got, err := client.Maybe(ctx, &note{Title: "a"}); err != nil || got != "a" {
		t.Errorf("a pointer to a struct (sent %q): %q, %v", log.last(t).body, got, err)
	}
	if got, err := client.Maybe(ctx, nil); err != nil || got != "nil" {
		t.Errorf("a nil pointer to a struct (sent %q): %q, %v", log.last(t).body, got, err)
	}
	notes := []note{{Title: "a\nb", Stars: 1}, {Title: "c"}}
	if got, err := client.Notes(ctx, notes); err != nil || !reflect.DeepEqual(got, notes) {
		t.Errorf("a list of structs (sent %q): %+v, %v", log.last(t).body, got, err)
	}
	if got, err := client.Labels(ctx, map[string]int{"a": 1}); err != nil || !reflect.DeepEqual(got, map[string]int{"a": 1}) {
		t.Errorf("a map (sent %q): %v, %v", log.last(t).body, got, err)
	}
	if got, err := client.Pair(ctx, [2]int{1, 2}); err != nil || got != [2]int{1, 2} {
		t.Errorf("an array (sent %q): %v, %v", log.last(t).body, got, err)
	}
	rows := [][]string{{"a", "b"}, {}, {"c"}}
	if got, err := client.Rows(ctx, rows); err != nil || !reflect.DeepEqual(got, rows) {
		t.Errorf("a list of lists (sent %q): %q, %v", log.last(t).body, got, err)
	}

	h, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"", "a note", `{"title":5}`, `{"title":"a"`} {
		if rec := serve(t, h, "POST", "/note", body, "Content-Type", "text/plain"); rec.Code != http.StatusBadRequest {
			t.Errorf("POST /note with the body %q: %d %q, want 400", body, rec.Code, rec.Body)
		}
	}
	// A field that is no part of the API is not set from a body of text either.
	rec := serve(t, h, "POST", "/note", `{"title":"a","Draft":"set"}`, "Content-Type", "text/plain", "Accept", "application/json")
	if rec.Code != 200 || strings.Contains(rec.Body.String(), "set") {
		t.Errorf("a hidden field in a text/plain body: %d %q", rec.Code, rec.Body)
	}
}

// TestTextBodyOfAValueWithOnlyAJSONForm checks that a value whose JSON is a
// string is sent in a text/plain body as it is in a query, without the quotes:
// the server puts quotes around what it reads, and the function got them as
// part of the value.
func TestTextBodyOfAValueWithOnlyAJSONForm(t *testing.T) {
	client, log := linked(t, newTextBodies())
	ctx := context.Background()
	for _, text := range []string{"hello", `"quoted"`, `a "b" c`, "two\nlines", "5", "true"} {
		if got, err := client.HeldQuery(ctx, xyz.New(text)); err != nil || got != text {
			t.Errorf("%q in a query (sent %q): %q, %v", text, log.last(t).query, got, err)
		}
		if got, err := client.Held(ctx, xyz.New(text)); err != nil || got != text {
			t.Errorf("%q in a text/plain body (sent %q): %q, %v", text, log.last(t).body, got, err)
		}
	}
}

type pairs struct {
	api.Specification

	Query func(context.Context, jsonPair) (string, error)   `rest:"GET /pair?p=%v"`
	Path  func(context.Context, jsonPair) (string, error)   `rest:"GET /pair/{p=%v}"`
	Body  func(context.Context, jsonPair) (string, error)   `rest:"POST(text/plain) /pair"`
	Many  func(context.Context, []jsonPair) (string, error) `rest:"GET /pairs?p=%v"`
}

// TestClientSendsASelfReadingSliceWhole checks that a slice which reads itself
// with UnmarshalJSON is sent as the one value the server reads it as, in a query
// and a path as it is in a text/plain body. It was sent element by element, and
// refused.
func TestClientSendsASelfReadingSliceWhole(t *testing.T) {
	show := func(_ context.Context, pair jsonPair) (string, error) { return fmt.Sprint([]int(pair)), nil }
	client, log := linked(t, pairs{
		Query: show, Path: show, Body: show,
		Many: func(_ context.Context, pairs []jsonPair) (string, error) { return fmt.Sprint(pairs), nil },
	})
	ctx := context.Background()
	for name, call := range map[string]func(context.Context, jsonPair) (string, error){
		"body": client.Body, "query": client.Query, "path": client.Path,
	} {
		if got, err := call(ctx, jsonPair{1, 2}); err != nil || got != "[1 2]" {
			last := log.last(t)
			t.Errorf("in a %s (sent %s?%s %q): %q, %v", name, last.path, last.query, last.body, got, err)
		}
	}
	if got, err := client.Many(ctx, []jsonPair{{1, 2}, {3, 4}}); err != nil || got != "[[1 2] [3 4]]" {
		t.Errorf("a list of them in a query (sent %q): %q, %v", log.last(t).query, got, err)
	}
}

type paging struct {
	Page int `json:"page"`
	size int
}

// Limits is embedded by its exported name, which was always a parameter.
type Limits struct {
	Limit int `json:"limit"`
}

type search struct {
	paging
	Limits
	Name string `json:"name"`
}

type hidden struct {
	Page int `json:"page"`
	Name string
}

// shadowing has a field of its own by the name of one it embeds, which hides
// the embedded one.
type shadowing struct {
	hidden
	Name string `json:"name"`
}

type searches struct {
	api.Specification

	Search func(context.Context, search) (string, error)    `rest:"GET /search?%v"`
	Post   func(context.Context, search) (string, error)    `rest:"POST /search"`
	Shadow func(context.Context, shadowing) (string, error) `rest:"GET /shadow?%v"`
}

// TestEmbeddedStructFieldsAreQueryParameters checks that the fields a struct
// gets from a struct it embeds are query parameters whatever the embedded type
// is called, as they are fields of its JSON. Those of a type that is not
// exported were neither sent nor read.
func TestEmbeddedStructFieldsAreQueryParameters(t *testing.T) {
	said := func(_ context.Context, s search) (string, error) {
		return fmt.Sprintf("page %d, limit %d of %s, size %d", s.Page, s.Limit, s.Name, s.size), nil
	}
	impl := searches{Search: said, Post: said, Shadow: func(_ context.Context, s shadowing) (string, error) {
		return fmt.Sprintf("page %d of %s, hidden %q", s.Page, s.Name, s.hidden.Name), nil
	}}
	client, log := linked(t, impl)
	ctx := context.Background()
	// A field that the struct hides with one of its own stays hidden: the name
	// is one parameter, as it was.
	shadowed := shadowing{hidden{Page: 2, Name: "inner"}, "outer"}
	if got, err := client.Shadow(ctx, shadowed); err != nil || got != `page 2 of outer, hidden ""` || log.last(t).query != "name=outer&page=2" {
		t.Errorf("a hidden embedded field (sent %q): %q, %v", log.last(t).query, got, err)
	}
	sent := search{paging{Page: 2, size: 9}, Limits{Limit: 3}, "x"}
	if got, err := client.Post(ctx, sent); err != nil || got != "page 2, limit 3 of x, size 0" {
		t.Fatalf("in a body: %q, %v", got, err)
	}
	if got, err := client.Search(ctx, sent); err != nil || got != "page 2, limit 3 of x, size 0" {
		t.Errorf("in a query (sent %q): %q, %v", log.last(t).query, got, err)
	}
	h, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	rec := serve(t, h, "GET", "/search?page=4&limit=5&name=y&size=7", "", "Accept", "text/plain")
	if rec.Code != 200 || rec.Body.String() != "page 4, limit 5 of y, size 0" {
		t.Errorf("GET /search?page=4&limit=5&name=y&size=7: %d %q", rec.Code, rec.Body)
	}
	doc := serve(t, h, "GET", "/documentation", "", "Accept", "application/json")
	if !strings.Contains(doc.Body.String(), `"name": "page"`) {
		t.Errorf("the OpenAPI document has no parameter for the embedded field:\n%s", doc.Body)
	}
}

type carried struct {
	api.Specification

	Drop func(context.Context, pathKey, string) (string, error)   `rest:"DELETE /things/{ID}"`
	Find func(context.Context, pathKey, string) (string, error)   `rest:"GET /things/{ID}"`
	Move func(context.Context, pathKey, string) (string, error)   `rest:"PUT /things/{ID}"`
	Join func(context.Context, pathKey, pathKey) (string, error)  `rest:"GET /join/{ID}"`
	Gone func(context.Context, *pathKey, string) (string, error)  `rest:"DELETE /maybe/{ID}"`
	One  func(context.Context, pathKey) (string, error)           `rest:"DELETE /one/{ID}"`
	Told func(context.Context, pathKey, string) (string, error)   `rest:"DELETE /told/{ID} (key,why)"`
	Many func(context.Context, pathKey, []string) (string, error) `rest:"DELETE /many/{ID}"`
}

// TestClientSendsACarriedStructBesideABody checks that a GET or DELETE with a
// struct whose field is in the path, and one more argument, can be called: the
// server reads an element of the body for each of the two, and the client left
// the struct out of what it sent.
func TestClientSendsACarriedStructBesideABody(t *testing.T) {
	said := func(_ context.Context, key pathKey, why string) (string, error) { return key.ID + " " + why, nil }
	impl := carried{
		Drop: said, Find: said, Move: said, Told: said,
		Join: func(_ context.Context, a, b pathKey) (string, error) { return a.ID + " " + b.ID, nil },
		Gone: func(_ context.Context, key *pathKey, why string) (string, error) {
			return fmt.Sprint(key.ID, " ", why), nil
		},
		One: func(_ context.Context, key pathKey) (string, error) { return key.ID, nil },
		Many: func(_ context.Context, key pathKey, whys []string) (string, error) {
			return key.ID + " " + strings.Join(whys, "+"), nil
		},
	}
	client, log := linked(t, impl)
	ctx := context.Background()
	for name, call := range map[string]func(context.Context, pathKey, string) (string, error){
		"PUT": client.Move, "DELETE": client.Drop, "GET": client.Find, "DELETE with rules": client.Told,
	} {
		if got, err := call(ctx, pathKey{ID: "1"}, "because"); err != nil || got != "1 because" {
			t.Errorf("%s (sent %s %q): %q, %v", name, log.last(t).path, log.last(t).body, got, err)
		}
	}
	if got, err := client.Join(ctx, pathKey{ID: "1"}, pathKey{ID: "2"}); err != nil || got != "1 2" {
		t.Errorf("two structs (sent %q): %q, %v", log.last(t).body, got, err)
	}
	if got, err := client.Gone(ctx, &pathKey{ID: "1"}, "because"); err != nil || got != "1 because" {
		t.Errorf("a pointer to the struct (sent %q): %q, %v", log.last(t).body, got, err)
	}
	if got, err := client.Many(ctx, pathKey{ID: "1"}, []string{"a", "b"}); err != nil || got != "1 a+b" {
		t.Errorf("beside a list (sent %q): %q, %v", log.last(t).body, got, err)
	}
	// With no other argument there is no body, and none is sent, as before.
	if got, err := client.One(ctx, pathKey{ID: "1"}); err != nil || got != "1" || log.last(t).body != "" {
		t.Errorf("the struct alone (sent %q): %q, %v", log.last(t).body, got, err)
	}
	// What a client that is not this one sends is read as it was: an element
	// for each argument, and the path over the body.
	h, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	for body, want := range map[string]int{
		`[{"id":"9"},"because"]`: 200,
		`[null,"because"]`:       200,
		`["because"]`:            400,
		`"because"`:              400,
	} {
		rec := serve(t, h, "DELETE", "/things/1", body, "Content-Type", "application/json", "Accept", "text/plain")
		if rec.Code != want || want == 200 && rec.Body.String() != "1 because" {
			t.Errorf("DELETE /things/1 with the body %s: %d %q, want %d", body, rec.Code, rec.Body, want)
		}
	}
}

// boxed is a type with a type argument, whose name encoding/xml cannot write as
// the name of an element.
type boxed[T any] string

func (b boxed[T]) MarshalText() ([]byte, error) { return []byte(b), nil }

func (b *boxed[T]) UnmarshalText(text []byte) error {
	*b = boxed[T](text)
	return nil
}

type page[T any] struct {
	Items []T `xml:"item" json:"items"`
}

type named struct {
	XMLName xml.Name `xml:"entry" json:"-"`
	Value   string   `xml:"value"`
}

type xmlValues struct {
	api.Specification

	Names func(context.Context, []string) ([]string, error)   `rest:"POST(application/xml) /names"`
	Nums  func(context.Context, []*int) (string, error)       `rest:"POST(application/xml) /nums"`
	Notes func(context.Context, []note) ([]note, error)       `rest:"POST(application/xml) /notes"`
	Bytes func(context.Context, []byte) (string, error)       `rest:"POST(application/xml) /bytes"`
	Named func(context.Context, []named) (string, error)      `rest:"POST(application/xml) /named"`
	Box   func(context.Context, boxed[int]) (string, error)   `rest:"POST(application/xml) /box"`
	Boxes func(context.Context, []boxed[int]) (string, error) `rest:"POST(application/xml) /boxes"`
	Page  func(context.Context, page[string]) (string, error) `rest:"POST(application/xml) /page"`

	GetBox   func(context.Context) (boxed[int], error)   `rest:"GET /box"`
	GetBoxes func(context.Context) ([]boxed[int], error) `rest:"GET /boxes"`
	GetPage  func(context.Context) (page[string], error) `rest:"GET /page"`
	GetNote  func(context.Context) (note, error)         `rest:"GET /note"`
}

func newXMLValues() xmlValues {
	return xmlValues{
		Names: same[[]string],
		Nums:  showPointers[int],
		Notes: same[[]note],
		Bytes: func(_ context.Context, b []byte) (string, error) { return string(b), nil },
		Named: func(_ context.Context, entries []named) (string, error) {
			values := make([]string, len(entries))
			for i, entry := range entries {
				values[i] = entry.Value
			}
			return strings.Join(values, ","), nil
		},
		Box:   func(_ context.Context, b boxed[int]) (string, error) { return string(b), nil },
		Boxes: func(_ context.Context, b []boxed[int]) (string, error) { return fmt.Sprint(b), nil },
		Page:  func(_ context.Context, p page[string]) (string, error) { return strings.Join(p.Items, ","), nil },

		GetBox:   func(context.Context) (boxed[int], error) { return "b", nil },
		GetBoxes: func(context.Context) ([]boxed[int], error) { return []boxed[int]{"a", "b"}, nil },
		GetPage:  func(context.Context) (page[string], error) { return page[string]{Items: []string{"a", "b"}}, nil },
		GetNote:  func(context.Context) (note, error) { return note{Title: "t"}, nil },
	}
}

// TestXMLListBodyArrivesWhole checks that a list sent as an application/xml
// body, which is an element for each value with nothing around them, is read to
// its end. The server read the first element and called the function with it.
func TestXMLListBodyArrivesWhole(t *testing.T) {
	impl := newXMLValues()
	client, log := linked(t, impl)
	ctx := context.Background()

	sent := []string{"a", "b", "c"}
	if got, err := client.Names(ctx, sent); err != nil || !reflect.DeepEqual(got, sent) {
		t.Errorf("Names(%q) sent %q: %q, %v", sent, log.last(t).body, got, err)
	}
	one, two := 1, 2
	if got, err := client.Nums(ctx, []*int{&one, &two}); err != nil || got != "1,2" {
		t.Errorf("a list of pointers sent %q: %q, %v", log.last(t).body, got, err)
	}
	notes := []note{{Title: "a", Stars: 1}, {Title: "b"}}
	if got, err := client.Notes(ctx, notes); err != nil || !reflect.DeepEqual(got, notes) {
		t.Errorf("a list of structs sent %q: %+v, %v", log.last(t).body, got, err)
	}
	if got, err := client.Named(ctx, []named{{Value: "a"}, {Value: "b"}}); err != nil || got != "a,b" {
		t.Errorf("a list of structs that name their element sent %q: %q, %v", log.last(t).body, got, err)
	}

	h, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	for body, want := range map[string]string{
		"<string>a</string>":                         `["a"]`,
		"<string>a</string>\n<string>b</string>\n":   `["a","b"]`,
		"<string>a</string><!-- no more -->":         `["a"]`,
		"<string>a</string><string>b</string><open>": "",
		"": "",
	} {
		rec := serve(t, h, "POST", "/names", body, "Content-Type", "application/xml", "Accept", "application/json")
		got := strings.Join(strings.Fields(rec.Body.String()), "")
		if want == "" && rec.Code != http.StatusBadRequest || want != "" && (rec.Code != 200 || got != want) {
			t.Errorf("POST /names with the body %q: %d %q, want %q", body, rec.Code, got, want)
		}
	}
	// A []byte is one element, and is still read as one.
	rec := serve(t, h, "POST", "/bytes", "<x>hi</x><x>no</x>", "Content-Type", "application/xml", "Accept", "text/plain")
	if rec.Code != 200 || rec.Body.String() != "hi" {
		t.Errorf("POST /bytes: %d %q", rec.Code, rec.Body)
	}
}

// TestXMLOfATypeWithTypeArgumentsIsXML checks that the element of a value whose
// type has type arguments has a name an XML parser reads. encoding/xml names it
// after all of the type's name, brackets and package path included, which is
// what was sent.
func TestXMLOfATypeWithTypeArgumentsIsXML(t *testing.T) {
	impl := newXMLValues()
	h, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]string{
		"/box":   "<boxed>b</boxed>",
		"/boxes": "<boxed>a</boxed><boxed>b</boxed>",
		"/page":  "<page><item>a</item><item>b</item></page>",
		// A type without type arguments is named as it was.
		"/note": "<note><Title>t</Title><Stars>0</Stars><Draft></Draft></note>",
	} {
		rec := serve(t, h, "GET", target, "", "Accept", "application/xml")
		if rec.Code != 200 || rec.Body.String() != want {
			t.Errorf("GET %s as application/xml: %d %q, want %q", target, rec.Code, rec.Body, want)
		}
	}
	client, log := linked(t, impl)
	ctx := context.Background()
	if got, err := client.Box(ctx, "b"); err != nil || got != "b" {
		t.Errorf("Box sent %q: %q, %v", log.last(t).body, got, err)
	}
	if got, err := client.Boxes(ctx, []boxed[int]{"a", "b"}); err != nil || got != "[a b]" {
		t.Errorf("Boxes sent %q: %q, %v", log.last(t).body, got, err)
	}
	if got, err := client.Page(ctx, page[string]{Items: []string{"a", "b"}}); err != nil || got != "a,b" {
		t.Errorf("Page sent %q: %q, %v", log.last(t).body, got, err)
	}
}

// unset is a text that cannot be written.
type unset string

func (unset) MarshalText() ([]byte, error) { return nil, errors.New("not set") }

type record struct {
	Name string `json:"name"`
	ID   unset  `json:"id"`
}

type attached struct {
	Name string    `json:"name"`
	File io.Reader `json:"file"`
	Last unset     `json:"last"`
}

type forms struct {
	api.Specification

	Bad   func(context.Context) (record, error)   `rest:"GET /bad"`
	Good  func(context.Context) (note, error)     `rest:"GET /good"`
	Files func(context.Context) (attached, error) `rest:"GET /files"`
}

// TestMultipartResultThatCannotBeWrittenIsAFailure checks that a result which
// fails to encode as multipart/form-data is answered as the failure it is in
// every other type. It was a 200 with the start of a form and the error's text
// after it, as the form was written straight to the response.
func TestMultipartResultThatCannotBeWrittenIsAFailure(t *testing.T) {
	h, err := rest.Handler(nil, forms{
		Bad:  func(context.Context) (record, error) { return record{Name: "a"}, nil },
		Good: func(context.Context) (note, error) { return note{Title: "t", Stars: 2}, nil },
		Files: func(context.Context) (attached, error) {
			return attached{Name: "a", File: strings.NewReader("content")}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, accept := range []string{"application/json", "multipart/form-data"} {
		rec := serve(t, h, "GET", "/bad", "", "Accept", accept)
		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "form-data") {
			t.Errorf("GET /bad as %s: %d, Content-Type %q, body %q; want 500 and no form", accept, rec.Code, rec.Header().Get("Content-Type"), rec.Body)
		}
	}
	// A form that can be written is sent as it was.
	rec := serve(t, h, "GET", "/good", "", "Accept", "multipart/form-data")
	fields := formFields(t, rec.Header().Get("Content-Type"), rec.Body)
	if rec.Code != 200 || !reflect.DeepEqual(fields, map[string]string{"title": "t", "stars": "2"}) {
		t.Errorf("GET /good as multipart/form-data: %d %v", rec.Code, fields)
	}
	// A part that is read is sent as it is read, so what fails after it can
	// only end the form early: the parts before it are still those of a form.
	rec = serve(t, h, "GET", "/files", "", "Accept", "multipart/form-data")
	if !strings.Contains(rec.Body.String(), "content") {
		t.Errorf("GET /files as multipart/form-data: %d %q", rec.Code, rec.Body)
	}
}

// formFields reads the fields of a multipart/form-data body.
func formFields(t *testing.T, contentType string, body io.Reader) map[string]string {
	t.Helper()
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("Content-Type %q: %v", contentType, err)
	}
	fields := make(map[string]string)
	reader := multipart.NewReader(body, params["boundary"])
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return fields
		}
		if err != nil {
			t.Fatal(err)
		}
		value, err := io.ReadAll(part)
		if err != nil {
			t.Fatal(err)
		}
		fields[part.FormName()] = string(value)
	}
}
