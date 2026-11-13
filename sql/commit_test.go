package sql_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/sql"
	"github.com/iqhive/runtime.link/sql/std/sodium"
)

// slowDatabase is a database whose inserts take until the test releases them.
type slowDatabase struct {
	*memoryDatabase
	release chan struct{}
}

func (db slowDatabase) Insert(table sodium.Table, index []sodium.Value, overwrite bool, query sodium.Query, value []sodium.Value) sodium.Job {
	inner := db.memoryDatabase.Insert(table, index, overwrite, query, value).(*memoryJob)
	return newMemoryWrite(func(ctx context.Context) (int, error) {
		<-db.release
		inner.execute(ctx)
		return inner.count, inner.err
	})
}

// The database is still busy with the insert when its caller gives up, so the
// rollback cannot be sent. Closing the channel must not be taken for a commit:
// the caller was told that the transaction failed.
func TestTimedOutTransactionIsNotCommitted(t *testing.T) {
	for _, standalone := range []bool{false, true} {
		name := "Manage"
		if standalone {
			name = "standalone"
		}
		t.Run(name, func(t *testing.T) {
			for range 40 {
				db := slowDatabase{newMemoryDatabase(), make(chan struct{})}
				DB := sql.Open[bank](db)
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Millisecond)
				var err error
				if standalone {
					err = DB.Accounts.Set(ctx, "a", account{Name: "a"})
				} else {
					err = DB.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
						return DB.Accounts.Set(ctx, "a", account{Name: "a"})
					})
				}
				cancel()
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expected the insert to time out, got %v", err)
				}
				close(db.release) // the insert completes after its caller gave up.
				db.settled()
				if n := db.rowCount(); n != 0 {
					t.Fatalf("a write that was reported as failed was kept: %d rows, %d commits", n, db.commits.Load())
				}
				if commits := db.commits.Load(); commits != 0 {
					t.Fatalf("a transaction that timed out was committed %d times", commits)
				}
			}
		})
	}
}

// A context that ends while fn runs has rolled the transaction back, whatever
// fn goes on to return, so Manage cannot report that it was committed.
func TestTransactionWhoseContextEndedIsNotCommitted(t *testing.T) {
	t.Run("Database", func(t *testing.T) {
		db := newMemoryDatabase()
		DB := sql.Open[bank](db)
		ctx, cancel := context.WithCancel(testContext(t))
		defer cancel()
		err := DB.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
			if err := DB.Accounts.Set(ctx, "a", account{Name: "a"}); err != nil {
				return err
			}
			cancel()
			return nil
		})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Manage gave %v, want the error of the context", err)
		}
		db.settled()
		if n, commits := db.rowCount(), db.commits.Load(); n != 0 || commits != 0 {
			t.Errorf("the database holds %d rows after %d commits", n, commits)
		}
	})
	// Nothing rolls back a transaction on RAM maps but its fn, so one that
	// returned nil is committed, as it always has been.
	t.Run("RAM", func(t *testing.T) {
		var m sql.Map[string, account]
		ctx, cancel := context.WithCancel(testContext(t))
		defer cancel()
		err := m.Manage(ctx, 0, func(ctx context.Context) error {
			if err := m.Set(ctx, "a", account{Name: "a"}); err != nil {
				return err
			}
			cancel()
			return nil
		})
		if err != nil {
			t.Errorf("Manage gave %v, want nil", err)
		}
		if n, err := m.Length(testContext(t)); n != 1 || err != nil {
			t.Errorf("the map holds %d rows, %v", n, err)
		}
	})
}

// A commit is something that is said. Each way a transaction can end without
// one rolls it back.
func TestMemoryDatabaseCommitsOnlyWhenTold(t *testing.T) {
	for name, test := range map[string]struct {
		end       func(jobs chan<- sodium.Job, cancel context.CancelFunc)
		committed bool
	}{
		"commit":           {func(jobs chan<- sodium.Job, _ context.CancelFunc) { jobs <- sodium.NewCommit(); close(jobs) }, true},
		"closed":           {func(jobs chan<- sodium.Job, _ context.CancelFunc) { close(jobs) }, false},
		"nil job":          {func(jobs chan<- sodium.Job, _ context.CancelFunc) { jobs <- nil; close(jobs) }, false},
		"context":          {func(_ chan<- sodium.Job, cancel context.CancelFunc) { cancel() }, false},
		"context and then": {func(jobs chan<- sodium.Job, cancel context.CancelFunc) { cancel(); close(jobs) }, false},
	} {
		t.Run(name, func(t *testing.T) {
			db := newMemoryDatabase()
			DB := sql.Open[bank](db)
			ctx, cancel := context.WithCancel(testContext(t))
			defer cancel()
			jobs, err := db.Manage(ctx, 0)
			if err != nil {
				t.Fatal(err)
			}
			insert := db.Insert(sodium.Table{Name: "accounts", Index: []sodium.Column{{Name: "id"}}}, sql.ValuesOf("a"), true, nil, nil)
			jobs <- insert
			if _, err := insert.Wait(ctx); err != nil {
				t.Fatal(err)
			}
			test.end(jobs, cancel)
			db.settled()
			if kept := db.rowCount() == 1; kept != test.committed {
				t.Errorf("the write was kept: %v, want %v", kept, test.committed)
			}
			if n, err := DB.Accounts.Length(testContext(t)); err != nil || (n == 1) != test.committed {
				t.Errorf("the map counts %d rows, %v", n, err)
			}
		})
	}
}

// A transaction is on one database. A map of another database, or of none,
// is not part of it: its job is not sent to a database that does not hold it.
func TestTransactionIsOnOneDatabase(t *testing.T) {
	ctx := testContext(t)
	first, second := newMemoryDatabase(), newMemoryDatabase()
	A, B := sql.Open[bank](first), sql.Open[bank](second)
	var memory sql.Map[string, account]
	failure := errors.New("changed my mind")
	err := A.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
		if err := A.Accounts.Set(ctx, "a", account{Name: "a"}); err != nil {
			return err
		}
		if err := B.Accounts.Set(ctx, "b", account{Name: "b"}); err != nil {
			return err
		}
		if err := memory.Set(ctx, "m", account{Name: "m"}); err != nil {
			return err
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	first.settled()
	second.settled()
	if n := first.rowCount(); n != 0 {
		t.Errorf("the database of the transaction kept %d rows after its rollback", n)
	}
	if commits, rollbacks := first.commits.Load(), first.rollbacks.Load(); commits != 0 || rollbacks != 1 {
		t.Errorf("the database of the transaction saw %d commits and %d rollbacks, want one rollback", commits, rollbacks)
	}
	// The other database made the write as a transaction of its own.
	if rows, commits := second.rowCount(), second.commits.Load(); rows != 1 || commits != 1 {
		t.Errorf("the other database holds %d rows after %d commits of its own, want 1 and 1", rows, commits)
	}
	if n, err := memory.Length(ctx); n != 1 || err != nil {
		t.Errorf("the map without a database holds %d rows, %v", n, err)
	}
}

// Another database's transaction in between does not hide the outer one from
// the maps that are in it.
func TestTransactionJoinsThroughAnotherDatabase(t *testing.T) {
	ctx := testContext(t)
	first, second := newMemoryDatabase(), newMemoryDatabase()
	A, B := sql.Open[bank](first), sql.Open[bank](second)
	failure := errors.New("changed my mind")
	err := A.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
		inner := B.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
			if err := B.Accounts.Set(ctx, "b", account{Name: "b"}); err != nil {
				return err
			}
			if err := A.Accounts.Set(ctx, "a", account{Name: "a"}); err != nil {
				return err
			}
			return A.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
				return A.Accounts.Set(ctx, "nested", account{Name: "nested"})
			})
		})
		if inner != nil {
			return inner
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	first.settled()
	second.settled()
	if n := first.rowCount(); n != 0 {
		t.Errorf("the outer database kept %d rows written inside the other database's transaction", n)
	}
	if managed := first.commits.Load() + first.rollbacks.Load(); managed != 1 {
		t.Errorf("the outer database ran %d transactions, want the one", managed)
	}
	if rows, commits := second.rowCount(), second.commits.Load(); rows != 1 || commits != 1 {
		t.Errorf("the inner database holds %d rows after %d commits, want 1 and 1", rows, commits)
	}
}

// uncomparableDatabase is a database whose value cannot be compared with ==.
type uncomparableDatabase struct {
	*memoryDatabase
	notes []string
}

// Maps of a database that cannot be compared still join its transaction.
func TestTransactionOnUncomparableDatabase(t *testing.T) {
	ctx := testContext(t)
	db := newMemoryDatabase()
	DB := sql.Open[bank](uncomparableDatabase{memoryDatabase: db})
	failure := errors.New("changed my mind")
	err := DB.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
		if err := DB.Accounts.Set(ctx, "a", account{Name: "a"}); err != nil {
			return err
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	db.settled()
	if rows, commits := db.rowCount(), db.commits.Load(); rows != 0 || commits != 0 {
		t.Errorf("the database holds %d rows after %d commits", rows, commits)
	}
}

type ledger struct {
	Accounts sql.Map[string, account] `sql:"accounts"`
	Entries  sql.Map[int, int]        `sql:"entries"`
}

// The maps of one Open were given one database, so they share its transaction
// even if that database is a value that cannot be compared with another.
func TestMapsOfOneOpenShareUncomparableDatabase(t *testing.T) {
	ctx := testContext(t)
	db := newMemoryDatabase()
	DB := sql.Open[ledger](uncomparableDatabase{memoryDatabase: db})
	failure := errors.New("changed my mind")
	err := DB.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
		if err := DB.Entries.Set(ctx, 1, 1); err != nil {
			return err
		}
		return DB.Entries.Manage(ctx, 0, func(ctx context.Context) error {
			if err := DB.Accounts.Set(ctx, "a", account{Name: "a"}); err != nil {
				return err
			}
			return failure
		})
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	db.settled()
	if rows, commits, rollbacks := db.rowCount(), db.commits.Load(), db.rollbacks.Load(); rows != 0 || commits != 0 || rollbacks != 1 {
		t.Errorf("the database holds %d rows after %d commits and %d rollbacks, want the one rollback", rows, commits, rollbacks)
	}
}

// Two databases of one type that cannot be compared may be two databases, and
// nothing says otherwise. A job is only ever sent to the database of its Map,
// so the map of the second is not taken into the transaction of the first:
// its write is a transaction of its own, on its own database.
func TestUncomparableDatabasesDoNotShareTransaction(t *testing.T) {
	ctx := testContext(t)
	first, second := newMemoryDatabase(), newMemoryDatabase()
	A := sql.Open[bank](uncomparableDatabase{memoryDatabase: first})
	B := sql.Open[bank](uncomparableDatabase{memoryDatabase: second})
	failure := errors.New("changed my mind")
	err := A.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
		if err := A.Accounts.Set(ctx, "a", account{Name: "a"}); err != nil {
			return err
		}
		if err := B.Accounts.Set(ctx, "b", account{Name: "b"}); err != nil {
			return err
		}
		return B.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
			if err := B.Accounts.Set(ctx, "nested", account{Name: "nested"}); err != nil {
				return err
			}
			return failure
		})
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	first.settled()
	second.settled()
	if rows, commits, rollbacks := first.rowCount(), first.commits.Load(), first.rollbacks.Load(); rows != 0 || commits != 0 || rollbacks != 1 {
		t.Errorf("the database of the transaction holds %d rows after %d commits and %d rollbacks, want the one rollback", rows, commits, rollbacks)
	}
	// One write of its own, committed, and one transaction of its own, rolled back.
	if rows, commits, rollbacks := second.rowCount(), second.commits.Load(), second.rollbacks.Load(); rows != 1 || commits != 1 || rollbacks != 1 {
		t.Errorf("the other database holds %d rows after %d commits and %d rollbacks, want 1, 1 and 1", rows, commits, rollbacks)
	}
}

// The same holds for one such database that is opened twice: the maps of the
// second Open cannot be told to be on the database of the first, so they are
// not in its transaction. Their writes are made, on the right database, and
// are not rolled back with it.
func TestUncomparableDatabaseOpenedTwiceIsNotJoined(t *testing.T) {
	ctx := testContext(t)
	db := newMemoryDatabase()
	shared := uncomparableDatabase{memoryDatabase: db}
	A, B := sql.Open[bank](shared), sql.Open[bank](shared)
	failure := errors.New("changed my mind")
	err := A.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
		if err := B.Accounts.Set(ctx, "b", account{Name: "b"}); err != nil {
			return err
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	db.settled()
	if rows, commits, rollbacks := db.rowCount(), db.commits.Load(), db.rollbacks.Load(); rows != 1 || commits != 1 || rollbacks != 1 {
		t.Errorf("the database holds %d rows after %d commits and %d rollbacks, want the write made on its own and the rollback of the transaction", rows, commits, rollbacks)
	}
}

// A database that can be compared is the same database however often it is
// opened, by Open or by OpenTable.
func TestComparableDatabaseOpenedTwiceIsJoined(t *testing.T) {
	ctx := testContext(t)
	db := newMemoryDatabase()
	A, B := sql.Open[bank](db), sql.Open[bank](db)
	C := sql.OpenTable[int, int](db, sodium.Table{Name: "entries"})
	failure := errors.New("changed my mind")
	err := A.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
		if err := B.Accounts.Set(ctx, "b", account{Name: "b"}); err != nil {
			return err
		}
		if err := C.Set(ctx, 1, 1); err != nil {
			return err
		}
		return C.Manage(ctx, 0, func(ctx context.Context) error { return failure })
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	db.settled()
	if rows, commits, rollbacks := db.rowCount(), db.commits.Load(), db.rollbacks.Load(); rows != 0 || commits != 0 || rollbacks != 1 {
		t.Errorf("the database holds %d rows after %d commits and %d rollbacks, want the one rollback", rows, commits, rollbacks)
	}
}
