package qnq_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/iqhive/runtime.link/qnq"
)

func TestPubSub(t *testing.T) {
	ctx := context.Background()

	var handled atomic.Int32

	var topic = qnq.Chan[string]{}
	topic.Listen(ctx, "1", func(ctx context.Context, s string) error {
		handled.Add(1)
		return nil
	})

	cctx, cancel := context.WithCancel(ctx)
	topic.Listen(cctx, "2", func(ctx context.Context, s string) error {
		handled.Add(1)
		return nil
	})

	topic.Listen(ctx, "3", func(ctx context.Context, s string) error {
		handled.Add(1)
		return nil
	})
	topic.Listen(ctx, "4", func(ctx context.Context, s string) error {
		handled.Add(1)
		return nil
	})

	if err := topic.Send(ctx, "hello"); err != nil {
		t.Fatal(err)
	}
	if handled.Load() != 4 {
		t.Fatal("expected 4 handlers to be called")
	}

	cancel()

	handled.Store(0)

	if err := topic.Send(ctx, "hello"); err != nil {
		t.Fatal(err)
	}
	if handled.Load() != 3 {
		t.Fatal("expected 3 handlers to be called")
	}
}

func TestCanceledListeners(t *testing.T) {
	for _, tc := range []struct {
		name     string
		count    int
		canceled []int
	}{
		{"only", 1, []int{0}},
		{"first", 4, []int{0}},
		{"last", 4, []int{3}},
		{"adjacent", 4, []int{1, 2}},
		{"all", 4, []int{0, 1, 2, 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			var topic qnq.Chan[int]
			calls := make([]int, tc.count)
			cancels := make([]context.CancelFunc, tc.count)
			for i := range tc.count {
				listenerCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				cancels[i] = cancel
				topic.Listen(listenerCtx, "listener", func(context.Context, int) error {
					calls[i]++
					return nil
				})
			}
			if err := topic.Send(ctx, 1); err != nil {
				t.Fatal(err)
			}
			for _, i := range tc.canceled {
				cancels[i]()
			}
			for range 2 {
				err := topic.Send(ctx, 2)
				if len(tc.canceled) == tc.count {
					if err != qnq.ErrEmptyChannel {
						t.Fatalf("got %v, want ErrEmptyChannel", err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
			}
			for i, got := range calls {
				want := 3
				for _, canceled := range tc.canceled {
					if i == canceled {
						want = 1
					}
				}
				if got != want {
					t.Errorf("listener %d called %d times, want %d", i, got, want)
				}
			}
		})
	}
}
