package sql_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/sql"
)

// scopedTeam is a scoped column for the types below to hold in places that
// are not columns.
type scopedTeam struct {
	Team string `sql:"team,scoped"`
}

// OwnedByTeam is embedded under a name that is exported, which makes columns
// of its fields.
type OwnedByTeam struct {
	Team string `sql:"team,scoped"`
}

// unexportedTeam is embedded under a name that is not exported.
type unexportedTeam struct {
	Team string `sql:"team,scoped"`
}

type embedsUnexportedTeam struct {
	unexportedTeam
	Text string
}

type privateTeam struct {
	Text string
	team string `sql:"team,scoped"`
}

type privateOwner struct {
	Text  string
	owner scopedTeam
}

type misspeltPrivateTeam struct {
	Text  string
	owner struct {
		Team string `sql:"team, scoped"`
	}
}

type privateTenantKey struct {
	ID     int
	tenant string `sql:"tenant,scoped"`
}

type teamBehindPointer struct {
	Text  string
	Owner *scopedTeam
}

type teamsInSlice struct {
	Text   string
	Owners []scopedTeam
}

type teamsInMap struct {
	Text   string
	Owners map[string]scopedTeam
}

type teamsAsMapKeys struct {
	Text   string
	Owners map[scopedTeam]bool
}

type teamBehindPointers struct {
	Text  string
	Owner [2]**struct{ Inner []*scopedTeam }
}

// teamChain leads back to itself, with a scoped column on the way.
type teamChain struct {
	Team string `sql:"team,scoped"`
	Next *teamChain
}

type chained struct {
	Text  string
	First *teamChain
}

// refusedForScope checks that an operation failed because of the scoped field,
// rather than for want of a scope, which a context could then bring.
func refusedForScope(t *testing.T, field string, err error) {
	t.Helper()
	if err == nil || errors.Is(err, sql.ErrUnauthorized) || errors.Is(err, sql.ErrAccessDenied) {
		t.Errorf("got %v, want the scoped field to be refused", err)
		return
	}
	if !strings.Contains(err.Error(), "scoped field "+field+" ") {
		t.Errorf("the error does not name the field %s: %v", field, err)
	}
}

// everyScope is a context with no scope, one with the scopes the types here
// ask for, and one with access to everything.
func everyScope(t *testing.T) map[string]context.Context {
	t.Helper()
	ctx := testContext(t)
	return map[string]context.Context{
		"no scope": ctx,
		"scoped":   sql.Scope(sql.Scope(ctx, "team", "red"), "tenant", "a"),
		"admin":    sql.Admin(ctx, "testing"),
	}
}

// unusable checks that no operation of a map can be made with any context,
// and that the map was left empty.
func unusable[K comparable, V any](t *testing.T, m *sql.Map[K, V], field string, key K, value V) {
	t.Helper()
	all := func(*K, *V) sql.Query { return sql.Query{sql.Slice(0, 10)} }
	patch := func(*V) sql.Patch { return nil }
	for name, use := range map[string]func(context.Context) error{
		"Insert": func(ctx context.Context) error { return m.Insert(ctx, key, sql.Create, value) },
		"Set":    func(ctx context.Context) error { return m.Set(ctx, key, value) },
		"Add":    func(ctx context.Context) error { _, err := m.Add(ctx, value); return err },
		"Lookup": func(ctx context.Context) error { _, _, err := m.Lookup(ctx, key); return err },
		"First":  func(ctx context.Context) error { _, _, _, err := m.First(ctx, nil); return err },
		"Count":  func(ctx context.Context) error { _, err := m.Count(ctx, nil); return err },
		"Search": func(ctx context.Context) error {
			var issue error
			for range m.Search(ctx, nil, &issue) {
			}
			return issue
		},
		"Update":       func(ctx context.Context) error { _, err := m.Update(ctx, all, patch); return err },
		"Mutate":       func(ctx context.Context) error { _, err := m.Mutate(ctx, key, nil, patch); return err },
		"Delete":       func(ctx context.Context) error { _, err := m.Delete(ctx, key, nil); return err },
		"UnsafeDelete": func(ctx context.Context) error { _, err := m.UnsafeDelete(ctx, all); return err },
	} {
		for scope, ctx := range everyScope(t) {
			t.Run(name+"/"+scope, func(t *testing.T) { refusedForScope(t, field, use(ctx)) })
		}
	}
}

// A field tagged as scoped that is not a column cannot be held to a scope with
// a database, which does not store it, so it must not leave the map open to a
// context with no scope, or to one with another: the map refuses to work, with
// and without a database, for any context.
func TestScopedFieldThatIsNotAColumnIsRefused(t *testing.T) {
	t.Run("embedded struct of a type that is not exported", func(t *testing.T) {
		eachBackend(t, func(t *testing.T, DB *struct {
			Rows sql.Map[string, embedsUnexportedTeam] `sql:"embedded_team_rows"`
		}) {
			unusable(t, &DB.Rows, "Team", "r", embedsUnexportedTeam{unexportedTeam{"red"}, "red's"})
		})
	})
	t.Run("field that is not exported", func(t *testing.T) {
		eachBackend(t, func(t *testing.T, DB *struct {
			Rows sql.Map[string, privateTeam] `sql:"private_team_rows"`
		}) {
			unusable(t, &DB.Rows, "team", "r", privateTeam{Text: "red's", team: "red"})
		})
	})
	t.Run("inside of a field that is not exported", func(t *testing.T) {
		eachBackend(t, func(t *testing.T, DB *struct {
			Rows sql.Map[string, privateOwner] `sql:"private_owner_rows"`
		}) {
			unusable(t, &DB.Rows, "Team", "r", privateOwner{Text: "red's", owner: scopedTeam{"red"}})
		})
	})
	t.Run("misspelt inside of a field that is not exported", func(t *testing.T) {
		eachBackend(t, func(t *testing.T, DB *struct {
			Rows sql.Map[string, misspeltPrivateTeam] `sql:"misspelt_private_rows"`
		}) {
			unusable(t, &DB.Rows, "Team", "r", misspeltPrivateTeam{Text: "red's"})
		})
	})
	t.Run("field of a key that is not exported", func(t *testing.T) {
		eachBackend(t, func(t *testing.T, DB *struct {
			Rows sql.Map[privateTenantKey, string] `sql:"private_tenant_rows"`
		}) {
			unusable(t, &DB.Rows, "tenant", privateTenantKey{ID: 1, tenant: "a"}, "a's")
		})
	})
	// A map with a database cannot be opened for what follows, and one
	// without a database stores it.
	t.Run("behind a pointer", func(t *testing.T) {
		var m sql.Map[string, teamBehindPointer]
		unusable(t, &m, "Team", "r", teamBehindPointer{Text: "red's", Owner: &scopedTeam{"red"}})
	})
	t.Run("value that is a pointer", func(t *testing.T) {
		var m sql.Map[string, *scopedTeam]
		unusable(t, &m, "Team", "r", &scopedTeam{"red"})
	})
	t.Run("in a slice", func(t *testing.T) {
		var m sql.Map[string, teamsInSlice]
		unusable(t, &m, "Team", "r", teamsInSlice{Text: "red's", Owners: []scopedTeam{{"red"}}})
	})
	t.Run("in a map", func(t *testing.T) {
		var m sql.Map[string, teamsInMap]
		unusable(t, &m, "Team", "r", teamsInMap{Text: "red's", Owners: map[string]scopedTeam{"o": {"red"}}})
	})
	t.Run("in the keys of a map", func(t *testing.T) {
		var m sql.Map[string, teamsAsMapKeys]
		unusable(t, &m, "Team", "r", teamsAsMapKeys{Text: "red's", Owners: map[scopedTeam]bool{{"red"}: true}})
	})
	t.Run("behind several", func(t *testing.T) {
		var m sql.Map[string, teamBehindPointers]
		unusable(t, &m, "Team", "r", teamBehindPointers{Text: "red's"})
	})
	t.Run("in a type that leads back to itself", func(t *testing.T) {
		var m sql.Map[string, chained]
		unusable(t, &m, "Team", "r", chained{Text: "red's", First: &teamChain{Team: "red"}})
	})
}

// What is refused is a scope that cannot be kept. A scoped column beside
// fields that are not columns is still held to its scope, and fields that are
// not columns and ask for no scope leave a map as open as it was.
func TestFieldsThatAreNotColumnsWithoutScope(t *testing.T) {
	type chain struct {
		Text string
		Next *chain
	}
	type hidden struct{ Team string }
	type plain struct {
		hidden
		Text  string
		note  string
		Chain *chain
		Tags  []string
		Extra map[string]hidden
	}
	type owned struct {
		Team  string `sql:"team,scoped"`
		Text  string
		note  string
		Chain *chain
		Tags  []string
	}
	ctx := testContext(t)
	t.Run("no scope asked for", func(t *testing.T) {
		var m sql.Map[string, plain]
		stored := plain{hidden: hidden{"red"}, Text: "t", note: "n", Chain: &chain{Text: "c", Next: &chain{}}, Tags: []string{"x"}}
		if err := m.Set(ctx, "p", stored); err != nil {
			t.Fatal(err)
		}
		got, ok, err := m.Lookup(ctx, "p")
		if !ok || err != nil {
			t.Fatalf("lookup gave %v and %v", ok, err)
		}
		if got.Team != "red" || got.Text != "t" || got.note != "n" || got.Chain.Text != "c" || got.Tags[0] != "x" {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("scoped column beside them", func(t *testing.T) {
		var m sql.Map[string, owned]
		red, blue := sql.Scope(ctx, "team", "red"), sql.Scope(ctx, "team", "blue")
		if err := m.Set(red, "r", owned{Team: "red", Text: "red's", Chain: &chain{}, Tags: []string{"x"}}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := m.Lookup(ctx, "r"); !errors.Is(err, sql.ErrUnauthorized) {
			t.Errorf("a lookup with no scope gave %v, want ErrUnauthorized", err)
		}
		if _, ok, err := m.Lookup(blue, "r"); ok || err != nil {
			t.Errorf("another team's lookup gave %v and %v", ok, err)
		}
		if err := m.Set(blue, "r", owned{Team: "blue"}); !errors.Is(err, sql.ErrAccessDenied) {
			t.Errorf("another team's write over the row gave %v, want ErrAccessDenied", err)
		}
		if got, ok, err := m.Lookup(red, "r"); !ok || err != nil || got.Text != "red's" {
			t.Errorf("the team's own lookup gave %+v, %v and %v", got, ok, err)
		}
	})
}

// An embedded struct of an exported type is columns, so a scoped column in it
// is held to its scope with and without a database, and is stored.
func TestScopedFieldOfEmbeddedExportedStruct(t *testing.T) {
	type report struct {
		OwnedByTeam
		Text string
	}
	eachBackend(t, func(t *testing.T, DB *struct {
		Reports sql.Map[string, report] `sql:"embedded_exported_reports"`
	}) {
		ctx := testContext(t)
		red, blue := sql.Scope(ctx, "team", "red"), sql.Scope(ctx, "team", "blue")
		if err := DB.Reports.Set(red, "r", report{OwnedByTeam{"red"}, "red's"}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := DB.Reports.Lookup(ctx, "r"); !errors.Is(err, sql.ErrUnauthorized) {
			t.Errorf("a lookup with no scope gave %v, want ErrUnauthorized", err)
		}
		if _, ok, err := DB.Reports.Lookup(blue, "r"); ok || err != nil {
			t.Errorf("another team's lookup gave %v and %v", ok, err)
		}
		if got, ok, err := DB.Reports.Lookup(red, "r"); !ok || err != nil || got.Team != "red" || got.Text != "red's" {
			t.Errorf("the team's own lookup gave %+v, %v and %v", got, ok, err)
		}
	})
}
