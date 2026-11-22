package sql_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/iqhive/runtime.link/sql"
	"github.com/iqhive/runtime.link/sql/std/sodium"
	"github.com/iqhive/runtime.link/xyz"
)

// writeBackends runs check against a map without a database, one with a
// database that answers its commits, and one with a database that commits when
// its channel is closed.
func writeBackends(t *testing.T, check func(t *testing.T, DB *bank)) {
	t.Helper()
	t.Run("RAM", func(t *testing.T) { check(t, sql.Open[bank](sql.New())) })
	t.Run("Database", func(t *testing.T) { check(t, sql.Open[bank](newMemoryDatabase())) })
	t.Run("Database that commits on close", func(t *testing.T) {
		db := newMemoryDatabase()
		db.commitsOnClose = true
		check(t, sql.Open[bank](db))
	})
}

// Mutate and Update read a value, patch it here and write it back. Two of them
// at once on one row must not both write what they made of the value they both
// read: each increment that is reported as made is in the value.
func TestConcurrentMutatesAndUpdatesLoseNothing(t *testing.T) {
	const workers, each = 4, 25
	older := func(a *account) sql.Patch { a.Age++; return nil }
	writes := map[string]func(ctx context.Context, DB *bank) error{
		"Mutate": func(ctx context.Context, DB *bank) error {
			for {
				ok, err := DB.Accounts.Mutate(ctx, "a", nil, older)
				if err == sql.ErrWriteConflict && !ok {
					continue // told, and nothing was written.
				}
				if err == nil && !ok {
					return errors.New("mutate found no value")
				}
				return err
			}
		},
		"Update": func(ctx context.Context, DB *bank) error {
			n, err := DB.Accounts.Update(ctx, everyAccount, older)
			if err == nil && n != 1 {
				return fmt.Errorf("update patched %d values", n)
			}
			return err
		},
	}
	for name, write := range writes {
		t.Run(name, func(t *testing.T) {
			writeBackends(t, func(t *testing.T, DB *bank) {
				ctx := testContext(t)
				if err := DB.Accounts.Set(ctx, "a", account{Name: "a"}); err != nil {
					t.Fatal(err)
				}
				var running sync.WaitGroup
				failures := make([]error, workers)
				for i := range workers {
					running.Add(1)
					go func() {
						defer running.Done()
						for range each {
							failures[i] = errors.Join(failures[i], write(ctx, DB))
						}
					}()
				}
				running.Wait()
				if err := errors.Join(failures...); err != nil {
					t.Fatal(err)
				}
				got, err := DB.Accounts.Get(ctx, "a")
				if err != nil {
					t.Fatal(err)
				}
				if got.Age != workers*each {
					t.Errorf("%d increments were each reported as made, and the value was incremented %d times", workers*each, got.Age)
				}
			})
		})
	}
}

// meanwhile is a patch that makes another write the first time it runs, after
// the value it is given was read and before what it makes of it is written.
func meanwhile(t *testing.T, runs *int, other func() error, patch sql.PatchFunc[account]) sql.PatchFunc[account] {
	return func(a *account) sql.Patch {
		*runs++
		if *runs == 1 {
			if err := other(); err != nil {
				t.Errorf("the write made in between gave %v", err)
			}
		}
		return patch(a)
	}
}

// A mutation that would write over a write made since it read the row is not
// made, with or without a database: it says so, and the other write is kept.
func TestMutateReportsWriteMadeSinceItRead(t *testing.T) {
	writeBackends(t, func(t *testing.T, DB *bank) {
		ctx := testContext(t)
		if err := DB.Accounts.Set(ctx, "a", account{Name: "a", Age: 1}); err != nil {
			t.Fatal(err)
		}
		runs := 0
		other := func() error { return DB.Accounts.Set(ctx, "a", account{Name: "a", Age: 50}) }
		ok, err := DB.Accounts.Mutate(ctx, "a", nil, meanwhile(t, &runs, other, func(a *account) sql.Patch { a.Age++; return nil }))
		if ok || err != sql.ErrWriteConflict {
			t.Errorf("mutate gave %v and %v, want false and ErrWriteConflict as it is", ok, err)
		}
		if runs != 1 {
			t.Errorf("the patch ran %d times, want once", runs)
		}
		if got, err := DB.Accounts.Get(ctx, "a"); got != (account{Name: "a", Age: 50}) || err != nil {
			t.Errorf("the row is %+v, %v: want the write made in between", got, err)
		}
	})
}

// With a database, only the columns that a mutation writes have to hold what
// was read. A write to another column in between is kept beside it, and a row
// that was deleted in between was not mutated, which is no error.
func TestMutateBesideWriteThatItDoesNotWriteOver(t *testing.T) {
	older := func(a *account) sql.Patch { a.Age++; return nil }
	t.Run("another column", func(t *testing.T) {
		ctx := testContext(t)
		DB := sql.Open[bank](newMemoryDatabase())
		if err := DB.Accounts.Set(ctx, "a", account{Name: "a", Age: 1}); err != nil {
			t.Fatal(err)
		}
		runs := 0
		other := func() error { return DB.Accounts.Set(ctx, "a", account{Name: "renamed", Age: 1}) }
		if ok, err := DB.Accounts.Mutate(ctx, "a", nil, meanwhile(t, &runs, other, older)); !ok || err != nil {
			t.Errorf("mutate gave %v and %v", ok, err)
		}
		if got, err := DB.Accounts.Get(ctx, "a"); got != (account{Name: "renamed", Age: 2}) || err != nil {
			t.Errorf("the row is %+v, %v: want both writes", got, err)
		}
	})
	t.Run("deleted", func(t *testing.T) {
		ctx := testContext(t)
		DB := sql.Open[bank](newMemoryDatabase())
		if err := DB.Accounts.Set(ctx, "a", account{Name: "a", Age: 1}); err != nil {
			t.Fatal(err)
		}
		runs := 0
		other := func() error { _, err := DB.Accounts.Delete(ctx, "a", nil); return err }
		if ok, err := DB.Accounts.Mutate(ctx, "a", nil, meanwhile(t, &runs, other, older)); ok || err != nil {
			t.Errorf("mutate gave %v and %v, want false and no error", ok, err)
		}
		if _, found, err := DB.Accounts.Lookup(ctx, "a"); found || err != nil {
			t.Errorf("the deleted row gave %v and %v", found, err)
		}
	})
}

// An update of its own that would write over a write made since it read the
// rows is made again from what they hold now, with or without a database.
func TestUpdateIsMadeAgainWhenARowChanged(t *testing.T) {
	writeBackends(t, func(t *testing.T, DB *bank) {
		ctx := testContext(t)
		for _, name := range []string{"a", "b"} {
			if err := DB.Accounts.Set(ctx, name, account{Name: name, Age: 1}); err != nil {
				t.Fatal(err)
			}
		}
		runs := 0
		other := func() error { return DB.Accounts.Set(ctx, "b", account{Name: "b", Age: 50}) }
		n, err := DB.Accounts.Update(ctx, everyAccount, meanwhile(t, &runs, other, func(a *account) sql.Patch { a.Age++; return nil }))
		if n != 2 || err != nil {
			t.Fatalf("update gave %d and %v", n, err)
		}
		// Each row was patched once from what it held last, however many
		// times the patch ran: a row written before the one that changed
		// was not left patched and then patched again.
		want := map[string]account{"a": {Name: "a", Age: 2}, "b": {Name: "b", Age: 51}}
		if got := accounts(t, DB); !maps.Equal(got, want) {
			t.Errorf("the rows are %+v, want %+v", got, want)
		}
	})
}

// An update that joined a transaction cannot be made again here, as what the
// transaction read and wrote before it is the caller's: it fails, the other
// write is kept, and rolling back is up to the caller.
func TestUpdateInsideTransactionReportsWriteConflict(t *testing.T) {
	ctx := testContext(t)
	db := newMemoryDatabase()
	DB := sql.Open[bank](db)
	if err := DB.Accounts.Set(ctx, "a", account{Name: "a", Age: 1}); err != nil {
		t.Fatal(err)
	}
	runs := 0
	other := func() error { return DB.Accounts.Set(ctx, "a", account{Name: "a", Age: 50}) }
	var n int
	err := DB.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
		var err error
		n, err = DB.Accounts.Update(ctx, everyAccount, meanwhile(t, &runs, other, func(a *account) sql.Patch { a.Age++; return nil }))
		return err
	})
	if n != 0 || err != sql.ErrWriteConflict {
		t.Errorf("the update gave %d and %v, want ErrWriteConflict as it is", n, err)
	}
	if runs != 1 {
		t.Errorf("the patch ran %d times, want once", runs)
	}
	if got, err := DB.Accounts.Get(ctx, "a"); got != (account{Name: "a", Age: 50}) || err != nil {
		t.Errorf("the row is %+v, %v: want the write made in between", got, err)
	}
}

// updateRecorder keeps the query of every update it is sent. If it is picky,
// it finds no value of a column equal to the one it is sent, as a database does
// for a column without a value that it reads as zero.
type updateRecorder struct {
	*memoryDatabase
	picky   bool
	queries *[]sodium.Query
}

func (db updateRecorder) Update(table sodium.Table, query sodium.Query, patch sodium.Patch) sodium.Job {
	*db.queries = append(*db.queries, query)
	if db.picky && len(valuesAskedFor(table, query)) > 0 {
		return newMemoryWrite(func(context.Context) (int, error) { return 0, nil })
	}
	return db.memoryDatabase.Update(table, query, patch)
}

// valuesAskedFor are the columns of the value of a table that a query asks to
// be equal to something, with what it asks them to be equal to.
func valuesAskedFor(table sodium.Table, query sodium.Query) []string {
	var asked []string
	for _, expr := range query {
		switch xyz.ValueOf(expr) {
		case sodium.Expressions.Group:
			asked = append(asked, valuesAskedFor(table, sodium.Expressions.Group.Get(expr))...)
		case sodium.Expressions.Index:
			column, value := sodium.Expressions.Index.Get(expr).Split()
			if slices.ContainsFunc(table.Value, func(c sodium.Column) bool { return c.Name == column.Name }) {
				asked = append(asked, column.Name+"="+value.String())
			}
		}
	}
	return asked
}

// What is asked of the written columns is asked the way a query asks for a
// column to equal a value, so a database is sent nothing that a query with
// sql.Index does not already send it.
func TestWriteBackAsksAsIndexDoes(t *testing.T) {
	ctx := testContext(t)
	var queries []sodium.Query
	DB := sql.Open[bank](updateRecorder{newMemoryDatabase(), false, &queries})
	if err := DB.Accounts.Set(ctx, "a", account{Name: "a", Age: 1}); err != nil {
		t.Fatal(err)
	}
	var asked sodium.Expression
	var issue error
	for range DB.Accounts.Search(ctx, func(_ *string, a *account) sql.Query {
		asked = sql.Index(&a.Age).Equals(1)
		return sql.Query{asked}
	}, &issue) {
	}
	if issue != nil {
		t.Fatal(issue)
	}
	if ok, err := DB.Accounts.Mutate(ctx, "a", nil, func(a *account) sql.Patch { a.Age++; return nil }); !ok || err != nil {
		t.Fatalf("mutate gave %v and %v", ok, err)
	}
	if len(queries) != 1 || !slices.ContainsFunc(queries[0], func(expr sodium.Expression) bool { return reflect.DeepEqual(expr, asked) }) {
		t.Errorf("the update was sent %v, which does not hold %v", queries, asked)
	}
}

// A row is written back by its key, as it always was, and now only if the
// columns being written still hold what was read. The other columns are not
// asked about: a write to them is not written over.
func TestWriteBackAsksForWrittenColumnsToHoldWhatWasRead(t *testing.T) {
	byHand := sodium.Modifications.Set.As(xyz.NewPair(sodium.Column{Name: "age"}, sodium.Values.Int64.As(7)))
	unknown := sodium.Modifications.Set.As(xyz.NewPair(sodium.Column{Name: "elsewhere"}, sodium.Values.Int64.As(7)))
	for name, test := range map[string]struct {
		patch sql.PatchFunc[account]
		want  []string
	}{
		"assigned":             {func(a *account) sql.Patch { a.Age++; return nil }, []string{"age=1"}},
		"set":                  {func(a *account) sql.Patch { return sql.Patch{sql.Set(&a.Name, "b")} }, []string{"name=a"}},
		"both":                 {func(a *account) sql.Patch { a.Age, a.Name = 2, "b"; return nil }, []string{"name=a", "age=1"}},
		"by hand":              {func(*account) sql.Patch { return sql.Patch{byHand} }, []string{"age=1"}},
		"assigned and by hand": {func(a *account) sql.Patch { a.Age++; return sql.Patch{byHand} }, []string{"age=1"}},
		"not a column":         {func(*account) sql.Patch { return sql.Patch{unknown} }, nil},
	} {
		t.Run(name, func(t *testing.T) {
			writes := map[string]func(context.Context, *bank) error{
				"Mutate": func(ctx context.Context, DB *bank) error {
					_, err := DB.Accounts.Mutate(ctx, "a", nil, test.patch)
					return err
				},
				"Update": func(ctx context.Context, DB *bank) error {
					_, err := DB.Accounts.Update(ctx, everyAccount, test.patch)
					return err
				},
			}
			for operation, write := range writes {
				t.Run(operation, func(t *testing.T) {
					ctx := testContext(t)
					var queries []sodium.Query
					db := newMemoryDatabase()
					DB := sql.Open[bank](updateRecorder{db, false, &queries})
					if err := DB.Accounts.Set(ctx, "a", account{Name: "a", Age: 1}); err != nil {
						t.Fatal(err)
					}
					if err := write(ctx, DB); err != nil {
						t.Fatal(err)
					}
					if len(queries) != 1 {
						t.Fatalf("the database was sent %d updates, want one", len(queries))
					}
					_, names := db.columns("accounts")
					var value []sodium.Column
					for _, name := range names {
						value = append(value, sodium.Column{Name: name})
					}
					if got := valuesAskedFor(sodium.Table{Value: value}, queries[0]); !slices.Equal(got, test.want) {
						t.Errorf("the update asked for %q, want %q", got, test.want)
					}
					if !slices.ContainsFunc(queries[0], func(expr sodium.Expression) bool {
						return xyz.ValueOf(expr) == sodium.Expressions.Range && sodium.Expressions.Range.Get(expr) == sodium.Range{From: 0, Upto: 1}
					}) {
						t.Errorf("the update is not of one row: %v", queries[0])
					}
				})
			}
		})
	}
}

// A database that does not find the value of a column equal to the one it sent
// is still written to, as it always was: the row is read again, and as it holds
// what was read, it is written whatever the database makes of its values.
func TestWriteBackToDatabaseThatFindsNoValueEqual(t *testing.T) {
	older := func(a *account) sql.Patch { a.Age++; return nil }
	writes := map[string]func(context.Context, *bank) error{
		"Mutate": func(ctx context.Context, DB *bank) error {
			ok, err := DB.Accounts.Mutate(ctx, "a", nil, older)
			if err == nil && !ok {
				return errors.New("mutate reported nothing mutated")
			}
			return err
		},
		"Update": func(ctx context.Context, DB *bank) error {
			n, err := DB.Accounts.Update(ctx, everyAccount, older)
			if err == nil && n != 1 {
				return fmt.Errorf("update patched %d values", n)
			}
			return err
		},
	}
	for name, write := range writes {
		t.Run(name, func(t *testing.T) {
			ctx := testContext(t)
			var queries []sodium.Query
			db := newMemoryDatabase()
			DB := sql.Open[bank](updateRecorder{db, true, &queries})
			if err := DB.Accounts.Set(ctx, "a", account{Name: "a", Age: 1}); err != nil {
				t.Fatal(err)
			}
			if err := write(ctx, DB); err != nil {
				t.Fatal(err)
			}
			if got, err := DB.Accounts.Get(ctx, "a"); got != (account{Name: "a", Age: 2}) || err != nil {
				t.Errorf("the row is %+v, %v", got, err)
			}
			if len(queries) != 2 {
				t.Errorf("the database was sent %d updates, want one that asks and one that does not", len(queries))
			}
		})
	}
	// A row that did change is still not written over.
	t.Run("changed", func(t *testing.T) {
		ctx := testContext(t)
		var queries []sodium.Query
		DB := sql.Open[bank](updateRecorder{newMemoryDatabase(), true, &queries})
		if err := DB.Accounts.Set(ctx, "a", account{Name: "a", Age: 1}); err != nil {
			t.Fatal(err)
		}
		runs := 0
		other := func() error { return DB.Accounts.Set(ctx, "a", account{Name: "a", Age: 50}) }
		if ok, err := DB.Accounts.Mutate(ctx, "a", nil, meanwhile(t, &runs, other, older)); ok || err != sql.ErrWriteConflict {
			t.Errorf("mutate gave %v and %v, want false and ErrWriteConflict", ok, err)
		}
		if got, err := DB.Accounts.Get(ctx, "a"); got != (account{Name: "a", Age: 50}) || err != nil {
			t.Errorf("the row is %+v, %v: want the write made in between", got, err)
		}
	})
}

// A patch can return modifications it made for itself, which a mutation sends
// to a database as an update does, rather than report a value as mutated that
// it left alone.
func TestMutateKeepsModificationsMadeByHand(t *testing.T) {
	ctx := testContext(t)
	DB := sql.Open[bank](newMemoryDatabase())
	if err := DB.Accounts.Set(ctx, "a", account{Name: "a", Age: 1}); err != nil {
		t.Fatal(err)
	}
	byHand := sodium.Modifications.Set.As(xyz.NewPair(sodium.Column{Name: "age"}, sodium.Values.Int64.As(7)))
	mutated, err := DB.Accounts.Mutate(ctx, "a", nil, func(a *account) sql.Patch {
		return sql.Patch{sql.Set(&a.Name, "renamed"), byHand}
	})
	if !mutated || err != nil {
		t.Fatalf("mutate gave %v and %v", mutated, err)
	}
	if got, err := DB.Accounts.Get(ctx, "a"); got != (account{Name: "renamed", Age: 7}) || err != nil {
		t.Errorf("the row is %+v, %v", got, err)
	}
}

// A modification that a patch made for itself is held to the scope by a
// mutation as it is by an update: it cannot move the row out of the scope, set
// a column of the key, or set a column the table does not have.
func TestScopedMutateChecksReturnedModifications(t *testing.T) {
	db := newMemoryDatabase()
	docs := sql.Open[updateDocuments](db)
	ctx := sql.Scope(testContext(t), "team", "red")
	if err := docs.Documents.Set(ctx, "one", updateDocument{Team: "red", Title: "one"}); err != nil {
		t.Fatal(err)
	}
	set := func(column string, value string) sql.PatchFunc[updateDocument] {
		return func(*updateDocument) sql.Patch {
			return sql.Patch{sodium.Modifications.Set.As(xyz.NewPair(sodium.Column{Name: column}, sodium.Values.String.As(value)))}
		}
	}
	for name, patch := range map[string]sql.PatchFunc[updateDocument]{
		"out of scope":   set("team", "blue"),
		"key":            set("id", "two"),
		"unknown column": set("elsewhere", "x"),
	} {
		t.Run(name, func(t *testing.T) {
			mutated, err := docs.Documents.Mutate(ctx, "one", nil, patch)
			if mutated || !errors.Is(err, sql.ErrAccessDenied) || db.updates.Load() != 0 {
				t.Errorf("mutate gave %v and %v after %d updates, want it refused before any", mutated, err, db.updates.Load())
			}
		})
	}
	if mutated, err := docs.Documents.Mutate(ctx, "one", nil, set("title", "renamed")); !mutated || err != nil {
		t.Fatalf("a modification inside of the scope gave %v and %v", mutated, err)
	}
	if got, err := docs.Documents.Get(ctx, "one"); got != (updateDocument{Team: "red", Title: "renamed"}) || err != nil {
		t.Errorf("the row is %+v, %v", got, err)
	}
}
