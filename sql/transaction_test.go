package sql_test

import (
	"context"
	"errors"
	"testing"

	"github.com/iqhive/runtime.link/sql"
	"github.com/iqhive/runtime.link/sql/std/sodium"
)

type transactionDatabase struct {
	sodium.Database
	channels []chan sodium.Job
}

func (d *transactionDatabase) Manage(context.Context, sodium.Transaction) (chan<- sodium.Job, error) {
	ch := make(chan sodium.Job, 16)
	d.channels = append(d.channels, ch)
	return ch, nil
}
func (*transactionDatabase) Insert(sodium.Table, []sodium.Value, bool, sodium.Query, []sodium.Value) sodium.Job {
	return transactionJob{}
}

type transactionJob struct{}

func (transactionJob) Wait(context.Context) (int, error) { return 1, nil }

func TestManageTransactionLifetime(t *testing.T) {
	for _, outcome := range []string{"commit", "error", "panic"} {
		t.Run(outcome, func(t *testing.T) {
			db := new(transactionDatabase)
			m := sql.OpenTable[int, int](db, sodium.Table{})
			failure := errors.New("callback failed")
			var err error
			var recovered any
			func() {
				defer func() { recovered = recover() }()
				err = m.Manage(context.Background(), 0, func(ctx context.Context) error {
					if err := m.Set(ctx, 1, 10); err != nil {
						return err
					}
					if err := m.Manage(ctx, 0, func(ctx context.Context) error { return m.Set(ctx, 2, 20) }); err != nil {
						return err
					}
					switch outcome {
					case "error":
						return failure
					case "panic":
						panic(failure)
					}
					return nil
				})
			}()
			if outcome == "panic" {
				if recovered != failure {
					t.Fatalf("panic = %v, want callback panic", recovered)
				}
			} else if recovered != nil {
				t.Fatalf("unexpected panic: %v", recovered)
			}
			if outcome == "error" && !errors.Is(err, failure) {
				t.Fatalf("error = %v", err)
			}
			if outcome == "commit" && err != nil {
				t.Fatal(err)
			}
			if len(db.channels) != 1 {
				t.Fatalf("opened %d transactions, want 1", len(db.channels))
			}
			var jobs, rollbacks int
			for {
				select {
				case job, open := <-db.channels[0]:
					if !open {
						if jobs != 2 {
							t.Errorf("jobs = %d, want 2", jobs)
						}
						want := 0
						if outcome != "commit" {
							want = 1
						}
						if rollbacks != want {
							t.Errorf("rollbacks = %d, want %d", rollbacks, want)
						}
						return
					}
					if job == nil {
						rollbacks++
					} else {
						jobs++
					}
				default:
					t.Fatal("transaction was not closed")
				}
			}
		})
	}
}

func TestStandaloneWriteCommits(t *testing.T) {
	db := new(transactionDatabase)
	m := sql.OpenTable[int, int](db, sodium.Table{})
	if err := m.Set(context.Background(), 1, 10); err != nil {
		t.Fatal(err)
	}
	if job := <-db.channels[0]; job == nil {
		t.Fatal("missing insert")
	}
	if _, open := <-db.channels[0]; open {
		t.Fatal("transaction was not closed")
	}
}
