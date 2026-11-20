package sql_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/iqhive/runtime.link/sql"
	"github.com/iqhive/runtime.link/sql/std/sodium"
)

type scopedNote struct {
	Text  string
	Group string `sql:",scoped"`
}

type scopedNotes struct {
	Notes sql.Map[int, scopedNote] `sql:"notes"`
}

func TestScopes(t *testing.T) {
	eachBackend(t, testScopes)
}

func testScopes(t *testing.T, db *scopedNotes) {
	ctx := testContext(t)
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
	// Access is only ever widened by Admin and Widen. Nothing that is passed to
	// Scope, which is where values that come from a client end up, does so.
	t.Run("OnlyAdminWidens", func(t *testing.T) {
		for name, ctx := range map[string]context.Context{
			"empty value":    sql.Scope(ctx, "group", ""),
			"wildcard value": sql.Scope(ctx, "group", "*", "%"),
			"admin value":    sql.Scope(ctx, "group", "admin", "testing"),
			"rescoped":       sql.Scope(sql.Scope(ctx, "group", "a"), "group", "a", "b", "c"),
		} {
			n := count(ctx)
			if want := map[string]int{"rescoped": 1}[name]; n != want {
				t.Errorf("%s: expected %v notes, got %v", name, want, n)
			}
		}
		for name, ctx := range map[string]context.Context{
			"other scope": sql.Scope(ctx, "other", "a"),
			"admin scope": sql.Scope(ctx, "admin", "testing"),
			"empty scope": sql.Scope(ctx, "", "a"),
		} {
			if _, err := db.Notes.Count(ctx, nil); !errors.Is(err, sql.ErrUnauthorized) {
				t.Errorf("%s: expected ErrUnauthorized, got %v", name, err)
			}
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

// denyingUpserts answers an upsert with [sql.ErrAccessDenied], as a database
// that enforces a scope itself does. An insert that is not an upsert it
// answers as the database it holds.
type denyingUpserts struct {
	*memoryDatabase
}

func (db denyingUpserts) Insert(table sodium.Table, index []sodium.Value, overwrite bool, query sodium.Query, value []sodium.Value) sodium.Job {
	if !overwrite {
		return db.memoryDatabase.Insert(table, index, overwrite, query, value)
	}
	return newMemoryWrite(func(context.Context) (int, error) {
		return 0, sql.ErrAccessDenied
	})
}

// A scoped upsert that the database refuses with one of the package's own
// errors is given it back as it was, so that a caller can tell which one it
// was, the way an insert returns [sql.ErrDuplicate].
func TestScopedUpsertKeepsTheSentinels(t *testing.T) {
	ctx := testContext(t)
	DB := sql.Open[scopedNotes](denyingUpserts{newMemoryDatabase()})
	if err := DB.Notes.Insert(sql.Admin(ctx, "setup"), 1, sql.Create, scopedNote{Text: "one", Group: "a"}); err != nil {
		t.Fatal(err)
	}
	a := sql.Scope(ctx, "group", "a")
	if err := DB.Notes.Set(a, 1, scopedNote{Text: "one", Group: "a"}); err != sql.ErrAccessDenied {
		t.Errorf("a refused upsert gave %v, want the refusal as it was", err)
	}
}

func TestRAMCallbacksOnlySeeAdmittedRows(t *testing.T) {
	ctx := testContext(t)
	admin := sql.Admin(ctx, "seed notes")
	var notes sql.Map[int, scopedNote]
	for key, group := range []string{"a", "a", "b"} {
		if err := notes.Set(admin, key+1, scopedNote{Group: group}); err != nil {
			t.Fatal(err)
		}
	}
	a := sql.Scope(ctx, "group", "a")
	query := func(k *int, v *scopedNote) sql.Query {
		if v.Group == "b" {
			t.Fatal("query was given a hidden row")
		}
		return sql.Query{sql.Order(k).Increasing(), sql.Slice(0, 3)}
	}
	var issue error
	var keys []int
	for k, v := range notes.Search(a, query, &issue) {
		if v.Group != "a" {
			t.Fatalf("yielded %v", v)
		}
		keys = append(keys, k)
		if k == 1 {
			if err := notes.Set(admin, 2, scopedNote{Group: "b"}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if issue != nil || !reflect.DeepEqual(keys, []int{1, 2}) {
		t.Fatalf("ordered snapshot: %v, %v", keys, issue)
	}
	if n, err := notes.Count(a, query); err != nil || n != 1 {
		t.Fatalf("count: %d, %v", n, err)
	}
	check := func(v *scopedNote) sql.Check {
		if v.Group == "b" {
			t.Fatal("check was given a hidden row")
		}
		return nil
	}
	if ok, err := notes.Mutate(a, 2, check, func(*scopedNote) sql.Patch { t.Fatal("patched hidden row"); return nil }); ok || err != nil {
		t.Fatalf("mutate: %v, %v", ok, err)
	}
	if ok, err := notes.Delete(a, 2, check); ok || err != nil {
		t.Fatalf("delete: %v, %v", ok, err)
	}
	if n, err := notes.Update(a, query, func(*scopedNote) sql.Patch { return nil }); n != 1 || err != nil {
		t.Fatalf("update: %d, %v", n, err)
	}
	if n, err := notes.UnsafeDelete(a, query); n != 1 || err != nil {
		t.Fatalf("delete query: %d, %v", n, err)
	}
}
