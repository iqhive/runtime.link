package sql_test

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/sql"
)

// partRead is a way for the function of a transaction to leave a search with
// rows still to come. It returns how to read the rest once the transaction is
// over, which gives the number of rows left and what ended them.
type partRead func(ctx context.Context, DB *bank) (rest func() (int, error), err error)

// handedBack reads one row and hands the search back to be read on.
func handedBack(ctx context.Context, DB *bank) (func() (int, error), error) {
	var issue error
	next, stop := iter.Pull2(DB.Accounts.Search(ctx, nil, &issue))
	if _, _, ok := next(); !ok {
		stop()
		return nil, fmt.Errorf("the search gave no row: %v", issue)
	}
	return func() (int, error) {
		defer stop()
		left := 0
		for {
			if _, _, ok := next(); !ok {
				return left, issue
			}
			left++
		}
	}, nil
}

// cancelled ends a search by cancelling the context it was given.
func cancelled(ctx context.Context, DB *bank) (func() (int, error), error) {
	searching, cancel := context.WithCancel(ctx)
	defer cancel()
	var issue error
	for range DB.Accounts.Search(searching, nil, &issue) {
		cancel()
		break
	}
	return func() (int, error) { return 0, nil }, nil
}

// commitsWith runs check for a database that answers its commits and for one
// that commits when its channel is closed.
func commitsWith(t *testing.T, check func(t *testing.T, db *memoryDatabase)) {
	t.Helper()
	for _, onClose := range []bool{false, true} {
		t.Run(fmt.Sprintf("commitsOnClose=%v", onClose), func(t *testing.T) {
			db := newMemoryDatabase()
			db.commitsOnClose = onClose
			check(t, db)
		})
	}
}

// settles fails the test if the database still has a transaction open after a
// while, which is one that nobody will ever end.
func settles(t *testing.T, db *memoryDatabase, what string) {
	t.Helper()
	settled := make(chan struct{})
	go func() { db.settled(); close(settled) }()
	select {
	case <-settled:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s: the database still has a transaction open", what)
	}
}

// A transaction whose function left a search part read is committed all the
// same, by a database that is sent the commit and by one that is told by its
// channel being closed: the rows the database has yet to send are taken first,
// so that it gets to what comes after them. The write is kept once the context
// ends, and a search that was handed back gives the rest of its rows.
func TestCommitWithSearchLeftPartRead(t *testing.T) {
	const seeded = 200
	for name, leave := range map[string]partRead{"handed back": handedBack, "cancelled": cancelled} {
		t.Run(name, func(t *testing.T) {
			commitsWith(t, func(t *testing.T, db *memoryDatabase) {
				DB := sql.Open[bank](db)
				seed := testContext(t)
				for i := range seeded {
					name := fmt.Sprintf("row%03d", i)
					if err := DB.Accounts.Set(seed, name, account{Name: name}); err != nil {
						t.Fatal(err)
					}
				}
				db.settled()
				commits := db.commits.Load()
				ctx, cancel := context.WithCancel(testContext(t))
				defer cancel()
				var rest func() (int, error)
				within(t, 5*time.Second, "a transaction that leaves a search part read", func() error {
					return DB.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
						if err := DB.Accounts.Set(ctx, "written", account{Name: "written"}); err != nil {
							return err
						}
						var err error
						rest, err = leave(ctx, DB)
						return err
					})
				})
				// The commit is made before the context ends, not by its end.
				settles(t, db, "after Manage returned nil")
				if made, rolled := db.commits.Load()-commits, db.rollbacks.Load(); made != 1 || rolled != 0 {
					t.Errorf("the database made %d commits and %d rollbacks, want one commit", made, rolled)
				}
				if rest != nil {
					left, err := rest()
					if err != nil {
						t.Errorf("reading on gave %v", err)
					}
					if name == "handed back" && left != seeded {
						t.Errorf("reading on gave %d rows, want the %d left of the search", left, seeded)
					}
				}
				cancel()
				if _, kept, err := DB.Accounts.Lookup(testContext(t), "written"); !kept || err != nil {
					t.Errorf("the write reported as committed gave %v and %v once the context ended", kept, err)
				}
			})
		})
	}
}

// A search left with more rows unread than a transaction holds fails the
// commit, whichever way the database commits: nothing is reported as made,
// nothing is kept, and the transaction is over.
func TestCommitWithTooManyRowsLeftUnread(t *testing.T) {
	commitsWith(t, func(t *testing.T, db *memoryDatabase) {
		DB := sql.Open[bank](db)
		seed := testContext(t)
		for i := range 2048 {
			name := fmt.Sprintf("row%04d", i)
			if err := DB.Accounts.Set(seed, name, account{Name: name}); err != nil {
				t.Fatal(err)
			}
		}
		db.settled()
		commits := db.commits.Load()
		ctx := testContext(t)
		var rest func() (int, error)
		var err error
		within(t, 10*time.Second, "a transaction that leaves a long search part read", func() error {
			err = DB.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
				if err := DB.Accounts.Set(ctx, "written", account{Name: "written"}); err != nil {
					return err
				}
				var err error
				rest, err = handedBack(ctx, DB)
				return err
			})
			return nil
		})
		if !errors.Is(err, sql.ErrUnreadRows) {
			t.Errorf("the transaction gave %v, want ErrUnreadRows", err)
		}
		settles(t, db, "after Manage failed")
		if made, rolled := db.commits.Load()-commits, db.rollbacks.Load(); made != 0 || rolled != 1 {
			t.Errorf("the database made %d commits and %d rollbacks, want one rollback", made, rolled)
		}
		if rest != nil {
			rest()
		}
		if _, kept, err := DB.Accounts.Lookup(testContext(t), "written"); kept || err != nil {
			t.Errorf("the write of the transaction that failed gave %v and %v", kept, err)
		}
	})
}
