package sql_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/sql"
)

// crowd returns a bank whose database holds more accounts than a search is
// given room for at once.
func crowd(t *testing.T, total int) (*memoryDatabase, *bank) {
	t.Helper()
	db := newMemoryDatabase()
	DB := sql.Open[bank](db)
	ctx := testContext(t)
	for i := range total {
		name := fmt.Sprintf("n%03d", i)
		if err := DB.Accounts.Set(ctx, name, account{Name: name, Age: i}); err != nil {
			t.Fatal(err)
		}
	}
	return db, DB
}

// within fails the test if fn has not returned in time, which is how a search
// that nobody is reading any more used to end.
func within(t *testing.T, limit time.Duration, what string, fn func() error) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("%s failed: %v", what, err)
		}
	case <-time.After(limit):
		t.Fatalf("%s never returned", what)
	}
}

// A transaction runs its jobs in turn, so a search that was stopped early has
// to be over before the next job can start.
func TestSearchStoppedEarlyInsideTransaction(t *testing.T) {
	_, DB := crowd(t, 300)
	ctx := testContext(t)
	within(t, 5*time.Second, "a transaction that reads the first row and then counts them all", func() error {
		return DB.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
			_, _, found, err := DB.Accounts.First(ctx, nil)
			if err != nil || !found {
				return fmt.Errorf("first: %v %v", found, err)
			}
			n, err := DB.Accounts.Count(ctx, nil)
			if err != nil {
				return fmt.Errorf("count after first: %w", err)
			}
			if n != 300 {
				return fmt.Errorf("counted %d", n)
			}
			var issue error
			seen := 0
			for range DB.Accounts.Search(ctx, nil, &issue) {
				if seen++; seen == 2 {
					break
				}
			}
			if issue != nil {
				return fmt.Errorf("search stopped at the second row: %w", issue)
			}
			return DB.Accounts.Set(ctx, "last", account{Name: "last"})
		})
	})
	if n, err := DB.Accounts.Length(ctx); n != 301 || err != nil {
		t.Errorf("after the transaction the table holds %d rows, %v", n, err)
	}
}

// On its own, a search that was stopped early is cancelled rather than read to
// the end, and leaves nothing of itself running in the database.
func TestSearchStoppedEarlyIsCancelled(t *testing.T) {
	db, DB := crowd(t, 300)
	ctx := testContext(t)
	for range 3 {
		if _, _, found, err := DB.Accounts.First(ctx, nil); !found || err != nil {
			t.Fatalf("first gave %v and %v", found, err)
		}
	}
	within(t, 5*time.Second, "the database's part of three searches that were stopped early", func() error {
		db.settled()
		return nil
	})
	if commits := db.commits.Load(); commits != 300 {
		t.Errorf("the database committed %d transactions, want only the 300 that stored the rows", commits)
	}
	within(t, 5*time.Second, "a write after searches that were stopped early", func() error {
		return DB.Accounts.Set(ctx, "last", account{Name: "last"})
	})
}

// The database waits for room to send a row, so a caller that takes its time
// over them gets every one.
func TestSearchSlowReaderGetsEveryRow(t *testing.T) {
	_, DB := crowd(t, 300)
	var issue error
	found := 0
	for range DB.Accounts.Search(testContext(t), nil, &issue) {
		if found == 0 {
			time.Sleep(50 * time.Millisecond) // long enough for the search to run out of room.
		}
		found++
	}
	if issue != nil || found != 300 {
		t.Errorf("a search of 300 rows gave %d of them and %v", found, issue)
	}
}

// A search that ran to its end is a transaction that worked, a lookup included.
func TestSearchToTheEndCommits(t *testing.T) {
	db, DB := crowd(t, 3)
	ctx := testContext(t)
	db.settled()
	before := db.commits.Load()
	if n := len(matchedKeys(t, DB, nil)); n != 3 {
		t.Fatalf("found %d rows", n)
	}
	db.settled()
	if commits, rollbacks := db.commits.Load()-before, db.rollbacks.Load(); commits != 1 || rollbacks != 0 {
		t.Errorf("a finished search made for %d commits and %d rollbacks, want one commit", commits, rollbacks)
	}
	if _, ok, err := DB.Accounts.Lookup(ctx, "n001"); !ok || err != nil {
		t.Errorf("lookup gave %v and %v", ok, err)
	}
}

// A query with more than one order is ordered by the first, and by each one
// after it among the rows that the ones before it leave equal, with and
// without a database.
func TestSearchOrdersInTheOrderWritten(t *testing.T) {
	byAgeThenNameDescending := func(_ *string, a *account) sql.Query {
		return sql.Query{sql.Order(&a.Age).Increasing(), sql.Order(&a.Name).Decreasing()}
	}
	byNameDescendingThenAge := func(_ *string, a *account) sql.Query {
		return sql.Query{sql.Order(&a.Name).Decreasing(), sql.Order(&a.Age).Increasing()}
	}
	secondOfThem := func(k *string, a *account) sql.Query {
		return append(byAgeThenNameDescending(k, a), sql.Slice(1, 2))
	}
	eachBackend(t, func(t *testing.T, DB *bank) {
		ctx := testContext(t)
		for i, name := range []string{"a", "b", "c", "d", "e", "f"} {
			if err := DB.Accounts.Set(ctx, name, account{Name: name, Age: i % 2}); err != nil {
				t.Fatal(err)
			}
		}
		found := func(query sql.QueryFunc[string, account]) string {
			var issue error
			var names string
			for name := range DB.Accounts.Search(ctx, query, &issue) {
				names += name
			}
			if issue != nil {
				t.Fatal(issue)
			}
			return names
		}
		if got := found(byAgeThenNameDescending); got != "ecafdb" {
			t.Errorf("by age and then by name descending gave %q, want %q", got, "ecafdb")
		}
		if got := found(byNameDescendingThenAge); got != "fedcba" {
			t.Errorf("by name descending and then by age gave %q, want %q", got, "fedcba")
		}
		// A write goes by the same order to pick its rows.
		if n, err := DB.Accounts.UnsafeDelete(ctx, secondOfThem); n != 1 || err != nil {
			t.Fatalf("delete gave %d and %v", n, err)
		}
		if got := found(byAgeThenNameDescending); got != "eafdb" {
			t.Errorf("deleting the second row left %q, want %q", got, "eafdb")
		}
	})
}
