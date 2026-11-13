//go:build linux

package cmdl_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/cmdl"
)

// promptly is well short of the five seconds that a call waits for output
// still held open by something its command left running.
const promptly = 4 * time.Second

func TestCancelledCallKillsItsProcessGroup(t *testing.T) {
	s := linkShell()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	if got, err := s.Text(ctx, "sleep 30 & wait"); err == nil {
		t.Errorf("got %q", got)
	}
	// A surviving descendant would hold the output open for the whole drain
	// delay, so a call that ends sooner proves the group died with it.
	if elapsed := time.Since(started); elapsed >= promptly {
		t.Errorf("took %v, the background sleep outlived the cancellation", elapsed)
	}
}

func TestCancelledCallLeavesNothingRunning(t *testing.T) {
	type Opts struct {
		Out io.Writer
	}
	A := api.Import[struct {
		api.Specification
		Run func(context.Context, string, Opts) error `cmdl:"-c %v %v"`
	}](cmdl.API, "sh", nil)
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// With no result to capture, the command and the sleep it starts write
	// straight to the pipe, so the pipe stays open for as long as either lives.
	finished := make(chan error, 1)
	go func() {
		finished <- A.Run(ctx, "sleep 30 & echo started; wait", Opts{Out: write})
	}()
	read.SetReadDeadline(time.Now().Add(streamTimeout))
	if line, err := bufio.NewReader(read).ReadString('\n'); err != nil || line != "started\n" {
		t.Fatalf("the command did not start: %q err=%v", line, err)
	}
	cancelled := time.Now()
	cancel()
	select {
	case err := <-finished:
		if err == nil {
			t.Error("a cancelled call reported no error")
		}
	case <-time.After(streamTimeout):
		t.Fatal("the cancelled call never returned")
	}
	if elapsed := time.Since(cancelled); elapsed >= promptly {
		t.Errorf("the cancelled call took %v to return", elapsed)
	}
	write.Close()
	read.SetReadDeadline(time.Now().Add(promptly))
	if _, err := read.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Errorf("the background sleep outlived the cancellation: %v", err)
	}
}

func TestCancelledStreamKillsItsProcessGroup(t *testing.T) {
	s := linkShell()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out, errs := s.Both(ctx, "sleep 30 & echo started; wait")
	select {
	case line := <-out:
		if string(line) != "started" {
			t.Fatalf("the command did not start: %q", line)
		}
	case <-time.After(streamTimeout):
		t.Fatal("the command did not start")
	}
	cancelled := time.Now()
	cancel()
	drainLines(t, out)
	drainErrors(t, errs)
	// The sleep holds both pipes, which only close early if it was killed.
	if elapsed := time.Since(cancelled); elapsed >= promptly {
		t.Errorf("the stream took %v to close, the background sleep outlived the cancellation", elapsed)
	}
}

func TestCommandThatFinishesLeavesItsProcessGroupAlone(t *testing.T) {
	type Opts struct {
		Out io.Writer
	}
	A := api.Import[struct {
		api.Specification
		Run func(context.Context, string, Opts) error `cmdl:"-c %v %v"`
	}](cmdl.API, "sh", nil)
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	// Only a cancelled context kills the group, so what a command leaves
	// running when it exits by itself goes on to write its line.
	err = A.Run(context.Background(), "(sleep 1; echo later) 2>/dev/null &", Opts{Out: write})
	write.Close()
	if err != nil {
		t.Fatal(err)
	}
	read.SetReadDeadline(time.Now().Add(streamTimeout))
	if line, err := bufio.NewReader(read).ReadString('\n'); err != nil || line != "later\n" {
		t.Errorf("got %q err=%v, want the line written after the command exited", line, err)
	}
}
