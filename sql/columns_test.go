package sql_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/sql"
	"github.com/iqhive/runtime.link/sql/std/sodium"
	"github.com/iqhive/runtime.link/xyz"
)

type point struct{ X, Y int }

type shape struct {
	Name  string
	At    point
	Sides [2]int
}

type shapes struct {
	Shapes sql.Map[string, shape] `sql:"shapes"`
}

// A struct or an array is empty when all of it is, so a database has to be
// asked about each of its columns, not the first of them.
func TestEmptyChecksEveryColumn(t *testing.T) {
	eachBackend(t, func(t *testing.T, DB *shapes) {
		ctx := testContext(t)
		for _, s := range []shape{
			{Name: "nowhere"},
			{Name: "on the x axis", At: point{X: 5}},
			{Name: "on the y axis", At: point{Y: 5}, Sides: [2]int{0, 4}},
			{Name: "sided", Sides: [2]int{3, 0}},
		} {
			if err := DB.Shapes.Set(ctx, s.Name, s); err != nil {
				t.Fatal(err)
			}
		}
		found := func(query sql.QueryFunc[string, shape]) []string {
			var issue error
			var names []string
			for name := range DB.Shapes.Search(ctx, query, &issue) {
				names = append(names, name)
			}
			if issue != nil {
				t.Fatal(issue)
			}
			slices.Sort(names)
			return names
		}
		for name, test := range map[string]struct {
			query sql.QueryFunc[string, shape]
			want  []string
		}{
			"struct":  {func(_ *string, s *shape) sql.Query { return sql.Query{sql.Empty(&s.At)} }, []string{"nowhere", "sided"}},
			"array":   {func(_ *string, s *shape) sql.Query { return sql.Query{sql.Empty(&s.Sides)} }, []string{"nowhere", "on the x axis"}},
			"field":   {func(_ *string, s *shape) sql.Query { return sql.Query{sql.Empty(&s.At.X)} }, []string{"nowhere", "on the y axis", "sided"}},
			"element": {func(_ *string, s *shape) sql.Query { return sql.Query{sql.Empty(&s.Sides[1])} }, []string{"nowhere", "on the x axis", "sided"}},
			"not": {func(_ *string, s *shape) sql.Query { return sql.Query{sql.Avoid(sql.Empty(&s.At))} },
				[]string{"on the x axis", "on the y axis"}},
		} {
			if got := found(test.query); !slices.Equal(got, test.want) {
				t.Errorf("%s: found %q, want %q", name, got, test.want)
			}
		}
		// A write built on it reaches only what is empty.
		n, err := DB.Shapes.UnsafeDelete(ctx, func(_ *string, s *shape) sql.Query {
			return sql.Query{sql.Empty(&s.At), sql.Slice(0, 10)}
		})
		if n != 2 || err != nil {
			t.Errorf("deleted %d shapes that are nowhere, %v, want 2", n, err)
		}
		if left := found(nil); !slices.Equal(left, []string{"on the x axis", "on the y axis"}) {
			t.Errorf("the shapes left are %q", left)
		}
	})
}

// A pointer is followed to its value, which is how what was scanned for a
// result is read, and a nil pointer has no value to give.
func TestValuesOfFollowsPointers(t *testing.T) {
	type row struct {
		A int
		P *int
	}
	seven := 7
	texts := func(values []sodium.Value) []string {
		var texts []string
		for _, value := range values {
			texts = append(texts, value.String())
		}
		return texts
	}
	if got := texts(sql.ValuesOf(&row{A: 1, P: &seven})); !slices.Equal(got, []string{"1", "7"}) {
		t.Errorf("a pointer to a struct that holds a pointer gave %q", got)
	}
	if got := texts(sql.ValuesOf(&seven)); !slices.Equal(got, []string{"7"}) {
		t.Errorf("a pointer to a number gave %q", got)
	}
	defer func() {
		if recover() == nil {
			t.Error("a nil pointer did not raise a panic")
		}
	}()
	sql.ValuesOf(row{A: 1})
}

type unionOfShapes xyz.Tagged[any, struct {
	Circle xyz.Case[unionOfShapes, float64]
	Label  xyz.Case[unionOfShapes, string]
}]

type drawing struct {
	Name  string
	Shape unionOfShapes
}

// A union holds a value of another type in each row, so it has no column to
// be stored in, and a map of it cannot be opened with a database. That is
// reported like any other type without a column, with the field and the type
// named, and no longer as a nil pointer that was dereferenced. Without a
// database the value is stored as it is.
func TestOpenNamesUnionItHasNoColumnFor(t *testing.T) {
	ctx := testContext(t)
	var memory sql.Map[string, drawing]
	if err := memory.Set(ctx, "a", drawing{Name: "a"}); err != nil {
		t.Fatalf("without a database: %v", err)
	}
	if got, ok, err := memory.Lookup(ctx, "a"); !ok || err != nil || got.Name != "a" {
		t.Fatalf("without a database: %+v, %v, %v", got, ok, err)
	}
	defer func() {
		raised, _ := recover().(string)
		for _, named := range []string{"sql.columnsOf: unsupported type", "unionOfShapes", "field Shape"} {
			if !strings.Contains(raised, named) {
				t.Errorf("opening the map raised %q, which does not hold %q", raised, named)
			}
		}
	}()
	sql.Open[struct {
		Drawings sql.Map[string, drawing] `sql:"drawings"`
	}](newMemoryDatabase())
	t.Error("a map of a union was opened with a database")
}
