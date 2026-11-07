package sql_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/iqhive/runtime.link/sql"
)

type scopedNote struct {
	Text  string
	Group string `sql:",scoped"`
}

type scopedNotes struct {
	Notes sql.Map[int, scopedNote] `sql:"notes"`
}

func TestScopes(t *testing.T) {
	ctx := context.Background()
	db := sql.Open[scopedNotes](sql.New())
	admin := sql.Admin(ctx, "testing")
	for i, group := range []string{"a", "b", "c"} {
		if err := db.Notes.Insert(admin, i+1, sql.Create, scopedNote{Text: group, Group: group}); err != nil {
			t.Fatal(err)
		}
	}
	count := func(ctx context.Context) int {
		t.Helper()
		n, err := db.Notes.Count(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	t.Run("Narrowing", func(t *testing.T) {
		ctx := sql.Scope(sql.Scope(ctx, "group", "a", "b"), "group", "b", "c")
		if n := count(ctx); n != 1 {
			t.Fatalf("expected only group b, got %v notes", n)
		}
	})
	t.Run("NoValues", func(t *testing.T) {
		ctx := sql.Scope[string](ctx, "group")
		if n := count(ctx); n != 0 {
			t.Fatalf("expected no notes, got %v", n)
		}
		if err := db.Notes.Insert(ctx, 4, sql.Create, scopedNote{Group: "a"}); !errors.Is(err, sql.ErrAccessDenied) {
			t.Fatalf("expected ErrAccessDenied, got %v", err)
		}
	})
	t.Run("Widen", func(t *testing.T) {
		if n := count(sql.Widen[string](sql.Scope(ctx, "group", "a"), "group")); n != 3 {
			t.Fatalf("expected every note, got %v", n)
		}
	})
	t.Run("Admin", func(t *testing.T) {
		if n := count(admin); n != 3 {
			t.Fatalf("expected every note, got %v", n)
		}
		if n := count(sql.Scope(admin, "group", "c")); n != 1 {
			t.Fatalf("expected scoping an admin context to narrow it, got %v notes", n)
		}
	})
	t.Run("AdminAfterScope", func(t *testing.T) {
		if n := count(sql.Admin(sql.Scope(ctx, "group", "a"), "testing")); n != 1 {
			t.Fatalf("expected admin to keep a narrower scope, got %v notes", n)
		}
		if n := count(sql.Scope(sql.Admin(sql.Scope(ctx, "group", "a"), "testing"), "group", "b")); n != 0 {
			t.Fatalf("expected scoping to keep narrowing past admin, got %v notes", n)
		}
	})
	t.Run("AdminReason", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected sql.Admin without a reason to panic")
			}
		}()
		sql.Admin(ctx, "")
	})
	t.Run("TypeMismatch", func(t *testing.T) {
		if _, err := db.Notes.Count(sql.Scope(ctx, "group", 1), nil); err == nil {
			t.Fatal("expected an error scoping a string column with an int")
		}
		if _, err := db.Notes.Count(sql.Widen[int](ctx, "group"), nil); err == nil {
			t.Fatal("expected an error widening a string column as an int")
		}
	})
	t.Run("Unscoped", func(t *testing.T) {
		if _, err := db.Notes.Count(ctx, nil); !errors.Is(err, sql.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})
	t.Run("UnscopedTable", func(t *testing.T) {
		plain := sql.Open[struct {
			Notes sql.Map[int, string] `sql:"plain"`
		}](sql.New())
		if err := plain.Notes.Insert(ctx, 1, sql.Create, "hello"); err != nil {
			t.Fatal(err)
		}
		if _, ok, err := plain.Notes.Lookup(ctx, 1); err != nil || !ok {
			t.Fatalf("expected a table without scopes to need none, got %v %v", ok, err)
		}
	})
}

func TestRejectedScopedPatchPreservesReferences(t *testing.T) {
	type document struct {
		Team    string `sql:"team,scoped"`
		Text    string
		Slice   []int
		Map     map[string][]int
		Pointer *int
		Any     any
	}
	original := func() document {
		n := 10
		return document{Team: "red", Text: "before", Slice: []int{10}, Map: map[string][]int{"a": {10}}, Pointer: &n, Any: []int{10}}
	}
	for _, update := range []bool{false, true} {
		for _, allowed := range []bool{false, true} {
			t.Run(fmt.Sprintf("update=%v/allowed=%v", update, allowed), func(t *testing.T) {
				ctx := sql.Scope(context.Background(), "team", "red")
				var m sql.Map[int, document]
				if err := m.Set(ctx, 1, original()); err != nil {
					t.Fatal(err)
				}
				patch := func(v *document) sql.Patch {
					if !allowed {
						v.Team = "blue"
					}
					v.Text = "after"
					v.Slice[0] = 99
					v.Map["a"][0] = 99
					*v.Pointer = 99
					v.Any.([]int)[0] = 99
					return nil
				}
				var err error
				if update {
					_, err = m.Update(ctx, func(*int, *document) sql.Query { return sql.Query{sql.Slice(0, 1)} }, patch)
				} else {
					_, err = m.Mutate(ctx, 1, nil, patch)
				}
				if allowed && err != nil || !allowed && !errors.Is(err, sql.ErrAccessDenied) {
					t.Fatalf("patch error: %v", err)
				}
				want := original()
				if allowed {
					patch(&want)
				}
				got, err := m.Get(ctx, 1)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("stored value: %+v, %v; want %+v", got, err, want)
				}
			})
		}
	}
}
