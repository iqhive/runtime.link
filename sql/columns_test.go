package sql_test

import (
	"slices"
	"testing"

	"github.com/iqhive/runtime.link/sql"
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
