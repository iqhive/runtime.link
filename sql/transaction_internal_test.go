package sql

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestManageRAMNestedWithOuterContextGivesUp(t *testing.T) {
	const wait = 20 * time.Millisecond
	ctx := context.Background()
	var m Map[int, int]
	finished := make(chan error, 1)
	go func() {
		finished <- manageRAM(ctx, wait, func(context.Context) error {
			// Not the context of the transaction, so this is a second transaction
			// that waits for the first, and a plain context never ends that wait.
			return manageRAM(ctx, wait, func(context.Context) error { return nil })
		})
	}()
	select {
	case err := <-finished:
		if !errors.Is(err, errTransactionBusy) {
			t.Fatalf("error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a nested transaction with the outer context never gave up")
	}
	if err := m.Manage(ctx, ReadWrites, func(ctx context.Context) error { return m.Set(ctx, 1, 1) }); err != nil {
		t.Fatalf("the transaction lock was not released: %v", err)
	}
}
