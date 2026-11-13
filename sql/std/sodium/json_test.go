package sodium_test

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/sql/std/sodium"
	"github.com/iqhive/runtime.link/xyz"
)

// sameValue reports whether two values are of one case and hold the same.
func sameValue(a, b sodium.Value) bool {
	if xyz.ValueOf(a) != xyz.ValueOf(b) {
		return false
	}
	switch xyz.ValueOf(a) {
	case sodium.Values.Time:
		return sodium.Values.Time.Get(a).Equal(sodium.Values.Time.Get(b))
	case sodium.Values.Bytes:
		return string(sodium.Values.Bytes.Get(a)) == string(sodium.Values.Bytes.Get(b))
	}
	return reflect.DeepEqual(a.Interface(), b.Interface())
}

// The socket carries values as JSON, where a number says nothing of the column
// type it is for. Each value is written under the name of its case, and read
// back as that case, at the edges of what the case holds too.
func TestValueKeepsItsCaseInJSON(t *testing.T) {
	values := []sodium.Value{
		sodium.Values.Bool.As(true),
		sodium.Values.Bool.As(false),
		sodium.Values.Int8.As(math.MinInt8),
		sodium.Values.Int16.As(math.MaxInt16),
		sodium.Values.Int32.As(math.MinInt32),
		sodium.Values.Int64.As(math.MinInt64),
		sodium.Values.Int64.As(0),
		sodium.Values.Uint8.As(math.MaxUint8),
		sodium.Values.Uint16.As(math.MaxUint16),
		sodium.Values.Uint32.As(math.MaxUint32),
		sodium.Values.Uint64.As(math.MaxUint64),
		sodium.Values.Float32.As(math.MaxFloat32),
		sodium.Values.Float32.As(1.5),
		sodium.Values.Float64.As(math.SmallestNonzeroFloat64),
		sodium.Values.Float64.As(1e300),
		sodium.Values.String.As(""),
		sodium.Values.String.As("5"),
		sodium.Values.String.As("text \"quoted\" é"),
		sodium.Values.Bytes.As([]byte{0, 255}),
		sodium.Values.Bytes.As(nil),
		sodium.Values.Time.As(time.Unix(1, 5).UTC()),
		sodium.Values.Time.As(time.Date(2026, 2, 3, 4, 5, 6, 7, time.FixedZone("east", 3600))),
		sodium.Values.Time.As(time.Time{}),
	}
	written := make(map[string]sodium.Value)
	for _, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			t.Errorf("%v %v: %v", xyz.ValueOf(value), value, err)
			continue
		}
		if other, clash := written[string(data)]; clash {
			t.Errorf("%v %v and %v %v are both written as %s", xyz.ValueOf(value), value, xyz.ValueOf(other), other, data)
		}
		written[string(data)] = value
		var back sodium.Value
		if err := json.Unmarshal(data, &back); err != nil {
			t.Errorf("%s: %v", data, err)
			continue
		}
		if !sameValue(back, value) {
			t.Errorf("%v %v is written as %s and read back as %v %v", xyz.ValueOf(value), value, data, xyz.ValueOf(back), back)
		}
	}
	if len(reflect.VisibleFields(reflect.TypeOf(sodium.Values))) != 14 {
		t.Error("a case of Value was added or removed, and this test covers 14")
	}
}

// What is not the JSON of a value is refused, rather than read as a value of
// no case that a database would store as nothing.
func TestValueRefusesJSONWithoutACase(t *testing.T) {
	for _, data := range []string{`5`, `"text"`, `true`, `[1]`, `{"int8":500}`, `{"uint8":-1}`, `{"bool":"yes"}`, `{"time":"yesterday"}`} {
		var value sodium.Value
		if err := json.Unmarshal([]byte(data), &value); err == nil {
			t.Errorf("%s was read as %v %q, want an error", data, xyz.ValueOf(value), value.String())
		}
	}
}

// A column says which case of Value it holds, by the name the case has in JSON.
func TestColumnKeepsItsTypeInJSON(t *testing.T) {
	cases := reflect.ValueOf(sodium.Values)
	names := make(map[string]bool)
	for i := range cases.NumField() {
		kind := cases.Field(i).Interface().(xyz.TypeOf[sodium.Value])
		column := sodium.Column{Name: "c", Type: kind, Tags: `sql:"c,scoped"`}
		data, err := json.Marshal(column)
		if err != nil {
			t.Fatalf("%v: %v", kind, err)
		}
		var back sodium.Column
		if err := json.Unmarshal(data, &back); err != nil {
			t.Fatalf("%s: %v", data, err)
		}
		if back != column {
			t.Errorf("a %v column is written as %s and read back as %+v", kind, data, back)
		}
		var fields struct{ Type string }
		if err := json.Unmarshal(data, &fields); err != nil || fields.Type == "" || names[fields.Type] {
			t.Errorf("a %v column has the type %q in %s, %v", kind, fields.Type, data, err)
		}
		names[fields.Type] = true
	}
	// A column of no type, as one made by hand to name a column, stays one.
	data, err := json.Marshal(sodium.Column{Name: "age"})
	if err != nil {
		t.Fatal(err)
	}
	var untyped sodium.Column
	if err := json.Unmarshal(data, &untyped); err != nil || untyped != (sodium.Column{Name: "age"}) {
		t.Errorf("a column of no type is written as %s and read back as %+v, %v", data, untyped, err)
	}
	kept := sodium.Column{Name: "kept", Type: sodium.Values.Bool}
	for _, data := range []string{`{"Name":"c","Type":"decimal"}`, `{"Name":"c","Type":5}`, `"c"`} {
		column := kept
		if err := json.Unmarshal([]byte(data), &column); err == nil {
			t.Errorf("%s was read as %+v, want an error", data, column)
		}
		if column != kept {
			t.Errorf("%s failed and left the column as %+v", data, column)
		}
	}
}

// roundTrip writes a value of the protocol as JSON and reads it back, and
// returns what was written the first and the second time.
func roundTrip[T any](t *testing.T, value T) (first, second string) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var back T
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("%s: %v", data, err)
	}
	again, err := json.Marshal(back)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), string(again)
}

// Every expression of a query is a different thing to ask of a database, so
// each is written differently, and read back as what was written.
func TestExpressionsAreToldApartInJSON(t *testing.T) {
	age := sodium.Column{Name: "age", Type: sodium.Values.Int64}
	name := sodium.Column{Name: "name", Type: sodium.Values.String}
	five := xyz.NewPair(age, sodium.Values.Int64.As(5))
	ad := xyz.NewPair(name, "ad")
	expressions := map[string]sodium.Expression{
		"true":       sodium.Expressions.Value.As(true),
		"false":      sodium.Expressions.Value.As(false),
		"equals":     sodium.Expressions.Index.As(five),
		"min":        sodium.Expressions.Where.As(sodium.WhereExpressions.Min.As(five)),
		"max":        sodium.Expressions.Where.As(sodium.WhereExpressions.Max.As(five)),
		"more than":  sodium.Expressions.Where.As(sodium.WhereExpressions.MoreThan.As(five)),
		"less than":  sodium.Expressions.Where.As(sodium.WhereExpressions.LessThan.As(five)),
		"contains":   sodium.Expressions.Match.As(sodium.MatchExpressions.Contains.As(ad)),
		"has prefix": sodium.Expressions.Match.As(sodium.MatchExpressions.HasPrefix.As(ad)),
		"has suffix": sodium.Expressions.Match.As(sodium.MatchExpressions.HasSuffix.As(ad)),
		"increasing": sodium.Expressions.Order.As(sodium.OrderExpressions.Increasing.As(age)),
		"decreasing": sodium.Expressions.Order.As(sodium.OrderExpressions.Decreasing.As(age)),
		"range":      sodium.Expressions.Range.As(sodium.Range{From: 1, Upto: 3}),
		"empty":      sodium.Expressions.Empty.As(age),
		"avoid":      sodium.Expressions.Avoid.As(sodium.Expressions.Index.As(five)),
		"cases":      sodium.Expressions.Cases.As([]sodium.Expression{sodium.Expressions.Index.As(five), sodium.Expressions.Empty.As(name)}),
		"group":      sodium.Expressions.Group.As([]sodium.Expression{sodium.Expressions.Index.As(five), sodium.Expressions.Empty.As(name)}),
		"no cases":   sodium.Expressions.Cases.As(nil),
	}
	written := make(map[string]string)
	for what, expression := range expressions {
		first, second := roundTrip(t, expression)
		if first != second {
			t.Errorf("%s is written as %s and read back as %s", what, first, second)
		}
		if other, clash := written[first]; clash {
			t.Errorf("%s and %s are both written as %s", what, other, first)
		}
		written[first] = what
		var back sodium.Expression
		if err := json.Unmarshal([]byte(first), &back); err != nil || xyz.ValueOf(back) != xyz.ValueOf(expression) {
			t.Errorf("%s is read back as the case %v, %v", what, xyz.ValueOf(back), err)
		}
	}
	// The whole of a query, as the socket is sent it.
	query := sodium.Query{expressions["min"], expressions["has prefix"], expressions["decreasing"], expressions["range"]}
	if first, second := roundTrip(t, query); first != second {
		t.Errorf("a query is written as %s and read back as %s", first, second)
	}
	var back sodium.Query
	data, _ := json.Marshal(query)
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	column, value := sodium.WhereExpressions.Min.Get(sodium.Expressions.Where.Get(back[0])).Split()
	if column != age || !sameValue(value, sodium.Values.Int64.As(5)) {
		t.Errorf("age >= 5 is read back as %+v >= %v %v", column, xyz.ValueOf(value), value)
	}
	if span := sodium.Expressions.Range.Get(back[3]); span != (sodium.Range{From: 1, Upto: 3}) {
		t.Errorf("the range is read back as %+v", span)
	}
}

func TestPatchAndStatsSurviveJSON(t *testing.T) {
	age := sodium.Column{Name: "age", Type: sodium.Values.Int64}
	patch := sodium.Patch{
		sodium.Modifications.Set.As(xyz.NewPair(age, sodium.Values.Int64.As(7))),
		sodium.Modifications.Arr.As([]sodium.Modification{
			sodium.Modifications.Set.As(xyz.NewPair(sodium.Column{Name: "name", Type: sodium.Values.String}, sodium.Values.String.As("7"))),
		}),
	}
	data, err := json.Marshal(patch)
	if err != nil {
		t.Fatal(err)
	}
	var patched sodium.Patch
	if err := json.Unmarshal(data, &patched); err != nil {
		t.Fatal(err)
	}
	if len(patched) != 2 || xyz.ValueOf(patched[0]) != sodium.Modifications.Set || xyz.ValueOf(patched[1]) != sodium.Modifications.Arr {
		t.Fatalf("the patch %s is read back as %v", data, patched)
	}
	column, value := sodium.Modifications.Set.Get(patched[0]).Split()
	if column != age || !sameValue(value, sodium.Values.Int64.As(7)) {
		t.Errorf("age = 7 is read back as %+v = %v %v", column, xyz.ValueOf(value), value)
	}
	_, nested := sodium.Modifications.Set.Get(sodium.Modifications.Arr.Get(patched[1])[0]).Split()
	if !sameValue(nested, sodium.Values.String.As("7")) {
		t.Errorf("the text 7 is read back as %v %v", xyz.ValueOf(nested), nested)
	}

	stats := sodium.Stats{
		sodium.Calculations.Add,
		sodium.Calculations.Sum.As(age),
		sodium.Calculations.Avg.As(age),
		sodium.Calculations.Top.As(age),
		sodium.Calculations.Min.As(age),
		sodium.Calculations.Max.As(age),
	}
	data, err = json.Marshal(stats)
	if err != nil {
		t.Fatal(err)
	}
	var counted sodium.Stats
	if err := json.Unmarshal(data, &counted); err != nil {
		t.Fatal(err)
	}
	if len(counted) != len(stats) {
		t.Fatalf("the stats %s are read back as %v", data, counted)
	}
	// A count has no column, and is recognised by being equal to Add.
	if counted[0] != sodium.Calculations.Add {
		t.Errorf("a count is read back as %v", xyz.ValueOf(counted[0]))
	}
	for i := 1; i < len(stats); i++ {
		if xyz.ValueOf(counted[i]) != xyz.ValueOf(stats[i]) {
			t.Errorf("%v is read back as %v", xyz.ValueOf(stats[i]), xyz.ValueOf(counted[i]))
		}
	}
	if sodium.Calculations.Sum.Get(counted[1]) != age || sodium.Calculations.Max.Get(counted[5]) != age {
		t.Errorf("the stats %s lost their column: %v", data, counted)
	}
}

// A result is what the socket answers with, a table what it is asked about.
func TestResultAndTableSurviveJSON(t *testing.T) {
	result := sodium.Result{
		Number: 3,
		Closed: true,
		Values: []sodium.Value{sodium.Values.Bool.As(true), sodium.Values.Int8.As(-5), sodium.Values.Uint64.As(5), sodium.Values.String.As("5")},
		Errors: []sodium.Error{sodium.Errors.Internal},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var back sodium.Result
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.Number != 3 || !back.Closed || len(back.Values) != len(result.Values) || len(back.Errors) != 1 || back.Errors[0] != sodium.Errors.Internal {
		t.Fatalf("the result %s is read back as %+v", data, back)
	}
	for i, value := range back.Values {
		if !sameValue(value, result.Values[i]) {
			t.Errorf("value %d of the result %s is read back as %v %v", i, data, xyz.ValueOf(value), value)
		}
	}

	id := sodium.Column{Name: "id", Type: sodium.Values.String}
	owner := sodium.Column{Name: "owner", Type: sodium.Values.String, Tags: `sql:"owner,scoped"`}
	table := sodium.Table{
		Name:  "accounts",
		Index: []sodium.Column{id},
		Value: []sodium.Column{owner, {Name: "at", Type: sodium.Values.Time}},
		Joins: []sodium.Join{{On: xyz.NewPair(owner, id), Table: sodium.Table{Name: "owners", Index: []sodium.Column{id}}}},
	}
	data, err = json.Marshal(table)
	if err != nil {
		t.Fatal(err)
	}
	var read sodium.Table
	if err := json.Unmarshal(data, &read); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read, table) {
		t.Errorf("the table %s is read back as %+v", data, read)
	}
}
