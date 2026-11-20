package sql

import (
	"context"
	"errors"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"
)

func TestRAMCloneReferences(t *testing.T) {
	if got := cloneRAMValue[any](nil); got != nil {
		t.Fatalf("nil interface: %v", got)
	}
	type node struct {
		Next   *node
		Values [1][]int
		Time   time.Time
	}
	original := &node{Values: [1][]int{{10}}, Time: time.Now()}
	original.Next = original
	got := cloneRAMValue(original)
	if got == original || got.Next != got {
		t.Fatal("clone did not preserve cycle in independent copy")
	}
	got.Values[0][0] = 99
	if original.Values[0][0] != 10 {
		t.Fatal("array element still aliases original slice")
	}
	if got.Time != original.Time {
		t.Fatal("opaque time value changed")
	}
	recursive := map[string]any{}
	recursive["self"] = recursive
	copied := cloneRAMValue(recursive)
	copied["self"].(map[string]any)["new"] = true
	if copied["new"] != true {
		t.Fatal("map cycle was lost")
	}
	if _, exists := recursive["new"]; exists {
		t.Fatal("map clone aliases original")
	}
}

func TestRAMCloneKeepsImmutableValues(t *testing.T) {
	stamp := time.Now().In(time.FixedZone("local", 3600))
	addr := netip.MustParseAddr("fe80::1%eth0")
	got := cloneRAMValue([]any{&stamp, addr})
	if p := got[0].(*time.Time); p == &stamp || *p != stamp {
		t.Fatal("copy lost the timestamp or shared its pointer")
	}
	if got[1].(netip.Addr) != addr {
		t.Fatal("copy changed the address zone")
	}
}

func TestRAMDeletionLifecycle(t *testing.T) {
	for _, commit := range []bool{false, true} {
		ctx := context.Background()
		var m Map[int, int]
		if err := m.Set(ctx, 1, 10); err != nil {
			t.Fatal(err)
		}
		failure := errors.New("rollback")
		err := m.Manage(ctx, ReadWrites, func(tx context.Context) error {
			if ok, err := m.Delete(tx, 1, nil); err != nil || !ok {
				t.Fatalf("delete: %v, %v", ok, err)
			}
			if _, exists, err := m.Lookup(tx, 1); exists || err != nil {
				t.Fatalf("Lookup saw deleted row: %v, %v", exists, err)
			}
			var searchErr error
			for range m.Search(tx, nil, &searchErr) {
				t.Fatal("Search saw deleted row")
			}
			if searchErr != nil {
				t.Fatal(searchErr)
			}
			// The stats are asked for their counters before any row is visited,
			// so it is the count that tells whether the deleted row was.
			var count atomic.Int64
			if err := m.Output(tx, nil, func(*int, *int) Stats { return Stats{Count(&count)} }); err != nil {
				return err
			}
			if count.Load() != 0 {
				t.Fatalf("Output saw %d deleted rows", count.Load())
			}
			// Reuse the deleted key, delete it again, and exercise reverse undo order.
			if err := m.Insert(tx, 1, Create, 20); err != nil {
				return err
			}
			if _, err := m.Delete(tx, 1, nil); err != nil {
				return err
			}
			if !commit {
				return failure
			}
			return nil
		})
		if commit && err != nil || !commit && !errors.Is(err, failure) {
			t.Fatal(err)
		}
		value, exists, err := m.Lookup(ctx, 1)
		if err != nil || exists == commit || exists && value != 10 {
			t.Fatalf("after transaction: %v, %v, %v", value, exists, err)
		}
		m.kv.Map.Range(func(_, value any) bool {
			if _, deleted := value.(*ramDeletion); deleted {
				t.Fatal("transaction left a deletion marker behind")
			}
			return true
		})
	}
}
