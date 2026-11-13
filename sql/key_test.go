package sql_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/sql"
)

type zoneKey struct {
	At   time.Time
	Zone [1]time.Time
}

type zoneRow struct {
	Group string `sql:",scoped"`
}

type zoneTables struct {
	Times  sql.Map[time.Time, string]  `sql:"zone_times"`
	Keys   sql.Map[zoneKey, string]    `sql:"zone_keys"`
	Scoped sql.Map[time.Time, zoneRow] `sql:"zone_scoped"`
}

// A key is only zero if every part of it is, and any other time is a key in
// whatever location it is given: a database finds the row by the same instant
// in another. A Map without one finds a key as Go compares it, as it always has.
func TestTimeKeyInAnotherZone(t *testing.T) {
	east := time.FixedZone("east", 3600)
	t.Run("Database", func(t *testing.T) {
		DB := sql.Open[zoneTables](newMemoryDatabase())
		ctx := testContext(t)
		at := time.Date(2026, 2, 3, 4, 5, 6, 7, east)
		if err := DB.Times.Insert(ctx, at, sql.Create, "kept"); err != nil {
			t.Fatal(err)
		}
		if got, ok, err := DB.Times.Lookup(ctx, at.UTC()); got != "kept" || !ok || err != nil {
			t.Errorf("the same instant in UTC gave %q, %v and %v", got, ok, err)
		}
		// One second past the zero time, and the zero time beside a time that is not.
		for _, key := range []zoneKey{
			{At: time.Time{}.Add(time.Second).In(east)},
			{At: time.Time{}.In(east), Zone: [1]time.Time{at}},
			{Zone: [1]time.Time{time.Time{}.Add(time.Nanosecond)}},
		} {
			if err := DB.Keys.Set(ctx, key, "partly zero"); err != nil {
				t.Errorf("%+v gave %v", key, err)
			}
			if got, ok, err := DB.Keys.Lookup(ctx, key); got != "partly zero" || !ok || err != nil {
				t.Errorf("looking up %+v gave %q, %v and %v", key, got, ok, err)
			}
		}
		if n, err := DB.Keys.Length(ctx); n != 3 || err != nil {
			t.Errorf("the map of composite keys holds %d rows, %v", n, err)
		}
	})
}

type namedUsers struct {
	Users sql.Map[string, account]    `sql:"named_users/UserID"`
	Pairs sql.Map[[2]string, account] `sql:"named_pairs/PairID"`
	Plain sql.Map[string, account]    `sql:"named_plain"`
	Parts sql.Map[zeroKeyPair, int]   `sql:"named_parts/Ignored"`
}

// The table of a Map names its key column after a slash, and the column has
// that name in lower case, as the name of a field is, as it always has.
func TestTableKeyColumnIsLowerCase(t *testing.T) {
	db := newMemoryDatabase()
	DB := sql.Open[namedUsers](db)
	ctx := testContext(t)
	if err := DB.Users.Set(ctx, "a", account{Name: "a", Age: 1}); err != nil {
		t.Fatal(err)
	}
	if err := DB.Pairs.Set(ctx, [2]string{"l", "r"}, account{Name: "pair"}); err != nil {
		t.Fatal(err)
	}
	if err := DB.Plain.Set(ctx, "a", account{Name: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := DB.Parts.Set(ctx, zeroKeyPair{Left: "l", Right: 1}, 1); err != nil {
		t.Fatal(err)
	}
	for table, want := range map[string][]string{
		"named_users": {"userid"},
		"named_pairs": {"pairid1", "pairid2"},
		"named_plain": {"id"},
		"named_parts": {"left", "right"}, // a struct is a key of its own columns.
	} {
		if index, _ := db.columns(table); !reflect.DeepEqual(index, want) {
			t.Errorf("the key columns of %s are %q, want %q", table, index, want)
		}
	}
	if _, value := db.columns("named_users"); !reflect.DeepEqual(value, []string{"name", "age"}) {
		t.Errorf("the value columns of named_users are %q", value)
	}
	// An expression on the key names the same column the rows are stored under.
	if got, ok, err := DB.Users.Lookup(ctx, "a"); got.Age != 1 || !ok || err != nil {
		t.Errorf("lookup gave %+v, %v and %v", got, ok, err)
	}
	if _, ok, err := DB.Users.Lookup(ctx, "b"); ok || err != nil {
		t.Errorf("lookup of a key that is not there gave %v and %v", ok, err)
	}
	if got, ok, err := DB.Pairs.Lookup(ctx, [2]string{"l", "r"}); got.Name != "pair" || !ok || err != nil {
		t.Errorf("lookup of a pair gave %+v, %v and %v", got, ok, err)
	}
	n, err := DB.Pairs.Count(ctx, func(k *[2]string, _ *account) sql.Query {
		return sql.Query{sql.Index(&k[1]).Equals("r"), sql.Order(&k[0]).Increasing()}
	})
	if n != 1 || err != nil {
		t.Errorf("counting by an element of the key gave %d and %v", n, err)
	}
	if ok, err := DB.Users.Delete(ctx, "a", nil); !ok || err != nil {
		t.Errorf("delete gave %v and %v", ok, err)
	}
}

// pickedOrNot is a key with a tenant, which picks itself only when told to.
type pickedOrNot struct {
	Tenant string `sql:"tenant,scoped"`
	ID     int
}

type unpickedTables struct {
	Docs sql.Map[pickedOrNot, string] `sql:"unpicked_docs"`
}

// Add needs a key for a Map whose key it cannot number, and says so with
// ErrInsertOnly whether or not the key has a scoped column: the zero key is
// not one that was picked and refused, it is the lack of one.
func TestAddWithoutKeyInScopeIsInsertOnly(t *testing.T) {
	t.Run("RAM", func(t *testing.T) {
		DB := sql.Open[unpickedTables](sql.New())
		ctx := testContext(t)
		if key, err := DB.Docs.Add(sql.Scope(ctx, "tenant", "a"), "doc"); !errors.Is(err, sql.ErrInsertOnly) || key != (pickedOrNot{}) {
			t.Errorf("add gave %+v and %v, want ErrInsertOnly", key, err)
		}
		if n, err := DB.Docs.Length(sql.Admin(ctx, "checking")); n != 0 || err != nil {
			t.Errorf("the map holds %d rows, %v", n, err)
		}
	})
	for name, generates := range map[string]bool{"Database": false, "Database that generates keys": true} {
		t.Run(name, func(t *testing.T) {
			db := newMemoryDatabase()
			db.generatesKeys = generates
			DB := sql.Open[unpickedTables](db)
			ctx := testContext(t)
			if key, err := DB.Docs.Add(sql.Scope(ctx, "tenant", "a"), "doc"); !errors.Is(err, sql.ErrInsertOnly) || key != (pickedOrNot{}) {
				t.Errorf("add gave %+v and %v, want ErrInsertOnly", key, err)
			}
			db.settled()
			// The tenant of a key is not for a database to pick, so it is not asked.
			if asked := db.commits.Load() + db.rollbacks.Load(); asked != 0 || db.rowCount() != 0 {
				t.Errorf("the database ran %d transactions and holds %d rows", asked, db.rowCount())
			}
			if generates {
				return // the keys this database generates are numbers, not tenants.
			}
			// With every tenant in scope, it is the database that says whether a key is needed.
			key, err := DB.Docs.Add(sql.Admin(ctx, "testing"), "doc")
			db.settled()
			if !errors.Is(err, sql.ErrInsertOnly) || db.rowCount() != 0 || db.rollbacks.Load() != 1 {
				t.Errorf("add without a key gave %+v and %v, with %d rows after %d rollbacks", key, err, db.rowCount(), db.rollbacks.Load())
			}
		})
	}
}
