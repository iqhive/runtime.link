package sql_test

import (
	"context"
	"fmt"
	"testing"

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
