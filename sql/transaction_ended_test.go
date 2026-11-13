package sql_test

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/iqhive/runtime.link/sql"
)

// operations are what can be done with a context, each reporting its error.
func operations(DB *bank) map[string]func(context.Context) error {
	older := func(a *account) sql.Patch { a.Age++; return nil }
	return map[string]func(context.Context) error{
		"Set":    func(ctx context.Context) error { return DB.Accounts.Set(ctx, "late", account{Name: "late"}) },
		"Insert": func(ctx context.Context) error { return DB.Accounts.Insert(ctx, "later", sql.Create, account{}) },
		"Add": func(ctx context.Context) error {
			// Told apart from the error a bank gives for having no key to add at.
			if _, err := DB.Accounts.Add(ctx, account{Name: "late"}); !errors.Is(err, sql.ErrInsertOnly) {
				return err
			}
			return nil
		},
		"Get":    func(ctx context.Context) error { _, err := DB.Accounts.Get(ctx, "a"); return err },
		"Lookup": func(ctx context.Context) error { _, _, err := DB.Accounts.Lookup(ctx, "a"); return err },
		"First":  func(ctx context.Context) error { _, _, _, err := DB.Accounts.First(ctx, nil); return err },
		"Search": func(ctx context.Context) error {
			var issue error
			for range DB.Accounts.Search(ctx, nil, &issue) {
			}
			return issue
		},
		"Count":        func(ctx context.Context) error { _, err := DB.Accounts.Count(ctx, nil); return err },
		"Length":       func(ctx context.Context) error { _, err := DB.Accounts.Length(ctx); return err },
		"Delete":       func(ctx context.Context) error { _, err := DB.Accounts.Delete(ctx, "a", nil); return err },
		"UnsafeDelete": func(ctx context.Context) error { _, err := DB.Accounts.UnsafeDelete(ctx, everyAccount); return err },
		"Update":       func(ctx context.Context) error { _, err := DB.Accounts.Update(ctx, everyAccount, older); return err },
		"Mutate":       func(ctx context.Context) error { _, err := DB.Accounts.Mutate(ctx, "a", nil, older); return err },
		"Manage": func(ctx context.Context) error {
			return DB.Accounts.Manage(ctx, 0, func(ctx context.Context) error {
				return errors.New("the function of a transaction that cannot be was run")
			})
		},
	}
}

// The context of a transaction outlives it: in a goroutine fn started, or in
// whatever fn handed back. With a database, nothing is done with it once the
// transaction is over, however that ended, and the caller is told why. Without
// one, it is used as it always has been, see TestRAMContextOfTransactionThatEnded.
func TestContextOfTransactionThatEnded(t *testing.T) {
	failure := errors.New("changed my mind")
	for name, end := range map[string]func() error{
		"committed":   func() error { return nil },
		"rolled back": func() error { return failure },
		"panicked":    func() error { panic(failure) },
	} {
		t.Run(name, func(t *testing.T) {
			func(DB *bank) {
				var ended context.Context
				func() {
					defer func() { recover() }()
					DB.Accounts.Manage(testContext(t), 0, func(ctx context.Context) error {
						ended = ctx
						return end()
					})
				}()
				before := accounts(t, DB)
				for what, operation := range operations(DB) {
					func() {
						defer func() {
							if recovered := recover(); recovered != nil {
								t.Errorf("%s with the context of a transaction that is over panicked: %v", what, recovered)
							}
						}()
						if err := operation(ended); !errors.Is(err, sql.ErrTransactionEnded) {
							t.Errorf("%s with the context of a transaction that is over gave %v, want ErrTransactionEnded", what, err)
						}
					}()
				}
				after := accounts(t, DB)
				if len(after) != len(before) {
					t.Fatalf("the accounts were %v and are %v", before, after)
				}
				for key, row := range before {
					if after[key] != row {
						t.Errorf("%s was %+v and is %+v", key, row, after[key])
					}
				}
				// The Map is as good as it was with any other context.
				for what, operation := range operations(DB) {
					if what == "Manage" {
						continue
					}
					if err := operation(testContext(t)); err != nil {
						t.Errorf("%s with a context of its own gave %v", what, err)
					}
				}
			}(sql.Open[bank](newMemoryDatabase()))
		})
	}
}

// Without a database, the context of a transaction that is over writes as any
// other context does, as it always has.
func TestRAMContextOfTransactionThatEnded(t *testing.T) {
	DB := sql.Open[bank](sql.New())
	var ended context.Context
	err := DB.Accounts.Manage(testContext(t), 0, func(ctx context.Context) error {
		ended = ctx
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := DB.Accounts.Set(ended, "late", account{Name: "late"}); err != nil {
		t.Fatalf("a write with the context of a transaction that is over gave %v", err)
	}
	if got, err := DB.Accounts.Get(testContext(t), "late"); got.Name != "late" || err != nil {
		t.Errorf("the write was not kept: %+v, %v", got, err)
	}
	if err := DB.Accounts.Manage(ended, 0, func(ctx context.Context) error {
		return DB.Accounts.Set(ctx, "nested", account{Name: "nested"})
	}); err != nil {
		t.Errorf("a transaction nested in one that is over gave %v", err)
	}
}

// A search that fn hands back unread is read with the context of a transaction
// that is over. With a database its rows were to come from that transaction,
// and the search says so. Without one the rows are in memory, and it gives them.
func TestSearchHandedOutOfTransaction(t *testing.T) {
	t.Run("Database", func(t *testing.T) {
		db := newMemoryDatabase()
		DB := sql.Open[bank](db)
		if err := DB.Accounts.Set(testContext(t), "a", account{Name: "a"}); err != nil {
			t.Fatal(err)
		}
		var issue error
		var rows iter.Seq2[string, account]
		err := DB.Accounts.Manage(testContext(t), 0, func(ctx context.Context) error {
			rows = DB.Accounts.Search(ctx, nil, &issue)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		for key := range rows {
			t.Errorf("a search of a transaction that is over gave the row %s", key)
		}
		if !errors.Is(issue, sql.ErrTransactionEnded) {
			t.Errorf("a search of a transaction that is over left %v, want ErrTransactionEnded", issue)
		}
	})
	t.Run("RAM", func(t *testing.T) {
		var m sql.Map[string, account]
		if err := m.Set(testContext(t), "a", account{Name: "a"}); err != nil {
			t.Fatal(err)
		}
		var issue error
		var rows iter.Seq2[string, account]
		err := m.Manage(testContext(t), 0, func(ctx context.Context) error {
			rows = m.Search(ctx, nil, &issue)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		found := 0
		for range rows {
			found++
		}
		if found != 1 || issue != nil {
			t.Errorf("the search gave %d rows and %v", found, issue)
		}
	})
}

// A transaction on one database that is over does not stand in the way of
// another database, or of maps without one, which were never part of it.
func TestContextOfEndedTransactionOnAnotherDatabase(t *testing.T) {
	ctx := testContext(t)
	A, B := sql.Open[bank](newMemoryDatabase()), sql.Open[bank](newMemoryDatabase())
	var memory sql.Map[string, account]
	var ended context.Context
	if err := A.Accounts.Manage(ctx, 0, func(ctx context.Context) error { ended = ctx; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := B.Accounts.Set(ended, "b", account{Name: "b"}); err != nil {
		t.Errorf("a write to another database gave %v", err)
	}
	if err := memory.Set(ended, "m", account{Name: "m"}); err != nil {
		t.Errorf("a write to a map without a database gave %v", err)
	}
	if err := A.Accounts.Set(ended, "a", account{Name: "a"}); !errors.Is(err, sql.ErrTransactionEnded) {
		t.Errorf("a write to the database of the transaction gave %v, want ErrTransactionEnded", err)
	}
}
