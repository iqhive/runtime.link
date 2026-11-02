package sql

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A transaction waits for its turn for as long as its context allows, so a
// nested call given the outer context, which waits for its own transaction,
// only ends with that context.
func TestManageRAMNestedWithOuterContextWaitsForContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var m Map[int, int]
	waiting := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		finished <- manageRAM(ctx, func(context.Context) error {
			close(waiting)
			// Not the context of the transaction, so this is a second transaction.
			return manageRAM(ctx, func(context.Context) error { return nil })
		})
	}()
	<-waiting
	select {
	case err := <-finished:
		t.Fatalf("a waiting transaction gave up with %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a waiting transaction outlived its context")
	}
	if err := m.Manage(context.Background(), ReadWrites, func(ctx context.Context) error { return m.Set(ctx, 1, 1) }); err != nil {
		t.Fatalf("the transaction lock was not released: %v", err)
	}
}
