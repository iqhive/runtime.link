package sql_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/iqhive/runtime.link/sql"
	"github.com/iqhive/runtime.link/sql/std/sodium"
	"github.com/iqhive/runtime.link/xyz"
)

func everyAccount(*string, *account) sql.Query { return sql.Query{sql.Slice(0, 10)} }

func accounts(t *testing.T, DB *bank) map[string]account {
	t.Helper()
	var issue error
	found := make(map[string]account)
	for key, value := range DB.Accounts.Search(testContext(t), nil, &issue) {
		found[key] = value
	}
	if issue != nil {
		t.Fatal(issue)
	}
	return found
}

// A patch changes a value by assigning to it as well as with Set, and an
// update applies either the same way with and without a database.
func TestUpdateAppliesPatchThatAssigns(t *testing.T) {
	for name, test := range map[string]struct {
		patch sql.PatchFunc[account]
		want  func(account) account
	}{
		"assign": {
			func(a *account) sql.Patch { a.Age = 99; return nil },
			func(a account) account { a.Age = 99; return a },
		},
		"assign zero": {
			func(a *account) sql.Patch { a.Age = 0; return nil },
			func(a account) account { a.Age = 0; return a },
		},
		"assign the whole value": {
			func(a *account) sql.Patch { *a = account{Name: "all", Age: 1}; return nil },
			func(account) account { return account{Name: "all", Age: 1} },
		},
		"set": {
			func(a *account) sql.Patch { return sql.Patch{sql.Set(&a.Age, 99)} },
			func(a account) account { a.Age = 99; return a },
		},
		"set zero": {
			func(a *account) sql.Patch { return sql.Patch{sql.Set(&a.Age, 0), sql.Set(&a.Name, "")} },
			func(account) account { return account{} },
		},
		"set and assign": {
			func(a *account) sql.Patch { a.Name = "renamed"; return sql.Patch{sql.Set(&a.Age, 1)} },
			func(account) account { return account{Name: "renamed", Age: 1} },
		},
		"nothing": {
			func(*account) sql.Patch { return nil },
			func(a account) account { return a },
		},
	} {
		t.Run(name, func(t *testing.T) {
			backends(t, func(t *testing.T, DB *bank) {
				before := accounts(t, DB)
				n, err := DB.Accounts.Update(testContext(t), everyAccount, test.patch)
				if n != 3 || err != nil {
					t.Fatalf("update gave %d and %v", n, err)
				}
				after := accounts(t, DB)
				for key, was := range before {
					if after[key] != test.want(was) {
						t.Errorf("%s is %+v, want %+v", key, after[key], test.want(was))
					}
				}
			})
		})
	}
}

// What a patch does to a value is only known by running it on that value, so
// an update runs it on each one, and gives the same with a database as without
// whatever the patch goes by.
func TestUpdateAppliesPatchThatDependsOnValue(t *testing.T) {
	for name, test := range map[string]struct {
		patch sql.PatchFunc[account]
		want  func(account) account
	}{
		"increment": {
			func(a *account) sql.Patch { a.Age++; return nil },
			func(a account) account { a.Age++; return a },
		},
		"set from the value": {
			func(a *account) sql.Patch { return sql.Patch{sql.Set(&a.Age, a.Age+1)} },
			func(a account) account { a.Age++; return a },
		},
		"append": {
			func(a *account) sql.Patch { a.Name += "!"; return nil },
			func(a account) account { a.Name += "!"; return a },
		},
		"upper case": {
			func(a *account) sql.Patch { a.Name = strings.ToUpper(a.Name); return nil },
			func(a account) account { a.Name = strings.ToUpper(a.Name); return a },
		},
		"threshold": {
			func(a *account) sql.Patch {
				if a.Age >= 2 {
					a.Name = "adult"
				}
				return nil
			},
			func(a account) account {
				if a.Age >= 2 {
					a.Name = "adult"
				}
				return a
			},
		},
		"clamp": {
			func(a *account) sql.Patch { a.Age = min(a.Age, 2); return nil },
			func(a account) account { a.Age = min(a.Age, 2); return a },
		},
		"one column from another": {
			func(a *account) sql.Patch { a.Name = strings.Repeat("x", a.Age); return nil },
			func(a account) account { a.Name = strings.Repeat("x", a.Age); return a },
		},
		"some values only": {
			func(a *account) sql.Patch {
				if a.Name == "b" {
					return sql.Patch{sql.Set(&a.Age, 0)}
				}
				return nil
			},
			func(a account) account {
				if a.Name == "b" {
					a.Age = 0
				}
				return a
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			backends(t, func(t *testing.T, DB *bank) {
				before := accounts(t, DB)
				n, err := DB.Accounts.Update(testContext(t), everyAccount, test.patch)
				if n != 3 || err != nil {
					t.Fatalf("update gave %d and %v", n, err)
				}
				after := accounts(t, DB)
				for key, was := range before {
					if after[key] != test.want(was) {
						t.Errorf("%s is %+v, want %+v", key, after[key], test.want(was))
					}
				}
			})
		})
	}
}

// A patch is caller's code, which can go by more than the value: an update
// runs it once for each value it finds, and writes what it made of each.
func TestUpdateRunsPatchOncePerValue(t *testing.T) {
	backends(t, func(t *testing.T, DB *bank) {
		runs := 0
		n, err := DB.Accounts.Update(testContext(t), everyAccount, func(a *account) sql.Patch {
			runs++
			a.Age = 100 + runs
			return nil
		})
		if n != 3 || err != nil || runs != 3 {
			t.Fatalf("update gave %d and %v after running the patch %d times", n, err, runs)
		}
		var ages []int
		for _, row := range accounts(t, DB) {
			ages = append(ages, row.Age)
		}
		slices.Sort(ages)
		if !slices.Equal(ages, []int{101, 102, 103}) {
			t.Errorf("the ages are %v", ages)
		}
	})
}

// An update patches the values its query finds, in its order and within its
// range, and no others.
func TestUpdatePatchesOnlyWhatTheQueryFinds(t *testing.T) {
	older := func(a *account) sql.Patch { a.Age += 10; return nil }
	for name, test := range map[string]struct {
		query sql.QueryFunc[string, account]
		want  map[string]int
	}{
		"key": {
			func(k *string, _ *account) sql.Query { return sql.Query{sql.Index(k).Equals("b"), sql.Slice(0, 10)} },
			map[string]int{"a": 1, "b": 12, "c": 3},
		},
		"where": {
			func(_ *string, a *account) sql.Query { return sql.Query{sql.Where(&a.Age).Min(2), sql.Slice(0, 10)} },
			map[string]int{"a": 1, "b": 12, "c": 13},
		},
		"the oldest": {
			func(_ *string, a *account) sql.Query {
				return sql.Query{sql.Order(&a.Age).Decreasing(), sql.Slice(0, 1)}
			},
			map[string]int{"a": 1, "b": 2, "c": 13},
		},
		"after the youngest": {
			func(_ *string, a *account) sql.Query {
				return sql.Query{sql.Order(&a.Age).Increasing(), sql.Slice(1, 3)}
			},
			map[string]int{"a": 1, "b": 12, "c": 13},
		},
		"nobody": {
			func(_ *string, a *account) sql.Query { return sql.Query{sql.Where(&a.Age).Min(50), sql.Slice(0, 10)} },
			map[string]int{"a": 1, "b": 2, "c": 3},
		},
	} {
		t.Run(name, func(t *testing.T) {
			backends(t, func(t *testing.T, DB *bank) {
				changed := 0
				for _, age := range test.want {
					if age > 10 {
						changed++
					}
				}
				n, err := DB.Accounts.Update(testContext(t), test.query, older)
				if n != changed || err != nil {
					t.Fatalf("update gave %d and %v, want %d", n, err, changed)
				}
				for key, row := range accounts(t, DB) {
					if row.Age != test.want[key] || row.Name != key {
						t.Errorf("%s is %+v, want the age %d", key, row, test.want[key])
					}
				}
			})
		})
	}
}

// levelDatabase keeps the level each of its transactions was started with.
type levelDatabase struct {
	*memoryDatabase
	levels *[]sodium.Transaction
}

func (db levelDatabase) Manage(ctx context.Context, level sodium.Transaction) (chan<- sodium.Job, error) {
	*db.levels = append(*db.levels, level)
	return db.memoryDatabase.Manage(ctx, level)
}

// The values are read and then written, so both are in one transaction that
// locks what it writes: nothing else changes a value in between, and either
// every value is updated or none.
func TestUpdateWithDatabaseIsOneLockingTransaction(t *testing.T) {
	var levels []sodium.Transaction
	db := newMemoryDatabase()
	DB := sql.Open[bank](levelDatabase{db, &levels})
	ctx := testContext(t)
	for i, name := range []string{"a", "b", "c"} {
		if err := DB.Accounts.Set(ctx, name, account{Name: name, Age: i}); err != nil {
			t.Fatal(err)
		}
	}
	db.settled()
	levels = nil
	ended, searches := db.commits.Load(), db.searches.Load()
	n, err := DB.Accounts.Update(ctx, everyAccount, func(a *account) sql.Patch { a.Age++; return nil })
	if n != 3 || err != nil {
		t.Fatalf("update gave %d and %v", n, err)
	}
	db.settled()
	if !slices.Equal(levels, []sodium.Transaction{sql.ReadWrites | sql.LockWrites}) {
		t.Errorf("the update ran in transactions of the levels %v, want one that reads its writes and locks them", levels)
	}
	if made := db.commits.Load() - ended; made != 1 {
		t.Errorf("the update ended %d transactions with a commit, want one", made)
	}
	if read := db.searches.Load() - searches; read != 1 {
		t.Errorf("the update searched %d times, want once", read)
	}
}

// brokenDatabase cannot make an update once it has made as many as it is good for.
type brokenDatabase struct {
	*memoryDatabase
	good *atomic.Int32
}

var errBrokenUpdate = errors.New("disk full")

func (db brokenDatabase) Update(table sodium.Table, query sodium.Query, patch sodium.Patch) sodium.Job {
	inner := db.memoryDatabase.Update(table, query, patch).(*memoryJob)
	return newMemoryWrite(func(ctx context.Context) (int, error) {
		if db.good.Add(-1) < 0 {
			return 0, errBrokenUpdate
		}
		inner.execute(ctx)
		return inner.count, inner.err
	})
}

// An update that fails part of the way leaves no value updated. Inside of the
// caller's transaction that is the caller's to see to, with the error.
func TestUpdateThatFailsUpdatesNothing(t *testing.T) {
	older := func(a *account) sql.Patch { a.Age += 10; return nil }
	fill := func(t *testing.T) (*memoryDatabase, *bank) {
		db := newMemoryDatabase()
		good := new(atomic.Int32)
		good.Store(2)
		DB := sql.Open[bank](brokenDatabase{db, good})
		for i, name := range []string{"a", "b", "c"} {
			if err := DB.Accounts.Set(testContext(t), name, account{Name: name, Age: i + 1}); err != nil {
				t.Fatal(err)
			}
		}
		return db, DB
	}
	updated := func(t *testing.T, DB *bank) int {
		t.Helper()
		count := 0
		for _, row := range accounts(t, DB) {
			if row.Age > 10 {
				count++
			}
		}
		return count
	}
	t.Run("on its own", func(t *testing.T) {
		db, DB := fill(t)
		n, err := DB.Accounts.Update(testContext(t), everyAccount, older)
		if n != 0 || !errors.Is(err, errBrokenUpdate) {
			t.Fatalf("update gave %d and %v", n, err)
		}
		db.settled()
		if left := updated(t, DB); left != 0 {
			t.Errorf("%d values were left updated", left)
		}
	})
	t.Run("in a transaction that fails with it", func(t *testing.T) {
		db, DB := fill(t)
		err := DB.Accounts.Manage(testContext(t), 0, func(ctx context.Context) error {
			_, err := DB.Accounts.Update(ctx, everyAccount, older)
			return err
		})
		if !errors.Is(err, errBrokenUpdate) {
			t.Fatalf("the transaction gave %v", err)
		}
		db.settled()
		if left := updated(t, DB); left != 0 {
			t.Errorf("%d values were left updated", left)
		}
	})
	t.Run("in a transaction that goes on", func(t *testing.T) {
		db, DB := fill(t)
		err := DB.Accounts.Manage(testContext(t), 0, func(ctx context.Context) error {
			if _, err := DB.Accounts.Update(ctx, everyAccount, older); !errors.Is(err, errBrokenUpdate) {
				return fmt.Errorf("update gave %v", err)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		db.settled()
		if kept := updated(t, DB); kept != 2 {
			t.Errorf("%d values are updated, want the 2 that the transaction wrote before the failure and then kept", kept)
		}
	})
}

// An update inside of a transaction is part of it.
func TestUpdateJoinsTransaction(t *testing.T) {
	backends(t, func(t *testing.T, DB *bank) {
		failure := errors.New("changed my mind")
		err := DB.Accounts.Manage(testContext(t), 0, func(ctx context.Context) error {
			n, err := DB.Accounts.Update(ctx, everyAccount, func(a *account) sql.Patch { a.Age *= 10; return nil })
			if n != 3 || err != nil {
				return fmt.Errorf("update gave %d and %v", n, err)
			}
			// The transaction reads what it wrote.
			if got, err := DB.Accounts.Get(ctx, "c"); got.Age != 30 || err != nil {
				return fmt.Errorf("inside of the transaction c is %+v, %v", got, err)
			}
			return failure
		})
		if !errors.Is(err, failure) {
			t.Fatal(err)
		}
		for key, row := range accounts(t, DB) {
			if row.Age >= 10 {
				t.Errorf("%s is %+v after the rollback", key, row)
			}
		}
	})
}

type updateDocument struct {
	Title string
	Team  string `sql:"team,scoped"`
}

type updateDocuments struct {
	Documents sql.Map[string, updateDocument] `sql:"update_documents"`
}

// A patch is refused for what it does to a value, whatever it goes by to do
// it, and with a database a refusal leaves every value as it was: the ones
// patched before the one that was refused too.
func TestUpdateRefusesPatchThatLeavesScopeForSomeValue(t *testing.T) {
	everyDocument := func(*string, *updateDocument) sql.Query { return sql.Query{sql.Slice(0, 10)} }
	moveSecond := func(d *updateDocument) sql.Patch {
		d.Title += "!"
		if d.Title == "two!" {
			d.Team = "blue"
		}
		return nil
	}
	fill := func(t *testing.T, DB *updateDocuments) context.Context {
		admin := sql.Admin(testContext(t), "testing")
		for id, doc := range map[string]updateDocument{
			"r1": {Title: "one", Team: "red"},
			"r2": {Title: "two", Team: "red"},
			"r3": {Title: "three", Team: "red"},
			"b1": {Title: "two", Team: "blue"},
		} {
			if err := DB.Documents.Insert(admin, id, sql.Create, doc); err != nil {
				t.Fatal(err)
			}
		}
		return admin
	}
	t.Run("Database", func(t *testing.T) {
		db := newMemoryDatabase()
		DB := sql.Open[updateDocuments](db)
		admin := fill(t, DB)
		red := sql.Scope(testContext(t), "team", "red")
		n, err := DB.Documents.Update(red, everyDocument, moveSecond)
		if n != 0 || !errors.Is(err, sql.ErrAccessDenied) {
			t.Fatalf("update gave %d and %v, want ErrAccessDenied", n, err)
		}
		db.settled()
		if updates := db.updates.Load(); updates != 0 {
			t.Errorf("the database was sent %d updates for a patch that was refused", updates)
		}
		var issue error
		for id, doc := range DB.Documents.Search(admin, nil, &issue) {
			if strings.HasSuffix(doc.Title, "!") || (doc.Team == "blue") != (id == "b1") {
				t.Errorf("%s is %+v after the refusal", id, doc)
			}
		}
		if issue != nil {
			t.Fatal(issue)
		}
	})
	t.Run("RAM", func(t *testing.T) {
		DB := sql.Open[updateDocuments](sql.New())
		admin := fill(t, DB)
		red := sql.Scope(testContext(t), "team", "red")
		if _, err := DB.Documents.Update(red, everyDocument, moveSecond); !errors.Is(err, sql.ErrAccessDenied) {
			t.Fatalf("update gave %v, want ErrAccessDenied", err)
		}
		if doc, err := DB.Documents.Get(admin, "r2"); doc != (updateDocument{Title: "two", Team: "red"}) || err != nil {
			t.Errorf("the value whose patch was refused is %+v, %v", doc, err)
		}
	})
	// The value the patch would move is not one the query finds, so it moves none.
	eachBackend(t, func(t *testing.T, DB *updateDocuments) {
		admin := fill(t, DB)
		red := sql.Scope(testContext(t), "team", "red")
		n, err := DB.Documents.Update(red, func(k *string, _ *updateDocument) sql.Query {
			return sql.Query{sql.Index(k).Equals("r1"), sql.Slice(0, 10)}
		}, moveSecond)
		if n != 1 || err != nil {
			t.Fatalf("update gave %d and %v", n, err)
		}
		if doc, err := DB.Documents.Get(admin, "r1"); doc != (updateDocument{Title: "one!", Team: "red"}) || err != nil {
			t.Errorf("r1 is %+v, %v", doc, err)
		}
		// The blue value with the title that the patch goes by is out of scope.
		if doc, err := DB.Documents.Get(admin, "b1"); doc != (updateDocument{Title: "two", Team: "blue"}) || err != nil {
			t.Errorf("b1 is %+v, %v", doc, err)
		}
	})
}

// The patch of an update is not run on anything that other updates share.
// Run with -race.
func TestUpdatesWithAssigningPatchesDoNotShare(t *testing.T) {
	db := newMemoryDatabase()
	DB := sql.Open[bank](db)
	ctx := testContext(t)
	for _, name := range []string{"a", "b"} {
		if err := DB.Accounts.Set(ctx, name, account{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	only := func(name string) sql.QueryFunc[string, account] {
		return func(k *string, _ *account) sql.Query {
			return sql.Query{sql.Index(k).Equals(name), sql.Slice(0, 1)}
		}
	}
	var updates sync.WaitGroup
	failures := make([]error, 2)
	for i, name := range []string{"a", "b"} {
		updates.Add(1)
		go func() {
			defer updates.Done()
			for age := 1; age <= 50; age++ {
				_, err := DB.Accounts.Update(ctx, only(name), func(a *account) sql.Patch { a.Age = age * (i + 1); return nil })
				failures[i] = errors.Join(failures[i], err)
			}
		}()
	}
	updates.Wait()
	if err := errors.Join(failures...); err != nil {
		t.Fatal(err)
	}
	if got := accounts(t, DB); got["a"].Age != 50 || got["b"].Age != 100 || got["a"].Name != "a" || got["b"].Name != "b" {
		t.Errorf("the accounts are %+v", got)
	}
	// The sentinal that queries are built from was not written to either.
	n, err := DB.Accounts.Count(ctx, func(_ *string, a *account) sql.Query { return sql.Query{sql.Empty(&a.Age)} })
	if n != 0 || err != nil {
		t.Errorf("counted %d accounts without an age, %v", n, err)
	}
}

// A patch can also return modifications it made for itself, which a database
// is still sent.
func TestUpdateKeepsModificationsMadeByHand(t *testing.T) {
	db := newMemoryDatabase()
	DB := sql.Open[bank](db)
	ctx := testContext(t)
	if err := DB.Accounts.Set(ctx, "a", account{Name: "a", Age: 1}); err != nil {
		t.Fatal(err)
	}
	byHand := sodium.Modifications.Set.As(xyz.NewPair(sodium.Column{Name: "age"}, sodium.Values.Int64.As(7)))
	n, err := DB.Accounts.Update(ctx, everyAccount, func(a *account) sql.Patch {
		return sql.Patch{sql.Set(&a.Name, "renamed"), byHand}
	})
	if n != 1 || err != nil {
		t.Fatalf("update gave %d and %v", n, err)
	}
	if got, err := DB.Accounts.Get(ctx, "a"); got != (account{Name: "renamed", Age: 7}) || err != nil {
		t.Errorf("the row is %+v, %v", got, err)
	}
}

// What a patch sets is found for every kind of column, and a column it leaves
// alone is not sent.
func TestUpdateSendsOnlyColumnsThePatchSets(t *testing.T) {
	type inner struct {
		On    bool
		Ratio float64
	}
	type row struct {
		Label string
		Data  []byte
		Inner inner
		Pair  [2]uint8
		Kept  string
	}
	db := newMemoryDatabase()
	DB := sql.Open[struct {
		Rows sql.Map[int, row] `sql:"patched_rows"`
	}](db)
	ctx := testContext(t)
	stored := row{Label: "l", Data: []byte("d"), Inner: inner{On: true, Ratio: 1}, Pair: [2]uint8{1, 1}, Kept: "kept"}
	if err := DB.Rows.Set(ctx, 1, stored); err != nil {
		t.Fatal(err)
	}
	n, err := DB.Rows.Update(ctx, func(*int, *row) sql.Query { return sql.Query{sql.Slice(0, 10)} }, func(r *row) sql.Patch {
		r.Data = nil
		r.Pair[1] = 0
		return sql.Patch{sql.Set(&r.Label, "1"), sql.Set(&r.Inner, inner{})}
	})
	if n != 1 || err != nil {
		t.Fatalf("update gave %d and %v", n, err)
	}
	got, err := DB.Rows.Get(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Label != "1" || len(got.Data) != 0 || got.Inner != (inner{}) || got.Pair != [2]uint8{1, 0} || got.Kept != "kept" {
		t.Errorf("the row is %+v", got)
	}
	db.settled()
	var sent []string
	for _, mod := range sodium.Modifications.Arr.Get(db.patches[len(db.patches)-1][0]) {
		column, _ := sodium.Modifications.Set.Get(mod).Split()
		sent = append(sent, column.Name)
	}
	if want := []string{"label", "data", "inner_on", "inner_ratio", "pair2"}; !slices.Equal(sent, want) {
		t.Errorf("the patch set the columns %q, want %q", sent, want)
	}
}
