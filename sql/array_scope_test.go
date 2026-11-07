package sql_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/iqhive/runtime.link/sql"
	"github.com/iqhive/runtime.link/sql/std/sodium"
)

type arrayTenant struct {
	Team string `sql:"team,scoped"`
}

func checkArrayScopes[V any](t *testing.T, row func(string, string) V, patch func(*V) sql.Patch) {
	t.Helper()
	for _, persistent := range []bool{false, true} {
		name := "ram"
		var db sql.Database
		if persistent {
			name = "database"
			db = newMemoryDatabase()
		}
		t.Run(name, func(t *testing.T) {
			ctx := testContext(t)
			admin := sql.Admin(ctx, "seed array scopes")
			red := sql.Scope(ctx, "team", "red")
			m := sql.OpenTable[int, V](db, sodium.Table{Name: "array_scopes"})
			if err := m.Set(ctx, 1, row("blue", "blue")); !errors.Is(err, sql.ErrUnauthorized) {
				t.Fatalf("unauthorized write: %v", err)
			}
			for key, value := range map[int]V{1: row("red", "red"), 2: row("red", "blue"), 3: row("blue", "blue")} {
				if err := m.Set(admin, key, value); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, err := m.Lookup(ctx, 1); !errors.Is(err, sql.ErrUnauthorized) {
				t.Fatalf("unauthorized read: %v", err)
			}
			if n, err := m.Count(red, nil); err != nil || n != 1 {
				t.Fatalf("scoped count: %d, %v", n, err)
			}
			if n, err := m.Count(sql.Scope(ctx, "team", "red", "blue"), nil); err != nil || n != 3 {
				t.Fatalf("multiple grants: %d, %v", n, err)
			}
			if n, err := m.Count(sql.Scope[string](ctx, "team"), nil); err != nil || n != 0 {
				t.Fatalf("empty grant: %d, %v", n, err)
			}
			for _, key := range []int{2, 3} {
				if _, exists, err := m.Lookup(red, key); err != nil || exists {
					t.Fatalf("out-of-scope lookup %d: %v, %v", key, exists, err)
				}
			}
			if err := m.Set(red, 4, row("red", "blue")); !errors.Is(err, sql.ErrAccessDenied) {
				t.Fatalf("out-of-scope insert: %v", err)
			}
			if err := m.Set(red, 2, row("red", "red")); !errors.Is(err, sql.ErrAccessDenied) {
				t.Fatalf("out-of-scope upsert: %v", err)
			}
			if _, err := m.Mutate(red, 1, nil, patch); !errors.Is(err, sql.ErrAccessDenied) {
				t.Fatalf("out-of-scope mutation: %v", err)
			}
			all := func(*int, *V) sql.Query { return sql.Query{sql.Slice(0, 10)} }
			if _, err := m.Update(red, all, patch); !errors.Is(err, sql.ErrAccessDenied) {
				t.Fatalf("out-of-scope update: %v", err)
			}
			if got, err := m.Get(red, 1); err != nil || !reflect.DeepEqual(got, row("red", "red")) {
				t.Fatalf("rejected patch persisted: %+v, %v", got, err)
			}
			if n, err := m.UnsafeDelete(red, all); err != nil || n != 1 {
				t.Fatalf("scoped deletion: %d, %v", n, err)
			}
			if n, err := m.Count(admin, nil); err != nil || n != 2 {
				t.Fatalf("foreign rows after deletion: %d, %v", n, err)
			}
		})
	}
}

func TestScopesInsideArrays(t *testing.T) {
	t.Run("root", func(t *testing.T) {
		checkArrayScopes(t, func(a, b string) [2]arrayTenant { return [2]arrayTenant{{a}, {b}} }, func(v *[2]arrayTenant) sql.Patch { return sql.Patch{sql.Set(&v[1].Team, "blue")} })
	})
	t.Run("nested", func(t *testing.T) {
		type row struct{ Tenants [2][1]arrayTenant }
		checkArrayScopes(t, func(a, b string) row { return row{Tenants: [2][1]arrayTenant{{{a}}, {{b}}}} }, func(v *row) sql.Patch { return sql.Patch{sql.Set(&v.Tenants[1][0].Team, "blue")} })
	})
}
