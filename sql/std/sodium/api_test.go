package sodium_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/sql/std/sodium"
)

// The operation codes are on the wire, so they stay as they were published,
// even where Delete and Update share one.
func TestSocketOperationCodesArePublished(t *testing.T) {
	want := map[string]string{
		"Read":   "0",
		"Search": "1(1,2)",
		"Output": "2(1,2,3)",
		"Delete": "3(1,2)",
		"Insert": "4(1,2,3,4,5)",
		"Update": "3(1,2,3)",
		"Manage": "6(1)",
	}
	rtype := reflect.TypeFor[sodium.Socket]()
	got := make(map[string]string)
	for i := range rtype.NumField() {
		field := rtype.Field(i)
		if tag, ok := field.Tag.Lookup("xyz"); ok {
			got[field.Name] = tag
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got operation codes %v, want %v", got, want)
	}
}

// committer is a database that may or may not answer commits.
type committer struct {
	sodium.Database
	answers bool
}

func (db committer) AnswersCommits() bool { return db.answers }

// Only a database that says it answers commits is sent one: any other was
// written to commit when its channel is closed.
func TestAnswersCommits(t *testing.T) {
	var plain sodium.Database
	for name, test := range map[string]struct {
		db   sodium.Database
		want bool
	}{
		"nil":        {plain, false},
		"plain":      {struct{ sodium.Database }{}, false},
		"answers":    {committer{answers: true}, true},
		"does not":   {committer{answers: false}, false},
		"by pointer": {&committer{answers: true}, true},
	} {
		if got := sodium.AnswersCommits(test.db); got != test.want {
			t.Errorf("%s: got %v, want %v", name, got, test.want)
		}
	}
}

// A commit travels on the channel of a transaction like the jobs before it,
// and a database tells it apart from them by its type.
func TestCommitIsAJob(t *testing.T) {
	jobs := make(chan sodium.Job, 1)
	sent := sodium.NewCommit()
	jobs <- sent
	job := <-jobs
	if commit, ok := job.(*sodium.Commit); !ok || commit != sent {
		t.Fatalf("a commit arrived as %T", job)
	}
	if sodium.NewCommit() == sodium.NewCommit() {
		t.Error("two transactions were given one commit to answer")
	}
}

// A commit is over when the database says so, with how it went.
func TestCommitWaitsForItsAnswer(t *testing.T) {
	failure := errors.New("serialization failure")
	for name, answer := range map[string]error{"committed": nil, "refused": failure} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			commit := sodium.NewCommit()
			waited := make(chan error, 1)
			go func() {
				_, err := commit.Wait(ctx)
				waited <- err
			}()
			select {
			case err := <-waited:
				t.Fatalf("a commit nobody has answered was reported as %v", err)
			case <-time.After(20 * time.Millisecond):
			}
			commit.Done(answer)
			if err := <-waited; err != answer {
				t.Errorf("waiting gave %v, want %v", err, answer)
			}
			// The answer stays, and the first one is the answer.
			commit.Done(errors.New("said twice"))
			if n, err := commit.Wait(ctx); n != 0 || err != answer {
				t.Errorf("waiting again gave %d and %v, want 0 and %v", n, err, answer)
			}
		})
	}
}

// The end of the context ends the wait, not the commit: an answer that is
// there is the outcome, whatever became of the context since.
func TestCommitWaitAndTheEndOfTheContext(t *testing.T) {
	ended, cancel := context.WithCancel(context.Background())
	cancel()
	unanswered := sodium.NewCommit()
	if _, err := unanswered.Wait(ended); !errors.Is(err, context.Canceled) {
		t.Errorf("waiting for no answer with an ended context gave %v", err)
	}
	failure := errors.New("connection lost")
	for name, answer := range map[string]error{"committed": nil, "refused": failure} {
		for range 200 {
			commit := sodium.NewCommit()
			commit.Done(answer)
			if _, err := commit.Wait(ended); err != answer {
				t.Fatalf("%s: an answered commit was reported as %v once its context had ended", name, err)
			}
		}
	}
	// An answer that arrives after the wait gave up is still there to be read.
	unanswered.Done(nil)
	if _, err := unanswered.Wait(ended); err != nil {
		t.Errorf("the late answer was read as %v", err)
	}
}

// Many may ask how one commit went, and many may try to answer.
func TestCommitIsSafeToShare(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	commit := sodium.NewCommit()
	failure := errors.New("refused")
	var group sync.WaitGroup
	results := make([]error, 8)
	for i := range results {
		group.Add(2)
		go func() {
			defer group.Done()
			_, results[i] = commit.Wait(ctx)
		}()
		go func() {
			defer group.Done()
			commit.Done(failure)
		}()
	}
	group.Wait()
	for i, err := range results {
		if err != failure {
			t.Errorf("waiter %d was told %v", i, err)
		}
	}
}
