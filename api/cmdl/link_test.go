package cmdl_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/cmdl"
)

// streamTimeout only has to outlast the delays of a working stream, a
// broken one is reported when it expires.
const streamTimeout = 15 * time.Second

type (
	namedText  string
	namedBytes []byte
)

type shell struct {
	api.Specification

	Text       func(context.Context, string) (string, error)           `cmdl:"-c %v"`
	Lines      func(context.Context, string) ([]string, error)         `cmdl:"-c %v"`
	NamedLines func(context.Context, string) ([]namedText, error)      `cmdl:"-c %v"`
	Bytes      func(context.Context, string) ([]byte, error)           `cmdl:"-c %v"`
	NamedBytes func(context.Context, string) (namedBytes, error)       `cmdl:"-c %v"`
	Named      func(context.Context, string) (namedText, error)        `cmdl:"-c %v"`
	Numbers    func(context.Context, string) ([]int, error)            `cmdl:"-c %v"`
	Number     func(context.Context, string) (int32, error)            `cmdl:"-c %v"`
	Stream     func(context.Context, string) chan []byte               `cmdl:"-c %v"`
	Checked    func(context.Context, string) (chan []byte, error)      `cmdl:"-c %v"`
	Both       func(context.Context, string) (chan []byte, chan error) `cmdl:"-c %v"`
	Errors     func(context.Context, string) chan error                `cmdl:"-c %v"`
}

func linkShell() shell {
	return api.Import[shell](cmdl.API, "sh", nil)
}

// drainLines reads ch until it closes, a channel that stays open fails the test.
func drainLines(t *testing.T, ch chan []byte) []string {
	t.Helper()
	timeout := time.After(streamTimeout)
	var got []string
	for {
		select {
		case line, ok := <-ch:
			if !ok {
				return got
			}
			got = append(got, string(line))
		case <-timeout:
			t.Fatalf("lines still open after %v, received %q", streamTimeout, got)
		}
	}
}

// drainErrors reads ch until it closes, a channel that stays open fails the test.
func drainErrors(t *testing.T, ch chan error) []string {
	t.Helper()
	timeout := time.After(streamTimeout)
	var got []string
	for {
		select {
		case err, ok := <-ch:
			if !ok {
				return got
			}
			got = append(got, err.Error())
		case <-timeout:
			t.Fatalf("errors still open after %v, received %q", streamTimeout, got)
		}
	}
}

// awaitGoroutines polls because a finished goroutine leaves the count
// some time after the channel it closed is observed.
func awaitGoroutines(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(streamTimeout)
	for runtime.NumGoroutine() > want {
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<16)
			t.Fatalf("%d goroutines, want at most %d:\n%s", runtime.NumGoroutine(), want, buf[:runtime.Stack(buf, true)])
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func argvScript(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "argv")
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '[%s]' \"$a\"; done\necho\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCancelledStreamClosesItsChannels(t *testing.T) {
	s := linkShell()
	before := runtime.NumGoroutine()
	for range 100 {
		ctx, cancel := context.WithCancel(context.Background())
		out, errs := s.Both(ctx, "sleep 30")
		cancel()
		drainLines(t, out)
		drainErrors(t, errs)
	}
	awaitGoroutines(t, before)
}

func TestCancellingAStreamMidwayClosesIt(t *testing.T) {
	s := linkShell()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := s.Stream(ctx, "yes")
	<-out
	cancel()
	drainLines(t, out)
}

func TestStreamClosesWhenTheCommandFails(t *testing.T) {
	s := linkShell()
	ctx := context.Background()
	if got := drainLines(t, s.Stream(ctx, "echo one; exit 1")); !slices.Equal(got, []string{"one"}) {
		t.Errorf("Stream: %q", got)
	}
	out, err := s.Checked(ctx, "echo one; echo oops >&2; exit 1")
	if err != nil {
		t.Fatal(err)
	}
	if got := drainLines(t, out); !slices.Equal(got, []string{"one"}) {
		t.Errorf("Checked: %q", got)
	}
}

func TestStreamReportsFailureOnItsErrorChannel(t *testing.T) {
	s := linkShell()
	out, errs := s.Both(context.Background(), "echo one; echo oops >&2; exit 3")
	if got := drainLines(t, out); !slices.Equal(got, []string{"one"}) {
		t.Errorf("stdout: %q", got)
	}
	if got := drainErrors(t, errs); !slices.Equal(got, []string{"oops", "exit status 3"}) {
		t.Errorf("errors: %q", got)
	}
}

func TestUnreadStreamErrorDoesNotOutliveTheCommand(t *testing.T) {
	s := linkShell()
	before := runtime.NumGoroutine()
	out, _ := s.Both(context.Background(), "exit 3")
	drainLines(t, out)
	awaitGoroutines(t, before)
}

func TestStreamWithTrailingErrorResultFindsItsErrorChannel(t *testing.T) {
	type API struct {
		api.Specification
		Run func(context.Context, string) (chan []byte, chan error, error) `cmdl:"-c %v"`
	}
	A := api.Import[API](cmdl.API, "sh", nil)
	out, errs, err := A.Run(context.Background(), "echo one; exit 4")
	if err != nil {
		t.Fatal(err)
	}
	if got := drainLines(t, out); !slices.Equal(got, []string{"one"}) {
		t.Errorf("stdout: %q", got)
	}
	if got := drainErrors(t, errs); !slices.Equal(got, []string{"exit status 4"}) {
		t.Errorf("errors: %q", got)
	}
}

func TestLoneErrorChannelCarriesStderrAndExitStatus(t *testing.T) {
	s := linkShell()
	got := drainErrors(t, s.Errors(context.Background(), "echo oops >&2; exit 2"))
	if !slices.Equal(got, []string{"oops", "exit status 2"}) {
		t.Errorf("errors: %q", got)
	}
	if got := drainErrors(t, s.Errors(context.Background(), "true")); len(got) != 0 {
		t.Errorf("a clean exit reported %q", got)
	}
}

type startFailure struct {
	Dir    string `cmdl:",dir"`
	Script string `cmdl:"-c %v"`
}

type startFailureAPI struct {
	api.Specification

	Stream  func(context.Context, startFailure) chan []byte               `cmdl:"%v"`
	Checked func(context.Context, startFailure) (chan []byte, error)      `cmdl:"%v"`
	Both    func(context.Context, startFailure) (chan []byte, chan error) `cmdl:"%v"`
	Errors  func(context.Context, startFailure) chan error                `cmdl:"%v"`
}

func checkFailedStreams(t *testing.T, ctx context.Context, A startFailureAPI, opts startFailure) {
	t.Helper()
	if got := drainLines(t, A.Stream(ctx, opts)); len(got) != 0 {
		t.Errorf("Stream delivered %q", got)
	}
	out, err := A.Checked(ctx, opts)
	if err == nil {
		t.Error("Checked: want the start failure returned")
	}
	if out != nil {
		drainLines(t, out)
	}
	out, errs := A.Both(ctx, opts)
	drainLines(t, out)
	if got := drainErrors(t, errs); len(got) != 1 {
		t.Errorf("Both: want the start failure, got %q", got)
	}
	if got := drainErrors(t, A.Errors(ctx, opts)); len(got) != 1 {
		t.Errorf("Errors: want the start failure, got %q", got)
	}
}

func TestStreamClosesWhenTheCommandCannotStart(t *testing.T) {
	A := api.Import[startFailureAPI](cmdl.API, "sh", nil)
	opts := startFailure{Dir: filepath.Join(t.TempDir(), "missing"), Script: "echo one"}
	checkFailedStreams(t, context.Background(), A, opts)
}

func TestStreamClosesWhenTheContextIsAlreadyCancelled(t *testing.T) {
	A := api.Import[startFailureAPI](cmdl.API, "sh", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	checkFailedStreams(t, ctx, A, startFailure{Script: "echo one"})
}

func TestStreamClosesWhenTheProgramIsMissing(t *testing.T) {
	A := api.Import[startFailureAPI](cmdl.API, filepath.Join(t.TempDir(), "missing"), nil)
	checkFailedStreams(t, context.Background(), A, startFailure{Script: "echo one"})
}

type failingText struct{}

func (failingText) MarshalText() ([]byte, error) { return nil, errors.New("cannot marshal") }

func TestStreamClosesWhenItsArgumentsCannotBeBuilt(t *testing.T) {
	type API struct {
		api.Specification
		Stream func(context.Context, failingText) chan []byte `cmdl:"%v"`
		Errors func(context.Context, failingText) chan error  `cmdl:"%v"`
	}
	A := api.Import[API](cmdl.API, "sh", nil)
	drainLines(t, A.Stream(context.Background(), failingText{}))
	if got := drainErrors(t, A.Errors(context.Background(), failingText{})); len(got) != 1 {
		t.Errorf("want the marshal failure, got %q", got)
	}
}

func TestMissingProgramIsStillAnErrorForPlainResults(t *testing.T) {
	A := api.Import[shell](cmdl.API, filepath.Join(t.TempDir(), "missing"), nil)
	if _, err := A.Text(context.Background(), "true"); err == nil {
		t.Error("want an error")
	}
}

func TestCapturedOutputIsComplete(t *testing.T) {
	s := linkShell()
	ctx := context.Background()
	want := 0
	for i := 1; i <= 100000; i++ {
		want += len(fmt.Sprint(i)) + 1
	}
	for range 30 {
		got, err := s.Text(ctx, "seq 1 100000")
		if err != nil {
			t.Fatal(err)
		}
		if len(got)+1 != want {
			t.Fatalf("got %d of %d bytes", len(got)+1, want)
		}
	}
	for range 100 {
		got, err := s.Text(ctx, "echo hello")
		if err != nil || got != "hello" {
			t.Fatalf("got %q, err=%v", got, err)
		}
		if _, err := s.Text(ctx, "echo oops >&2; exit 3"); err == nil || !strings.Contains(err.Error(), "oops") {
			t.Fatalf("stderr missing from the error: %v", err)
		}
	}
}

func TestLinkedCallsReleaseTheirDescriptors(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("counts /proc/self/fd")
	}
	count := func() int {
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Fatal(err)
		}
		return len(entries)
	}
	s := linkShell()
	ctx := context.Background()
	before := count()
	for range 25 {
		if _, err := s.Text(ctx, "echo hello"); err != nil {
			t.Fatal(err)
		}
		drainLines(t, s.Stream(ctx, "echo hello"))
		out, errs := s.Both(ctx, "echo hello; echo oops >&2")
		drainLines(t, out)
		drainErrors(t, errs)
	}
	deadline := time.Now().Add(streamTimeout)
	for count() > before {
		if time.Now().After(deadline) {
			t.Fatalf("%d descriptors, was %d", count(), before)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestFinalLineWithoutNewlineIsKept(t *testing.T) {
	s := linkShell()
	ctx := context.Background()
	want := []string{"a", "", "b"}
	lines, err := s.Lines(ctx, `printf 'a\n\nb'`)
	if err != nil || !slices.Equal(lines, want) {
		t.Errorf("Lines: %q err=%v", lines, err)
	}
	if got := drainLines(t, s.Stream(ctx, `printf 'a\n\nb'`)); !slices.Equal(got, want) {
		t.Errorf("Stream: %q", got)
	}
	if got := drainErrors(t, s.Errors(ctx, `printf 'oops' >&2`)); !slices.Equal(got, []string{"oops"}) {
		t.Errorf("Errors: %q", got)
	}
	lines, err = s.Lines(ctx, "true")
	if err != nil || len(lines) != 0 {
		t.Errorf("no output: %q err=%v", lines, err)
	}
}

func TestBytesResultIsTheExactOutput(t *testing.T) {
	s := linkShell()
	ctx := context.Background()
	got, err := s.Bytes(ctx, `printf 'a\nb\n'`)
	if err != nil || string(got) != "a\nb\n" {
		t.Errorf("[]byte: %q err=%v", got, err)
	}
	named, err := s.NamedBytes(ctx, `printf 'a\n'`)
	if err != nil || string(named) != "a\n" {
		t.Errorf("named: %q err=%v", named, err)
	}
}

func TestNamedStringResults(t *testing.T) {
	s := linkShell()
	ctx := context.Background()
	got, err := s.Named(ctx, "echo hi")
	if err != nil || got != "hi" {
		t.Errorf("named string: %q err=%v", got, err)
	}
	lines, err := s.NamedLines(ctx, "echo a; echo b")
	if err != nil || !slices.Equal(lines, []namedText{"a", "b"}) {
		t.Errorf("named lines: %q err=%v", lines, err)
	}
}

func TestUnsupportedSliceResultIsAnError(t *testing.T) {
	if got, err := linkShell().Numbers(context.Background(), "echo 1"); err == nil {
		t.Errorf("got %v", got)
	}
}

func TestEmptySliceResultsAreNotNil(t *testing.T) {
	s := linkShell()
	ctx := context.Background()
	if got, err := s.Numbers(ctx, "printf 1"); err != nil || got == nil || len(got) != 0 {
		t.Errorf("no line ended, so an empty []int, as always: %#v err=%v", got, err)
	}
	if got, err := s.Bytes(ctx, "true"); err != nil || got == nil || len(got) != 0 {
		t.Errorf("no output is an empty []byte: %#v err=%v", got, err)
	}
}

func TestInt32ResultRejectsWhatItCannotHold(t *testing.T) {
	s := linkShell()
	got, err := s.Number(context.Background(), "echo 42")
	if err != nil || got != 42 {
		t.Errorf("got %d err=%v", got, err)
	}
	if got, err := s.Number(context.Background(), "echo 4294967338"); err == nil {
		t.Errorf("4294967338 read as %d", got)
	}
	if _, err := s.Number(context.Background(), "echo x"); err == nil || !strings.Contains(err.Error(), "strconv.Atoi") {
		t.Errorf("want the error strconv.Atoi has always given, got %v", err)
	}
}

func TestPointerFlags(t *testing.T) {
	type Opts struct {
		Count *int    `cmdl:"--count=%v,omitempty"`
		Name  *string `cmdl:"--name=%v"`
		Env   *string `cmdl:"LINK_TEST_VALUE,env"`
	}
	A := api.Import[struct {
		api.Specification
		Run func(context.Context, Opts) (string, error) `cmdl:"%v"`
	}](cmdl.API, argvScript(t), nil)
	five, ada := 5, "ada"
	got, err := A.Run(context.Background(), Opts{Count: &five, Name: &ada})
	if err != nil || got != "[--count=5][--name=ada]" {
		t.Errorf("set: %q err=%v", got, err)
	}
	got, err = A.Run(context.Background(), Opts{})
	if err != nil || got != "" {
		t.Errorf("a nil pointer has no value to pass: %q err=%v", got, err)
	}

	env := api.Import[struct {
		api.Specification
		Run func(context.Context, string, Opts) (string, error) `cmdl:"-c %v %v"`
	}](cmdl.API, "sh", nil)
	value := "from-pointer"
	got, err = env.Run(context.Background(), `printf %s "$LINK_TEST_VALUE"`, Opts{Env: &value})
	if err != nil || got != "from-pointer" {
		t.Errorf("env: %q err=%v", got, err)
	}
}

func TestNilPointerFlagWithoutAVerbIsStillPassed(t *testing.T) {
	type Opts struct {
		Force *bool `cmdl:"--force"`
	}
	A := api.Import[struct {
		api.Specification
		Run func(context.Context, Opts) (string, error) `cmdl:"%v"`
	}](cmdl.API, argvScript(t), nil)
	if got, err := A.Run(context.Background(), Opts{}); err != nil || got != "[--force]" {
		t.Errorf("got %q err=%v", got, err)
	}
}

type textLevel int

func (level textLevel) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf("level-%d", int(level))), nil
}

func TestFlagValuesPrintAsFmtPrintsThem(t *testing.T) {
	type Opts struct {
		Since time.Time  `cmdl:"--since=%v"`
		Until *time.Time `cmdl:"--until=%v"`
		Level textLevel  `cmdl:"--level=%v"`
		At    time.Time  `cmdl:"LINK_TEST_AT,env"`
	}
	A := api.Import[struct {
		api.Specification
		Run func(context.Context, Opts) (string, error) `cmdl:"%v"`
	}](cmdl.API, argvScript(t), nil)
	since := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	opts := Opts{Since: since, Until: &since, Level: 3, At: since}
	got, err := A.Run(context.Background(), opts)
	want := "[--since=2024-01-02 03:04:05 +0000 UTC][--until=2024-01-02 03:04:05 +0000 UTC][--level=3]"
	if err != nil || got != want {
		t.Errorf("got %q err=%v, want %q", got, err, want)
	}
	env := api.Import[struct {
		api.Specification
		Run func(context.Context, string, Opts) (string, error) `cmdl:"-c %v %v"`
	}](cmdl.API, "sh", nil)
	got, err = env.Run(context.Background(), `printf %s "$LINK_TEST_AT"`, opts)
	if want := "2024-01-02 03:04:05 +0000 UTC"; err != nil || got != want {
		t.Errorf("env: got %q err=%v, want %q", got, err, want)
	}
}

func TestBoolFlagWithAVerbOtherThanVIsLeftOutWhenFalse(t *testing.T) {
	type Opts struct {
		On    bool `cmdl:"--on=%t"`
		Index bool `cmdl:"--index=%[1]v"`
		Plain bool `cmdl:"--plain=%v"`
	}
	A := api.Import[struct {
		api.Specification
		Run func(context.Context, Opts) (string, error) `cmdl:"%v"`
	}](cmdl.API, argvScript(t), nil)
	got, err := A.Run(context.Background(), Opts{})
	if err != nil || got != "[--plain=false]" {
		t.Errorf("all false: %q err=%v", got, err)
	}
	got, err = A.Run(context.Background(), Opts{On: true, Index: true, Plain: true})
	if err != nil || got != "[--on=true][--index=true][--plain=true]" {
		t.Errorf("all true: %q err=%v", got, err)
	}
}

func TestInvertedFlags(t *testing.T) {
	type Opts struct {
		Color bool `cmdl:"--no-color,invert"`
		Wrap  bool `cmdl:"--wrap=%v,invert"`
		Trim  bool `cmdl:"--no-trim,invert,omitempty"`
	}
	A := api.Import[struct {
		api.Specification
		Run func(context.Context, Opts) (string, error) `cmdl:"%v"`
	}](cmdl.API, argvScript(t), nil)
	got, err := A.Run(context.Background(), Opts{Color: true, Wrap: true, Trim: true})
	if err != nil || got != "[--wrap=false]" {
		t.Errorf("all true: %q err=%v", got, err)
	}
	got, err = A.Run(context.Background(), Opts{})
	if err != nil || got != "[--no-color][--wrap=true][--no-trim]" {
		t.Errorf("all false: %q err=%v", got, err)
	}
}

func TestUntaggedFieldsPassAnEmptyArgument(t *testing.T) {
	type Opts struct {
		Name    string
		Count   int
		Paths   []string
		Quiet   bool
		Skipped string `cmdl:"-"`
		Option  string `cmdl:",omitempty"`
		Hook    func()
		Pointer *string
		Nested  struct {
			Verbose bool `cmdl:"-v"`
		}
		In  io.Reader
		Out io.Writer
	}
	A := api.Import[struct {
		api.Specification
		Run func(context.Context, Opts) (string, error) `cmdl:"--x %v"`
	}](cmdl.API, argvScript(t), nil)
	name := "ada"
	opts := Opts{
		Name: "ada", Count: 2, Paths: []string{"a", "b"}, Quiet: true, Skipped: "no", Option: "opt", Hook: func() {}, Pointer: &name,
	}
	opts.Nested.Verbose = true
	got, err := A.Run(context.Background(), opts)
	if err != nil || got != "[--x][][][][][][][][][][]" {
		t.Errorf("set: %q err=%v", got, err)
	}
	got, err = A.Run(context.Background(), Opts{})
	if err != nil || got != "[--x][][][][][][][][]" {
		t.Errorf("unset: %q err=%v", got, err)
	}
}

func TestReaderFieldIsStdin(t *testing.T) {
	type Opts struct {
		In io.Reader
	}
	A := api.Import[struct {
		api.Specification
		Run func(context.Context, Opts) (string, error) `cmdl:"%v"`
	}](cmdl.API, "cat", nil)
	got, err := A.Run(context.Background(), Opts{In: strings.NewReader("hello")})
	if err != nil || got != "hello" {
		t.Errorf("reader: %q err=%v", got, err)
	}
	if got, err := A.Run(context.Background(), Opts{}); err == nil {
		t.Errorf("a nil reader is an empty argument, which cat cannot open: %q", got)
	}

	file := filepath.Join(t.TempDir(), "in")
	if err := os.WriteFile(file, []byte("from a file"), 0o644); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	got, err = A.Run(context.Background(), Opts{In: in})
	if err != nil || got != "from a file" {
		t.Errorf("file: %q err=%v", got, err)
	}
}

func TestConcreteReaderFieldIsStdin(t *testing.T) {
	type Opts struct {
		In *strings.Reader
	}
	A := api.Import[struct {
		api.Specification
		Run func(context.Context, Opts) (string, error) `cmdl:"%v"`
	}](cmdl.API, "cat", nil)
	got, err := A.Run(context.Background(), Opts{In: strings.NewReader("hello")})
	if err != nil || got != "hello" {
		t.Errorf("reader: %q err=%v", got, err)
	}
	if got, err := A.Run(context.Background(), Opts{}); err == nil {
		t.Errorf("a nil reader is an empty argument, which cat cannot open: %q", got)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("cannot write") }

func TestFailingWriterFieldCannotStallAStream(t *testing.T) {
	type Opts struct {
		Out io.Writer
	}
	A := api.Import[struct {
		api.Specification
		Run func(context.Context, string, Opts) (chan []byte, chan error) `cmdl:"-c %v %v"`
	}](cmdl.API, "sh", nil)
	out, errs := A.Run(context.Background(), "yes", Opts{Out: failingWriter{}})
	drainLines(t, out)
	if got := drainErrors(t, errs); len(got) != 1 {
		t.Errorf("want the stream reported as cut short, got %q", got)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("cannot read") }

func TestReaderFailureIsReported(t *testing.T) {
	type Opts struct {
		In io.Reader
	}
	A := api.Import[struct {
		api.Specification
		Run func(context.Context, Opts) (string, error) `cmdl:"%v"`
	}](cmdl.API, "cat", nil)
	if got, err := A.Run(context.Background(), Opts{In: failingReader{}}); err == nil {
		t.Errorf("truncated input went unreported: %q", got)
	}
}

func TestCommandThatIgnoresStdinIsNotAnError(t *testing.T) {
	type Opts struct {
		In io.Reader
	}
	A := api.Import[struct {
		api.Specification
		Run func(context.Context, string, Opts) (string, error) `cmdl:"-c %v %v"`
	}](cmdl.API, "sh", nil)
	input := bytes.Repeat([]byte("x"), 1<<20)
	got, err := A.Run(context.Background(), "echo done", Opts{In: bytes.NewReader(input)})
	if err != nil || got != "done" {
		t.Errorf("%q err=%v", got, err)
	}
}

func TestStdinReaderThatNeverReturnsCannotHoldTheCallOpen(t *testing.T) {
	t.Parallel()
	type Opts struct {
		In io.Reader
	}
	A := api.Import[struct {
		api.Specification
		Run    func(context.Context, string, Opts) (string, error)           `cmdl:"-c %v %v"`
		Stream func(context.Context, string, Opts) (chan []byte, chan error) `cmdl:"-c %v %v"`
	}](cmdl.API, "sh", nil)

	blocked, release := io.Pipe()
	defer release.Close()

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		if got, err := A.Run(context.Background(), "echo done", Opts{In: blocked}); err != nil || got != "done" {
			t.Errorf("%q err=%v", got, err)
		}
	}()
	select {
	case <-finished:
	case <-time.After(streamTimeout):
		t.Fatal("call still blocked on stdin after the command exited")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out, errs := A.Stream(ctx, "sleep 30", Opts{In: blocked})
	cancel()
	drainLines(t, out)
	drainErrors(t, errs)
}

func TestWriterFieldsReceiveOutput(t *testing.T) {
	type Opts struct {
		Out io.Writer
		Err io.Writer `cmdl:",stderr"`
	}
	A := api.Import[struct {
		api.Specification
		Run    func(context.Context, string, Opts) (string, error)           `cmdl:"-c %v %v"`
		Silent func(context.Context, string, Opts) error                     `cmdl:"-c %v %v"`
		Stream func(context.Context, string, Opts) (chan []byte, chan error) `cmdl:"-c %v %v"`
	}](cmdl.API, "sh", nil)
	const script = "echo out; echo err >&2"

	var stdout, stderr bytes.Buffer
	got, err := A.Run(context.Background(), script, Opts{Out: &stdout, Err: &stderr})
	if err != nil || got != "out" || stdout.String() != "out\n" || stderr.String() != "err\n" {
		t.Errorf("Run: %q err=%v stdout=%q stderr=%q", got, err, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	err = A.Silent(context.Background(), script, Opts{Out: &stdout, Err: &stderr})
	if err != nil || stdout.String() != "out\n" || stderr.String() != "err\n" {
		t.Errorf("Silent: err=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	out, errs := A.Stream(context.Background(), script, Opts{Out: &stdout, Err: &stderr})
	if got := drainLines(t, out); !slices.Equal(got, []string{"out"}) {
		t.Errorf("Stream stdout: %q", got)
	}
	if got := drainErrors(t, errs); !slices.Equal(got, []string{"err"}) {
		t.Errorf("Stream stderr: %q", got)
	}
	if stdout.String() != "out\n" || stderr.String() != "err\n" {
		t.Errorf("Stream: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestNamespacePrefixesAccumulate(t *testing.T) {
	var A struct {
		api.Specification
		Top func(context.Context) (string, error) `cmdl:"top"`
		Git struct {
			Status func(context.Context) (string, error) `cmdl:"status"`
			Bare   func(context.Context) (string, error)
			Remote struct {
				List func(context.Context) (string, error) `cmdl:"list"`
			} `cmdl:"remote"`
			Untagged struct {
				List func(context.Context) (string, error) `cmdl:"list"`
			}
		} `cmdl:"git"`
	}
	if err := cmdl.API.Link(api.StructureOf(&A), argvScript(t), nil); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for want, fn := range map[string]func(context.Context) (string, error){
		"[top]":               A.Top,
		"[git][status]":       A.Git.Status,
		"[git][remote][list]": A.Git.Remote.List,
		"[git][list]":         A.Git.Untagged.List,
		// A tag is split on every space, as it always was, so a function with
		// no tag of its own still passes the empty word after its prefix.
		"[git][]": A.Git.Bare,
	} {
		if got, err := fn(ctx); err != nil || got != want {
			t.Errorf("got %q err=%v, want %q", got, err, want)
		}
	}
}

func TestDescendantHoldingOutputCannotHoldTheCallOpen(t *testing.T) {
	t.Parallel()
	s := linkShell()
	started := time.Now()
	got, err := s.Text(context.Background(), "sleep 9 & echo hello")
	if err != nil || got != "hello" {
		t.Errorf("got %q err=%v, want what the command wrote and no error, as before", got, err)
	}
	if elapsed := time.Since(started); elapsed >= 9*time.Second {
		t.Errorf("waited %v for a descendant to exit", elapsed)
	}
}

func TestDescendantHoldingOutputCannotHoldAStreamOpen(t *testing.T) {
	t.Parallel()
	s := linkShell()
	started := time.Now()
	out, errs := s.Both(context.Background(), "sleep 9 & echo hello")
	if got := drainLines(t, out); !slices.Equal(got, []string{"hello"}) {
		t.Errorf("stdout: %q", got)
	}
	if got := drainErrors(t, errs); len(got) != 1 {
		t.Errorf("want one report that the stream was cut short, got %q", got)
	}
	if elapsed := time.Since(started); elapsed >= 9*time.Second {
		t.Errorf("waited %v for a descendant to exit", elapsed)
	}
}

func TestSlowConsumerReceivesAllOfAFinishedCommand(t *testing.T) {
	t.Parallel()
	s := linkShell()
	// Small enough to fit in the pipe, so the command has exited, and the
	// clock on a silent pipe is running, while the rest waits to be read.
	const lines = 3000
	out := s.Stream(context.Background(), fmt.Sprintf("seq 1 %d", lines))
	first := <-out
	// Longer than a command may hold up its caller: that limit is for a
	// silent pipe, what is already in it is the consumer's to read.
	time.Sleep(7 * time.Second)
	got := drainLines(t, out)
	if len(got)+1 != lines || string(first) != "1" || got[len(got)-1] != fmt.Sprint(lines) {
		t.Errorf("received %d of %d lines", len(got)+1, lines)
	}
}

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
	if elapsed := time.Since(started); elapsed >= 4*time.Second {
		t.Errorf("took %v, the background sleep outlived the cancellation", elapsed)
	}
}

func TestPlaceholderInsideAWord(t *testing.T) {
	A := api.Import[struct {
		api.Specification

		Name   func(context.Context, string) (string, error)            `cmdl:"--name=%v"`
		Range  func(context.Context, int, int) (string, error)          `cmdl:"--range=%v-%v"`
		Index  func(context.Context, string, string) (string, error)    `cmdl:"--b=%[2]v --a=%[1]v"`
		Suffix func(context.Context, string) (string, error)            `cmdl:"%v.txt"`
		Mixed  func(context.Context, string, string) (string, error)    `cmdl:"plain %v --flag=%v"`
		Time   func(context.Context, time.Time) (string, error)         `cmdl:"--at=%v"`
		Unset  func(context.Context, *int, string) (string, error)      `cmdl:"--count=%v %v"`
		Gap    func(context.Context, *int, int, string) (string, error) `cmdl:"--span=%v-%v %v"`
	}](cmdl.API, argvScript(t), nil)
	ctx := context.Background()
	check := func(got string, err error) func(string) {
		return func(want string) {
			t.Helper()
			if err != nil || got != want {
				t.Errorf("got %q err=%v, want %q", got, err, want)
			}
		}
	}
	check(A.Name(ctx, "ada"))("[--name=ada]")
	check(A.Range(ctx, 1, 5))("[--range=1-5]")
	check(A.Index(ctx, "A", "B"))("[--b=B][--a=A]")
	check(A.Suffix(ctx, "x"))("[x.txt]")
	check(A.Mixed(ctx, "a", "b"))("[plain][a][--flag=b]")
	check(A.Time(ctx, time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)))("[--at=2024-01-02T03:04:05Z]")
	// The nil has no value to pass, but the argument after it keeps its own.
	check(A.Unset(ctx, nil, "x"))("[x]")
	check(A.Gap(ctx, nil, 5, "x"))("[x]")
}

func TestPlaceholderInsideAWordWithoutAParameterIsLiteral(t *testing.T) {
	A := api.Import[struct {
		api.Specification

		None   func(context.Context) (string, error)         `cmdl:"--name=%v"`
		Before func(context.Context, string) (string, error) `cmdl:"--fmt=%v %v"`
		Both   func(context.Context, string) (string, error) `cmdl:"-a=%v -b=%v"`
		Index  func(context.Context, string) (string, error) `cmdl:"--b=%[2]v"`
	}](cmdl.API, argvScript(t), nil)
	ctx := context.Background()
	check := func(got string, err error) func(string) {
		return func(want string) {
			t.Helper()
			if err != nil || got != want {
				t.Errorf("got %q err=%v, want %q", got, err, want)
			}
		}
	}
	check(A.None(ctx))("[--name=%v]")
	check(A.Before(ctx, "x"))("[--fmt=%v][x]")
	check(A.Both(ctx, "x"))("[-a=%v][-b=%v]")
	check(A.Index(ctx, "x"))("[--b=%[2]v]")
}

type bracedOptions struct {
	Name string
}

func TestWordsPassAsWritten(t *testing.T) {
	A := api.Import[struct {
		api.Specification

		Doubled func(context.Context) (string, error)                `cmdl:"x%%v +%%v"`
		Escape  func(context.Context, string) (string, error)        `cmdl:"+%%v %v"`
		Spaces  func(context.Context) (string, error)                `cmdl:"a  b"`
		Open    func(context.Context, bracedOptions) (string, error) `cmdl:"{Name"`
		Closed  func(context.Context, string) (string, error)        `cmdl:"%v}"`
	}](cmdl.API, argvScript(t), nil)
	ctx := context.Background()
	check := func(got string, err error) func(string) {
		return func(want string) {
			t.Helper()
			if err != nil || got != want {
				t.Errorf("got %q err=%v, want %q", got, err, want)
			}
		}
	}
	check(A.Doubled(ctx))("[x%%v][+%%v]")
	check(A.Escape(ctx, "x"))("[+%%v][x]")
	check(A.Spaces(ctx))("[a][][b]")
	check(A.Open(ctx, bracedOptions{Name: "ada"}))("[ada]")
	check(A.Closed(ctx, "x"))("[x]")
}

func TestPercentThatIsNotAPlaceholderIsLiteral(t *testing.T) {
	A := api.Import[struct {
		api.Specification

		Date      func(context.Context) (string, error)         `cmdl:"date +%Y-%m-%d"`
		Printf    func(context.Context, string) (string, error) `cmdl:"%s %v"`
		Percent   func(context.Context) (string, error)         `cmdl:"100% %"`
		Doubled   func(context.Context) (string, error)         `cmdl:"%% +%%Y"`
		BadIndex  func(context.Context) (string, error)         `cmdl:"%[x]v %[]v %[1"`
		Escaped   func(context.Context) (string, error)         `cmdl:"%%v %%[1]v %%%v"`
		Together  func(context.Context, string) (string, error) `cmdl:"--x=%%v=%v"`
		Find      func(context.Context) (string, error)         `cmdl:"-exec {} {"`
		Unmatched func(context.Context) (string, error)         `cmdl:"{open"`
	}](cmdl.API, argvScript(t), nil)
	ctx := context.Background()
	check := func(got string, err error) func(string) {
		return func(want string) {
			t.Helper()
			if err != nil || got != want {
				t.Errorf("got %q err=%v, want %q", got, err, want)
			}
		}
	}
	check(A.Date(ctx))("[date][+%Y-%m-%d]")
	check(A.Printf(ctx, "x"))("[%s][x]")
	check(A.Percent(ctx))("[100%][%]")
	check(A.Doubled(ctx))("[%%][+%%Y]")
	check(A.BadIndex(ctx))("[%[x]v][%[]v][%[1]")
	check(A.Escaped(ctx))("[%v][%[1]v][%%v]")
	check(A.Together(ctx, "a"))("[--x=%v=a]")
	check(A.Find(ctx))("[-exec][{}][{]")
	check(A.Unmatched(ctx))("[{open]")
}

func TestUnsupportedResultRunsAndThenFails(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	script := "touch " + marker
	A := api.Import[struct {
		api.Specification

		Int    func(context.Context, string) (int, error)             `cmdl:"-c %v"`
		Float  func(context.Context, string) (float64, error)         `cmdl:"-c %v"`
		Ints   func(context.Context, string) ([]int, error)           `cmdl:"-c %v"`
		Struct func(context.Context, string) (struct{ A int }, error) `cmdl:"-c %v"`
		Two    func(context.Context, string) (string, string, error)  `cmdl:"-c %v"`
		JSON   func(context.Context, string) (chan int, error)        `cmdl:"-c %v | json"`

		NoError func(context.Context, string) int `cmdl:"-c %v"`
	}](cmdl.API, "sh", nil)
	ctx := context.Background()
	failures := map[string]func() error{
		"cmdl: return type int: not implemented":              func() error { _, err := A.Int(ctx, script); return err },
		"cmdl: return type float64: not implemented":          func() error { _, err := A.Float(ctx, script); return err },
		"cmdl: return type []int: not implemented":            func() error { _, err := A.Ints(ctx, script+"; echo 1"); return err },
		"cmdl: return type struct { A int }: not implemented": func() error { _, err := A.Struct(ctx, script); return err },
		"cmdl: return type string: not implemented":           func() error { _, _, err := A.Two(ctx, script); return err },
		"EOF": func() error {
			_, err := A.JSON(ctx, script)
			return err
		},
		"panic: cmdl: return type int: not implemented": func() (err error) {
			defer func() {
				if failure, ok := recover().(error); ok {
					err = fmt.Errorf("panic: %w", failure)
				}
			}()
			A.NoError(ctx, script)
			return nil
		},
	}
	for want, call := range failures {
		if err := call(); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want %q, got %v", want, err)
		}
		if err := os.Remove(marker); err != nil {
			t.Errorf("%s: the command did not run first, as it always has: %v", want, err)
		}
	}
}

func TestStreamsWithOtherResultsStillStream(t *testing.T) {
	A := api.Import[struct {
		api.Specification

		Late    func(context.Context, string) (chan []byte, string, error) `cmdl:"-c %v"`
		Partial func(context.Context, string) (chan []byte, bool)          `cmdl:"-c %v"`
		Errors  func(context.Context, string) (string, chan error)         `cmdl:"-c %v"`
		Early   func(context.Context, string) (string, chan error, error)  `cmdl:"-c %v"`
	}](cmdl.API, "sh", nil)
	ctx := context.Background()
	const script = "echo out; echo err >&2; exit 3"

	lines, text, err := A.Late(ctx, "echo out")
	if got := drainLines(t, lines); !slices.Equal(got, []string{"out"}) || text != "" || err != nil {
		t.Errorf("late: %q %q err=%v", got, text, err)
	}
	lines, ok := A.Partial(ctx, "echo out")
	if got := drainLines(t, lines); !slices.Equal(got, []string{"out"}) || ok {
		t.Errorf("partial: %q %v", got, ok)
	}
	text, errs := A.Errors(ctx, script)
	if got := drainErrors(t, errs); !slices.Equal(got, []string{"err", "exit status 3"}) || text != "" {
		t.Errorf("errors: %q %q", got, text)
	}
	// This one used to fail after running, it is found as a stream now that
	// a chan error in front of an error result is.
	text, errs, err = A.Early(ctx, script)
	if got := drainErrors(t, errs); !slices.Equal(got, []string{"err", "exit status 3"}) || text != "" || err != nil {
		t.Errorf("early: %q %q err=%v", got, text, err)
	}
}

// swapStderr points os.Stderr at a file for the length of a test, so that what
// a linked command inherits can be read back.
func swapStderr(t *testing.T) (read func() string) {
	t.Helper()
	file, err := os.Create(filepath.Join(t.TempDir(), "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stderr
	os.Stderr = file
	t.Cleanup(func() {
		os.Stderr = original
		file.Close()
	})
	return func() string {
		data, err := os.ReadFile(file.Name())
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
}

func TestStreamWithAnErrorResultDropsStderr(t *testing.T) {
	read := swapStderr(t)
	s := linkShell()
	out, err := s.Checked(context.Background(), "echo out; echo err >&2")
	if got := drainLines(t, out); !slices.Equal(got, []string{"out"}) || err != nil {
		t.Errorf("checked: %q err=%v", got, err)
	}
	drainLines(t, s.Stream(context.Background(), "echo inherited >&2"))
	if got := read(); got != "inherited\n" {
		t.Errorf("stderr: %q, want only what the function with no error result inherits", got)
	}
}

func TestDebugCmdPrintsTheArguments(t *testing.T) {
	t.Setenv("DEBUG_CMD", "1")
	file, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	original := os.Stdout
	os.Stdout = file
	_, err = linkShell().Text(context.Background(), "true")
	os.Stdout = original
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file.Name())
	if err != nil || string(data) != "sh {[-c true] [] }\n" {
		t.Errorf("got %q err=%v", data, err)
	}
}

func jsonArgvScript(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "json-argv")
	script := "#!/bin/sh\nprintf '{\"args\":\"%s\"}' \"$*\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestJSONOptionOnALink(t *testing.T) {
	type Reply struct {
		Args string `json:"args"`
	}
	var A struct {
		api.Specification

		Version func(context.Context) (Reply, error)         `cmdl:"version,json"`
		Show    func(context.Context, string) (Reply, error) `cmdl:"show %v,json"`
		Piped   func(context.Context) (Reply, error)         `cmdl:"piped | json"`
		Git     struct {
			Status func(context.Context) (Reply, error) `cmdl:"status,json"`
		} `cmdl:"git"`
	}
	if err := cmdl.API.Link(api.StructureOf(&A), jsonArgvScript(t), nil); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	check := func(got Reply, err error) func(string) {
		return func(want string) {
			t.Helper()
			if err != nil || got.Args != want {
				t.Errorf("got %q err=%v, want %q", got.Args, err, want)
			}
		}
	}
	check(A.Version(ctx))("version")
	check(A.Show(ctx, "x"))("show x")
	check(A.Piped(ctx))("piped")
	check(A.Git.Status(ctx))("git status")
}

func TestCommaThatIsNotTheJSONOptionIsLiteral(t *testing.T) {
	type Opts struct {
		Out io.Writer
	}
	A := api.Import[struct {
		api.Specification

		List    func(context.Context) (string, error)         `cmdl:"-o pid,comm"`
		Middle  func(context.Context) (string, error)         `cmdl:"-o a,json -b"`
		Longer  func(context.Context) (string, error)         `cmdl:"list,jsonish"`
		Passed  func(context.Context, string) (string, error) `cmdl:"-o %v"`
		Dropped func(context.Context, Opts) error             `cmdl:"status %v,json"`
	}](cmdl.API, argvScript(t), nil)
	ctx := context.Background()
	check := func(got string, err error) func(string) {
		return func(want string) {
			t.Helper()
			if err != nil || got != want {
				t.Errorf("got %q err=%v, want %q", got, err, want)
			}
		}
	}
	check(A.List(ctx))("[-o][pid,comm]")
	check(A.Middle(ctx))("[-o][a,json][-b]")
	check(A.Longer(ctx))("[list,jsonish]")
	// A placeholder is how to pass an argument that really ends in ,json.
	check(A.Passed(ctx, "name,json"))("[-o][name,json]")
	var out bytes.Buffer
	if err := A.Dropped(ctx, Opts{Out: &out}); err != nil || out.String() != "[status]\n" {
		t.Errorf("an option with no result to decode still is not an argument: %q err=%v", out.String(), err)
	}
}

func TestTrailingJSONIsAnArgumentWhereItAlwaysWorkedAsOne(t *testing.T) {
	type Opts struct {
		Out io.Writer
	}
	A := api.Import[struct {
		api.Specification

		Text   func(context.Context) (string, error)      `cmdl:"version,json"`
		Bytes  func(context.Context) ([]byte, error)      `cmdl:"raw,json"`
		Silent func(context.Context, Opts) error          `cmdl:"%v quiet,json"`
		Stream func(context.Context) (chan []byte, error) `cmdl:"tail,json"`
	}](cmdl.API, argvScript(t), nil)
	ctx := context.Background()
	if got, err := A.Text(ctx); err != nil || got != "[version,json]" {
		t.Errorf("string: %q err=%v", got, err)
	}
	if got, err := A.Bytes(ctx); err != nil || string(got) != "[raw,json]\n" {
		t.Errorf("[]byte: %q err=%v", got, err)
	}
	var out bytes.Buffer
	if err := A.Silent(ctx, Opts{Out: &out}); err != nil || out.String() != "[quiet,json]\n" {
		t.Errorf("no result: %q err=%v", out.String(), err)
	}
	lines, err := A.Stream(ctx)
	if got := drainLines(t, lines); !slices.Equal(got, []string{"[tail,json]"}) || err != nil {
		t.Errorf("stream: %q err=%v", got, err)
	}
}

type LinkCommon struct {
	Verbose bool `cmdl:"-v"`
}

type linkHidden struct {
	Name string `cmdl:"--name=%v"`
}

type linkSince struct {
	time.Time `cmdl:"--since=%v"`
}

func TestEmbeddedFlagsOnALink(t *testing.T) {
	type Opts struct {
		LinkCommon
		linkHidden
		Since linkSince
	}
	type Pointers struct {
		*LinkCommon
		*linkHidden
	}
	A := api.Import[struct {
		api.Specification

		Run   func(context.Context, Opts) (string, error)      `cmdl:"%v"`
		Ptr   func(context.Context, Pointers) (string, error)  `cmdl:"%v"`
		Since func(context.Context, linkSince) (string, error) `cmdl:"%v"`
	}](cmdl.API, argvScript(t), nil)
	ctx := context.Background()
	since := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	var opts Opts
	opts.Verbose = true
	opts.Name = "ada"
	opts.Since.Time = since
	got, err := A.Run(ctx, opts)
	if want := "[-v][--name=ada][]"; err != nil || got != want {
		t.Errorf("got %q err=%v, want %q", got, err, want)
	}
	got, err = A.Ptr(ctx, Pointers{LinkCommon: &LinkCommon{Verbose: true}, linkHidden: &linkHidden{Name: "ada"}})
	if err != nil || got != "[-v][--name=ada]" {
		t.Errorf("pointers: %q err=%v", got, err)
	}
	got, err = A.Ptr(ctx, Pointers{})
	if err != nil || got != "[]" {
		t.Errorf("nil pointers: %q err=%v", got, err)
	}
	// A struct that is a marshaler passes its text, as it always has.
	got, err = A.Since(ctx, linkSince{since})
	if err != nil || got != "[2024-01-02T03:04:05Z]" {
		t.Errorf("marshaler: %q err=%v", got, err)
	}

	type Skipped struct {
		LinkCommon `cmdl:"-"`
		Tagged     struct {
			LinkCommon `cmdl:"--ignored"`
		}
	}
	B := api.Import[struct {
		api.Specification
		Run func(context.Context, Skipped) (string, error) `cmdl:"%v"`
	}](cmdl.API, argvScript(t), nil)
	var skipped Skipped
	skipped.Verbose = true
	skipped.Tagged.Verbose = true
	got, err = B.Run(ctx, skipped)
	if err != nil || got != "[-v][]" {
		t.Errorf("an embedded struct passes its flags whatever its tag: %q err=%v", got, err)
	}
}

type percentFlags struct {
	Stamp  bool    `cmdl:"+%Y"`
	Escape bool    `cmdl:"+%%d"`
	Mixed  string  `cmdl:"--f=%Y-%v"`
	Hex    int     `cmdl:"--hex=%x"`
	Pad    int     `cmdl:"--pad=%03d"`
	On     bool    `cmdl:"--on=%t"`
	Suffix string  `cmdl:"--file=%v.txt"`
	Twice  string  `cmdl:"--pair=%[1]v-%[1]v"`
	Size   int     `cmdl:"-size %vMB"`
	Off    bool    `cmdl:"100%,invert"`
	Ratio  float64 `cmdl:"--ratio=%.2f,omitempty"`
	Done   bool    `cmdl:"100% done"`
}

func TestPercentInAFieldTag(t *testing.T) {
	A := api.Import[struct {
		api.Specification
		Run func(context.Context, percentFlags) (string, error) `cmdl:"%v"`
	}](cmdl.API, argvScript(t), nil)
	ctx := context.Background()
	got, err := A.Run(ctx, percentFlags{
		Stamp: true, Escape: true, Mixed: "x", Hex: 255, Pad: 7, Suffix: "a", Twice: "b", Size: 5, Ratio: 1.5, Done: true,
	})
	want := "[+%Y][+%d][--f=%Y-x][--hex=ff][--pad=007][--file=a.txt][--pair=b-b][-size][5MB][100%][--ratio=1.50][100%][done]"
	if err != nil || got != want {
		t.Errorf("got  %q err=%v\nwant %q", got, err, want)
	}

	type Words struct {
		Done bool `cmdl:"100% done"`
	}
	B := api.Import[struct {
		api.Specification
		Run func(context.Context, Words) (string, error) `cmdl:"%v"`
	}](cmdl.API, argvScript(t), nil)
	if got, err := B.Run(ctx, Words{}); err != nil || got != "" {
		t.Errorf("'%% d' is not a verb, so this is a flag with no value to print: %q err=%v", got, err)
	}
}
