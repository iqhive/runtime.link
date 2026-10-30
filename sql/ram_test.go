package sql_test

import (
	"context"
	"testing"

	"github.com/iqhive/runtime.link/sql"
)

func TestRAM(t *testing.T) {
	if err := sql.Test(context.Background(), nil); err != nil {
		t.Fatal(err)
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
					return sql.Query{sql.Order(v).Decreasing()}
				}
				return sql.Query{sql.Order(v).Increasing()}
			}, &issue) {
				if v != (k*13)%31 {
					t.Fatalf("key %d has mismatched value %d", k, v)
				}
				got = append(got, v)
			}
			if issue != nil {
				t.Fatal(issue)
			}
			if len(got) != 31 {
				t.Fatalf("got %d results, want 31", len(got))
			}
			for i, v := range got {
				want := i
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
