package cmdl_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/cmdl"
)

type brokenWriter struct{ err error }

func (w brokenWriter) Write([]byte) (int, error) { return 0, w.err }

// A result that could not be written, to a closed pipe or a full disk, was
// once a failure for a []byte and for JSON only. Any other result returned
// nil, so the program exited 0 having printed nothing.
func TestResultThatCannotBeWrittenFailsTheCommand(t *testing.T) {
	type API struct {
		api.Specification `
			prints things`

		Text   func(context.Context) (string, error)   `cmdl:"text"`
		Lines  func(context.Context) ([]string, error) `cmdl:"lines"`
		Bytes  func(context.Context) ([]byte, error)   `cmdl:"bytes"`
		Number func(context.Context) (int32, error)    `cmdl:"number"`
		JSON   func(context.Context) ([]int, error)    `cmdl:"json,json"`
		Stream func(context.Context) chan []byte       `cmdl:"stream"`
		None   func(context.Context) error             `cmdl:"none"`
	}
	program := API{
		Text:   func(context.Context) (string, error) { return "hello", nil },
		Lines:  func(context.Context) ([]string, error) { return []string{"a", "b"}, nil },
		Bytes:  func(context.Context) ([]byte, error) { return []byte("hello"), nil },
		Number: func(context.Context) (int32, error) { return 7, nil },
		JSON:   func(context.Context) ([]int, error) { return []int{1, 2}, nil },
		Stream: func(context.Context) chan []byte {
			lines := make(chan []byte, 1)
			lines <- []byte("hello")
			close(lines)
			return lines
		},
		None: func(context.Context) error { return nil },
	}
	broken := errors.New("broken pipe")
	run := func(args ...string) error {
		return cmdl.System{Args: append([]string{"prog"}, args...), Stdout: brokenWriter{broken}}.Run(program)
	}
	for _, command := range []string{"text", "lines", "bytes", "number", "json", "stream"} {
		if err := run(command); !errors.Is(err, broken) {
			t.Errorf("%s: Run returned %v although nothing could be written", command, err)
		}
	}
	if err := run(); !errors.Is(err, broken) {
		t.Errorf("help: Run returned %v although nothing could be written", err)
	}
	if err := run("none"); err != nil {
		t.Errorf("a command with nothing to write: %v", err)
	}
	// What can be written still is, in full.
	want := map[string]string{"text": "hello\n", "lines": "a\nb\n", "bytes": "104\n101\n108\n108\n111\n", "number": "7\n", "json": "[\n\t1,\n\t2\n]\n", "stream": "hello\n", "none": ""}
	for command, text := range want {
		if out, err := (cmdl.System{Args: []string{"prog", command}}).Output(program); err != nil || string(out) != text {
			t.Errorf("%s: wrote %q err=%v, want %q", command, out, err, text)
		}
	}
}

type streamsAPI struct {
	api.Specification

	Tail     func(context.Context) chan []byte               `cmdl:"tail"`
	Checked  func(context.Context) (chan []byte, error)      `cmdl:"checked"`
	Fail     func(context.Context) chan error                `cmdl:"fail"`
	Both     func(context.Context) (chan []byte, chan error) `cmdl:"both"`
	Clean    func(context.Context) (chan []byte, chan error) `cmdl:"clean"`
	Nothing  func(context.Context) (chan []byte, chan error) `cmdl:"nothing"`
	Refused  func(context.Context) (chan []byte, error)      `cmdl:"refused"`
	Numbers  func(context.Context) chan int                  `cmdl:"numbers"`
	Texts    func(context.Context) (chan string, error)      `cmdl:"texts"`
	Reversed func(context.Context) (chan error, chan []byte) `cmdl:"reversed"`
	Encoded  func(context.Context) chan []byte               `cmdl:"encoded,json"`
	Counted  func(context.Context) (chan int, error)         `cmdl:"counted,json"`
}

func linesOf(lines ...string) chan []byte {
	sent := make(chan []byte, len(lines))
	for _, line := range lines {
		sent <- []byte(line)
	}
	close(sent)
	return sent
}

func errorsOf(texts ...string) chan error {
	sent := make(chan error, len(texts))
	for _, text := range texts {
		sent <- errors.New(text)
	}
	close(sent)
	return sent
}

// A link reads a chan []byte as the lines of stdout and a chan error as those
// of stderr. A program with such a result once printed the address of the
// channel and exited 0 without reading it.
func TestChannelResultsAreWrittenAsALinkReadsThem(t *testing.T) {
	var called []string
	program := streamsAPI{
		Tail:    func(context.Context) chan []byte { return linesOf("one", "two", "") },
		Checked: func(context.Context) (chan []byte, error) { return linesOf("one"), nil },
		Fail:    func(context.Context) chan error { return errorsOf("first", "second", "third") },
		Both: func(context.Context) (chan []byte, chan error) {
			return linesOf("one", "two"), errorsOf("first", "second")
		},
		Clean:   func(context.Context) (chan []byte, chan error) { return linesOf("one"), errorsOf() },
		Nothing: func(context.Context) (chan []byte, chan error) { return nil, nil },
		Refused: func(context.Context) (chan []byte, error) { return nil, errors.New("refused") },
		Numbers: func(context.Context) chan int {
			called = append(called, "numbers")
			return nil
		},
		Texts: func(context.Context) (chan string, error) {
			called = append(called, "texts")
			return nil, nil
		},
		Reversed: func(context.Context) (chan error, chan []byte) {
			called = append(called, "reversed")
			return nil, nil
		},
		Encoded: func(context.Context) chan []byte { return linesOf(`{"n":1}`) },
		Counted: func(context.Context) (chan int, error) {
			called = append(called, "counted")
			return nil, nil
		},
	}
	for _, test := range []struct {
		command        string
		stdout, stderr string
		failure        string
	}{
		{"tail", "one\ntwo\n\n", "", ""},
		{"checked", "one\n", "", ""},
		{"fail", "", "first\nsecond\n", "third"},
		{"both", "one\ntwo\n", "first\n", "second"},
		{"clean", "one\n", "", ""},
		{"nothing", "", "", ""},
		{"refused", "", "", "refused"},
		{"encoded", "{\"n\":1}\n", "", ""},
	} {
		var stdout, stderr bytes.Buffer
		err := cmdl.System{Args: []string{"prog", test.command}, Stdout: &stdout, Stderr: &stderr}.Run(program)
		if stdout.String() != test.stdout || stderr.String() != test.stderr {
			t.Errorf("%s: wrote %q to stdout and %q to stderr, want %q and %q", test.command, stdout.String(), stderr.String(), test.stdout, test.stderr)
		}
		// The last error is the one the command fails with, which Main
		// writes to stderr after the others.
		if (err == nil) != (test.failure == "") || err != nil && err.Error() != test.failure {
			t.Errorf("%s: Run returned %v, want %q", test.command, err, test.failure)
		}
	}
	// Without a stderr the errors have nowhere to go but the one returned.
	if _, err := runProgram(program, "fail"); err == nil || err.Error() != "first\nsecond\nthird" {
		t.Errorf("fail without a stderr: Run returned %q", err)
	}
}

// reportingWriter tells of each write, so that a test can see a line arrive
// while the function that sent it is still running.
type reportingWriter chan string

func (w reportingWriter) Write(data []byte) (int, error) {
	w <- string(data)
	return len(data), nil
}

func TestChannelResultsAreWrittenAsTheyAreSent(t *testing.T) {
	written := make(reportingWriter, 1)
	seen := make(chan string, 3)
	program := streamsAPI{
		Both: func(ctx context.Context) (chan []byte, chan error) {
			lines, errs := make(chan []byte), make(chan error)
			go func() {
				defer close(lines)
				defer close(errs)
				// Each is only sent once the one before it has been written.
				lines <- []byte("one")
				seen <- <-written
				errs <- errors.New("first")
				errs <- errors.New("last")
				seen <- <-written
				lines <- []byte("two")
				seen <- <-written
			}()
			return lines, errs
		},
	}
	finished := make(chan error, 1)
	go func() {
		finished <- cmdl.System{Args: []string{"prog", "both"}, Stdout: written, Stderr: written}.Run(program)
	}()
	select {
	case err := <-finished:
		if err == nil || err.Error() != "last" {
			t.Errorf("Run returned %v, want the last error", err)
		}
	case <-time.After(streamTimeout):
		t.Fatal("the program waited for its channels to close before it wrote anything")
	}
	for _, want := range []string{"one\n", "first\n", "two\n"} {
		select {
		case got := <-seen:
			if got != want {
				t.Errorf("wrote %q, want %q", got, want)
			}
		default:
			t.Fatalf("%q was not written before the function carried on", want)
		}
	}
}

// A function that is still sending when its lines can no longer be written
// is told so by its context, as nothing reads its channel any more.
func TestChannelResultThatCannotBeWrittenEndsTheContext(t *testing.T) {
	stopped := make(chan struct{})
	program := streamsAPI{
		Tail: func(ctx context.Context) chan []byte {
			lines := make(chan []byte)
			go func() {
				defer close(stopped)
				for {
					select {
					case lines <- []byte("more"):
					case <-ctx.Done():
						return
					}
				}
			}()
			return lines
		},
	}
	broken := errors.New("broken pipe")
	if err := (cmdl.System{Args: []string{"prog", "tail"}, Stdout: brokenWriter{broken}}).Run(program); !errors.Is(err, broken) {
		t.Errorf("Run returned %v, want the error of the write", err)
	}
	select {
	case <-stopped:
	case <-time.After(streamTimeout):
		t.Fatal("the function was never told that its lines are not read any more")
	}
}
