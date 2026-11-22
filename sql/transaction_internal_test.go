package sql

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/sql/std/sodium"
)

// holdRAMTransactions starts a transaction that holds the lock on RAM maps
// until the returned function is called, which also waits for it to end.
func holdRAMTransactions(t *testing.T) (release func()) {
	t.Helper()
	held, done, finished := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		finished <- manageRAM(context.Background(), func(context.Context) error {
			close(held)
			<-done
			return nil
		})
	}()
	select {
	case <-held:
	case <-time.After(5 * time.Second):
		t.Fatal("the transaction lock was never free")
	}
	return func() {
		t.Helper()
		close(done)
		if err := <-finished; err != nil {
			t.Fatalf("the holding transaction failed: %v", err)
		}
	}
}

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

// A transaction that is only slow must not fail the ones waiting behind it.
func TestManageRAMWaitsBehindSlowTransaction(t *testing.T) {
	release := holdRAMTransactions(t)
	finished := make(chan error, 1)
	go func() {
		finished <- manageRAM(context.Background(), func(context.Context) error { return nil })
	}()
	select {
	case err := <-finished:
		t.Fatalf("a transaction did not wait its turn: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	release()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("a transaction that waited its turn failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a waiting transaction never got its turn")
	}
}

// A nested call joins the transaction of its context, so it does not wait.
func TestManageRAMNestedJoinNeverWaits(t *testing.T) {
	err := manageRAM(context.Background(), func(ctx context.Context) error {
		return manageRAM(ctx, func(context.Context) error { return nil })
	})
	if err != nil {
		t.Fatalf("a nested transaction with the context of the outer one gave %v", err)
	}
}

// silentDatabase takes the jobs of a transaction and answers none of them.
type silentDatabase struct {
	Database
	closed chan struct{}
}

func (db silentDatabase) Manage(context.Context, TransactionLevel) (chan<- sodium.Job, error) {
	jobs := make(chan sodium.Job)
	go func() {
		defer close(db.closed)
		for range jobs {
		}
	}()
	return jobs, nil
}

// The conformance test does not wait for as long as its context lasts for a
// commit to be answered: a context without a deadline would never say what
// is wrong with the database.
func TestCommitThatIsNotAnsweredIsReported(t *testing.T) {
	db := silentDatabase{closed: make(chan struct{})}
	err := testCommitAnswered(context.Background(), db, 20*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "sodium.Commit.Done") {
		t.Errorf("a commit that was not answered gave %v", err)
	}
	select {
	case <-db.closed:
	case <-time.After(5 * time.Second):
		t.Error("the transaction was left open")
	}
	// A context that ends first is reported as that.
	db = silentDatabase{closed: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := testCommitAnswered(ctx, db, time.Hour); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a context that ended gave %v", err)
	}
	<-db.closed
	if commitAnswerWait < time.Second {
		t.Errorf("a wait of %v fails a database that is merely slow to commit", commitAnswerWait)
	}
}

// A commit is the last thing a transaction is sent, after which its database
// takes no more jobs. A job sent then is refused at once, rather than left
// waiting, with the lock that ending the transaction needs, for as long as
// its context lasts.
func TestJobSentAfterCommitIsRefused(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	jobs := make(chan sodium.Job, 1) // room for the commit, and nobody to take what follows.
	tx := &transaction{jobs: jobs, answers: true}
	if err := tx.sendCommit(ctx, sodium.NewCommit()); err != nil {
		t.Fatal(err)
	}
	if err := tx.send(ctx, sodium.NewCommit(), nil); err != ErrTransactionEnded {
		t.Errorf("a job sent after the commit gave %v, want ErrTransactionEnded", err)
	}
	ended := make(chan struct{})
	go func() { tx.end(ctx, true); close(ended) }()
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("the transaction could not be ended")
	}
	if _, open := <-jobs; !open {
		t.Fatal("the commit was not sent")
	}
	if job, open := <-jobs; open {
		t.Errorf("the transaction was sent %v after its commit", job)
	}
}
