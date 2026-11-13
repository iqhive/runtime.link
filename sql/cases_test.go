package sql_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/iqhive/runtime.link/sql"
	"github.com/iqhive/runtime.link/sql/std/sodium"
)

// backends runs check against a RAM map and a database, each holding the
// accounts a, b and c.
func backends(t *testing.T, check func(t *testing.T, DB *bank)) {
	t.Helper()
	for name, open := range map[string]func() *bank{
		"RAM":      func() *bank { return sql.Open[bank](sql.New()) },
		"Database": func() *bank { return sql.Open[bank](newMemoryDatabase()) },
	} {
		t.Run(name, func(t *testing.T) {
			ctx := testContext(t)
			DB := open()
			for i, name := range []string{"a", "b", "c"} {
				if err := DB.Accounts.Set(ctx, name, account{Name: name, Age: i + 1}); err != nil {
					t.Fatal(err)
				}
			}
			check(t, DB)
		})
	}
}

// matchedKeys returns the sorted keys a query finds.
func matchedKeys(t *testing.T, DB *bank, query sql.QueryFunc[string, account]) []string {
	t.Helper()
	var issue error
	var keys []string
	for key := range DB.Accounts.Search(testContext(t), query, &issue) {
		keys = append(keys, key)
	}
	if issue != nil {
		t.Fatal(issue)
	}
	slices.Sort(keys)
	return keys
}

// namedAnyOf is the query a caller builds to find the accounts with any of the
// given names, which with no names used to find every account.
func namedAnyOf(names ...string) sql.QueryFunc[string, account] {
	return func(_ *string, a *account) sql.Query {
		var cases []sodium.Expression
		for _, name := range names {
			cases = append(cases, sql.Index(&a.Name).Equals(name))
		}
		return sql.Query{sql.Cases(cases...), sql.Slice(0, 10)}
	}
}

func TestCasesStillMatchesItsAlternatives(t *testing.T) {
	backends(t, func(t *testing.T, DB *bank) {
		if keys := matchedKeys(t, DB, namedAnyOf("a")); !reflect.DeepEqual(keys, []string{"a"}) {
			t.Errorf("one name found %q", keys)
		}
		if keys := matchedKeys(t, DB, namedAnyOf("a", "c")); !reflect.DeepEqual(keys, []string{"a", "c"}) {
			t.Errorf("two names found %q", keys)
		}
		if keys := matchedKeys(t, DB, namedAnyOf("z")); len(keys) != 0 {
			t.Errorf("a name no account has found %q", keys)
		}
		ctx := testContext(t)
		if n, err := DB.Accounts.UnsafeDelete(ctx, namedAnyOf("b")); n != 1 || err != nil {
			t.Errorf("delete gave %d and %v", n, err)
		}
		if keys := matchedKeys(t, DB, func(*string, *account) sql.Query { return sql.Query{sql.Slice(0, 10)} }); !reflect.DeepEqual(keys, []string{"a", "c"}) {
			t.Errorf("left %q", keys)
		}
	})
}
