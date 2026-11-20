package sql_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/sql"
)

// tenantKey is the composite key a tenant column usually lives in.
type tenantKey struct {
	Tenant string `sql:"tenant,scoped"`
	ID     int
}

type tenantDoc struct {
	Text string
	Hits int
}

type tenantTables struct {
	Docs sql.Map[tenantKey, tenantDoc] `sql:"tenant_docs"`
}

func tenantDocs(t *testing.T, ctx context.Context, DB *tenantTables) map[tenantKey]string {
	t.Helper()
	var issue error
	found := make(map[tenantKey]string)
	for key, doc := range DB.Docs.Search(ctx, nil, &issue) {
		found[key] = doc.Text
	}
	if issue != nil {
		t.Fatal(issue)
	}
	return found
}

// A scoped column of the key is enforced like one of the value, by every
// operation and on both backends.
func TestScopedKeyColumn(t *testing.T) {
	a1, a2, b1 := tenantKey{"a", 1}, tenantKey{"a", 2}, tenantKey{"b", 1}
	all := func(*tenantKey, *tenantDoc) sql.Query { return sql.Query{sql.Slice(0, 10)} }
	touch := func(doc *tenantDoc) sql.Patch { return sql.Patch{sql.Set(&doc.Text, "touched")} }
	eachBackend(t, func(t *testing.T, DB *tenantTables) {
		ctx := testContext(t)
		admin := sql.Admin(ctx, "setup")
		for _, key := range []tenantKey{a1, a2, b1} {
			if err := DB.Docs.Insert(admin, key, sql.Create, tenantDoc{Text: "of " + key.Tenant, Hits: 1}); err != nil {
				t.Fatal(err)
			}
		}
		a := sql.Scope(ctx, "tenant", "a")

		t.Run("no scope", func(t *testing.T) {
			if n, err := DB.Docs.Count(ctx, nil); !errors.Is(err, sql.ErrUnauthorized) {
				t.Errorf("count gave %d and %v, want ErrUnauthorized", n, err)
			}
			if _, _, err := DB.Docs.Lookup(ctx, a1); !errors.Is(err, sql.ErrUnauthorized) {
				t.Errorf("lookup gave %v, want ErrUnauthorized", err)
			}
			if err := DB.Docs.Set(ctx, a1, tenantDoc{}); !errors.Is(err, sql.ErrUnauthorized) {
				t.Errorf("set gave %v, want ErrUnauthorized", err)
			}
			if _, err := DB.Docs.Add(ctx, tenantDoc{}); !errors.Is(err, sql.ErrUnauthorized) {
				t.Errorf("add gave %v, want ErrUnauthorized", err)
			}
			if _, err := DB.Docs.Delete(ctx, a1, nil); !errors.Is(err, sql.ErrUnauthorized) {
				t.Errorf("delete gave %v, want ErrUnauthorized", err)
			}
			if _, err := DB.Docs.Mutate(ctx, a1, nil, touch); !errors.Is(err, sql.ErrUnauthorized) {
				t.Errorf("mutate gave %v, want ErrUnauthorized", err)
			}
			if _, err := DB.Docs.Update(ctx, all, touch); !errors.Is(err, sql.ErrUnauthorized) {
				t.Errorf("update gave %v, want ErrUnauthorized", err)
			}
			if _, err := DB.Docs.UnsafeDelete(ctx, all); !errors.Is(err, sql.ErrUnauthorized) {
				t.Errorf("unsafe delete gave %v, want ErrUnauthorized", err)
			}
		})
		t.Run("reads", func(t *testing.T) {
			if n, err := DB.Docs.Count(a, nil); n != 2 || err != nil {
				t.Errorf("count gave %d and %v, want 2", n, err)
			}
			if found := tenantDocs(t, a, DB); len(found) != 2 || found[a1] == "" || found[a2] == "" {
				t.Errorf("search found %v", found)
			}
			if doc, ok, err := DB.Docs.Lookup(a, b1); ok || err != nil {
				t.Errorf("lookup of another tenant's key gave %+v, %v and %v", doc, ok, err)
			}
			if doc, err := DB.Docs.Get(a, b1); doc != (tenantDoc{}) || err != nil {
				t.Errorf("get of another tenant's key gave %+v and %v", doc, err)
			}
			if doc, ok, err := DB.Docs.Lookup(a, a1); !ok || err != nil || doc.Text != "of a" {
				t.Errorf("lookup of its own key gave %+v, %v and %v", doc, ok, err)
			}
			if key, _, ok, err := DB.Docs.First(a, func(k *tenantKey, _ *tenantDoc) sql.Query {
				return sql.Query{sql.Index(&k.Tenant).Equals("b")}
			}); ok || err != nil {
				t.Errorf("a query for another tenant found %v, %v", key, err)
			}
			var hits int
			err := DB.Docs.Output(a, nil, func(_ *tenantKey, doc *tenantDoc) sql.Stats {
				return sql.Stats{sql.Sum(&doc.Hits, &hits)}
			})
			if hits != 2 || err != nil {
				t.Errorf("sum gave %d and %v, want 2", hits, err)
			}
		})
		t.Run("writes", func(t *testing.T) {
			if err := DB.Docs.Insert(a, b1, sql.Upsert, tenantDoc{Text: "taken"}); !errors.Is(err, sql.ErrAccessDenied) {
				t.Errorf("upsert over another tenant's row gave %v, want ErrAccessDenied", err)
			}
			if err := DB.Docs.Insert(a, tenantKey{"b", 9}, sql.Create, tenantDoc{Text: "planted"}); !errors.Is(err, sql.ErrAccessDenied) {
				t.Errorf("create at another tenant's key gave %v, want ErrAccessDenied", err)
			}
			if ok, err := DB.Docs.Mutate(a, b1, nil, touch); ok || err != nil {
				t.Errorf("mutate of another tenant's row gave %v and %v", ok, err)
			}
			if ok, err := DB.Docs.Delete(a, b1, nil); ok || err != nil {
				t.Errorf("delete of another tenant's row gave %v and %v", ok, err)
			}
			if n, err := DB.Docs.Update(a, all, touch); n != 2 || err != nil {
				t.Errorf("update gave %d and %v, want 2", n, err)
			}
			if ok, err := DB.Docs.Mutate(a, a1, nil, func(doc *tenantDoc) sql.Patch {
				return sql.Patch{sql.Set(&doc.Hits, 5)}
			}); !ok || err != nil {
				t.Errorf("mutate of its own row gave %v and %v", ok, err)
			}
			if err := DB.Docs.Insert(a, a2, sql.Upsert, tenantDoc{Text: "replaced"}); err != nil {
				t.Errorf("upsert of its own row gave %v", err)
			}
			want := map[tenantKey]string{a1: "touched", a2: "replaced", b1: "of b"}
			if found := tenantDocs(t, admin, DB); len(found) != len(want) || found[a1] != want[a1] || found[a2] != want[a2] || found[b1] != want[b1] {
				t.Errorf("the table holds %v, want %v", found, want)
			}
			if n, err := DB.Docs.UnsafeDelete(a, all); n != 2 || err != nil {
				t.Errorf("unsafe delete gave %d and %v, want 2", n, err)
			}
			if found := tenantDocs(t, admin, DB); len(found) != 1 || found[b1] != "of b" {
				t.Errorf("after tenant a deleted everything it can see, the table holds %v", found)
			}
		})
		t.Run("other access", func(t *testing.T) {
			if n, err := DB.Docs.Count(sql.Widen[string](ctx, "tenant"), nil); n != 1 || err != nil {
				t.Errorf("a widened count gave %d and %v", n, err)
			}
			if n, err := DB.Docs.Count(sql.Scope[string](ctx, "tenant"), nil); n != 0 || err != nil {
				t.Errorf("a count in a scope of nothing gave %d and %v", n, err)
			}
			if _, err := DB.Docs.Count(sql.Scope(ctx, "tenant", 1), nil); err == nil {
				t.Error("a scope held as an int was accepted for a string column")
			}
		})
	})
}

// pickedKey is a key type that picks its own keys, in the tenant set here.
type pickedKey struct {
	Tenant string `sql:"tenant,scoped"`
	ID     int
}

func (key *pickedKey) Randomize() { *key = pickedKey{Tenant: "b", ID: 7} }

type pickedTables struct {
	Docs sql.Map[pickedKey, string] `sql:"picked_docs"`
}

// Add has no key to check until one has been picked, and must not keep a row
// at one that is out of scope.
func TestScopedKeyColumnAdd(t *testing.T) {
	eachBackend(t, func(t *testing.T, DB *pickedTables) {
		ctx := testContext(t)
		if key, err := DB.Docs.Add(sql.Scope(ctx, "tenant", "a"), "planted"); !errors.Is(err, sql.ErrAccessDenied) {
			t.Errorf("add at a key of another tenant gave %v and %v, want ErrAccessDenied", key, err)
		}
		if n, err := DB.Docs.Length(sql.Admin(ctx, "checking")); n != 0 || err != nil {
			t.Errorf("a refused add left %d rows, %v", n, err)
		}
		key, err := DB.Docs.Add(sql.Scope(ctx, "tenant", "b"), "kept")
		if err != nil || key != (pickedKey{Tenant: "b", ID: 7}) {
			t.Fatalf("add in scope gave %v and %v", key, err)
		}
		if got, err := DB.Docs.Get(sql.Scope(ctx, "tenant", "b"), key); got != "kept" || err != nil {
			t.Errorf("the added row reads as %q, %v", got, err)
		}
	})
}

// A scope can be held by a column of the key and one of the value at once.
func TestScopedKeyAndValueColumns(t *testing.T) {
	type row struct {
		Text string
		Team string `sql:"team,scoped"`
	}
	type tables struct {
		Rows sql.Map[tenantKey, row] `sql:"tenant_team_rows"`
	}
	eachBackend(t, func(t *testing.T, DB *tables) {
		ctx := testContext(t)
		admin := sql.Admin(ctx, "setup")
		for key, team := range map[tenantKey]string{{"a", 1}: "red", {"a", 2}: "blue", {"b", 1}: "red"} {
			if err := DB.Rows.Insert(admin, key, sql.Create, row{Text: "t", Team: team}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := DB.Rows.Count(sql.Scope(ctx, "tenant", "a"), nil); !errors.Is(err, sql.ErrUnauthorized) {
			t.Errorf("a count with one of two scopes gave %v, want ErrUnauthorized", err)
		}
		both := sql.Scope(sql.Scope(ctx, "tenant", "a"), "team", "red")
		if n, err := DB.Rows.Count(both, nil); n != 1 || err != nil {
			t.Errorf("count gave %d and %v, want 1", n, err)
		}
		if _, ok, err := DB.Rows.Lookup(both, tenantKey{"a", 2}); ok || err != nil {
			t.Errorf("a row of another team was found: %v, %v", ok, err)
		}
		if _, ok, err := DB.Rows.Lookup(both, tenantKey{"b", 1}); ok || err != nil {
			t.Errorf("a row of another tenant was found: %v, %v", ok, err)
		}
		if err := DB.Rows.Set(both, tenantKey{"a", 3}, row{Team: "blue"}); !errors.Is(err, sql.ErrAccessDenied) {
			t.Errorf("a row of another team was stored: %v", err)
		}
		if err := DB.Rows.Set(both, tenantKey{"a", 3}, row{Team: "red"}); err != nil {
			t.Errorf("a row in both scopes was refused: %v", err)
		}
	})
}

// A patch that names a column of the key rewrites the key of the row, which
// moves it: onto a key that may be another row's, and one that no scope
// grants. It is refused, even when the value it would move the key onto is
// one that the scope holds.
func TestScopedPatchCannotMoveAKey(t *testing.T) {
	ctx := testContext(t)
	DB := sql.Open[tenantTables](newMemoryDatabase())
	admin := sql.Admin(ctx, "setup")
	a1, b1 := tenantKey{"a", 1}, tenantKey{"b", 1}
	for key, text := range map[tenantKey]string{a1: "of a", b1: "of b"} {
		if err := DB.Docs.Insert(admin, key, sql.Create, tenantDoc{Text: text}); err != nil {
			t.Fatal(err)
		}
	}
	both := sql.Scope(ctx, "tenant", "a", "b")
	for _, tenant := range []string{"a", "b"} {
		var index *tenantKey
		n, err := DB.Docs.Update(both, func(k *tenantKey, _ *tenantDoc) sql.Query {
			// The first call is made with the sentinal of the key, which a
			// patch can name a column of; the ones after it are made with
			// keys that are not it, and are not the one to keep.
			if index == nil {
				index = k
			}
			return sql.Query{sql.Slice(0, 10)}
		}, func(_ *tenantDoc) sql.Patch {
			return sql.Patch{sql.Set(&index.Tenant, tenant)}
		})
		if n != 0 || !errors.Is(err, sql.ErrAccessDenied) {
			t.Errorf("a patch onto tenant %q gave %d and %v, want it refused", tenant, n, err)
		}
	}
	if found := tenantDocs(t, admin, DB); len(found) != 2 || found[a1] != "of a" || found[b1] != "of b" {
		t.Errorf("the rows were left as %v", found)
	}
}

// A tag that means the scoped option but does not say exactly that must not
// leave its column open: the Map refuses to work, for any context.
func TestUnknownTagOptionIsRefused(t *testing.T) {
	type spaced struct {
		Text string
		Team string `sql:"team, scoped"`
	}
	type capital struct {
		Team string `sql:"team,Scoped"`
	}
	type nested struct {
		Inner [2]struct{ Owner spaced }
	}
	type key struct {
		Tenant string `sql:"tenant, scoped"`
		ID     int
	}
	ctx := testContext(t)
	refused := func(t *testing.T, option string, err error) {
		t.Helper()
		if err == nil || errors.Is(err, sql.ErrUnauthorized) || errors.Is(err, sql.ErrAccessDenied) {
			t.Errorf("got %v, want the tag to be refused", err)
			return
		}
		if !strings.Contains(err.Error(), `"`+option+`"`) {
			t.Errorf("the error does not name the option %q: %v", option, err)
		}
	}
	check := func(t *testing.T, option string, use func(context.Context) error) {
		t.Helper()
		for name, ctx := range map[string]context.Context{
			"no scope": ctx,
			"scoped":   sql.Scope(sql.Scope(ctx, "team", "red"), "tenant", "a"),
			"admin":    sql.Admin(ctx, "testing"),
		} {
			t.Run(name, func(t *testing.T) { refused(t, option, use(ctx)) })
		}
	}
	t.Run("space", func(t *testing.T) {
		eachBackend(t, func(t *testing.T, DB *struct {
			Rows sql.Map[int, spaced] `sql:"spaced_rows"`
		}) {
			all := func(*int, *spaced) sql.Query { return sql.Query{sql.Slice(0, 10)} }
			patch := func(row *spaced) sql.Patch { return sql.Patch{sql.Set(&row.Text, "x")} }
			for name, use := range map[string]func(context.Context) error{
				"Insert": func(ctx context.Context) error { return DB.Rows.Insert(ctx, 1, sql.Create, spaced{Team: "blue"}) },
				"Set":    func(ctx context.Context) error { return DB.Rows.Set(ctx, 1, spaced{Team: "blue"}) },
				"Add":    func(ctx context.Context) error { _, err := DB.Rows.Add(ctx, spaced{}); return err },
				"Lookup": func(ctx context.Context) error { _, _, err := DB.Rows.Lookup(ctx, 1); return err },
				"Get":    func(ctx context.Context) error { _, err := DB.Rows.Get(ctx, 1); return err },
				"First":  func(ctx context.Context) error { _, _, _, err := DB.Rows.First(ctx, nil); return err },
				"Count":  func(ctx context.Context) error { _, err := DB.Rows.Count(ctx, nil); return err },
				"Length": func(ctx context.Context) error { _, err := DB.Rows.Length(ctx); return err },
				"Search": func(ctx context.Context) error {
					var issue error
					for range DB.Rows.Search(ctx, nil, &issue) {
					}
					return issue
				},
				"Update":       func(ctx context.Context) error { _, err := DB.Rows.Update(ctx, all, patch); return err },
				"Mutate":       func(ctx context.Context) error { _, err := DB.Rows.Mutate(ctx, 1, nil, patch); return err },
				"Delete":       func(ctx context.Context) error { _, err := DB.Rows.Delete(ctx, 1, nil); return err },
				"UnsafeDelete": func(ctx context.Context) error { _, err := DB.Rows.UnsafeDelete(ctx, all); return err },
			} {
				t.Run(name, func(t *testing.T) { check(t, " scoped", use) })
			}
		})
	})
	t.Run("capital", func(t *testing.T) {
		var m sql.Map[int, capital]
		check(t, "Scoped", func(ctx context.Context) error { _, err := m.Length(ctx); return err })
	})
	t.Run("nested", func(t *testing.T) {
		var m sql.Map[int, nested]
		check(t, " scoped", func(ctx context.Context) error { _, err := m.Length(ctx); return err })
	})
	t.Run("key", func(t *testing.T) {
		eachBackend(t, func(t *testing.T, DB *struct {
			Rows sql.Map[key, string] `sql:"spaced_keys"`
		}) {
			check(t, " scoped", func(ctx context.Context) error { return DB.Rows.Set(ctx, key{"b", 1}, "x") })
			check(t, " scoped", func(ctx context.Context) error { _, err := DB.Rows.Length(ctx); return err })
		})
	})
}

// A tag without options, the scoped option written as documented, and options
// that are nothing like it, still work: those are left alone, as they always
// have been.
func TestKnownTagOptionsAreAccepted(t *testing.T) {
	type row struct {
		Plain  string `sql:"plain"`
		Team   string `sql:"team,scoped"`
		Region string `sql:",scoped"`
		Twice  string `sql:"twice,scoped,scoped"`
		Other  string `sql:"other,omitempty"`
		Extra  string `sql:"extra,scoped,indexed"`
		Empty  string `sql:"empty,"`
		Short  string `sql:"short,scope"`
	}
	eachBackend(t, func(t *testing.T, DB *struct {
		Rows sql.Map[int, row] `sql:"known_options"`
	}) {
		ctx := sql.Scope(sql.Scope(sql.Scope(sql.Scope(testContext(t), "team", "red"), "region", "north"), "twice", "x"), "extra", "e")
		stored := row{Plain: "p", Team: "red", Region: "north", Twice: "x", Other: "o", Extra: "e", Empty: "m", Short: "s"}
		if err := DB.Rows.Set(ctx, 1, stored); err != nil {
			t.Fatal(err)
		}
		if got, err := DB.Rows.Get(ctx, 1); got != stored || err != nil {
			t.Errorf("got %+v and %v", got, err)
		}
		if _, ok, err := DB.Rows.Lookup(sql.Scope(ctx, "twice", "y"), 1); ok || err != nil {
			t.Errorf("a row outside of the scope was found: %v, %v", ok, err)
		}
	})
}

type ownedRow struct {
	Text string
	Team string `sql:",scoped"`
}

type ownedTables struct {
	Rows sql.Map[int, ownedRow] `sql:"owned_rows"`
}

// A patch that moves a value out of the scope is refused with and without a
// database, and nothing is refused when there was no value to move. With a
// database nothing is reported as updated. Without one, the value is left as
// it was and counted, as it always has been.
func TestDeniedPatchAgrees(t *testing.T) {
	type outcome struct {
		n      int
		ok     bool
		denied bool
		other  error
	}
	report := func(n int, ok bool, err error) outcome {
		if errors.Is(err, sql.ErrAccessDenied) {
			return outcome{n: n, ok: ok, denied: true}
		}
		return outcome{n: n, ok: ok, other: err}
	}
	all := func(*int, *ownedRow) sql.Query { return sql.Query{sql.Slice(0, 10)} }
	none := func(k *int, _ *ownedRow) sql.Query { return sql.Query{sql.Index(k).Equals(99), sql.Slice(0, 10)} }
	steal := func(row *ownedRow) sql.Patch { return sql.Patch{sql.Set(&row.Team, "blue")} }
	assign := func(row *ownedRow) sql.Patch { row.Team = "blue"; return nil }
	stay := func(row *ownedRow) sql.Patch { return sql.Patch{sql.Set(&row.Team, "red"), sql.Set(&row.Text, "kept")} }
	want := map[string]outcome{
		"Update of rows":              {denied: true},
		"Update of rows by assigning": {denied: true},
		"Update of no rows":           {},
		"Update of a missing key":     {},
		"Update that stays":           {n: 2},
		"Mutate":                      {denied: true},
		"Mutate by assigning":         {denied: true},
		"Mutate of a missing key":     {},
		"Mutate that stays":           {ok: true},
	}
	wantRAM := map[string]outcome{
		"Update of rows":              {n: 2, denied: true},
		"Update of rows by assigning": {n: 2, denied: true},
		"Mutate":                      {ok: true, denied: true},
		"Mutate by assigning":         {ok: true, denied: true},
	}
	eachBackend(t, func(t *testing.T, DB *ownedTables) {
		want := maps.Clone(want)
		if strings.HasSuffix(t.Name(), "/RAM") {
			maps.Copy(want, wantRAM)
		}
		ctx := testContext(t)
		red := sql.Scope(ctx, "team", "red")
		for key := 1; key <= 2; key++ {
			if err := DB.Rows.Set(red, key, ownedRow{Text: "mine", Team: "red"}); err != nil {
				t.Fatal(err)
			}
		}
		got := make(map[string]outcome)
		n, err := DB.Rows.Update(red, all, steal)
		got["Update of rows"] = report(n, false, err)
		n, err = DB.Rows.Update(red, all, assign)
		got["Update of rows by assigning"] = report(n, false, err)
		n, err = DB.Rows.Update(red, none, steal)
		got["Update of a missing key"] = report(n, false, err)
		n, err = DB.Rows.Update(sql.Scope(red, "team", "green"), all, steal)
		got["Update of no rows"] = report(n, false, err)
		ok, err := DB.Rows.Mutate(red, 1, nil, steal)
		got["Mutate"] = report(0, ok, err)
		ok, err = DB.Rows.Mutate(red, 1, nil, assign)
		got["Mutate by assigning"] = report(0, ok, err)
		ok, err = DB.Rows.Mutate(red, 99, nil, steal)
		got["Mutate of a missing key"] = report(0, ok, err)
		for key := 1; key <= 2; key++ {
			if row, err := DB.Rows.Get(red, key); row != (ownedRow{Text: "mine", Team: "red"}) || err != nil {
				t.Errorf("a refused patch left row %d as %+v, %v", key, row, err)
			}
		}
		ok, err = DB.Rows.Mutate(red, 1, nil, stay)
		got["Mutate that stays"] = report(0, ok, err)
		n, err = DB.Rows.Update(red, all, stay)
		got["Update that stays"] = report(n, false, err)
		names := make([]string, 0, len(want))
		for name := range want {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			if got[name] != want[name] {
				t.Errorf("%s gave %+v, want %+v", name, got[name], want[name])
			}
		}
	})
}

// Without a database a patch can depend on the value, so it can leave the
// scope for one row only. That row is left as it was, and the update goes on.
func TestRAMDeniedPatchIsLeftOut(t *testing.T) {
	ctx := sql.Scope(testContext(t), "team", "red")
	var m sql.Map[int, ownedRow]
	for key, text := range map[int]string{1: "stays", 2: "leaves"} {
		if err := m.Set(ctx, key, ownedRow{Text: text, Team: "red"}); err != nil {
			t.Fatal(err)
		}
	}
	n, err := m.Update(ctx, func(_ *int, row *ownedRow) sql.Query {
		return sql.Query{sql.Order(&row.Text).Decreasing(), sql.Slice(0, 10)}
	}, func(row *ownedRow) sql.Patch {
		if row.Text == "leaves" {
			row.Team = "blue"
		}
		row.Text += " patched"
		return nil
	})
	if n != 2 || !errors.Is(err, sql.ErrAccessDenied) {
		t.Errorf("update gave %d and %v, want both rows counted and ErrAccessDenied", n, err)
	}
	if row, err := m.Get(ctx, 1); row.Text != "stays patched" || err != nil {
		t.Errorf("the row patched before the refusal is %+v, %v", row, err)
	}
	if row, err := m.Get(ctx, 2); row != (ownedRow{Text: "leaves", Team: "red"}) || err != nil {
		t.Errorf("the row whose patch was refused is %+v, %v", row, err)
	}
}
