package sql_test

import (
	"context"
	"errors"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/sql"
	"github.com/iqhive/runtime.link/sql/std/sodium"
)

// remoteDatabase commits the way a database reached over a connection does:
// the commit takes a moment, and following [sodium.Database.Manage] it is not
// made once the context has ended.
type remoteDatabase struct {
	*memoryDatabase
	roundTrip time.Duration
}

func (db remoteDatabase) Manage(ctx context.Context, level sodium.Transaction) (chan<- sodium.Job, error) {
	inner, err := db.memoryDatabase.Manage(ctx, level)
	if err != nil {
		return nil, err
	}
	jobs := make(chan sodium.Job)
	db.running.Add(1)
	go func() {
		defer db.running.Done()
		defer close(inner) // without a commit, which rolls back.
		for job := range jobs {
			commit, committing := job.(*sodium.Commit)
			if committing {
				time.Sleep(db.roundTrip)
				if err := ctx.Err(); err != nil {
					commit.Done(err)
					return
				}
			}
			// The database behind the connection is gone with the context,
			// and a commit that did not reach it is answered here.
			select {
			case inner <- job:
			case <-ctx.Done():
				if committing {
					commit.Done(ctx.Err())
				}
				return
			}
		}
	}()
	return jobs, nil
}

// writes are the ways a Map writes on its own, each of which is a transaction
// that has to be committed before the write can be reported as made.
func writes() map[string]func(context.Context, sql.Database) error {
	return map[string]func(context.Context, sql.Database) error{
		"Set": func(ctx context.Context, db sql.Database) error {
			return sql.Open[bank](db).Accounts.Set(ctx, "new", account{Name: "new"})
		},
		"Insert": func(ctx context.Context, db sql.Database) error {
			return sql.Open[bank](db).Accounts.Insert(ctx, "new", sql.Create, account{Name: "new"})
		},
		"Add": func(ctx context.Context, db sql.Database) error {
			_, err := sql.Open[randomizedTables](db).Rows.Add(ctx, 1)
			return err
		},
		"Mutate": func(ctx context.Context, db sql.Database) error {
			_, err := sql.Open[bank](db).Accounts.Mutate(ctx, "old", nil, func(a *account) sql.Patch { a.Age++; return nil })
			return err
		},
		"Update": func(ctx context.Context, db sql.Database) error {
			_, err := sql.Open[bank](db).Accounts.Update(ctx, everyAccount, func(a *account) sql.Patch { a.Age++; return nil })
			return err
		},
		"Delete": func(ctx context.Context, db sql.Database) error {
			_, err := sql.Open[bank](db).Accounts.Delete(ctx, "old", nil)
			return err
		},
		"Manage": func(ctx context.Context, db sql.Database) error {
			DB := sql.Open[bank](db)
			return DB.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
				return DB.Accounts.Set(ctx, "new", account{Name: "new"})
			})
		},
	}
}

// A commit can fail: a serialization failure, a deferred constraint, a lost
// connection. The database says so, and the write is reported as that failure
// rather than as made.
func TestWriteReportsCommitThatFailed(t *testing.T) {
	failure := errors.New("serialization failure")
	for name, write := range writes() {
		t.Run(name, func(t *testing.T) {
			ctx := testContext(t)
			db := newMemoryDatabase()
			DB := sql.Open[bank](db)
			if err := DB.Accounts.Set(ctx, "old", account{Name: "old"}); err != nil {
				t.Fatal(err)
			}
			db.settled()
			before := db.snapshot()
			db.commitFailure = failure
			if err := write(ctx, db); !errors.Is(err, failure) {
				t.Errorf("the write gave %v, want the failure of its commit", err)
			}
			db.settled()
			if after := db.snapshot(); !sameTables(before, after) {
				t.Errorf("the database could not commit and holds %v, it held %v", after, before)
			}
		})
	}
}

// A caller with a deadline cancels its context on the way out. A write that was
// reported as made by then has been committed, and stays.
func TestWriteReportedAsMadeOutlivesItsContext(t *testing.T) {
	for name, write := range writes() {
		t.Run(name, func(t *testing.T) {
			for range 10 {
				db := newMemoryDatabase()
				remote := remoteDatabase{db, 2 * time.Millisecond}
				DB := sql.Open[bank](remote)
				if err := DB.Accounts.Set(testContext(t), "old", account{Name: "old"}); err != nil {
					t.Fatal(err)
				}
				db.settled()
				before, commits := db.snapshot(), db.commits.Load()
				called := func() error {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					return write(ctx, remote)
				}
				if err := called(); err != nil {
					t.Fatal(err)
				}
				db.settled()
				if made := db.commits.Load() - commits; made != 1 {
					t.Fatalf("the write was reported as made after %d commits", made)
				}
				if sameTables(before, db.snapshot()) {
					t.Fatalf("the write was reported as made and the database holds what it did before: %v", before)
				}
			}
		})
	}
}

// Whenever the context ends, a write that is reported as made was committed,
// and one that failed for any other reason than the end of its context was not.
func TestWriteIsOnlyReportedOnceCommitted(t *testing.T) {
	for i := range 300 {
		db := newMemoryDatabase()
		DB := sql.Open[bank](remoteDatabase{db, time.Duration(rand.IntN(300)) * time.Microsecond})
		ctx, cancel := context.WithCancel(testContext(t))
		ended := time.AfterFunc(time.Duration(rand.IntN(600))*time.Microsecond, cancel)
		var err error
		if i%2 == 0 {
			err = DB.Accounts.Set(ctx, "a", account{Name: "a"})
		} else {
			err = DB.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
				return DB.Accounts.Set(ctx, "a", account{Name: "a"})
			})
		}
		ended.Stop()
		cancel()
		db.settled()
		kept := db.rowCount() == 1
		if err == nil && !kept {
			t.Fatalf("run %d: the write was reported as made and the database holds %d rows after %d commits", i, db.rowCount(), db.commits.Load())
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("run %d: the write failed with %v", i, err)
		}
	}
}

// A database that never says how a commit went leaves its caller waiting, for
// no longer than the context lasts, and never with a write reported as made.
func TestWriteWaitsForCommitToBeAnswered(t *testing.T) {
	db := newMemoryDatabase()
	db.commitUnheard = true
	DB := sql.Open[bank](db)
	ctx, cancel := context.WithTimeout(testContext(t), 50*time.Millisecond)
	defer cancel()
	if err := DB.Accounts.Set(ctx, "a", account{Name: "a"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a write whose commit was never answered gave %v, want the end of its context", err)
	}
	ctx, cancel = context.WithTimeout(testContext(t), 50*time.Millisecond)
	defer cancel()
	err := DB.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
		return DB.Accounts.Set(ctx, "b", account{Name: "b"})
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a transaction whose commit was never answered gave %v, want the end of its context", err)
	}
}

// A failed commit is the last thing a transaction is sent: no rollback is
// announced after it, and the channel is closed.
func TestNothingIsSentAfterCommit(t *testing.T) {
	failure := errors.New("deferred constraint")
	for name, answer := range map[string]error{"made": nil, "failed": failure} {
		t.Run(name, func(t *testing.T) {
			db := &answeringDatabase{answer: answer}
			m := sql.OpenTable[int, int](db, sodium.Table{})
			err := m.Manage(testContext(t), 0, func(ctx context.Context) error { return m.Set(ctx, 1, 10) })
			if !errors.Is(err, answer) || (answer == nil) != (err == nil) {
				t.Errorf("the transaction gave %v, want %v", err, answer)
			}
			db.settled()
			var kinds []string
			for job := range db.channels[0] {
				switch job.(type) {
				case nil:
					kinds = append(kinds, "rollback")
				case *sodium.Commit:
					kinds = append(kinds, "commit")
				default:
					kinds = append(kinds, "job")
				}
			}
			if got := strings.Join(kinds, " "); got != "job commit" {
				t.Errorf("the transaction was sent: %s", got)
			}
		})
	}
}

// answeringDatabase answers every commit the same way.
type answeringDatabase struct {
	transactionDatabase
	answer error
}

func (db *answeringDatabase) Manage(context.Context, sodium.Transaction) (chan<- sodium.Job, error) {
	jobs := make(chan sodium.Job)
	sent := make(chan sodium.Job, 16)
	db.channels = append(db.channels, sent)
	db.running.Add(1)
	go func() {
		defer db.running.Done()
		defer close(sent)
		for job := range jobs {
			sent <- job
			if commit, ok := job.(*sodium.Commit); ok {
				commit.Done(db.answer)
			}
		}
	}()
	return jobs, nil
}

// sameTables reports whether two snapshots of a database hold the same rows.
func sameTables(a, b map[string][]memoryRow) bool {
	if len(a) != len(b) {
		return false
	}
	for name, rows := range a {
		if len(rows) != len(b[name]) {
			return false
		}
		for _, row := range rows {
			if !slices.ContainsFunc(b[name], func(other memoryRow) bool { return maps.EqualFunc(row, other, sameText) }) {
				return false
			}
		}
	}
	return true
}

func sameText(a, b sodium.Value) bool { return a.String() == b.String() }
