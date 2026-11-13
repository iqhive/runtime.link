package sql_test

import (
	"reflect"
	"testing"

	"github.com/iqhive/runtime.link/sql"
)

type marked struct {
	Name string
	Mark struct{}
	None [0]int
}

type marks struct {
	Rows sql.Map[string, marked] `sql:"marked_rows"`
}

// A field that has no columns, an empty struct or an array of nothing, has
// one value: it is equal to anything of its type and always empty, whether
// the values are kept in a database or not.
func TestFieldWithoutColumns(t *testing.T) {
	eachBackend(t, func(t *testing.T, DB *marks) {
		ctx := testContext(t)
		for _, name := range []string{"a", "b"} {
			if err := DB.Rows.Set(ctx, name, marked{Name: name}); err != nil {
				t.Fatal(err)
			}
		}
		for name, test := range map[string]struct {
			query func(*marked) sql.Query
			want  int
		}{
			"equal":       {func(v *marked) sql.Query { return sql.Query{sql.Index(&v.Mark).Equals(struct{}{})} }, 2},
			"equal array": {func(v *marked) sql.Query { return sql.Query{sql.Index(&v.None).Equals([0]int{})} }, 2},
			"empty":       {func(v *marked) sql.Query { return sql.Query{sql.Empty(&v.Mark)} }, 2},
			"empty array": {func(v *marked) sql.Query { return sql.Query{sql.Empty(&v.None)} }, 2},
			"not equal":   {func(v *marked) sql.Query { return sql.Query{sql.Avoid(sql.Index(&v.Mark).Equals(struct{}{}))} }, 0},
			"not empty":   {func(v *marked) sql.Query { return sql.Query{sql.Avoid(sql.Empty(&v.None))} }, 0},
			"with a column": {func(v *marked) sql.Query {
				return sql.Query{sql.Index(&v.Mark).Equals(struct{}{}), sql.Index(&v.Name).Equals("a")}
			}, 1},
			"merged": {func(v *marked) sql.Query {
				return sql.Query{sql.Merge(sql.Empty(&v.Mark), sql.Index(&v.Name).Equals("b"))}
			}, 1},
			"or a column": {func(v *marked) sql.Query {
				return sql.Query{sql.Cases(sql.Avoid(sql.Empty(&v.Mark)), sql.Index(&v.Name).Equals("b"))}
			}, 1},
			"the whole value": {func(v *marked) sql.Query { return sql.Query{sql.Index(v).Equals(marked{Name: "a"})} }, 1},
		} {
			n, err := DB.Rows.Count(ctx, func(_ *string, v *marked) sql.Query { return test.query(v) })
			if n != test.want || err != nil {
				t.Errorf("%s counted %d rows, %v, want %d", name, n, err, test.want)
			}
		}
		n, err := DB.Rows.Update(ctx, func(*string, *marked) sql.Query { return sql.Query{sql.Slice(0, 10)} }, func(v *marked) sql.Patch {
			return sql.Patch{sql.Set(&v.Mark, struct{}{}), sql.Set(&v.None, [0]int{}), sql.Set(&v.Name, "set")}
		})
		if n != 2 || err != nil {
			t.Errorf("update gave %d and %v", n, err)
		}
		if got, err := DB.Rows.Get(ctx, "a"); got != (marked{Name: "set"}) || err != nil {
			t.Errorf("the row is %+v, %v", got, err)
		}
	})
}

// Given what a QueryFunc for a database is given, there is nothing for Set to
// set in a field without columns: the modification is none, as the one of a
// Set that has assigned.
func TestSetOfFieldWithoutColumnsIsNoModification(t *testing.T) {
	DB := sql.Open[marks](newMemoryDatabase())
	var given *marked
	var issue error
	for range DB.Rows.Search(testContext(t), func(_ *string, v *marked) sql.Query { given = v; return nil }, &issue) {
	}
	if issue != nil || given == nil {
		t.Fatal(issue)
	}
	if mod := sql.Set(&given.Mark, struct{}{}); !reflect.ValueOf(mod).IsZero() {
		t.Errorf("setting an empty struct made the modification %v", mod)
	}
	if mod := sql.Set(&given.None, [0]int{}); !reflect.ValueOf(mod).IsZero() {
		t.Errorf("setting an empty array made the modification %v", mod)
	}
	if mod := sql.Set(&given.Name, "set"); reflect.ValueOf(mod).IsZero() {
		t.Error("setting a column made no modification")
	}
}
