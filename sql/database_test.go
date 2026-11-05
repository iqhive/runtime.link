package sql_test

import (
	"context"
	stdsql "database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/sql"
	"github.com/iqhive/runtime.link/sql/std/sodium"
	"github.com/iqhive/runtime.link/xyz"
)

// memoryDatabase keeps its tables in memory and follows the documentation of
// [sodium.Database], so that [sql.Map] can be tested against more than a stub.
// It understands the expressions that [sql.Test] needs.
type memoryDatabase struct {
	mutex   sync.Mutex
	tables  map[string][]memoryRow
	schemas map[string]sodium.Table
	running sync.WaitGroup

	searchFailure  error          // every search fails like this, after sending its rows.
	nilRow         bool           // a nil row is sent after the rows of every search.
	stallSearch    bool           // a search sends its rows but never finishes.
	outputs        []sodium.Value // replaces the result of every output.
	generatesKeys  bool           // a zero key is replaced with the next integer.
	ignoresRepeats bool           // an insert without the overwrite flag overwrites.

	searches, updates, deletes, commits, rollbacks atomic.Int32

	patches []sodium.Patch
	nextKey int64
}

type memoryRow map[string]sodium.Value

func newMemoryDatabase() *memoryDatabase {
	return &memoryDatabase{tables: make(map[string][]memoryRow), schemas: make(map[string]sodium.Table)}
}

func (db *memoryDatabase) note(table sodium.Table) {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	db.schemas[table.Name] = table
}

// columns returns the names of the columns of a table the database has been given.
func (db *memoryDatabase) columns(table string) (index, value []string) {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	for _, column := range db.schemas[table].Index {
		index = append(index, column.Name)
	}
	for _, column := range db.schemas[table].Value {
		value = append(value, column.Name)
	}
	return index, value
}

func (db *memoryDatabase) rowCount() int {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	count := 0
	for _, rows := range db.tables {
		count += len(rows)
	}
	return count
}

// settled waits for every transaction to end, so that it can be inspected.
func (db *memoryDatabase) settled() { db.running.Wait() }

type memoryJob struct {
	run   func() (int, error)
	done  chan struct{}
	count int
	err   error
}

func newMemoryJob(run func() (int, error)) *memoryJob {
	return &memoryJob{run: run, done: make(chan struct{})}
}

func (job *memoryJob) execute() {
	job.count, job.err = job.run()
	close(job.done)
}

func (job *memoryJob) Wait(ctx context.Context) (int, error) {
	select {
	case <-job.done:
		return job.count, job.err
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func memoryMatches(row memoryRow, expr sodium.Expression) bool {
	switch xyz.ValueOf(expr) {
	case sodium.Expressions.Value:
		return sodium.Expressions.Value.Get(expr)
	case sodium.Expressions.Index:
		column, value := sodium.Expressions.Index.Get(expr).Split()
		return reflect.DeepEqual(row[column.Name], value)
	case sodium.Expressions.Avoid:
		return !memoryMatches(row, sodium.Expressions.Avoid.Get(expr))
	case sodium.Expressions.Cases:
		for _, option := range sodium.Expressions.Cases.Get(expr) {
			if memoryMatches(row, option) {
				return true
			}
		}
		return false
	case sodium.Expressions.Group:
		return memoryMatchesAll(row, sodium.Expressions.Group.Get(expr))
	}
	return true
}

func memoryMatchesAll(row memoryRow, exprs []sodium.Expression) bool {
	for _, expr := range exprs {
		if !memoryMatches(row, expr) {
			return false
		}
	}
	return true
}

func memoryLess(a, b sodium.Value) bool {
	if xyz.ValueOf(a) == sodium.Values.Int64 {
		return sodium.Values.Int64.Get(a) < sodium.Values.Int64.Get(b)
	}
	return a.String() < b.String()
}

// selected returns the positions of the rows chosen by a query.
func (db *memoryDatabase) selected(table sodium.Table, query sodium.Query) []int {
	var chosen []int
	for i, row := range db.tables[table.Name] {
		if memoryMatchesAll(row, query) {
			chosen = append(chosen, i)
		}
	}
	rows := db.tables[table.Name]
	for _, expr := range query {
		if xyz.ValueOf(expr) != sodium.Expressions.Order {
			continue
		}
		order := sodium.Expressions.Order.Get(expr)
		switch xyz.ValueOf(order) {
		case sodium.OrderExpressions.Increasing:
			name := sodium.OrderExpressions.Increasing.Get(order).Name
			sort.SliceStable(chosen, func(i, j int) bool { return memoryLess(rows[chosen[i]][name], rows[chosen[j]][name]) })
		case sodium.OrderExpressions.Decreasing:
			name := sodium.OrderExpressions.Decreasing.Get(order).Name
			sort.SliceStable(chosen, func(i, j int) bool { return memoryLess(rows[chosen[j]][name], rows[chosen[i]][name]) })
		}
	}
	if span, ok := memoryRange(query); ok {
		chosen = chosen[min(span.From, len(chosen)):min(span.Upto, len(chosen))]
	}
	return chosen
}

func memoryRange(query sodium.Query) (sodium.Range, bool) {
	for _, expr := range query {
		if xyz.ValueOf(expr) == sodium.Expressions.Range {
			return sodium.Expressions.Range.Get(expr), true
		}
	}
	return sodium.Range{}, false
}

var errMemoryEmptyRange = errors.New("empty range")

func memoryFinite(query sodium.Query) error {
	span, ok := memoryRange(query)
	if !ok || span.Upto <= span.From {
		return errMemoryEmptyRange
	}
	return nil
}

func (db *memoryDatabase) Search(table sodium.Table, query sodium.Query, ch chan<- []sodium.Value) sodium.Job {
	db.note(table)
	return newMemoryJob(func() (int, error) {
		db.searches.Add(1)
		db.mutex.Lock()
		defer db.mutex.Unlock()
		rows := db.tables[table.Name]
		for _, i := range db.selected(table, query) {
			var values []sodium.Value
			for _, column := range table.Index {
				values = append(values, rows[i][column.Name])
			}
			for _, column := range table.Value {
				values = append(values, rows[i][column.Name])
			}
			ch <- values
		}
		if db.nilRow {
			ch <- nil
		}
		if db.stallSearch {
			return 0, nil
		}
		close(ch)
		return 0, db.searchFailure
	})
}

func (db *memoryDatabase) Output(table sodium.Table, query sodium.Query, stats sodium.Stats, ch chan<- []sodium.Value) sodium.Job {
	db.note(table)
	return newMemoryJob(func() (int, error) {
		db.mutex.Lock()
		defer db.mutex.Unlock()
		defer close(ch)
		if db.outputs != nil {
			ch <- db.outputs
			return 0, nil
		}
		chosen := db.selected(table, query)
		var values []sodium.Value
		for _, stat := range stats {
			if stat == sodium.Calculations.Add {
				values = append(values, sodium.Values.Uint64.As(uint64(len(chosen))))
				continue
			}
			var total int64
			for _, i := range chosen {
				total += sodium.Values.Int64.Get(db.tables[table.Name][i][sodium.Calculations.Sum.Get(stat).Name])
			}
			values = append(values, sodium.Values.Int64.As(total))
		}
		ch <- values
		return 0, nil
	})
}

func (db *memoryDatabase) Delete(table sodium.Table, query sodium.Query) sodium.Job {
	db.note(table)
	return newMemoryJob(func() (int, error) {
		db.deletes.Add(1)
		if err := memoryFinite(query); err != nil {
			return 0, err
		}
		db.mutex.Lock()
		defer db.mutex.Unlock()
		chosen := db.selected(table, query)
		sort.Sort(sort.Reverse(sort.IntSlice(chosen)))
		for _, i := range chosen {
			rows := db.tables[table.Name]
			db.tables[table.Name] = append(rows[:i:i], rows[i+1:]...)
		}
		return len(chosen), nil
	})
}

func (db *memoryDatabase) Insert(table sodium.Table, index []sodium.Value, overwrite bool, query sodium.Query, value []sodium.Value) sodium.Job {
	db.note(table)
	return newMemoryJob(func() (int, error) {
		db.mutex.Lock()
		defer db.mutex.Unlock()
		if db.generatesKeys && index[0].IsZero() {
			db.nextKey++
			index[0] = sodium.Values.Int64.As(db.nextKey)
		}
		row := make(memoryRow)
		for i, column := range table.Index {
			row[column.Name] = index[i]
		}
		for i, column := range table.Value {
			row[column.Name] = value[i]
		}
		for i, existing := range db.tables[table.Name] {
			same := true
			for _, column := range table.Index {
				same = same && reflect.DeepEqual(existing[column.Name], row[column.Name])
			}
			if !same {
				continue
			}
			if !overwrite && !db.ignoresRepeats {
				return -1, nil
			}
			if !memoryMatchesAll(existing, query) {
				return 0, nil
			}
			db.tables[table.Name] = append(db.tables[table.Name][:i:i], db.tables[table.Name][i+1:]...)
			break
		}
		db.tables[table.Name] = append(db.tables[table.Name], row)
		return 1, nil
	})
}

func memoryApply(row memoryRow, mods []sodium.Modification) memoryRow {
	updated := make(memoryRow, len(row))
	for name, value := range row {
		updated[name] = value
	}
	for _, mod := range mods {
		switch xyz.ValueOf(mod) {
		case sodium.Modifications.Arr:
			updated = memoryApply(updated, sodium.Modifications.Arr.Get(mod))
		case sodium.Modifications.Set:
			column, value := sodium.Modifications.Set.Get(mod).Split()
			updated[column.Name] = value
		}
	}
	return updated
}

func (db *memoryDatabase) Update(table sodium.Table, query sodium.Query, patch sodium.Patch) sodium.Job {
	db.note(table)
	return newMemoryJob(func() (int, error) {
		db.updates.Add(1)
		if err := memoryFinite(query); err != nil {
			return 0, err
		}
		db.mutex.Lock()
		defer db.mutex.Unlock()
		db.patches = append(db.patches, patch)
		chosen := db.selected(table, query)
		for _, i := range chosen {
			db.tables[table.Name][i] = memoryApply(db.tables[table.Name][i], patch)
		}
		return len(chosen), nil
	})
}

func (db *memoryDatabase) snapshot() map[string][]memoryRow {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	copied := make(map[string][]memoryRow, len(db.tables))
	for name, rows := range db.tables {
		copied[name] = append([]memoryRow(nil), rows...)
	}
	return copied
}

func (db *memoryDatabase) restore(snapshot map[string][]memoryRow) {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	db.tables = snapshot
	db.rollbacks.Add(1)
}

func (db *memoryDatabase) Manage(ctx context.Context, _ sodium.Transaction) (chan<- sodium.Job, error) {
	tx := make(chan sodium.Job)
	before := db.snapshot()
	db.running.Add(1)
	go func() {
		defer db.running.Done()
		for {
			select {
			case job, open := <-tx:
				if !open {
					db.commits.Add(1)
					return
				}
				if job == nil {
					db.restore(before)
					return
				}
				job.(*memoryJob).execute()
			case <-ctx.Done():
				db.restore(before)
				return
			}
		}
	}()
	return tx, nil
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

type account struct {
	Name string
	Age  int
}

type bank struct {
	Accounts sql.Map[string, account] `sql:"accounts"`
}

func TestSearchReportsFailedJob(t *testing.T) {
	failure := errors.New("connection lost")
	for _, nilRow := range []bool{false, true} {
		t.Run(fmt.Sprintf("nilRow=%v", nilRow), func(t *testing.T) {
			ctx := testContext(t)
			db := newMemoryDatabase()
			db.searchFailure = failure
			db.nilRow = nilRow
			DB := sql.Open[bank](db)
			var issue error
			rows := 0
			for range DB.Accounts.Search(ctx, nil, &issue) {
				rows++
			}
			if rows != 0 || !errors.Is(issue, failure) {
				t.Errorf("search gave %d rows and %v", rows, issue)
			}
			if _, ok, err := DB.Accounts.Lookup(ctx, "k"); ok || !errors.Is(err, failure) {
				t.Errorf("lookup gave %v and %v", ok, err)
			}
			if _, _, ok, err := DB.Accounts.First(ctx, nil); ok || !errors.Is(err, failure) {
				t.Errorf("first gave %v and %v", ok, err)
			}
		})
	}
}

func TestSearchReportsFailureAfterRows(t *testing.T) {
	ctx := testContext(t)
	failure := errors.New("connection lost")
	db := newMemoryDatabase()
	DB := sql.Open[bank](db)
	if err := DB.Accounts.Set(ctx, "a", account{Name: "a"}); err != nil {
		t.Fatal(err)
	}
	db.searchFailure = failure
	var issue error
	rows := 0
	for range DB.Accounts.Search(ctx, nil, &issue) {
		rows++
	}
	if rows != 1 || !errors.Is(issue, failure) {
		t.Fatalf("got %d rows and %v", rows, issue)
	}
}

func TestSearchNilRowWithoutFailureIsNotARow(t *testing.T) {
	ctx := testContext(t)
	db := newMemoryDatabase()
	DB := sql.Open[bank](db)
	if err := DB.Accounts.Set(ctx, "a", account{Name: "a"}); err != nil {
		t.Fatal(err)
	}
	db.nilRow = true
	var issue error
	var keys []string
	for key := range DB.Accounts.Search(ctx, nil, &issue) {
		keys = append(keys, key)
	}
	if issue != nil {
		t.Fatal(issue)
	}
	if !reflect.DeepEqual(keys, []string{"a"}) {
		t.Fatalf("got keys %q", keys)
	}
}

func TestSearchClearsEarlierIssue(t *testing.T) {
	ctx := testContext(t)
	DB := sql.Open[bank](newMemoryDatabase())
	issue := errors.New("from an earlier search")
	for range DB.Accounts.Search(ctx, nil, &issue) {
	}
	if issue != nil {
		t.Fatalf("a search that worked left %v", issue)
	}
}

func TestSearchEndsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(testContext(t))
	defer cancel()
	db := newMemoryDatabase()
	DB := sql.Open[bank](db)
	if err := DB.Accounts.Set(ctx, "a", account{Name: "a"}); err != nil {
		t.Fatal(err)
	}
	db.stallSearch = true
	finished := make(chan error, 1)
	go func() {
		var issue error
		for range DB.Accounts.Search(ctx, nil, &issue) {
			cancel()
		}
		finished <- issue
	}()
	select {
	case issue := <-finished:
		if !errors.Is(issue, context.Canceled) {
			t.Fatalf("search ended with %v", issue)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("search outlived its context")
	}
}

func TestConformancePassesAndLeavesRecords(t *testing.T) {
	db := newMemoryDatabase()
	if err := sql.Test(testContext(t), db); err != nil {
		t.Fatalf("%+v", err)
	}
	db.settled()
	if n := db.rowCount(); n == 0 {
		t.Fatal("a passing test removed the records it created, it used to leave them")
	}
}

func TestConformanceRejectsDatabaseWithoutDuplicates(t *testing.T) {
	db := newMemoryDatabase()
	db.ignoresRepeats = true
	if err := sql.Test(testContext(t), db); err == nil {
		t.Fatal("expected a database that overwrites on create to fail")
	}
}

func TestConformanceLeavesRecordsOnFailure(t *testing.T) {
	db := newMemoryDatabase()
	db.outputs = []sodium.Value{sodium.Values.Uint64.As(1)}
	if err := sql.Test(testContext(t), db); err == nil {
		t.Fatal("expected a miscounting database to fail")
	}
	db.settled()
	if db.rowCount() == 0 {
		t.Fatal("the testing records should be left to help with debugging")
	}
}

func TestCasesOfNothingMatchesEverything(t *testing.T) {
	ctx := testContext(t)
	var none []sodium.Expression
	everything := func(*string, *account) sql.Query { return sql.Query{sql.Cases(none...), sql.Slice(0, 10)} }
	raises := func(a *account) sql.Patch { return sql.Patch{sql.Set(&a.Age, 99)} }
	check := func(t *testing.T, DB *bank) {
		t.Helper()
		if err := DB.Accounts.Set(ctx, "a", account{Name: "a", Age: 1}); err != nil {
			t.Fatal(err)
		}
		var issue error
		var keys []string
		for key := range DB.Accounts.Search(ctx, everything, &issue) {
			keys = append(keys, key)
		}
		if issue != nil || !reflect.DeepEqual(keys, []string{"a"}) {
			t.Errorf("search found %q and %v", keys, issue)
		}
		if n, err := DB.Accounts.Count(ctx, everything); n != 1 || err != nil {
			t.Errorf("count gave %d and %v", n, err)
		}
		if n, err := DB.Accounts.Update(ctx, everything, raises); n != 1 || err != nil {
			t.Errorf("update gave %d and %v", n, err)
		}
		if got, err := DB.Accounts.Get(ctx, "a"); err != nil || got.Age != 99 {
			t.Errorf("the row became %+v, %v", got, err)
		}
		if n, err := DB.Accounts.UnsafeDelete(ctx, everything); n != 1 || err != nil {
			t.Errorf("delete gave %d and %v", n, err)
		}
		if _, ok, err := DB.Accounts.Lookup(ctx, "a"); ok || err != nil {
			t.Errorf("the row is still there: %v, %v", ok, err)
		}
	}
	t.Run("RAM", func(t *testing.T) { check(t, sql.Open[bank](sql.New())) })
	t.Run("Database", func(t *testing.T) { check(t, sql.Open[bank](newMemoryDatabase())) })
}

func TestCasesAndMergeKeepEveryExpression(t *testing.T) {
	column := sodium.Expressions.Empty.As(sodium.Column{Name: "n"})
	yes, no := sql.Merge(), sql.Avoid(sql.Merge())
	for name, test := range map[string]struct{ got, want sodium.Expression }{
		"CasesAfterColumn":  {sql.Cases(column, yes), sodium.Expressions.Cases.As([]sodium.Expression{column, yes})},
		"CasesBeforeColumn": {sql.Cases(no, column), sodium.Expressions.Cases.As([]sodium.Expression{no, column})},
		"CasesTrueFirst":    {sql.Cases(yes, column), yes},
		"MergeAfterColumn":  {sql.Merge(column, no), sodium.Expressions.Group.As([]sodium.Expression{column, no})},
		"MergeBeforeColumn": {sql.Merge(yes, column), sodium.Expressions.Group.As([]sodium.Expression{yes, column})},
		"MergeFalseFirst":   {sql.Merge(no, column), no},
	} {
		if !reflect.DeepEqual(test.got, test.want) {
			t.Errorf("%s gave %v, want %v", name, test.got, test.want)
		}
	}
}

func TestMergeOfNothingMatchesEverything(t *testing.T) {
	ctx := testContext(t)
	var m sql.Map[string, account]
	if err := m.Set(ctx, "a", account{}); err != nil {
		t.Fatal(err)
	}
	n, err := m.Count(ctx, func(*string, *account) sql.Query { return sql.Query{sql.Merge()} })
	if n != 1 || err != nil {
		t.Fatalf("got %d and %v", n, err)
	}
}

func TestConstantExpressionsNeverReachDatabase(t *testing.T) {
	ctx := testContext(t)
	db := newMemoryDatabase()
	DB := sql.Open[bank](db)
	if err := DB.Accounts.Set(ctx, "a", account{Name: "a"}); err != nil {
		t.Fatal(err)
	}
	// A false constant at the top of a query is known to match nothing.
	never := func(_ *string, a *account) sql.Query {
		return sql.Query{sql.Index(&a.Name).Equals("a"), sql.Avoid(sql.Merge()), sql.Slice(0, 10)}
	}
	if n, err := DB.Accounts.Count(ctx, never); n != 0 || err != nil {
		t.Errorf("count gave %d and %v", n, err)
	}
	var issue error
	for range DB.Accounts.Search(ctx, never, &issue) {
		t.Error("search matched a query holding something false")
	}
	if issue != nil {
		t.Error(issue)
	}
	if n, err := DB.Accounts.Update(ctx, never, func(a *account) sql.Patch { return sql.Patch{sql.Set(&a.Age, 99)} }); n != 0 || err != nil {
		t.Errorf("update gave %d and %v", n, err)
	}
	if n, err := DB.Accounts.UnsafeDelete(ctx, never); n != 0 || err != nil {
		t.Errorf("delete gave %d and %v", n, err)
	}
	if searches, updates, deletes := db.searches.Load(), db.updates.Load(), db.deletes.Load(); searches != 0 || updates != 0 || deletes != 0 {
		t.Errorf("the database saw %d searches, %d updates and %d deletes that can match nothing", searches, updates, deletes)
	}
}

func TestWritesWithoutAFiniteRange(t *testing.T) {
	ctx := testContext(t)
	raises := func(a *account) sql.Patch { return sql.Patch{sql.Set(&a.Age, 99)} }
	setup := func(t *testing.T, DB *bank) {
		t.Helper()
		if err := DB.Accounts.Set(ctx, "a", account{Age: 1}); err != nil {
			t.Fatal(err)
		}
	}
	unchanged := func(t *testing.T, DB *bank) {
		t.Helper()
		if got, ok, err := DB.Accounts.Lookup(ctx, "a"); !ok || err != nil || got.Age != 1 {
			t.Errorf("the row became %+v, %v, %v", got, ok, err)
		}
	}
	t.Run("NoQuery", func(t *testing.T) {
		for name, DB := range map[string]*bank{"RAM": sql.Open[bank](sql.New()), "Database": sql.Open[bank](newMemoryDatabase())} {
			setup(t, DB)
			if n, err := DB.Accounts.Update(ctx, nil, raises); n != 0 || err == nil {
				t.Errorf("%s update gave %d and %v", name, n, err)
			}
			if n, err := DB.Accounts.UnsafeDelete(ctx, nil); n != 0 || err == nil {
				t.Errorf("%s delete gave %d and %v", name, n, err)
			}
			unchanged(t, DB)
		}
	})
	queries := map[string]sql.QueryFunc[string, account]{
		"NoSlice":   func(*string, *account) sql.Query { return sql.Query{} },
		"ZeroWidth": func(*string, *account) sql.Query { return sql.Query{sql.Slice(2, 2)} },
		"Inverted":  func(*string, *account) sql.Query { return sql.Query{sql.Slice(4, 2)} },
	}
	for name, query := range queries {
		// Without a database, an empty range writes nothing and is no failure.
		t.Run(name+"/RAM", func(t *testing.T) {
			DB := sql.Open[bank](sql.New())
			setup(t, DB)
			if n, err := DB.Accounts.Update(ctx, query, raises); n != 0 || err != nil {
				t.Errorf("update gave %d and %v", n, err)
			}
			if n, err := DB.Accounts.UnsafeDelete(ctx, query); n != 0 || err != nil {
				t.Errorf("delete gave %d and %v", n, err)
			}
			unchanged(t, DB)
		})
		// A database is given the query, and it is the SODIUM interface that fails it.
		t.Run(name+"/Database", func(t *testing.T) {
			db := newMemoryDatabase()
			DB := sql.Open[bank](db)
			setup(t, DB)
			if n, err := DB.Accounts.Update(ctx, query, raises); n != 0 || !errors.Is(err, errMemoryEmptyRange) {
				t.Errorf("update gave %d and %v", n, err)
			}
			if n, err := DB.Accounts.UnsafeDelete(ctx, query); n != 0 || !errors.Is(err, errMemoryEmptyRange) {
				t.Errorf("delete gave %d and %v", n, err)
			}
			unchanged(t, DB)
			if updates, deletes := db.updates.Load(), db.deletes.Load(); updates != 1 || deletes != 1 {
				t.Errorf("the database saw %d updates and %d deletes", updates, deletes)
			}
		})
	}
}

func TestOpenTableWithDatabase(t *testing.T) {
	ctx := testContext(t)
	db := newMemoryDatabase()
	m := sql.OpenTable[string, account](db, sodium.Table{Name: "opened"})
	if err := m.Set(ctx, "a", account{Name: "Alice", Age: 30}); err != nil {
		t.Fatal(err)
	}
	if n, err := m.Count(ctx, nil); n != 1 || err != nil {
		t.Fatalf("count gave %d and %v", n, err)
	}
	if got, ok, err := m.Lookup(ctx, "a"); !ok || err != nil || got.Age != 30 {
		t.Fatalf("lookup gave %+v, %v, %v", got, ok, err)
	}
	var issue error
	for key, value := range m.Search(ctx, func(_ *string, a *account) sql.Query {
		return sql.Query{sql.Index(&a.Name).Equals("Alice")}
	}, &issue) {
		if key != "a" || value.Age != 30 {
			t.Fatalf("search gave %q, %+v", key, value)
		}
	}
	if issue != nil {
		t.Fatal(issue)
	}
	if deleted, err := m.Delete(ctx, "a", nil); !deleted || err != nil {
		t.Fatalf("delete gave %v and %v", deleted, err)
	}
	index, value := db.columns("opened")
	if !reflect.DeepEqual(index, []string{"id"}) || !reflect.DeepEqual(value, []string{"name", "age"}) {
		t.Fatalf("got columns %v and %v", index, value)
	}
}

func TestOpenTableUsesGivenColumns(t *testing.T) {
	ctx := testContext(t)
	db := newMemoryDatabase()
	m := sql.OpenTable[string, account](db, sodium.Table{
		Name:  "chosen",
		Index: []sodium.Column{{Name: "pk", Type: sodium.Values.String}},
		Value: []sodium.Column{{Name: "full_name", Type: sodium.Values.String}, {Name: "years", Type: sodium.Values.Int64}},
	})
	if err := m.Set(ctx, "a", account{Name: "Alice", Age: 30}); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := m.Lookup(ctx, "a"); !ok || err != nil || got.Name != "Alice" {
		t.Fatalf("lookup gave %+v, %v, %v", got, ok, err)
	}
	n, err := m.Count(ctx, func(_ *string, a *account) sql.Query {
		return sql.Query{sql.Index(&a.Age).Equals(30)}
	})
	if n != 1 || err != nil {
		t.Fatalf("a query on a given column counted %d and %v", n, err)
	}
	other := sql.OpenTable[string, account](db, sodium.Table{Name: "chosen"})
	if err := other.Set(ctx, "b", account{Name: "Bob"}); err != nil {
		t.Fatal(err)
	}
	n, err = m.Count(ctx, func(_ *string, a *account) sql.Query {
		return sql.Query{sql.Index(&a.Age).Equals(30)}
	})
	if n != 1 || err != nil {
		t.Fatalf("opening the table with other columns changed the query: %d and %v", n, err)
	}
}

// A sql tag with only options names the column, so stored tables have a column
// with no name, and it has to stay that way to keep finding them.
func TestScopedTagWithoutNameHasEmptyColumn(t *testing.T) {
	type row struct {
		Title string
		Group string `sql:",scoped"`
	}
	ctx := testContext(t)
	db := newMemoryDatabase()
	DB := sql.Open[struct {
		Rows sql.Map[string, row] `sql:"scoped_rows"`
	}](db)
	if err := DB.Rows.Insert(sql.Admin(ctx, "testing"), "a", sql.Create, row{Title: "t", Group: "g"}); err != nil {
		t.Fatal(err)
	}
	if _, value := db.columns("scoped_rows"); !reflect.DeepEqual(value, []string{"title", ""}) {
		t.Fatalf("got columns %q", value)
	}
	n, err := DB.Rows.Count(sql.Scope(ctx, "group", "g"), nil)
	if n != 1 || err != nil {
		t.Fatalf("a scoped count gave %d and %v", n, err)
	}
}

func TestUnexportedFieldsAreNotColumns(t *testing.T) {
	type embedded struct{ Skipped string }
	type row struct {
		secret string
		Name   string
		embedded
		hidden struct{ Age int }
	}
	ctx := testContext(t)
	db := newMemoryDatabase()
	DB := sql.Open[struct {
		Rows sql.Map[string, row] `sql:"hidden_rows"`
	}](db)
	if err := DB.Rows.Set(ctx, "a", row{secret: "s", Name: "n", embedded: embedded{"e"}}); err != nil {
		t.Fatal(err)
	}
	if _, value := db.columns("hidden_rows"); !reflect.DeepEqual(value, []string{"name"}) {
		t.Fatalf("got columns %q", value)
	}
	got, ok, err := DB.Rows.Lookup(ctx, "a")
	if !ok || err != nil {
		t.Fatalf("lookup gave %v and %v", ok, err)
	}
	if got.Name != "n" || got.secret != "" || got.embedded.Skipped != "" || got.hidden.Age != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestNestedColumnNames(t *testing.T) {
	type inner struct{ C int }
	type middle struct {
		B inner
		H [2]string
		L [2]inner
	}
	type outer struct {
		A middle
		E inner `sql:"tagged"`
	}
	ctx := testContext(t)
	db := newMemoryDatabase()
	DB := sql.Open[struct {
		Rows sql.Map[string, outer] `sql:"nested_rows"`
	}](db)
	row := outer{A: middle{B: inner{1}, H: [2]string{"x", "y"}, L: [2]inner{{2}, {3}}}, E: inner{4}}
	if err := DB.Rows.Set(ctx, "a", row); err != nil {
		t.Fatal(err)
	}
	// The path of a nested column repeats, as it always has, and stored tables
	// have these names.
	_, value := db.columns("nested_rows")
	want := []string{"a_a_b_c", "a_a_h1", "a_a_h2", "a_a_a_l1_c", "a_a_a_l2_c", "tagged_c"}
	if !reflect.DeepEqual(value, want) {
		t.Fatalf("got columns %q, want %q", value, want)
	}
	// A query only finds the row if it agrees with the table about the columns.
	for name, query := range map[string]sql.QueryFunc[string, outer]{
		"struct": func(_ *string, v *outer) sql.Query { return sql.Query{sql.Index(&v.A.B.C).Equals(1)} },
		"array":  func(_ *string, v *outer) sql.Query { return sql.Query{sql.Index(&v.A.H[1]).Equals("y")} },
		"both":   func(_ *string, v *outer) sql.Query { return sql.Query{sql.Index(&v.A.L[1].C).Equals(3)} },
		"tagged": func(_ *string, v *outer) sql.Query { return sql.Query{sql.Index(&v.E.C).Equals(4)} },
	} {
		if n, err := DB.Rows.Count(ctx, query); n != 1 || err != nil {
			t.Errorf("%s query gave %d and %v", name, n, err)
		}
	}
}

func TestMutatePatchesCurrentValue(t *testing.T) {
	type row struct {
		N     int
		Label string
	}
	ctx := testContext(t)
	db := newMemoryDatabase()
	DB := sql.Open[struct {
		Rows sql.Map[string, row] `sql:"mutated_rows"`
	}](db)
	if err := DB.Rows.Set(ctx, "k", row{N: 5, Label: "kept"}); err != nil {
		t.Fatal(err)
	}
	increment := func(r *row) sql.Patch { return sql.Patch{sql.Set(&r.N, r.N+1)} }
	if ok, err := DB.Rows.Mutate(ctx, "k", nil, increment); !ok || err != nil {
		t.Fatalf("mutate gave %v and %v", ok, err)
	}
	if got, err := DB.Rows.Get(ctx, "k"); err != nil || got != (row{N: 6, Label: "kept"}) {
		t.Fatalf("the row became %+v, %v", got, err)
	}
	db.settled()
	if len(db.patches) != 1 {
		t.Fatalf("sent %d patches", len(db.patches))
	}
	mods := sodium.Modifications.Arr.Get(db.patches[0][0])
	if len(mods) != 1 {
		t.Fatalf("sent %d changes, only one column changed", len(mods))
	}
	if column, value := sodium.Modifications.Set.Get(mods[0]).Split(); column.Name != "n" || value.String() != "6" {
		t.Fatalf("sent %s=%v", column.Name, value)
	}

	if ok, err := DB.Rows.Mutate(ctx, "missing", nil, increment); ok || err != nil {
		t.Fatalf("mutating a missing key gave %v and %v", ok, err)
	}
	other := func(r *row) sql.Check { return sql.Check{sql.Index(&r.N).Equals(100)} }
	if ok, err := DB.Rows.Mutate(ctx, "k", other, increment); ok || err != nil {
		t.Fatalf("mutating with a failed check gave %v and %v", ok, err)
	}
	same := func(r *row) sql.Check { return sql.Check{sql.Index(&r.N).Equals(6)} }
	if ok, err := DB.Rows.Mutate(ctx, "k", same, increment); !ok || err != nil {
		t.Fatalf("mutating with a passed check gave %v and %v", ok, err)
	}
	if got, err := DB.Rows.Get(ctx, "k"); err != nil || got.N != 7 {
		t.Fatalf("the row became %+v, %v", got, err)
	}
}

func TestMutateJoinsTransaction(t *testing.T) {
	ctx := testContext(t)
	db := newMemoryDatabase()
	DB := sql.Open[bank](db)
	if err := DB.Accounts.Set(ctx, "a", account{Age: 1}); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("failed")
	err := DB.Accounts.Manage(ctx, sql.ReadWrites, func(ctx context.Context) error {
		ok, err := DB.Accounts.Mutate(ctx, "a", nil, func(a *account) sql.Patch { return sql.Patch{sql.Set(&a.Age, a.Age+1)} })
		if !ok || err != nil {
			t.Errorf("mutate gave %v and %v", ok, err)
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	db.settled()
	if got, err := DB.Accounts.Get(ctx, "a"); err != nil || got.Age != 1 {
		t.Fatalf("a rolled back mutate left %+v, %v", got, err)
	}
}

func openDuringUse[V any](db sql.Database) {
	sql.Open[struct {
		Rows sql.Map[string, V] `sql:"opened_during_use"`
	}](db)
}

func TestOpenDuringUse(t *testing.T) {
	ctx := testContext(t)
	db := newMemoryDatabase()
	DB := sql.Open[bank](db)
	var stop atomic.Bool
	var working sync.WaitGroup
	working.Add(1)
	go func() {
		defer working.Done()
		for !stop.Load() {
			if _, err := DB.Accounts.Count(ctx, nil); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	openDuringUse[[1]int](db)
	openDuringUse[[2]int](db)
	openDuringUse[[3]int](db)
	openDuringUse[[4]int](db)
	openDuringUse[[5]int](db)
	openDuringUse[[6]int](db)
	openDuringUse[[7]int](db)
	openDuringUse[[8]int](db)
	stop.Store(true)
	working.Wait()
}

func TestScopedBoolColumn(t *testing.T) {
	type row struct {
		Name   string
		Active bool `sql:"active,scoped"`
	}
	ctx := testContext(t)
	db := newMemoryDatabase()
	DB := sql.Open[struct {
		Rows sql.Map[string, row] `sql:"flagged_rows"`
	}](db)
	admin := sql.Admin(ctx, "testing")
	for key, active := range map[string]bool{"on": true, "off": false} {
		if err := DB.Rows.Set(admin, key, row{Name: key, Active: active}); err != nil {
			t.Fatal(err)
		}
	}
	var issue error
	var names []string
	for _, value := range DB.Rows.Search(sql.Scope(ctx, "active", true), nil, &issue) {
		names = append(names, value.Name)
	}
	if issue != nil {
		t.Fatal(issue)
	}
	if !reflect.DeepEqual(names, []string{"on"}) {
		t.Fatalf("got %q", names)
	}
}

type zeroKeyRow struct {
	Group string `sql:",scoped"`
}

type zeroKeyTables struct {
	Rows sql.Map[string, zeroKeyRow] `sql:"zero_key_rows"`
}

func TestZeroKeyIsInvalidForDatabase(t *testing.T) {
	ctx := testContext(t)
	noPatch := func(*account) sql.Patch { return nil }
	db := newMemoryDatabase()
	DB, scoped := sql.Open[bank](db), sql.Open[zeroKeyTables](db)
	operations := map[string]func() error{
		"Insert": func() error { return DB.Accounts.Insert(ctx, "", sql.Create, account{}) },
		"Set":    func() error { return DB.Accounts.Set(ctx, "", account{}) },
		"Get":    func() error { _, err := DB.Accounts.Get(ctx, ""); return err },
		"Lookup": func() error { _, _, err := DB.Accounts.Lookup(ctx, ""); return err },
		"Delete": func() error { _, err := DB.Accounts.Delete(ctx, "", nil); return err },
		"Mutate": func() error { _, err := DB.Accounts.Mutate(ctx, "", nil, noPatch); return err },
		"Scoped": func() error {
			return scoped.Rows.Set(sql.Scope(ctx, "group", "a"), "", zeroKeyRow{Group: "a"})
		},
	}
	for name, operation := range operations {
		if err := operation(); !errors.Is(err, sql.ErrInvalidKey) {
			t.Errorf("%s gave %v", name, err)
		}
	}
}

func TestRAMAcceptsZeroKey(t *testing.T) {
	ctx := testContext(t)
	DB, scoped := sql.Open[bank](sql.New()), sql.Open[zeroKeyTables](sql.New())
	if err := DB.Accounts.Insert(ctx, "", sql.Create, account{Name: "created"}); err != nil {
		t.Errorf("insert gave %v", err)
	}
	if err := DB.Accounts.Set(ctx, "", account{Name: "set"}); err != nil {
		t.Errorf("set gave %v", err)
	}
	if got, err := DB.Accounts.Get(ctx, ""); err != nil || got.Name != "set" {
		t.Errorf("get gave %+v and %v", got, err)
	}
	mutated, err := DB.Accounts.Mutate(ctx, "", nil, func(a *account) sql.Patch { return sql.Patch{sql.Set(&a.Age, 1)} })
	if !mutated || err != nil {
		t.Errorf("mutate gave %v and %v", mutated, err)
	}
	if got, ok, err := DB.Accounts.Lookup(ctx, ""); !ok || err != nil || got != (account{Name: "set", Age: 1}) {
		t.Errorf("lookup gave %+v, %v and %v", got, ok, err)
	}
	if deleted, err := DB.Accounts.Delete(ctx, "", nil); !deleted || err != nil {
		t.Errorf("delete gave %v and %v", deleted, err)
	}
	if err := scoped.Rows.Set(sql.Scope(ctx, "group", "a"), "", zeroKeyRow{Group: "a"}); err != nil {
		t.Errorf("a scoped set gave %v", err)
	}
}

func TestCreateRefusesSameValue(t *testing.T) {
	check := func(t *testing.T, DB *bank) {
		t.Helper()
		ctx := testContext(t)
		alice := account{Name: "Alice", Age: 30}
		if err := DB.Accounts.Insert(ctx, "a", sql.Create, alice); err != nil {
			t.Fatal(err)
		}
		if err := DB.Accounts.Insert(ctx, "a", sql.Create, alice); err != sql.ErrDuplicate {
			t.Errorf("creating the same value gave %v", err)
		}
		if err := DB.Accounts.Insert(ctx, "a", sql.Create, account{Name: "Bob"}); err != sql.ErrDuplicate {
			t.Errorf("creating a different value gave %v", err)
		}
		if got, err := DB.Accounts.Get(ctx, "a"); err != nil || got != alice {
			t.Errorf("the row became %+v, %v", got, err)
		}
	}
	t.Run("RAM", func(t *testing.T) { check(t, sql.Open[bank](sql.New())) })
	t.Run("Database", func(t *testing.T) { check(t, sql.Open[bank](newMemoryDatabase())) })
}

func TestOutputSkipsDatabaseButClearsCounters(t *testing.T) {
	ctx := testContext(t)
	db := newMemoryDatabase()
	DB := sql.Open[bank](db)
	if err := DB.Accounts.Set(ctx, "a", account{Age: 3}); err != nil {
		t.Fatal(err)
	}
	var count atomic.Int64
	var total int
	stats := func(_ *string, a *account) sql.Stats { return sql.Stats{sql.Count(&count), sql.Sum(&a.Age, &total)} }
	if err := DB.Accounts.Output(ctx, nil, stats); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 1 || total != 3 {
		t.Fatalf("got count %d and total %d", count.Load(), total)
	}
	// As a database counts nothing for a query that matches nothing.
	if err := DB.Accounts.Output(ctx, func(*string, *account) sql.Query { return sql.Query{sql.Avoid(sql.Merge())} }, stats); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 0 || total != 0 {
		t.Fatalf("matching nothing left count %d and total %d", count.Load(), total)
	}
}

func TestOutputWithEmptyScopeLeavesCounters(t *testing.T) {
	type row struct {
		Group string `sql:"group,scoped"`
	}
	ctx := testContext(t)
	db := newMemoryDatabase()
	DB := sql.Open[struct {
		Rows sql.Map[string, row] `sql:"counted_rows"`
	}](db)
	if err := DB.Rows.Set(sql.Admin(ctx, "testing"), "a", row{Group: "g"}); err != nil {
		t.Fatal(err)
	}
	var count atomic.Int64
	stats := func(*string, *row) sql.Stats { return sql.Stats{sql.Count(&count)} }
	if err := DB.Rows.Output(sql.Scope(ctx, "group", "g"), nil, stats); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 1 {
		t.Fatalf("got count %d", count.Load())
	}
	if err := DB.Rows.Output(sql.Scope[string](ctx, "group"), nil, stats); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 1 {
		t.Fatalf("an empty scope changed the count to %d, it used to leave it", count.Load())
	}
}

func TestCountSaturatesInt32(t *testing.T) {
	ctx := testContext(t)
	for _, test := range []struct {
		output int64
		want   int32
	}{
		{math.MaxInt32, math.MaxInt32},
		{math.MaxInt32 + 1, -1},
		{3_000_000_000, -1},
		{math.MaxUint32, -1},
		{math.MaxUint32 + 1, -1},
	} {
		db := newMemoryDatabase()
		db.outputs = []sodium.Value{sodium.Values.Int64.As(test.output)}
		DB := sql.Open[bank](db)
		var count atomic.Int32
		err := DB.Accounts.Output(ctx, nil, func(*string, *account) sql.Stats { return sql.Stats{sql.Count(&count)} })
		if err != nil {
			t.Fatal(err)
		}
		if count.Load() != test.want {
			t.Errorf("a count of %d gave %d, want %d", test.output, count.Load(), test.want)
		}
	}
}

// aggregateRow is a [database/sql] driver with a single row of a single column.
type aggregateRow struct{ value driver.Value }

func (row aggregateRow) Connect(context.Context) (driver.Conn, error) { return row, nil }
func (row aggregateRow) Driver() driver.Driver                        { return nil }
func (row aggregateRow) Prepare(string) (driver.Stmt, error)          { return row, nil }
func (row aggregateRow) Close() error                                 { return nil }
func (row aggregateRow) Begin() (driver.Tx, error)                    { return nil, errors.New("no transactions") }
func (row aggregateRow) NumInput() int                                { return 0 }
func (row aggregateRow) Exec([]driver.Value) (driver.Result, error) {
	return nil, errors.New("no writes")
}
func (row aggregateRow) Query([]driver.Value) (driver.Rows, error) {
	return &aggregateRows{row: row}, nil
}

type aggregateRows struct {
	row  aggregateRow
	read bool
}

func (rows *aggregateRows) Columns() []string { return []string{"aggregate"} }
func (rows *aggregateRows) Close() error      { return nil }
func (rows *aggregateRows) Next(dest []driver.Value) error {
	if rows.read {
		return io.EOF
	}
	rows.read = true
	dest[0] = rows.row.value
	return nil
}

func TestNewOutputScansIntoPlainPointers(t *testing.T) {
	column := sodium.Column{Name: "age", Type: sodium.Values.Int64}
	calculations := map[string]sodium.Calculation{
		"Sum": sodium.Calculations.Sum.With(column),
		"Avg": sodium.Calculations.Avg.With(column),
		"Max": sodium.Calculations.Max.With(column),
		"Min": sodium.Calculations.Min.With(column),
		"Top": sodium.Calculations.Top.With(column),
	}
	for name, calculation := range calculations {
		t.Run(name, func(t *testing.T) {
			// A scanner written for the pointer of the column's type.
			values, err := sql.NewOutput([]sodium.Calculation{calculation}, func(ptrs ...any) error {
				ptr, ok := ptrs[0].(*int64)
				if !ok {
					return fmt.Errorf("the scanner was given a %T", ptrs[0])
				}
				*ptr = 7
				return nil
			})
			if err != nil || len(values) != 1 || sodium.Values.Int64.Get(values[0]) != 7 {
				t.Fatalf("got %v and %v", values, err)
			}
			db := stdsql.OpenDB(aggregateRow{int64(7)})
			defer db.Close()
			values, err = sql.NewOutput([]sodium.Calculation{calculation}, db.QueryRow("SELECT aggregate").Scan)
			if err != nil || len(values) != 1 || sodium.Values.Int64.Get(values[0]) != 7 {
				t.Fatalf("database/sql gave %v and %v", values, err)
			}
			// Over no rows the aggregate is NULL, which database/sql will not
			// scan into a plain pointer, as documented on NewOutput.
			null := stdsql.OpenDB(aggregateRow{nil})
			defer null.Close()
			if values, err := sql.NewOutput([]sodium.Calculation{calculation}, null.QueryRow("SELECT aggregate").Scan); err == nil {
				t.Fatalf("NULL gave %v", values)
			}
		})
	}
}

func TestAddWithoutKeyIsNotKept(t *testing.T) {
	ctx := testContext(t)
	db := newMemoryDatabase()
	DB := sql.Open[bank](db)
	if _, err := DB.Accounts.Add(ctx, account{Name: "a"}); !errors.Is(err, sql.ErrInsertOnly) {
		t.Fatalf("got %v", err)
	}
	db.settled()
	if n := db.rowCount(); n != 0 {
		t.Fatalf("a failed add left %d rows", n)
	}
	if commits, rollbacks := db.commits.Load(), db.rollbacks.Load(); commits != 0 || rollbacks != 1 {
		t.Fatalf("got %d commits and %d rollbacks", commits, rollbacks)
	}
}

func TestAddInsideTransactionLeavesRollbackToCaller(t *testing.T) {
	ctx := testContext(t)
	db := newMemoryDatabase()
	DB := sql.Open[bank](db)
	err := DB.Accounts.Manage(ctx, sql.ReadWrites, func(ctx context.Context) error {
		_, err := DB.Accounts.Add(ctx, account{Name: "a"})
		return err
	})
	if !errors.Is(err, sql.ErrInsertOnly) {
		t.Fatalf("got %v", err)
	}
	db.settled()
	if n := db.rowCount(); n != 0 {
		t.Fatalf("a failed add left %d rows", n)
	}
}

func TestAddKeepsGeneratedKey(t *testing.T) {
	ctx := testContext(t)
	db := newMemoryDatabase()
	db.generatesKeys = true
	DB := sql.Open[struct {
		Rows sql.Map[int, account] `sql:"generated_keys"`
	}](db)
	key, err := DB.Rows.Add(ctx, account{Name: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if key != 1 {
		t.Fatalf("got key %d", key)
	}
	if got, err := DB.Rows.Get(ctx, key); err != nil || got.Name != "a" {
		t.Fatalf("got %+v, %v", got, err)
	}
}
