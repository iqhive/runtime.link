package sql_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

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

// ramContents reads every row of the map without assuming how it got there.
func ramContents(t *testing.T, ctx context.Context, m *sql.Map[int, int]) map[int]int {
	t.Helper()
	var issue error
	contents := make(map[int]int)
	for k, v := range m.Search(ctx, nil, &issue) {
		contents[k] = v
	}
	if issue != nil {
		t.Fatal(issue)
	}
	return contents
}

func TestManageRAMRollsBackEveryWrite(t *testing.T) {
	all := func(*int, *int) sql.Query { return sql.Query{sql.Slice(0, 10)} }
	set := func(v *int) sql.Patch { return sql.Patch{sql.Set(v, 99)} }
	writes := map[string]func(ctx context.Context, m *sql.Map[int, int]) error{
		"InsertNew": func(ctx context.Context, m *sql.Map[int, int]) error { return m.Insert(ctx, 3, sql.Create, 30) },
		"Upsert":    func(ctx context.Context, m *sql.Map[int, int]) error { return m.Set(ctx, 1, 99) },
		"Add": func(ctx context.Context, m *sql.Map[int, int]) error {
			// Keys are numbered from 1, and Add refuses the ones already used.
			for {
				if _, err := m.Add(ctx, 99); err != sql.ErrDuplicate {
					return err
				}
			}
		},
		"Update": func(ctx context.Context, m *sql.Map[int, int]) error { _, err := m.Update(ctx, all, set); return err },
		"Mutate": func(ctx context.Context, m *sql.Map[int, int]) error {
			_, err := m.Mutate(ctx, 1, nil, set)
			return err
		},
		"Delete": func(ctx context.Context, m *sql.Map[int, int]) error { _, err := m.Delete(ctx, 1, nil); return err },
		"UnsafeDelete": func(ctx context.Context, m *sql.Map[int, int]) error {
			_, err := m.UnsafeDelete(ctx, all)
			return err
		},
		"Repeated": func(ctx context.Context, m *sql.Map[int, int]) error {
			if err := m.Set(ctx, 1, 98); err != nil {
				return err
			}
			if _, err := m.Delete(ctx, 1, nil); err != nil {
				return err
			}
			return m.Set(ctx, 1, 99)
		},
	}
	want := map[int]int{1: 10, 2: 20}
	for name, write := range writes {
		for _, outcome := range []string{"error", "panic"} {
			t.Run(name+"/"+outcome, func(t *testing.T) {
				ctx := context.Background()
				var m sql.Map[int, int]
				for k, v := range want {
					if err := m.Set(ctx, k, v); err != nil {
						t.Fatal(err)
					}
				}
				failure := errors.New("failed")
				var err error
				recovered := func() (recovered any) {
					defer func() { recovered = recover() }()
					err = m.Manage(ctx, sql.ReadWrites, func(ctx context.Context) error {
						if err := write(ctx, &m); err != nil {
							t.Error(err)
						}
						if reflect.DeepEqual(ramContents(t, ctx, &m), want) {
							t.Error("the write did not happen")
						}
						if outcome == "panic" {
							panic(failure)
						}
						return failure
					})
					return nil
				}()
				if outcome == "panic" && recovered != failure {
					t.Fatalf("panic = %v", recovered)
				}
				if outcome == "error" && !errors.Is(err, failure) {
					t.Fatalf("error = %v", err)
				}
				if got := ramContents(t, ctx, &m); !reflect.DeepEqual(got, want) {
					t.Fatalf("got %v, want %v", got, want)
				}
			})
		}
	}
}

func TestManageRAMRollsBackScopedUpsert(t *testing.T) {
	ctx := context.Background()
	var m sql.Map[int, scopedNote]
	scoped := sql.Scope(ctx, "group", "a")
	before := scopedNote{Text: "before", Group: "a"}
	if err := m.Set(scoped, 1, before); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("failed")
	err := m.Manage(scoped, sql.ReadWrites, func(ctx context.Context) error {
		if err := m.Set(ctx, 1, scopedNote{Text: "after", Group: "a"}); err != nil {
			return err
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if got, err := m.Get(scoped, 1); err != nil || got != before {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestManageRAMCommits(t *testing.T) {
	ctx := context.Background()
	var m sql.Map[int, int]
	err := m.Manage(ctx, sql.ReadWrites, func(ctx context.Context) error { return m.Set(ctx, 1, 10) })
	if err != nil {
		t.Fatal(err)
	}
	if got := ramContents(t, ctx, &m); !reflect.DeepEqual(got, map[int]int{1: 10}) {
		t.Fatalf("got %v", got)
	}
}

func TestManageRAMNestedJoinsOuterTransaction(t *testing.T) {
	failure := errors.New("failed")
	for _, test := range []struct {
		name   string
		inner  error
		outer  error
		wanted map[int]int
	}{
		{name: "BothCommit", wanted: map[int]int{1: 1}},
		{name: "OuterFails", outer: failure, wanted: map[int]int{}},
		{name: "InnerFails", inner: failure, outer: failure, wanted: map[int]int{}},
		{name: "InnerFailureHandled", inner: failure, wanted: map[int]int{1: 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var a, b sql.Map[int, int]
			finished := make(chan error, 1)
			go func() {
				finished <- a.Manage(ctx, sql.ReadWrites, func(ctx context.Context) error {
					err := b.Manage(ctx, sql.ReadWrites, func(ctx context.Context) error {
						if err := a.Set(ctx, 1, 1); err != nil {
							return err
						}
						if err := b.Set(ctx, 1, 1); err != nil {
							return err
						}
						return test.inner
					})
					if !errors.Is(err, test.inner) {
						t.Errorf("inner error = %v", err)
					}
					return test.outer
				})
			}()
			select {
			case err := <-finished:
				if !errors.Is(err, test.outer) {
					t.Fatalf("error = %v", err)
				}
			case <-ctx.Done():
				t.Fatal("nested transactions never finished")
			}
			if got := ramContents(t, ctx, &a); !reflect.DeepEqual(got, test.wanted) {
				t.Errorf("first map holds %v, want %v", got, test.wanted)
			}
			if got := ramContents(t, ctx, &b); !reflect.DeepEqual(got, test.wanted) {
				t.Errorf("second map holds %v, want %v", got, test.wanted)
			}
		})
	}
}

func TestManageRAMWaitEndsWithContext(t *testing.T) {
	ctx := context.Background()
	var m sql.Map[int, int]
	finished := make(chan error, 1)
	go func() {
		finished <- m.Manage(ctx, sql.ReadWrites, func(context.Context) error {
			timed, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
			defer cancel()
			// Not the context of the transaction, so this is a second transaction.
			return m.Manage(timed, sql.ReadWrites, func(context.Context) error { return nil })
		})
	}()
	select {
	case err := <-finished:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a transaction waited past the end of its context")
	}
	if err := m.Manage(ctx, sql.ReadWrites, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestManageRAMTransactionsRunOneAtATime(t *testing.T) {
	const transactions = 16
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var m sql.Map[int, int]
	if err := m.Set(ctx, 1, 0); err != nil {
		t.Fatal(err)
	}
	var running sync.WaitGroup
	for range transactions {
		running.Add(1)
		go func() {
			defer running.Done()
			err := m.Manage(ctx, sql.ReadWrites, func(ctx context.Context) error {
				value, err := m.Get(ctx, 1)
				if err != nil {
					return err
				}
				return m.Set(ctx, 1, value+1)
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	running.Wait()
	if got, _ := m.Get(ctx, 1); got != transactions {
		t.Fatalf("got %d, want %d", got, transactions)
	}
}

func TestManageRAMRollbackKeepsWritesFromOutside(t *testing.T) {
	ctx := context.Background()
	var m sql.Map[int, int]
	if err := m.Set(ctx, 1, 10); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("failed")
	err := m.Manage(ctx, sql.ReadWrites, func(inner context.Context) error {
		if err := m.Set(inner, 1, 11); err != nil {
			return err
		}
		if err := m.Set(inner, 2, 21); err != nil {
			return err
		}
		// Acknowledged outside of the transaction, so a rollback must not undo it.
		if err := m.Set(ctx, 1, 12); err != nil {
			return err
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if got := ramContents(t, ctx, &m); !reflect.DeepEqual(got, map[int]int{1: 12}) {
		t.Fatalf("got %v", got)
	}
}

func TestManageRAMRollbackKeepsOutsideDeletion(t *testing.T) {
	ctx := context.Background()
	var m sql.Map[int, int]
	if err := m.Set(ctx, 1, 10); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("rollback")
	err := m.Manage(ctx, sql.ReadWrites, func(tx context.Context) error {
		if ok, err := m.Delete(tx, 1, nil); err != nil || !ok {
			t.Fatalf("transaction delete: %v, %v", ok, err)
		}
		if err := m.Set(ctx, 1, 20); err != nil {
			return err
		}
		if ok, err := m.Delete(ctx, 1, nil); err != nil || !ok {
			t.Fatalf("outside delete: %v, %v", ok, err)
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if value, exists, err := m.Lookup(ctx, 1); err != nil || exists {
		t.Fatalf("deleted row resurrected: %v, %v, %v", value, exists, err)
	}
}

func TestManageRAMRollbackReferencePatch(t *testing.T) {
	type row struct {
		Slice   []int
		Map     map[string][]int
		Pointer *int
		Any     any
	}
	for _, update := range []bool{false, true} {
		for _, panicPatch := range []bool{false, true} {
			t.Run(fmt.Sprintf("update=%v/panic=%v", update, panicPatch), func(t *testing.T) {
				ctx := context.Background()
				var m sql.Map[int, row]
				n := 10
				original := row{Slice: []int{10}, Map: map[string][]int{"a": {10}}, Pointer: &n, Any: []int{10}}
				if err := m.Set(ctx, 1, original); err != nil {
					t.Fatal(err)
				}
				failure := errors.New("rollback")
				patch := func(v *row) sql.Patch {
					v.Slice[0] = 99
					v.Map["a"][0] = 99
					*v.Pointer = 99
					v.Any.([]int)[0] = 99
					if panicPatch {
						panic(failure)
					}
					return nil
				}
				func() {
					defer func() {
						if got := recover(); got != nil && (!panicPatch || got != failure) {
							panic(got)
						}
					}()
					err := m.Manage(ctx, sql.ReadWrites, func(tx context.Context) error {
						if update {
							_, err := m.Update(tx, func(*int, *row) sql.Query { return sql.Query{sql.Slice(0, 1)} }, patch)
							if err != nil {
								return err
							}
						} else {
							if ok, err := m.Mutate(tx, 1, nil, patch); err != nil || !ok {
								t.Fatalf("Mutate: %v, %v", ok, err)
							}
						}
						current, err := m.Get(tx, 1)
						if err != nil || current.Slice[0] != 99 || current.Map["a"][0] != 99 || *current.Pointer != 99 || current.Any.([]int)[0] != 99 {
							t.Fatalf("patch was not applied: %+v, %v", current, err)
						}
						return failure
					})
					if panicPatch {
						t.Fatal("expected patch panic")
					}
					if !errors.Is(err, failure) {
						t.Fatal(err)
					}
				}()
				got, err := m.Get(ctx, 1)
				wantN := 10
				want := row{Slice: []int{10}, Map: map[string][]int{"a": {10}}, Pointer: &wantN, Any: []int{10}}
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("rollback value: %+v, %v; want %+v", got, err, want)
				}
			})
		}
	}
}
