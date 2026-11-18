package sql_test

import (
	"context"
	"errors"
	"testing"

	"runtime.link/sql"
)

// group is the scope the notes are scoped by.
type group string

type scopedNote struct {
	Text  string
	Group group `sql:",scoped"`
}

type scopedNotes struct {
	Notes sql.Map[int, scopedNote] `sql:"notes"`
}

func TestScopes(t *testing.T) {
	ctx := context.Background()
	db := sql.Open[scopedNotes](sql.New())
	admin := sql.Admin(ctx, "testing")
	for i, g := range []group{"a", "b", "c"} {
		if err := db.Notes.Insert(admin, i+1, sql.Create, scopedNote{Text: string(g), Group: g}); err != nil {
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
		ctx := sql.Scope(sql.Scope(ctx, sql.Reads, group("a"), "b"), sql.Reads, group("b"), "c")
		if n := count(ctx); n != 1 {
			t.Fatalf("expected only group b, got %v notes", n)
		}
	})
	t.Run("NoValues", func(t *testing.T) {
		ctx := sql.Scope[group](sql.Scope[group](ctx, sql.Reads), sql.Write)
		if n := count(ctx); n != 0 {
			t.Fatalf("expected no notes, got %v", n)
		}
		if err := db.Notes.Insert(ctx, 4, sql.Create, scopedNote{Group: "a"}); !errors.Is(err, sql.ErrAccessDenied) {
			t.Fatalf("expected ErrAccessDenied, got %v", err)
		}
	})
	t.Run("Widen", func(t *testing.T) {
		if n := count(sql.Widen[group](sql.Scope(ctx, sql.Reads, group("a")), sql.Reads)); n != 3 {
			t.Fatalf("expected every note, got %v", n)
		}
	})
	t.Run("Admin", func(t *testing.T) {
		if n := count(admin); n != 3 {
			t.Fatalf("expected every note, got %v", n)
		}
		if n := count(sql.Scope(admin, sql.Reads, group("c"))); n != 1 {
			t.Fatalf("expected scoping an admin context to narrow it, got %v notes", n)
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
	t.Run("ScopeOfAnUnnamedType", func(t *testing.T) {
		for name, scope := range map[string]func(){
			"Scope": func() { sql.Scope(ctx, sql.Reads, "a") },
			"Widen": func() { sql.Widen[int](ctx, sql.Write) },
		} {
			func() {
				defer func() {
					if recover() == nil {
						t.Errorf("expected sql.%v of an unnamed type to panic", name)
					}
				}()
				scope()
			}()
		}
	})
	t.Run("ScopedColumnOfAnUnnamedType", func(t *testing.T) {
		unnamed := sql.Open[struct {
			Notes sql.Map[int, struct {
				Group string `sql:",scoped"`
			}] `sql:"unnamed"`
		}](sql.New())
		if _, err := unnamed.Notes.Count(admin, nil); err == nil {
			t.Fatal("expected a scoped column of an unnamed type to be refused")
		}
	})
	t.Run("ScopesAreTypes", func(t *testing.T) {
		// A scope of a different type, with the same name and values, is a
		// different scope.
		type other string
		if _, err := db.Notes.Count(sql.Widen[other](ctx, sql.Reads), nil); !errors.Is(err, sql.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
		// Whatever the column is called.
		renamed := sql.Open[struct {
			Notes sql.Map[int, struct {
				Team group `sql:"team,scoped"`
			}] `sql:"renamed"`
		}](sql.New())
		if _, err := renamed.Notes.Count(sql.Widen[group](ctx, sql.Reads), nil); err != nil {
			t.Fatalf("expected the scope of the type to cover a column of another name, got %v", err)
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
	t.Run("ReadOnly", func(t *testing.T) {
		ctx := sql.Widen[group](ctx, sql.Reads)
		if n := count(ctx); n != 3 {
			t.Fatalf("expected every note, got %v", n)
		}
		if err := db.Notes.Insert(ctx, 4, sql.Create, scopedNote{Group: "a"}); !errors.Is(err, sql.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized writing with only read access, got %v", err)
		}
	})
	t.Run("ReadEverythingWriteSome", func(t *testing.T) {
		ctx := sql.Scope(sql.Widen[group](ctx, sql.Reads), sql.Write, group("a"))
		if n := count(ctx); n != 3 {
			t.Fatalf("expected to read every note, got %v", n)
		}
		if _, err := db.Notes.Mutate(ctx, 2, nil, func(n *scopedNote) sql.Patch {
			return sql.Patch{sql.Set(&n.Text, "changed")}
		}); !errors.Is(err, sql.ErrAccessDenied) {
			t.Fatalf("expected ErrAccessDenied changing a readable note outside of the write scope, got %v", err)
		}
		if ok, err := db.Notes.Mutate(ctx, 1, nil, func(n *scopedNote) sql.Patch {
			return sql.Patch{sql.Set(&n.Text, "changed")}
		}); err != nil || !ok {
			t.Fatalf("expected to change a note inside of the write scope, got %v %v", ok, err)
		}
	})
	t.Run("WriteNeedsRead", func(t *testing.T) {
		ctx := sql.Scope(sql.Scope(ctx, sql.Reads, group("a")), sql.Write, group("a"), "b")
		if err := db.Notes.Insert(ctx, 5, sql.Create, scopedNote{Group: "b"}); !errors.Is(err, sql.ErrAccessDenied) {
			t.Fatalf("expected ErrAccessDenied writing outside of the read scope, got %v", err)
		}
	})
}
