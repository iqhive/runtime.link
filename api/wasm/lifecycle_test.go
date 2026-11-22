//go:build !wasm

package wasm

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tetratelabs/wazero"
	wasm_api "github.com/tetratelabs/wazero/api"
)

// pauseTimeout is how long these tests wait for something that should take
// milliseconds. It only decides when a hang is reported.
const pauseTimeout = 60 * time.Second

// pausingContext is a context whose Value stops, once, at the first
// lookup of a key whose type has the given name. wazero looks a few of its own
// keys up in the context of a call at fixed points, so this pins a Run at one
// of them without sleeping: reached is closed when the Run is there, and it
// goes on when resume is closed.
type pausingContext struct {
	context.Context
	key     string
	once    *sync.Once
	reached chan struct{}
	resume  chan struct{}
}

func pauseAt(ctx context.Context, key string) pausingContext {
	return pausingContext{
		Context: ctx,
		key:     key,
		once:    new(sync.Once),
		reached: make(chan struct{}),
		resume:  make(chan struct{}),
	}
}

func (c pausingContext) Value(key any) any {
	if fmt.Sprintf("%T", key) == c.key {
		c.once.Do(func() {
			close(c.reached)
			<-c.resume
		})
	}
	return c.Context.Value(key)
}

// Where wazero v1.9.0 looks its keys up:
const (
	// in Runtime.CompileModule, after the module is decoded and before the
	// engine compiles it.
	beforeEngineCompiles = "expctxkeys.FunctionListenerFactoryKey"
	// in Runtime.InstantiateModule, after the module is compiled and before
	// it is instantiated.
	beforeInstantiation = "sock.ConfigKey"
)

// ended is how a call of Run ended: with an error, or with a panic.
type ended struct {
	err      error
	panicked any
}

// runApart calls Run in a goroutine of its own, and keeps a panic of Run's
// from ending the test binary.
func runApart(runner *Runner, ctx context.Context) <-chan ended {
	done := make(chan ended, 1)
	go func() {
		var outcome ended
		defer func() {
			if r := recover(); r != nil {
				outcome.panicked = r
			}
			done <- outcome
		}()
		outcome.err = runner.Run(ctx)
	}()
	return done
}

// await returns what arrives on a channel, and fails the test if nothing does:
// a hang is reported, and doesn't keep the test binary from ending.
func await[T any](t *testing.T, what string, from <-chan T) T {
	t.Helper()
	var value T
	select {
	case value = <-from:
	case <-time.After(pauseTimeout):
		t.Fatalf("%s: nothing after %v", what, pauseTimeout)
	}
	return value
}

// awaitPause waits for a Run to get to where its context pauses it.
func awaitPause(t *testing.T, reached <-chan struct{}, done <-chan ended) {
	t.Helper()
	select {
	case <-reached:
	case outcome := <-done:
		t.Fatalf("Run ended before it got to where the test pauses it: %+v", outcome)
	case <-time.After(pauseTimeout):
		t.Fatalf("Run didn't get to where the test pauses it in %v", pauseTimeout)
	}
}

// touchingGuest returns a guest that calls host.Touch and ends, or loops for
// ever after the call.
func touchingGuest(loops bool) []byte {
	if loops {
		return loopingGuest()
	}
	return startGuest([]hostImport{{"host", "Touch", signature{}}}, []byte{call, 0, end}, "")
}

// slowToCompile returns a guest of the given number of functions, each
// with its own type, that does nothing: all it costs is the time to compile.
func slowToCompile(functions int) []byte {
	const i64Add, i64Mul = 0x7C, 0x7E
	var types, declared, bodies [][]byte
	for i := range functions {
		// Function i takes i%8 parameters of i64 and adds them up.
		sig := signature{results: oneHandle}
		for range i % 8 {
			sig.params = append(sig.params, i64)
		}
		types = append(types, funcSignature(sig))
		declared = append(declared, uleb(uint64(i)))
		body := []byte{noLocals, i64Const, 1}
		for p := range i % 8 {
			body = append(body, localGet, byte(p), i64Add)
		}
		for range 32 {
			body = append(body, i64Const, 3, i64Mul, i64Const, 5, i64Add)
		}
		bodies = append(bodies, sized(append(body, end)))
	}
	return slices.Concat(
		[]byte(emptyModule),
		longSection(1, longVector(types...)),
		longSection(3, longVector(declared...)),
		longSection(10, longVector(bodies...)),
	)
}

// Close says: "A Run that another goroutine is in has its guest stopped, and
// returns an error." A Run that was given no compiled module compiles its guest
// first, outside the Runner's lock, and wazero panics when a module finishes
// compiling in a runtime that was closed meanwhile: the panic came out of Run.
// The runtime is now closed once the Run has returned.
func TestCloseWhileRunCompilesItsGuest(t *testing.T) {
	var runner Runner
	closeRunner(t, &runner)
	runner.Set(loopingGuest())
	runner.Add(host{Touch: func(context.Context) {}})

	ctx := pauseAt(context.Background(), beforeEngineCompiles)
	done := runApart(&runner, ctx)
	awaitPause(t, ctx.reached, done)
	// Run is between decoding its guest and compiling it. Close comes now.
	if err := runner.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	close(ctx.resume)
	outcome := await(t, "Run after Close", done)
	if outcome.panicked != nil {
		t.Fatalf("Run panicked when the Runner was closed while it compiled its guest: %v", outcome.panicked)
	}
	if !errors.Is(outcome.err, context.Canceled) {
		t.Errorf("Run returned %v from a Runner that was closed before its guest, which loops, had started, want an error that is %v", outcome.err, context.Canceled)
	}
	// Close let go of the runtime at once, and the Runner works on.
	runner.Set([]byte(emptyModule))
	if err := runner.Run(context.Background()); err != nil {
		t.Errorf("Run after Close: %v", err)
	}
}

// The same without pinning anything: a guest that takes a while to compile,
// and a Close that comes while Run is at it. It only fails for a panic, so it
// passes if Close happens to come before Run has started on the guest.
func TestCloseWhileRunCompilesASlowGuest(t *testing.T) {
	var runner Runner
	closeRunner(t, &runner)
	// Run goes on to the end of compiling, which nothing interrupts, so the
	// guest is one that takes a second or two and not a minute.
	runner.Set(slowToCompile(2000))
	done := runApart(&runner, context.Background())
	// Run makes the runtime first, under the lock, and compiles after it.
	deadline := time.Now().Add(pauseTimeout)
	for {
		runner.mu.Lock()
		started := runner.runtime != nil
		runner.mu.Unlock()
		if started {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Run never started")
		}
		time.Sleep(time.Millisecond)
	}
	// This guest takes wazero a second to compile, so Run is at it by now.
	time.Sleep(20 * time.Millisecond)
	if err := runner.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	outcome := await(t, "Run after Close", done)
	if outcome.panicked != nil {
		t.Fatalf("Run panicked when the Runner was closed while it compiled its guest: %v", outcome.panicked)
	}
}

// A Runner's guests can run at once (TestGuestsRunningAtOnceHoldSeparateHandles
// does it with a compiled module). Without Compile each Run compiled the
// guest for itself, and wazero keeps one compiled module for the same bytes:
// the first Run to end took it from a Run that was about to start its guest.
func TestRunsAtOnceWithoutCompileShareTheCompiledCode(t *testing.T) {
	var runner Runner
	closeRunner(t, &runner)
	runner.Add(host{Sum: func(_ context.Context, a, b int64) int64 { return 0 }})
	runner.Set(makesAString("mine"))

	// The second Run has compiled its guest, and is about to instantiate it.
	second := pauseAt(context.Background(), beforeInstantiation)
	done := runApart(&runner, second)
	awaitPause(t, second.reached, done)
	// Another Run of the same guest starts and ends meanwhile.
	if between := await(t, "the Run in between", runApart(&runner, context.Background())); between.err != nil || between.panicked != nil {
		t.Fatalf("the Run in between: %+v", between)
	}
	close(second.resume)
	outcome := await(t, "the paused Run", done)
	if outcome.panicked != nil {
		t.Fatalf("Run panicked: %v", outcome.panicked)
	}
	if outcome.err != nil {
		t.Errorf("a Run failed because another Run of the same guest ended while it started: %v", outcome.err)
	}
}

// The same without pinning anything: goroutines that each Run the guest over
// and over, as TestGuestsRunningAtOnceHoldSeparateHandles does, but without
// Compile.
func TestRunsAtOnceWithoutCompileAllSucceed(t *testing.T) {
	var runner Runner
	closeRunner(t, &runner)
	runner.Add(host{Sum: func(_ context.Context, a, b int64) int64 { return 0 }})
	runner.Set(makesAString("mine"))
	const guests, runs = 8, 250
	var mu sync.Mutex
	failures := make(map[string]int)
	var running sync.WaitGroup
	for range guests {
		running.Go(func() {
			for range runs {
				outcome := <-runApart(&runner, context.Background())
				if outcome.err != nil || outcome.panicked != nil {
					mu.Lock()
					failures[fmt.Sprint(outcome.err, outcome.panicked)]++
					mu.Unlock()
				}
			}
		})
	}
	running.Wait()
	for failure, count := range failures {
		t.Errorf("%d of %d Runs of a guest that works ended with: %s", count, guests*runs, failure)
	}
}

// Without Compile, Runs that compiled the guest at the same time each got
// compiled code of their own, and wazero only let go of the one it listed
// last: the others stayed for as long as the runtime was open, so the Runner
// grew with its Runs, until Close.
func TestRunsAtOnceWithoutCompileLeaveNothingBehind(t *testing.T) {
	var runner Runner
	runner.Add(host{Sum: func(_ context.Context, a, b int64) int64 { return 0 }})
	runner.Set(makesAString("mine"))
	heap := func() int64 {
		for range 4 {
			runtime.GC()
			time.Sleep(10 * time.Millisecond) // for the finalizers of what was let go
		}
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		return int64(stats.HeapAlloc)
	}
	const guests, runs = 8, 500
	var failed atomic.Int64
	run := func(runs int) {
		var running sync.WaitGroup
		for range guests {
			running.Go(func() {
				for range runs {
					if outcome := <-runApart(&runner, context.Background()); outcome.err != nil || outcome.panicked != nil {
						failed.Add(1) // which is TestRunsAtOnceWithoutCompileAllSucceed
					}
				}
			})
		}
		running.Wait()
	}
	run(5)
	before := heap()
	failed.Store(0)
	run(runs)
	after := heap()
	if err := runner.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	closed := heap()
	t.Logf("heap in use: %d before, %d after %d Runs (%d of them failed), %d after Close", before, after, guests*runs, failed.Load(), closed)
	// Guests that have ended leave nothing: the same Runs of a Runner that
	// was compiled first grow the heap by under 10KiB in all.
	const margin = 512 << 10
	if grown := after - before; grown > margin {
		t.Errorf("%d Runs that had all ended left %d bytes of heap in use (beside the compiled code, which is outside the heap), and Close gave %d back", guests*runs, grown, after-closed)
	}
}

// Compile while a Run that compiled its guest for itself was in progress got
// the same compiled code from wazero, and that Run took it away when it ended:
// every Run after it failed, until Set or Close.
func TestCompileDuringARunKeepsTheCompiledCode(t *testing.T) {
	var runner Runner
	closeRunner(t, &runner)
	entered := make(chan struct{}, 1)
	leave := make(chan struct{})
	runner.Add(host{Touch: func(context.Context) {
		select {
		case entered <- struct{}{}:
			<-leave
		default: // only the first guest waits
		}
	}})
	runner.Set(touchingGuest(false))
	first := runApart(&runner, context.Background())
	await(t, "the first guest", entered)
	if err := runner.Compile(context.Background()); err != nil {
		t.Fatal(err)
	}
	close(leave)
	if outcome := await(t, "the first guest", first); outcome.err != nil || outcome.panicked != nil {
		t.Fatalf("the first Run ended with %+v", outcome)
	}
	for i := range 3 {
		if err := runner.Run(context.Background()); err != nil {
			t.Errorf("Run %d after a Compile that came during an earlier Run: %v", i+1, err)
		}
	}
}

// Two APIs of one name can't both be modules of a runtime. Run and Compile
// return an error, and that is what a Runner that can't be set up is for
// (TestMemoryLimitAboveWhatAGuestCanAddress): but Run panicked, with the
// Runner's lock held, so the Close that a caller deferred never returned.
func TestTwoAPIsOfOneNameAreAnError(t *testing.T) {
	var runner Runner
	runner.Add(host{Touch: func(context.Context) {}})
	runner.Add(host{Touch: func(context.Context) {}})
	runner.Set([]byte(emptyModule))
	// The mistake is met by every call that needs the runtime.
	for range 2 {
		outcome := await(t, "Run", runApart(&runner, context.Background()))
		switch {
		case outcome.panicked != nil:
			t.Errorf("Run panicked: %v", outcome.panicked)
		case outcome.err == nil:
			t.Errorf("Run returned no error for two APIs of one name")
		}
	}
	if err := runner.Compile(context.Background()); err == nil {
		t.Errorf("Compile returned no error for two APIs of one name")
	}
	if runner.runtime != nil {
		t.Error("the runtime that could not be set up was kept")
	}
	closed := make(chan error, 1)
	go func() { closed <- runner.Close(context.Background()) }()
	select {
	case err := <-closed:
		if err != nil {
			t.Errorf("Close: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Close was still waiting for the Runner's lock 10s after Run had panicked with it held")
	}
}

// pausingDone is a context whose Done stops once, the first time it is
// called. Run calls it when it derives the guest's context from the one it was
// given, after it has let go of the Runner's lock and before it starts the guest.
type pausingDone struct {
	context.Context
	once    *sync.Once
	reached chan struct{}
	resume  chan struct{}
}

func (c pausingDone) Done() <-chan struct{} {
	c.once.Do(func() {
		close(c.reached)
		<-c.resume
	})
	return c.Context.Done()
}

// Run says that a Runner closed under it makes it "return an error that wraps
// the reason: the error of ctx, or [context.Canceled] for a closed Runner".
// When Close came after Run had taken the runtime and before the guest was
// started, Run returned wazero's error for a closed runtime as it was.
func TestRunClosedBeforeItsGuestStartsIsCanceled(t *testing.T) {
	failures := make(map[string]int)
	const attempts = 50
	for range attempts {
		var runner Runner
		runner.Set(touchingGuest(false))
		runner.Add(host{Touch: func(context.Context) {}})
		if err := runner.Compile(context.Background()); err != nil {
			t.Fatal(err)
		}
		ctx := pausingDone{Context: context.Background(), once: new(sync.Once), reached: make(chan struct{}), resume: make(chan struct{})}
		done := runApart(&runner, ctx)
		awaitPause(t, ctx.reached, done)
		if err := runner.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
		close(ctx.resume)
		outcome := await(t, "Run after Close", done)
		switch {
		case outcome.panicked != nil:
			failures[fmt.Sprint("panic: ", outcome.panicked)]++
		case outcome.err == nil:
			failures["no error"]++
		case !errors.Is(outcome.err, context.Canceled):
			failures[outcome.err.Error()]++
		}
		if err := runner.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	for failure, count := range failures {
		t.Errorf("%d of %d Runs of a Runner that was closed under them returned, in place of an error that is %v: %s", count, attempts, context.Canceled, failure)
	}
}

// A module with a name of its own can run twice (TestNamedModuleCanRunTwice),
// but could not at once: wazero takes the name from the module and refuses a
// second instance of it while the first is running.
func TestNamedGuestCanRunTwiceAtOnce(t *testing.T) {
	var runner Runner
	closeRunner(t, &runner)
	entered := make(chan struct{}, 2)
	leave := make(chan struct{})
	runner.Add(host{Touch: func(context.Context) {
		entered <- struct{}{}
		<-leave
	}})
	// A guest that calls host.Touch and ends, named "guest" in its name section.
	runner.Set(append(touchingGuest(false), namedModule("guest")[len(emptyModule):]...))
	if err := runner.Compile(t.Context()); err != nil {
		t.Fatal(err)
	}
	first := runApart(&runner, context.Background())
	await(t, "the first guest", entered)
	// The first guest is in its host call. A second Run of the same Runner:
	second := runApart(&runner, context.Background())
	select {
	case <-entered:
	case outcome := <-second:
		close(leave)
		await(t, "the first guest", first)
		t.Fatalf("a second Run of a named guest, while the first is running, ended with %v (panic: %v)", outcome.err, outcome.panicked)
	case <-time.After(pauseTimeout):
		t.Fatal("the second guest never started")
	}
	close(leave)
	for _, done := range []<-chan ended{first, second} {
		if outcome := await(t, "a guest", done); outcome.err != nil || outcome.panicked != nil {
			t.Errorf("a Run ended with %+v", outcome)
		}
	}
}

// writingGuest returns a guest that writes "abc\n" to its standard output for
// ever, with WASI's fd_write.
func writingGuest() []byte {
	fdWrite := signature{params: []wasm_api.ValueType{i32, i32, i32, i32}, results: []wasm_api.ValueType{i32}}
	// At address 8 an iovec of the 4 bytes at address 16, then those bytes.
	const data = "\x10\x00\x00\x00\x04\x00\x00\x00abc\n"
	return startGuest([]hostImport{{"wasi_snapshot_preview1", "fd_write", fdWrite}},
		[]byte{loop, emptyType, i32Const, 1, i32Const, 8, i32Const, 1, i32Const, 32, call, 0, drop, br, 0, end, end}, data)
}

type countingWriter struct {
	writes  atomic.Int64
	started chan struct{}
	once    sync.Once
}

func (w *countingWriter) Write(p []byte) (int, error) {
	if w.writes.Add(1) == 10 {
		w.once.Do(func() { close(w.started) })
	}
	return len(p), nil
}

// Close stops a Run that another goroutine is in. It closed the runtime right
// after it cancelled the guest's context, and wazero's Runtime.Close releases
// the files of a guest that is still running: the guest's next WASI call read
// what Close was writing. This one needs -race to fail.
func TestCloseWhileAGuestWritesDoesNotRace(t *testing.T) {
	for _, compile := range []bool{true, false} {
		for range 40 {
			var runner Runner
			output := &countingWriter{started: make(chan struct{})}
			runner.SetSystemInterface(SystemInterface{Stdout: output})
			runner.Set(writingGuest())
			if compile {
				if err := runner.Compile(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			done := runApart(&runner, context.Background())
			select {
			case <-output.started:
			case outcome := <-done:
				t.Fatalf("the guest ended: %+v", outcome)
			case <-time.After(pauseTimeout):
				t.Fatal("the guest wrote nothing")
			}
			if err := runner.Close(context.Background()); err != nil {
				t.Fatal(err)
			}
			outcome := await(t, "Run after Close", done)
			if outcome.panicked != nil {
				t.Fatalf("Run panicked: %v", outcome.panicked)
			}
			if !errors.Is(outcome.err, context.Canceled) {
				t.Errorf("Run returned %v from a closed Runner, want an error that is %v", outcome.err, context.Canceled)
			}
		}
	}
}

// hoardingGuest returns a guest that grows its memory to the given pages and
// then, the given number of times: makes a string of all of that memory, makes
// a pointer to a string on the host (pointer_new of the string's own type),
// writes the string into it, and frees the string's handle. It ends with a
// call of host.Touch. Every handle it was given for a string is freed. A
// guest that doesn't keep its strings leaves the pointer out.
func hoardingGuest(pages, times int64, keeps bool) []byte {
	const (
		i64Add, i64LessThan, localTee, brIf = 0x7C, 0x54, 0x22, 0x0D
		memoryGrow, i64Load, i64Store       = 0x40, 0x29, 0x37
	)
	var (
		load  = func(address byte) []byte { return []byte{i32Const, address, i64Load, 3, 0} }
		store = []byte{i64Store, 3, 0}
	)
	var kept []byte
	if keeps {
		kept = slices.Concat(
			// A pointer to a string of the host's, kept at address 16.
			[]byte{i32Const, 16}, load(0), []byte{call, 1, call, 2}, store,
			// The string goes into the pointer.
			load(16), load(0), []byte{call, 3},
		)
	}
	return startGuest([]hostImport{
		{bridgeModule, "string_new", makesString},
		{bridgeModule, "struct_type", handleToHandle},
		{bridgeModule, "pointer_new", handleToHandle},
		{bridgeModule, "encode_string", takesTwo},
		{bridgeModule, "string_free", takesHandle},
		{"host", "Touch", signature{}},
	}, slices.Concat(
		[]byte{i32Const}, sleb(pages-1), []byte{memoryGrow, 0, drop},
		[]byte{loop, emptyType},
		// The string, kept at address 0 of the guest's memory.
		[]byte{i32Const, 0, i64Const, 0, i32Const}, sleb(pages*wasmPageSize), []byte{call, 0}, store,
		kept,
		// The handle of the string is freed.
		load(0), []byte{call, 4},
		[]byte{localGet, 0, i64Const, 1, i64Add, localTee, 0},
		[]byte{i64Const}, sleb(times), []byte{i64LessThan, brIf, 0},
		[]byte{end, call, 5, end},
	), "")
}

// The open handles of a guest may hold 1GiB of the host's memory (maxHeldBytes
// in internal/ffi): "a guest that only makes and never frees would otherwise
// have the host allocate for as long as the bridge lasts". A string was charged
// to the handle string_new returns, and the charge went when that handle was
// freed, though the string had been written to a pointer the guest still
// held. This guest, under the default memory limit, had the host hold 20
// strings of 60MiB that way, and is now stopped at the bound.
//
// It is the guest's side of TestHeldBytesFollowWhatAGuestMovesOutOfItsHandle
// in internal/ffi, which lowers the bound and needs no memory to speak of.
// This one has the host allocate up to the bound itself, 1GiB, so it is
// skipped with -short, and under the race detector, which would take several
// times that.
func TestGuestIsStoppedAtTheBoundOnWhatItHoldsOfTheHost(t *testing.T) {
	if testing.Short() || raceDetector {
		t.Skip("has the host allocate up to 1GiB")
	}
	const (
		pages = 960 // 60MiB of the 64MiB a guest may have by default
		times = 20  // 1.2GiB in all, where the bound is 1GiB
	)
	for name, test := range map[string]struct {
		keeps   bool
		stopped bool
	}{
		"a guest that keeps every string in a pointer": {keeps: true, stopped: true},
		// The same strings, each freed and kept nowhere, are never more
		// than one at a time.
		"a guest that frees every string": {keeps: false, stopped: false},
	} {
		t.Run(name, func(t *testing.T) {
			var runner Runner
			closeRunner(t, &runner)
			touched := false
			runner.Add(host{Touch: func(context.Context) { touched = true }})
			runner.Set(hoardingGuest(pages, times, test.keeps))
			err := runner.Run(context.Background())
			switch {
			case test.stopped && err == nil:
				t.Errorf("the guest ran to its end with %d bytes of the host's memory in what it holds", int64(pages)*wasmPageSize*times)
			case test.stopped && !strings.Contains(err.Error(), "hold more than"):
				t.Errorf("the guest was stopped for something else: %v", err)
			case !test.stopped && (err != nil || !touched):
				t.Errorf("the guest was stopped, or never got to its end (%v): %v", touched, err)
			}
		})
	}
}

// dlsym gives a guest a handle to a function, and func_free frees a handle.
// A guest that freed the handle and looked the function up again was given the
// same number, which was no longer a handle: the function "was found" and
// could never be called again by that guest.
func TestFunctionLinkedAgainAfterItsHandleWasFreedCanBeCalled(t *testing.T) {
	ctx := t.Context()
	var runner Runner
	closeRunner(t, &runner)
	touched := 0
	runner.Add(host{Touch: func(context.Context) { touched++ }})
	runner.Set([]byte(emptyModule))
	if err := runner.Compile(ctx); err != nil {
		t.Fatal(err)
	}
	call := callsOutsideRun(t, &runner)
	calls := signature{params: []wasm_api.ValueType{i64, i64}, results: oneHandle}

	first := call.linked(t, "host", "Touch")
	if _, err := call(bridgeModule, "func_call", "", calls, first, call.arguments(t, first)); err != nil || touched != 1 {
		t.Fatalf("the first call of host.Touch: %v, touched %d times", err, touched)
	}
	if _, err := call(bridgeModule, "func_free", "", takesHandle, first); err != nil {
		t.Fatalf("func_free: %v", err)
	}
	again := call.linked(t, "host", "Touch") // fails the test if dlsym answers 0
	arguments, err := call(bridgeModule, "func_args", "", handleToHandle, again)
	if err != nil {
		t.Fatalf("dlsym found host.Touch again as handle %d (it was %d before func_free), and func_args of that: %v", again, first, err)
	}
	if _, err := call(bridgeModule, "func_call", "", calls, again, arguments[0]); err != nil || touched != 2 {
		t.Errorf("dlsym found host.Touch again as handle %d (it was %d before func_free), and calling that: %v", again, first, err)
	}
	// A handle that is still open is the one dlsym answers with, as it was.
	if same := call.linked(t, "host", "Touch"); same != again {
		t.Errorf("dlsym gave handle %d for a function that is linked as %d", same, again)
	}
}

// closed reports whether a runtime has been closed, which is when wazero
// refuses to compile anything more in it.
func closed(runtime wazero.Runtime) bool {
	_, err := runtime.CompileModule(context.Background(), []byte(emptyModule))
	return err != nil
}

// Close does not wait for a guest: one that is inside a call of the host is
// only stopped when that call returns, which is up to the host. The runtime
// is left open under it, and is released when its Run has returned.
func TestCloseLeavesTheRuntimeToARunThatIsStillInIt(t *testing.T) {
	var runner Runner
	closeRunner(t, &runner)
	entered := make(chan struct{})
	leave := make(chan struct{})
	runner.Add(host{Touch: func(context.Context) {
		close(entered)
		<-leave
	}})
	runner.Set(loopingGuest())
	done := runApart(&runner, context.Background())
	await(t, "the guest", entered)
	used := runner.runtime

	if err := await(t, "Close", closing(&runner)); err != nil {
		t.Fatal(err)
	}
	if runner.runtime != nil || runner.module != nil {
		t.Error("the Runner still holds what it closed")
	}
	if closed(used) {
		t.Error("the runtime was closed under a guest that is still in a call of the host")
	}
	// The Runner is usable at once, with a runtime of its own.
	runner.Set([]byte(emptyModule))
	if err := runner.Run(context.Background()); err != nil {
		t.Errorf("Run after Close, beside the guest that is still stopping: %v", err)
	}

	close(leave)
	outcome := await(t, "Run after Close", done)
	if outcome.panicked != nil || !errors.Is(outcome.err, context.Canceled) {
		t.Errorf("Run returned %v (panic: %v) from a closed Runner, want an error that is %v", outcome.err, outcome.panicked, context.Canceled)
	}
	if !closed(used) {
		t.Error("the runtime was still open after the last Run in it had returned")
	}
}

// closing calls Close in a goroutine of its own, so that a Close that waits
// fails the test instead of hanging it.
func closing(runner *Runner) <-chan error {
	done := make(chan error, 1)
	go func() { done <- runner.Close(context.Background()) }()
	return done
}

// Set replaces the guest for the Runs that come after it. A Run that is in
// progress goes on with the code it started with, which is closed once that
// Run has ended, and not before.
func TestSetWhileARunIsInProgress(t *testing.T) {
	var runner Runner
	closeRunner(t, &runner)
	entered := make(chan struct{}, 1)
	leave := make(chan struct{})
	var touched atomic.Int64
	runner.Add(host{Touch: func(context.Context) {
		if touched.Add(1) == 1 {
			entered <- struct{}{}
			<-leave
		}
	}})
	runner.Set(touchingGuest(false))
	first := runApart(&runner, context.Background())
	await(t, "the first guest", entered)
	old := runner.module

	runner.Set([]byte(emptyModule))
	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("a Run of the guest that Set gave: %v", err)
	}
	if got := touched.Load(); got != 1 {
		t.Errorf("the Run after Set ran the guest from before it: host.Touch was called %d times", got)
	}
	if !slices.Contains(runner.retired, old) {
		t.Error("the code of a guest that is still running was let go of")
	}
	// The guest that was replaced can still be started while its code is there.
	runner.Set(touchingGuest(false))
	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("a Run of the guest that was replaced and set again: %v", err)
	}
	if runner.module != old {
		t.Error("Set with the guest it had replaced compiled that guest again")
	}

	runner.Set([]byte(emptyModule))
	close(leave)
	if outcome := await(t, "the first guest", first); outcome.err != nil || outcome.panicked != nil {
		t.Errorf("the Run that was in progress at Set ended with %+v", outcome)
	}
	if len(runner.retired) != 0 {
		t.Errorf("%d compiled guests that nothing is using were kept", len(runner.retired))
	}
	// Closed, the code of the first guest is gone from the runtime: it is
	// compiled again, not found.
	if err := runner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	runner.Set(touchingGuest(false))
	if err := runner.Run(context.Background()); err != nil {
		t.Errorf("a Run of a guest whose code had been closed: %v", err)
	}
	if runner.module == old {
		t.Error("the code that was closed is in use again")
	}
}

// Set with the bytes that the Runner has already compiled keeps the compiled
// code, as Compile after Set did, and Set with other bytes in the same slice
// does not.
func TestSetOfTheSameGuestKeepsItsCompiledCode(t *testing.T) {
	ctx := t.Context()
	var runner Runner
	closeRunner(t, &runner)
	var touched int
	runner.Add(host{Touch: func(context.Context) { touched++ }})
	guest := touchingGuest(false)
	runner.Set(guest)
	if err := runner.Run(ctx); err != nil {
		t.Fatal(err)
	}
	first := runner.module
	again := slices.Clone(guest)
	runner.Set(again)
	if err := runner.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if runner.module != first {
		t.Error("the same guest was compiled a second time")
	}
	if touched != 2 {
		t.Errorf("host.Touch was called %d times by 2 Runs", touched)
	}

	// The caller's slice, with another guest written over the first.
	again = append(again[:0], emptyModule...)
	runner.Set(again)
	if err := runner.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if runner.module == first || touched != 2 {
		t.Errorf("Run ran the guest that the slice held before: host.Touch was called %d times", touched)
	}
}

// The only guest of a runtime keeps the name it gives itself, which is in the
// errors that wazero makes for it. One that runs beside another has no name.
func TestNamedGuestThatRunsAloneKeepsItsName(t *testing.T) {
	const unreachable = 0x00
	var runner Runner
	closeRunner(t, &runner)
	runner.Set(append(startGuest(nil, []byte{unreachable, end}, ""), namedModule("guest")[len(emptyModule):]...))
	for range 2 {
		err := runner.Run(t.Context())
		if err == nil || !strings.Contains(err.Error(), "module[guest]") {
			t.Errorf("a guest named guest that traps ended with %v, want an error that names it", err)
		}
	}
}
