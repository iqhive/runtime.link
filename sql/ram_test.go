package sql_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/sql"
)

func TestRAM(t *testing.T) {
	if err := sql.Test(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestRAMSearchSlice(t *testing.T) {
	ctx := context.Background()
	var m sql.Map[int, int]
	for i := 1; i <= 8; i++ {
		if err := m.Set(ctx, i, i); err != nil {
			t.Fatal(err)
		}
	}
	for _, ordered := range []bool{false, true} {
		for _, span := range [][2]int{{0, 0}, {2, 2}, {2, 4}, {6, 12}, {10, 12}} {
			t.Run(fmt.Sprintf("ordered=%v/%d:%d", ordered, span[0], span[1]), func(t *testing.T) {
				var issue error
				var keys []int
				for k := range m.Search(ctx, func(k, v *int) sql.Query {
					// Filter before slicing: leave seven matching rows.
					q := sql.Query{sql.Avoid(sql.Index(v).Equals(1)), sql.Slice(span[0], span[1])}
					if ordered {
						q = append(q, sql.Order(k).Increasing())
					}
					return q
				}, &issue) {
					keys = append(keys, k)
				}
				if issue != nil {
					t.Fatal(issue)
				}
				want := max(0, min(span[1], 7)-span[0])
				if len(keys) != want {
					t.Fatalf("got keys %v, want %d results", keys, want)
				}
				if ordered {
					for i, k := range keys {
						if k != span[0]+i+2 {
							t.Fatalf("got keys %v, want consecutive keys starting at %d", keys, span[0]+2)
						}
					}
				}
			})
		}
	}
}

func TestRAMSearchOrderByValue(t *testing.T) {
	ctx := context.Background()
	var m sql.Map[int, int]
	for k := 1; k <= 31; k++ {
		if err := m.Set(ctx, k, (k*13)%31); err != nil {
			t.Fatal(err)
		}
	}
	for _, decreasing := range []bool{false, true} {
		for attempt := 0; attempt < 4; attempt++ {
			var issue error
			var got []int
			for k, v := range m.Search(ctx, func(k, v *int) sql.Query {
				if decreasing {
					return sql.Query{sql.Order(v).Decreasing(), sql.Slice(3, 20)}
				}
				return sql.Query{sql.Order(v).Increasing(), sql.Slice(3, 20)}
			}, &issue) {
				if v != (k*13)%31 {
					t.Fatalf("key %d has mismatched value %d", k, v)
				}
				got = append(got, v)
			}
			if issue != nil {
				t.Fatal(issue)
			}
			if len(got) != 17 {
				t.Fatalf("got %d results, want 17", len(got))
			}
			for i, v := range got {
				want := i + 3
				if decreasing {
					want = 30 - want
				}
				if v != want {
					t.Fatalf("decreasing=%v: got %v; value at %d should be %d", decreasing, got, i, want)
				}
			}
		}
	}
}

func TestRAMWriteSlice(t *testing.T) {
	ctx := context.Background()
	for _, operation := range []string{"delete", "update"} {
		for _, span := range [][2]int{{0, 0}, {3, 3}, {4, 2}, {1, 3}} {
			t.Run(fmt.Sprintf("%s/%d:%d", operation, span[0], span[1]), func(t *testing.T) {
				var m sql.Map[int, int]
				for k := 1; k <= 8; k++ {
					if err := m.Set(ctx, k, k); err != nil {
						t.Fatal(err)
					}
				}
				query := func(k, v *int) sql.Query {
					return sql.Query{sql.Slice(span[0], span[1])}
				}
				var count int
				var err error
				if operation == "delete" {
					count, err = m.UnsafeDelete(ctx, query)
				} else {
					count, err = m.Update(ctx, query, func(v *int) sql.Patch {
						return sql.Patch{sql.Set(v, 99)}
					})
				}
				want := max(0, span[1]-span[0])
				if (err != nil) != (want == 0) {
					t.Fatalf("got error %v for a slice of %d", err, want)
				}
				if count != want {
					t.Errorf("reported %d changes, want %d", count, want)
				}
				changed := 0
				for k := 1; k <= 8; k++ {
					v, ok, err := m.Lookup(ctx, k)
					if err != nil {
						t.Fatal(err)
					}
					if !ok || v != k {
						changed++
					}
				}
				if changed != want {
					t.Errorf("actually changed %d records, want %d", changed, want)
				}
			})
		}
	}
}

func TestRAMSearchMultipleOrders(t *testing.T) {
	type row struct{ A, B, C int }
	ctx := context.Background()
	var m sql.Map[int, row]
	for k := 0; k < 36; k++ {
		if err := m.Set(ctx, k+1, row{k % 3, (k / 3) % 3, k % 2}); err != nil {
			t.Fatal(err)
		}
	}
	for _, descending := range []bool{false, true} {
		var issue error
		var previous row
		count := 0
		for _, v := range m.Search(ctx, func(k *int, v *row) sql.Query {
			middle := sql.Order(&v.B).Increasing()
			if descending {
				middle = sql.Order(&v.B).Decreasing()
			}
			return sql.Query{sql.Order(&v.A).Increasing(), middle, sql.Order(&v.C).Increasing()}
		}, &issue) {
			if count > 0 {
				badMiddle := previous.B > v.B
				if descending {
					badMiddle = previous.B < v.B
				}
				if previous.A > v.A || previous.A == v.A && (badMiddle || previous.B == v.B && previous.C > v.C) {
					t.Fatalf("descending=%v: %v appears before %v", descending, previous, v)
				}
			}
			previous = v
			count++
		}
		if issue != nil {
			t.Fatal(issue)
		}
		if count != 36 {
			t.Fatalf("got %d rows, want 36", count)
		}
	}
}

func TestRAMOrderedWrites(t *testing.T) {
	type row struct{ Group, Value int }
	ctx := context.Background()
	for _, operation := range []string{"update", "delete"} {
		for _, descending := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/descending=%v", operation, descending), func(t *testing.T) {
				var m sql.Map[int, row]
				for k := 1; k <= 36; k++ {
					if err := m.Set(ctx, k, row{Group: k % 3, Value: 100 - k}); err != nil {
						t.Fatal(err)
					}
				}
				query := func(k *int, v *row) sql.Query {
					order := sql.Order(&v.Value).Increasing()
					if descending {
						order = sql.Order(&v.Value).Decreasing()
					}
					return sql.Query{sql.Avoid(sql.Index(&v.Group).Equals(0)), sql.Order(&v.Group).Increasing(), order, sql.Slice(2, 7)}
				}
				// Group 1 comes first, sorted by value in the requested direction.
				keys := []int{28, 25, 22, 19, 16}
				if descending {
					keys = []int{7, 10, 13, 16, 19}
				}
				expected := make(map[int]bool)
				for _, k := range keys {
					expected[k] = true
				}
				var count int
				var err error
				if operation == "delete" {
					count, err = m.UnsafeDelete(ctx, query)
				} else {
					count, err = m.Update(ctx, query, func(v *row) sql.Patch { return sql.Patch{sql.Set(&v.Value, -1)} })
				}
				if err != nil {
					t.Fatal(err)
				}
				if count != len(keys) {
					t.Fatalf("changed %d rows, want %d", count, len(keys))
				}
				for k := 1; k <= 36; k++ {
					value, ok, err := m.Lookup(ctx, k)
					if err != nil {
						t.Fatal(err)
					}
					if operation == "delete" {
						if ok == expected[k] {
							t.Errorf("key %d exists=%v, should change=%v", k, ok, expected[k])
						}
					} else {
						want := 100 - k
						if expected[k] {
							want = -1
						}
						if !ok || value.Value != want {
							t.Errorf("key %d: got %+v (exists=%v), want value=%d", k, value, ok, want)
						}
					}
				}
			})
		}
	}
}

func TestRAMSum(t *testing.T) {
	ctx := context.Background()
	var m sql.Map[string, int]
	for key, value := range map[string]int{"a": 1, "b": 2, "c": 3} {
		if err := m.Set(ctx, key, value); err != nil {
			t.Fatal(err)
		}
	}
	var total int
	stats := func(_ *string, value *int) sql.Stats { return sql.Stats{sql.Sum(value, &total)} }
	for range 2 {
		if err := m.Output(ctx, nil, stats); err != nil {
			t.Fatal(err)
		}
		if total != 6 {
			t.Fatalf("got total %d, want 6", total)
		}
	}
}

func TestRAMOutputReplacesCounters(t *testing.T) {
	ctx := context.Background()
	var m sql.Map[int, int]
	for k := 1; k <= 3; k++ {
		if err := m.Set(ctx, k, k); err != nil {
			t.Fatal(err)
		}
	}
	var count atomic.Int64
	stats := func(*int, *int) sql.Stats { return sql.Stats{sql.Count(&count)} }
	for range 2 {
		if err := m.Output(ctx, nil, stats); err != nil {
			t.Fatal(err)
		}
		if count.Load() != 3 {
			t.Fatalf("got count %d, want 3", count.Load())
		}
	}
	if err := m.Output(ctx, func(k, _ *int) sql.Query { return sql.Query{sql.Index(k).Equals(9)} }, stats); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 0 {
		t.Fatalf("no matches left count %d", count.Load())
	}
}

func TestRAMOutputWithoutStats(t *testing.T) {
	var m sql.Map[int, int]
	if err := m.Set(context.Background(), 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := m.Output(context.Background(), nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestRAMUpdateLeavesDeletedRowDeleted(t *testing.T) {
	ctx := context.Background()
	var m sql.Map[int, int]
	if err := m.Set(ctx, 1, 1); err != nil {
		t.Fatal(err)
	}
	patching, release := make(chan struct{}), make(chan struct{})
	updated := make(chan error, 1)
	go func() {
		var once sync.Once
		_, err := m.Update(ctx, func(*int, *int) sql.Query { return sql.Query{sql.Slice(0, 10)} }, func(v *int) sql.Patch {
			once.Do(func() { close(patching) })
			<-release
			return sql.Patch{sql.Set(v, 2)}
		})
		updated <- err
	}()
	<-patching
	deleted, err := m.Delete(ctx, 1, nil)
	close(release)
	if err := <-updated; err != nil {
		t.Fatal(err)
	}
	if !deleted || err != nil {
		t.Fatalf("delete gave %v and %v", deleted, err)
	}
	if _, exists, _ := m.Lookup(ctx, 1); exists {
		t.Fatal("update brought a deleted row back")
	}
}

func TestRAMUpdateRepatchesChangedRow(t *testing.T) {
	ctx := context.Background()
	var m sql.Map[int, int]
	if err := m.Set(ctx, 1, 10); err != nil {
		t.Fatal(err)
	}
	patching, release := make(chan struct{}), make(chan struct{})
	updated := make(chan error, 1)
	go func() {
		var once sync.Once
		_, err := m.Update(ctx, func(*int, *int) sql.Query { return sql.Query{sql.Slice(0, 10)} }, func(v *int) sql.Patch {
			once.Do(func() { close(patching) })
			<-release
			return sql.Patch{sql.Set(v, *v+1)}
		})
		updated <- err
	}()
	<-patching
	if err := m.Set(ctx, 1, 20); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-updated; err != nil {
		t.Fatal(err)
	}
	if got, _ := m.Get(ctx, 1); got != 21 {
		t.Fatalf("got %d, want the upsert and the update", got)
	}
}

func TestRAMUpdateDropsRowThatStopsMatching(t *testing.T) {
	ctx := context.Background()
	var m sql.Map[int, int]
	if err := m.Set(ctx, 1, 10); err != nil {
		t.Fatal(err)
	}
	patching, release := make(chan struct{}), make(chan struct{})
	type result struct {
		count int
		err   error
	}
	updated := make(chan result, 1)
	go func() {
		var once sync.Once
		count, err := m.Update(ctx, func(_, v *int) sql.Query {
			return sql.Query{sql.Index(v).Equals(10), sql.Slice(0, 10)}
		}, func(v *int) sql.Patch {
			once.Do(func() { close(patching) })
			<-release
			return sql.Patch{sql.Set(v, 11)}
		})
		updated <- result{count, err}
	}()
	<-patching
	if err := m.Set(ctx, 1, 20); err != nil {
		t.Fatal(err)
	}
	close(release)
	if got := <-updated; got.count != 0 || got.err != nil {
		t.Fatalf("update gave %+v", got)
	}
	if got, _ := m.Get(ctx, 1); got != 20 {
		t.Fatalf("got %d", got)
	}
}

func TestRAMUpdateOfUncomparableValues(t *testing.T) {
	type row struct {
		List  []int
		Ratio float64
	}
	ctx := context.Background()
	var m sql.Map[int, row]
	if err := m.Set(ctx, 1, row{List: []int{1}, Ratio: math.NaN()}); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		_, err := m.Update(ctx, func(*int, *row) sql.Query { return sql.Query{sql.Slice(0, 1)} }, func(r *row) sql.Patch {
			return sql.Patch{sql.Set(&r.List, []int{2})}
		})
		finished <- err
	}()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("update never finished")
	}
	if got, _ := m.Get(ctx, 1); len(got.List) != 1 || got.List[0] != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestRAMUpsertIsNotLostToMutate(t *testing.T) {
	ctx := context.Background()
	var m sql.Map[int, int]
	if err := m.Set(ctx, 1, 1); err != nil {
		t.Fatal(err)
	}
	patching, release := make(chan struct{}), make(chan struct{})
	mutated := make(chan error, 1)
	go func() {
		_, err := m.Mutate(ctx, 1, nil, func(v *int) sql.Patch {
			close(patching)
			<-release
			return sql.Patch{sql.Set(v, 2)}
		})
		mutated <- err
	}()
	<-patching
	upserted := make(chan error, 1)
	go func() { upserted <- m.Set(ctx, 1, 3) }()
	// Without this pause an upsert that does not wait for Mutate could still come
	// second, and the test would pass with the bug present.
	select {
	case err := <-upserted:
		close(release)
		<-mutated
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := m.Get(ctx, 1); got != 3 {
			t.Fatalf("an acknowledged upsert was lost: got %d", got)
		}
		return
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	if err := <-mutated; err != nil {
		t.Fatal(err)
	}
	if err := <-upserted; err != nil {
		t.Fatal(err)
	}
	if got, _ := m.Get(ctx, 1); got != 3 {
		t.Fatalf("an acknowledged upsert was lost: got %d", got)
	}
}

func TestRAMConcurrentWritesLoseNothing(t *testing.T) {
	const writers, rounds = 8, 200
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var m sql.Map[int, int]
	if err := m.Set(ctx, 1, 0); err != nil {
		t.Fatal(err)
	}
	increment := func(v *int) sql.Patch { return sql.Patch{sql.Set(v, *v+1)} }
	var running sync.WaitGroup
	for writer := range writers {
		running.Add(1)
		go func() {
			defer running.Done()
			for range rounds {
				var err error
				if writer%2 == 0 {
					_, err = m.Update(ctx, func(*int, *int) sql.Query { return sql.Query{sql.Slice(0, 1)} }, increment)
				}
				if writer%2 == 1 {
					_, err = m.Mutate(ctx, 1, nil, increment)
				}
				if err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	running.Wait()
	if got, _ := m.Get(ctx, 1); got != writers*rounds {
		t.Fatalf("got %d, want %d", got, writers*rounds)
	}
}

func TestRAMUnsafeDeleteWaitsForMutate(t *testing.T) {
	ctx := context.Background()
	var m sql.Map[int, int]
	if err := m.Set(ctx, 1, 1); err != nil {
		t.Fatal(err)
	}
	patching, release := make(chan struct{}), make(chan struct{})
	mutated := make(chan error, 1)
	go func() {
		_, err := m.Mutate(ctx, 1, nil, func(v *int) sql.Patch {
			close(patching)
			<-release
			return sql.Patch{sql.Set(v, 2)}
		})
		mutated <- err
	}()
	<-patching
	deleted := make(chan int, 1)
	go func() {
		n, err := m.UnsafeDelete(ctx, func(_, v *int) sql.Query {
			return sql.Query{sql.Index(v).Equals(1), sql.Slice(0, 1)}
		})
		if err != nil {
			t.Error(err)
		}
		deleted <- n
	}()
	select {
	case n := <-deleted:
		t.Errorf("deleted %d rows a mutate was changing", n)
		close(release)
		<-mutated
		return
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	if err := <-mutated; err != nil {
		t.Fatal(err)
	}
	if n := <-deleted; n != 0 {
		t.Fatalf("deleted %d rows that no longer matched", n)
	}
	if got, ok, _ := m.Lookup(ctx, 1); !ok || got != 2 {
		t.Fatalf("got %d, %v", got, ok)
	}
}

func TestRAMAddSkipsUsedKeys(t *testing.T) {
	ctx := context.Background()
	var m sql.Map[int, string]
	for k := 1; k <= 3; k++ {
		if err := m.Set(ctx, k, "taken"); err != nil {
			t.Fatal(err)
		}
	}
	key, err := m.Add(ctx, "added")
	if err != nil {
		t.Fatal(err)
	}
	if key != 4 {
		t.Fatalf("got key %d, want 4", key)
	}
	if got, _ := m.Get(ctx, 1); got != "taken" {
		t.Fatalf("add replaced a row: %q", got)
	}
}

func TestRAMAddRunsOutOfKeys(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var m sql.Map[int8, int]
	for want := 1; want <= math.MaxInt8; want++ {
		key, err := m.Add(ctx, want)
		if err != nil {
			t.Fatal(err)
		}
		if int(key) != want {
			t.Fatalf("got key %d, want %d", key, want)
		}
	}
	if key, err := m.Add(ctx, 0); !errors.Is(err, sql.ErrDuplicate) {
		t.Fatalf("a full key space gave %d and %v", key, err)
	}
}

type randomizedKey int

func (key *randomizedKey) Randomize() { *key = 42 }

func TestRAMAddRandomizesIntegerKeys(t *testing.T) {
	var m sql.Map[randomizedKey, int]
	key, err := m.Add(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if key != 42 {
		t.Fatalf("got key %d, want the random one", key)
	}
}
