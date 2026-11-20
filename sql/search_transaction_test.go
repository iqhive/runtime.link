package sql_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/sql"
)

// byName is every account in the order of its name.
func byName(_ *string, a *account) sql.Query {
	return sql.Query{sql.Order(&a.Name).Increasing()}
}

// A transaction runs its jobs in turn, and a search is not over until its last
// row has been taken. An operation made for a row of a search is still made,
// however many rows the search has left: more than it is given room for at
// once, or fewer.
func TestOperationsForEachRowOfSearchInsideTransaction(t *testing.T) {
	for _, total := range []int{1, 10, 64, 65, 66, 300} {
		t.Run(fmt.Sprint(total, " rows"), func(t *testing.T) {
			db, DB := crowd(t, total)
			ctx := testContext(t)
			within(t, 10*time.Second, "a transaction that works on each row of a search", func() error {
				return DB.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
					var issue error
					seen := 0
					for name, row := range DB.Accounts.Search(ctx, byName, &issue) {
						// The rows come in the order of the search, whether they
						// were read as they were sent or kept for later.
						if want := fmt.Sprintf("n%03d", seen); name != want || row.Age != seen {
							return fmt.Errorf("row %d is %s %+v", seen, name, row)
						}
						seen++
						if got, ok, err := DB.Accounts.Lookup(ctx, name); got != row || !ok || err != nil {
							return fmt.Errorf("lookup of %s gave %+v, %v and %v", name, got, ok, err)
						}
						if ok, err := DB.Accounts.Mutate(ctx, name, nil, func(a *account) sql.Patch { a.Age += 1000; return nil }); !ok || err != nil {
							return fmt.Errorf("mutate of %s gave %v and %v", name, ok, err)
						}
						if n, err := DB.Accounts.Count(ctx, nil); n != total || err != nil {
							return fmt.Errorf("count gave %d and %v", n, err)
						}
						err := DB.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
							return DB.Accounts.Set(ctx, "also "+name, account{Name: "also " + name})
						})
						if err != nil {
							return fmt.Errorf("a nested transaction for %s: %w", name, err)
						}
						total++
					}
					if issue != nil {
						return issue
					}
					if seen*2 != total {
						return fmt.Errorf("the search gave %d rows", seen)
					}
					return nil
				})
			})
			db.settled()
			if commits, rollbacks := db.commits.Load(), db.rollbacks.Load(); rollbacks != 0 {
				t.Errorf("the database saw %d commits and %d rollbacks", commits, rollbacks)
			}
			mutated, added := 0, 0
			for name, row := range accounts(t, DB) {
				switch {
				case row.Name == "also "+name[len(name)-4:]:
					added++
				case row.Age >= 1000:
					mutated++
				}
			}
			if mutated*2 != total || added*2 != total {
				t.Errorf("of %d rows, %d were mutated and %d added", total, mutated, added)
			}
		})
	}
}

// A job made beside a search that is still being read is given the rows the
// search has not read yet, so that the database can move on to it. A search
// that no slice bounds can have more rows left than the transaction has room
// to hold: the job is refused, with [sql.ErrUnreadRows], and the search is
// left for whoever was reading it to read on to its end.
func TestJobBesideUnboundedSearchIsRefused(t *testing.T) {
	db, DB := crowd(t, 2048)
	ctx := testContext(t)
	var read int
	within(t, 10*time.Second, "a job beside a search with too many rows unread", func() error {
		return DB.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
			var issue error
			for name := range DB.Accounts.Search(ctx, byName, &issue) {
				read++
				if read != 1 {
					continue
				}
				// The first is refused once the rows held reach the room the
				// transaction has for them, the second right away, on the
				// rows the first left held.
				for range 2 {
					if _, ok, err := DB.Accounts.Lookup(ctx, name); ok || !errors.Is(err, sql.ErrUnreadRows) {
						return fmt.Errorf("a lookup beside row %d gave %v and %v", read, ok, err)
					}
				}
			}
			if issue != nil {
				return issue
			}
			if read != 2048 {
				return fmt.Errorf("the search read %d rows, want all 2048", read)
			}
			return nil
		})
	})
	db.settled()
	if commits, rollbacks := db.commits.Load(), db.rollbacks.Load(); rollbacks != 0 {
		t.Errorf("the database saw %d commits and %d rollbacks", commits, rollbacks)
	}
}

// A search can be made for each row of another, and either can be left early.
func TestSearchInsideSearchInsideTransaction(t *testing.T) {
	const total = 150
	db, DB := crowd(t, total)
	ctx := testContext(t)
	within(t, 10*time.Second, "a transaction that searches for each row of a search", func() error {
		return DB.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
			var outer, inner error
			rows := 0
			for name := range DB.Accounts.Search(ctx, byName, &outer) {
				if want := fmt.Sprintf("n%03d", rows); name != want {
					return fmt.Errorf("outer row %d is %s", rows, name)
				}
				rows++
				found := 0
				for other := range DB.Accounts.Search(ctx, byName, &inner) {
					if want := fmt.Sprintf("n%03d", found); other != want {
						return fmt.Errorf("inner row %d is %s", found, other)
					}
					found++
					// Every other inner search is left with rows to come.
					if rows%2 == 0 && found == 3 {
						break
					}
				}
				if inner != nil {
					return inner
				}
				if rows%2 == 1 && found != total {
					return fmt.Errorf("the inner search gave %d rows", found)
				}
				if rows == 100 {
					break
				}
			}
			if outer != nil {
				return outer
			}
			// The transaction goes on after the search that was left.
			if n, err := DB.Accounts.Count(ctx, nil); n != total || err != nil {
				return fmt.Errorf("count gave %d and %v", n, err)
			}
			return nil
		})
	})
	db.settled()
	if commits, rollbacks := db.commits.Load(), db.rollbacks.Load(); rollbacks != 0 {
		t.Errorf("the database saw %d commits and %d rollbacks", commits, rollbacks)
	}
}

// However the body of a search loop is left, the transaction it is in ends as
// it should, and at once: the rows nobody took do not keep the database from
// hearing about the rollback.
func TestLeavingSearchInsideTransaction(t *testing.T) {
	failure := errors.New("changed my mind")
	for name, leave := range map[string]func(){
		"panic":  func() { panic(failure) },
		"goexit": func() { runtime.Goexit() },
	} {
		t.Run(name, func(t *testing.T) {
			db, DB := crowd(t, 300)
			// Not one that ends: a transaction that hangs here hangs for good.
			ctx := context.Background()
			done := make(chan any, 1)
			go func() {
				exited := true
				defer func() {
					if recovered := recover(); recovered != nil || exited {
						done <- recovered
					}
				}()
				DB.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
					if err := DB.Accounts.Set(ctx, "written", account{Name: "written"}); err != nil {
						return err
					}
					var issue error
					for range DB.Accounts.Search(ctx, nil, &issue) {
						leave()
					}
					return issue
				})
				exited = false
				done <- "returned"
			}()
			select {
			case recovered := <-done:
				if name == "panic" && recovered != failure {
					t.Errorf("recovered %v, want the panic of the loop", recovered)
				}
				if name == "goexit" && recovered != nil {
					t.Errorf("recovered %v from a goroutine that exited", recovered)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("the transaction never ended")
			}
			db.settled()
			if commits, rollbacks := db.commits.Load(), db.rollbacks.Load(); commits != 300 || rollbacks != 1 {
				t.Errorf("the database saw %d commits and %d rollbacks, want the 300 of the rows and one rollback", commits, rollbacks)
			}
			if n := db.rowCount(); n != 300 {
				t.Errorf("the database holds %d rows after the rollback", n)
			}
		})
	}
	t.Run("return", func(t *testing.T) {
		db, DB := crowd(t, 300)
		within(t, 10*time.Second, "a transaction that fails for a row of a search", func() error {
			err := DB.Accounts.Manage(context.Background(), 0, func(ctx context.Context) error {
				var issue error
				for range DB.Accounts.Search(ctx, nil, &issue) {
					return failure
				}
				return issue
			})
			if !errors.Is(err, failure) {
				return fmt.Errorf("the transaction gave %v", err)
			}
			return nil
		})
		db.settled()
		if rollbacks := db.rollbacks.Load(); rollbacks != 1 {
			t.Errorf("the database saw %d rollbacks", rollbacks)
		}
	})
}

// The goroutines of a transaction share it: one can write while another reads
// the rows of a search. Run with -race.
func TestSearchAndWritesShareTransaction(t *testing.T) {
	const total = 200
	db, DB := crowd(t, total)
	ctx := testContext(t)
	within(t, 10*time.Second, "a transaction that reads and writes at once", func() error {
		return DB.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
			var writers sync.WaitGroup
			failures := make([]error, 4)
			for i := range failures {
				writers.Add(1)
				go func() {
					defer writers.Done()
					for n := range 25 {
						name := fmt.Sprintf("w%d-%d", i, n)
						failures[i] = errors.Join(failures[i], DB.Accounts.Set(ctx, name, account{Name: name}))
					}
				}()
			}
			var issue error
			read := 0
			for name, row := range DB.Accounts.Search(ctx, func(_ *string, a *account) sql.Query {
				return sql.Query{sql.Match(&a.Name).HasPrefix("n"), sql.Order(&a.Name).Increasing()}
			}, &issue) {
				if want := fmt.Sprintf("n%03d", read); name != want || row.Age != read {
					return fmt.Errorf("row %d is %s %+v", read, name, row)
				}
				read++
			}
			writers.Wait()
			if err := errors.Join(append(failures, issue)...); err != nil {
				return err
			}
			if read != total {
				return fmt.Errorf("the search gave %d rows", read)
			}
			return nil
		})
	})
	db.settled()
	if n := db.rowCount(); n != total+100 {
		t.Errorf("the database holds %d rows", n)
	}
}
