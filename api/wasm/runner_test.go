//go:build !wasm

package wasm

import (
	"context"
	"errors"
	"math"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api"
)

func TestWallTimeWorksWithoutNanoTime(t *testing.T) {
	var calls int
	var runner Runner
	runner.Set(clockModule(clockRealtime))
	runner.SetSystemInterface(SystemInterface{
		WallTime: func() (int64, int32) {
			calls++
			return 1, 2
		},
	})
	if err := runner.Run(t.Context()); err != nil {
		t.Fatal(err)
	}
	if calls == 0 {
		t.Fatal("guest read the wall clock, but WallTime wasn't called")
	}
}

func TestNanoTimeWorksWithoutWallTime(t *testing.T) {
	var calls int
	var runner Runner
	runner.Set(clockModule(clockMonotonic))
	runner.SetSystemInterface(SystemInterface{
		NanoTime: func() int64 {
			calls++
			return 1
		},
	})
	if err := runner.Run(t.Context()); err != nil {
		t.Fatal(err)
	}
	if calls == 0 {
		t.Fatal("guest read the monotonic clock, but NanoTime wasn't called")
	}
}

func TestCompiledModuleSurvivesRun(t *testing.T) {
	ctx := t.Context()
	var runner Runner
	runner.Add(host{})
	runner.Set([]byte(emptyModule))
	if err := runner.Compile(ctx); err != nil {
		t.Fatal(err)
	}
	compiled, runtime := runner.module, runner.runtime
	if err := runner.Run(ctx); err != nil {
		t.Fatalf("Compile's module was orphaned: %v", err)
	}
	if runner.module != compiled || runner.runtime != runtime {
		t.Fatal("Run replaced the runtime that Compile had set up")
	}
}

func TestRunCanRepeat(t *testing.T) {
	ctx := t.Context()
	for name, prepare := range map[string]func(*Runner) error{
		"bare":     func(*Runner) error { return nil },
		"with API": func(r *Runner) error { r.Add(host{}); return nil },
		"compiled": func(r *Runner) error { return r.Compile(ctx) },
	} {
		t.Run(name, func(t *testing.T) {
			var runner Runner
			runner.Set([]byte(emptyModule))
			if err := prepare(&runner); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := runner.Run(ctx); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestChangeAfterCompileRecompiles(t *testing.T) {
	ctx := t.Context()
	var runner Runner
	runner.Set([]byte(emptyModule))
	if err := runner.Compile(ctx); err != nil {
		t.Fatal(err)
	}
	runner.Add(host{})
	if err := runner.Run(ctx); err != nil {
		t.Fatalf("Run used a module from the runtime it had just closed: %v", err)
	}
}

func TestSetDropsTheCompiledModule(t *testing.T) {
	ctx := t.Context()
	var calls int
	var runner Runner
	runner.SetSystemInterface(SystemInterface{
		WallTime: func() (int64, int32) {
			calls++
			return 1, 2
		},
		NanoTime: func() int64 { return 1 },
	})
	runner.Set([]byte(emptyModule))
	if err := runner.Compile(ctx); err != nil {
		t.Fatal(err)
	}
	runner.Set(clockModule(clockRealtime))
	if err := runner.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if calls == 0 {
		t.Fatal("Run ran the module compiled before Set")
	}
}

func TestReplacingTheRuntimeCancelsOnlyTheOldContext(t *testing.T) {
	ctx := t.Context()
	var runner Runner
	runner.Set([]byte(emptyModule))
	if err := runner.Compile(ctx); err != nil {
		t.Fatal(err)
	}
	old := runner.context
	runner.Add(host{})
	if err := runner.Compile(ctx); err != nil {
		t.Fatal(err)
	}
	if old == runner.context {
		t.Fatal("the runtime wasn't replaced")
	}
	if old.Err() == nil {
		t.Error("the replaced runtime's context is still live")
	}
	if err := runner.context.Err(); err != nil {
		t.Errorf("the new runtime's context is already cancelled: %v", err)
	}
}

func closeRunner(t *testing.T, runner *Runner) {
	t.Helper()
	t.Cleanup(func() {
		if err := runner.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
}

// memoryModule returns a module that needs the given pages of memory to start.
func memoryModule(pages byte) []byte {
	return slices.Concat([]byte(emptyModule), section(5, vector([]byte{0, pages})))
}

// [SystemInterface.Memory] is a limit in bytes. One above what a guest's 32-bit
// addresses reach is no limit at all, and no reason for Run or Compile, which
// return an error, to panic.
func TestMemoryLimitAboveWhatAGuestCanAddress(t *testing.T) {
	ctx := t.Context()
	for _, limit := range []int{math.MaxInt, math.MaxInt32} {
		var runner Runner
		closeRunner(t, &runner)
		runner.SetSystemInterface(SystemInterface{Memory: limit})
		runner.Set(memoryModule(2))
		if err := runner.Compile(ctx); err != nil {
			t.Errorf("Compile under a limit of %d bytes: %v", limit, err)
		}
		if err := runner.Run(ctx); err != nil {
			t.Errorf("Run under a limit of %d bytes: %v", limit, err)
		}
	}
	if strconv.IntSize == 64 && memoryLimitPages(math.MaxInt) != wasmMaxPages {
		t.Errorf("a limit of every byte is %d pages, want all %d", memoryLimitPages(math.MaxInt), wasmMaxPages)
	}
}

func TestMemoryLimitIsInWholePages(t *testing.T) {
	ctx := t.Context()
	for _, test := range []struct {
		limit int
		pages byte
		fits  bool
	}{
		{wasmPageSize, 1, true},
		{wasmPageSize, 2, false},
		{2*wasmPageSize - 1, 2, false},
		{2 * wasmPageSize, 2, true},
		{wasmPageSize - 1, 1, false},
	} {
		var runner Runner
		closeRunner(t, &runner)
		runner.SetSystemInterface(SystemInterface{Memory: test.limit})
		runner.Set(memoryModule(test.pages))
		err := runner.Run(ctx)
		if fits := err == nil; fits != test.fits {
			t.Errorf("a guest that needs %d pages under a limit of %d bytes: %v", test.pages, test.limit, err)
		}
	}
	for limit, want := range map[int]uint32{
		1:                  0,
		wasmPageSize:       1,
		3*wasmPageSize + 5: 3,
		math.MaxInt32:      32767,
	} {
		if got := memoryLimitPages(limit); got != want {
			t.Errorf("a limit of %d bytes is %d pages, want %d", limit, got, want)
		}
	}
}

// A guest's memory is held at a default when [SystemInterface.Memory] names
// no limit: a guest that grew its memory without end would have the host
// feed it until it had none left.
func TestMemoryIsHeldAtADefaultWhenNoLimitIsNamed(t *testing.T) {
	ctx := t.Context()
	var runner Runner
	closeRunner(t, &runner)
	// A guest still runs, one whole byte of pages under the default.
	runner.Set(memoryModule(127))
	if err := runner.Run(ctx); err != nil {
		t.Errorf("a guest of 127 pages: %v", err)
	}
	pages := defaultMemory/wasmPageSize + 1
	runner.Set(slices.Concat([]byte(emptyModule), section(5, vector(slices.Concat([]byte{0}, uleb(uint64(pages)))))))
	if err := runner.Run(ctx); err == nil {
		t.Errorf("a guest of %d pages ran, want it held at the default of %d bytes", pages, defaultMemory)
	}
}

// A guest reflects on the types of what it holds and makes values of them, as
// it always could: the bridge links every one of these functions.
func TestGuestReflectsOnHostTypesAndMakesValuesOfThem(t *testing.T) {
	ctx := t.Context()
	var runner Runner
	closeRunner(t, &runner)
	runner.Add(host{Sum: func(_ context.Context, a, b int64) int64 { return a + b }})
	runner.Set([]byte(emptyModule))
	if err := runner.Compile(ctx); err != nil {
		t.Fatal(err)
	}
	definitions := runner.runtime.Module(bridgeModule).ExportedFunctionDefinitions()
	sigOf := func(name string) signature {
		t.Helper()
		definition := definitions[name]
		if definition == nil {
			t.Fatalf("%s isn't exported", name)
		}
		return signature{params: definition.ParamTypes(), results: definition.ResultTypes()}
	}
	for _, name := range []string{"struct_type", "slice_make", "chan_make", "pointer_new", "map_make", "type_kind", "type_arg", "type_elem", "field_type"} {
		sigOf(name)
	}
	call := callsOutsideRun(t, &runner)
	function := call.linked(t, "host", "Sum")
	fn, err := call(bridgeModule, "struct_type", "", sigOf("struct_type"), function)
	if err != nil {
		t.Fatalf("struct_type of a function: %v", err)
	}
	if kind, err := call(bridgeModule, "type_kind", "", sigOf("type_kind"), fn[0]); err != nil || reflect.Kind(kind[0]) != reflect.Func {
		t.Fatalf("type_kind of a function = %v, %v", kind, err)
	}
	arg, err := call(bridgeModule, "type_arg", "", sigOf("type_arg"), fn[0], 1)
	if err != nil {
		t.Fatalf("type_arg: %v", err)
	}
	if kind, err := call(bridgeModule, "type_kind", "", sigOf("type_kind"), arg[0]); err != nil || reflect.Kind(kind[0]) != reflect.Int64 {
		t.Errorf("type_kind of Sum's first argument = %v, %v", kind, err)
	}
	if made, err := call(bridgeModule, "pointer_new", "", sigOf("pointer_new"), arg[0]); err != nil || made[0] == 0 {
		t.Errorf("pointer_new of an int64 = %v, %v", made, err)
	}
}

func TestNamedModuleCanRunTwice(t *testing.T) {
	ctx := t.Context()
	var runner Runner
	closeRunner(t, &runner)
	runner.Set(namedModule("guest"))
	for range 2 {
		if err := runner.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCompiledNamedModuleCanRunTwice(t *testing.T) {
	ctx := t.Context()
	var runner Runner
	closeRunner(t, &runner)
	runner.Set(namedModule("guest"))
	if err := runner.Compile(ctx); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := runner.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCloseReleasesTheRuntime(t *testing.T) {
	ctx := t.Context()
	var runner Runner
	runner.Set([]byte(emptyModule))
	if err := runner.Compile(ctx); err != nil {
		t.Fatal(err)
	}
	used := runner.context
	if err := runner.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if used.Err() == nil {
		t.Error("the closed runtime's context is still live")
	}
	if runner.runtime != nil || runner.module != nil {
		t.Error("the Runner still holds what it closed")
	}
}

func TestCloseCanRepeatOrComeFirst(t *testing.T) {
	ctx := t.Context()
	var runner Runner
	for range 2 {
		if err := runner.Close(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunnerWorksAfterClose(t *testing.T) {
	ctx := t.Context()
	var runner Runner
	closeRunner(t, &runner)
	runner.Add(host{})
	runner.Set([]byte(emptyModule))
	if err := runner.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if err := runner.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := runner.Run(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestRunningOnceReleasesTheRuntime(t *testing.T) {
	for name, module := range map[string][]byte{
		"guest that works": []byte(emptyModule),
		"guest that fails": []byte("not wasm"),
	} {
		t.Run(name, func(t *testing.T) {
			var runner Runner
			runner.once(t.Context(), module, []api.WithSpecification{host{}})
			if runner.runtime != nil {
				t.Fatal("the runtime is still open")
			}
		})
	}
}

func TestPackageFunctionsRun(t *testing.T) {
	ctx := t.Context()
	if err := Run(ctx, []byte(emptyModule)); err != nil {
		t.Fatal(err)
	}
	if _, err := CombinedOutput(ctx, []byte(emptyModule)); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, []byte("not wasm")); err == nil {
		t.Fatal("Run lost the error")
	}
}

// loopingGuest returns a guest that calls host.Touch, so that the host knows
// it is running, and then loops for ever.
func loopingGuest() []byte {
	return startGuest([]hostImport{{"host", "Touch", signature{}}}, []byte{call, 0, loop, emptyType, br, 0, end, end}, "")
}

// stopTimeout is how long a test waits for a guest to be stopped, which takes
// wazero milliseconds. A Run that is still going by then never returns.
const stopTimeout = 30 * time.Second

// returned waits for a call that runs a guest to return, in a goroutine of its
// own so that a guest nothing stops fails the test instead of hanging it.
func returned(t *testing.T, run func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- run() }()
	select {
	case err := <-done:
		return err
	case <-time.After(stopTimeout):
		t.Fatalf("the guest was still running %v after it should have been stopped", stopTimeout)
		return nil
	}
}

// Nothing but its context can stop a guest that the caller of Run has lost
// patience with, and a guest that loops never ends by itself.
func TestRunReturnsWhenItsContextEnds(t *testing.T) {
	looping := host{Touch: func(context.Context) {}}

	t.Run("deadline", func(t *testing.T) {
		var runner Runner
		closeRunner(t, &runner)
		runner.Add(looping)
		runner.Set(loopingGuest())
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		err := returned(t, func() error { return runner.Run(ctx) })
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Run returned %v, want an error that is %v", err, context.DeadlineExceeded)
		}
		// The Runner is as usable as after any other guest.
		runner.Set([]byte(emptyModule))
		if err := runner.Run(t.Context()); err != nil {
			t.Errorf("the next Run: %v", err)
		}
	})

	t.Run("cancelled", func(t *testing.T) {
		var runner Runner
		closeRunner(t, &runner)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		// Cancelled once the guest is in its loop, by the function it calls first.
		runner.Add(host{Touch: func(context.Context) { cancel() }})
		runner.Set(loopingGuest())
		if err := runner.Compile(ctx); err != nil {
			t.Fatal(err)
		}
		err := returned(t, func() error { return runner.Run(ctx) })
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run returned %v, want an error that is %v", err, context.Canceled)
		}
	})

	t.Run("ended before the guest starts", func(t *testing.T) {
		var runner Runner
		closeRunner(t, &runner)
		runner.Add(looping)
		runner.Set(loopingGuest())
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		err := returned(t, func() error { return runner.Run(ctx) })
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run returned %v, want an error that is %v", err, context.Canceled)
		}
	})

	t.Run("package functions", func(t *testing.T) {
		for name, run := range map[string]func(context.Context) error{
			"Run":            func(ctx context.Context) error { return Run(ctx, loopingGuest(), looping) },
			"CombinedOutput": func(ctx context.Context) error { _, err := CombinedOutput(ctx, loopingGuest(), looping); return err },
		} {
			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			err := returned(t, func() error { return run(ctx) })
			cancel()
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("%s returned %v, want an error that is %v", name, err, context.DeadlineExceeded)
			}
		}
	})
}

// A context that ends after the guest has is no error of the guest's.
func TestRunThatEndedIsNotStoppedByItsContext(t *testing.T) {
	var runner Runner
	closeRunner(t, &runner)
	runner.Set([]byte(emptyModule))
	ctx, cancel := context.WithCancel(t.Context())
	if err := runner.Run(ctx); err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := runner.Run(t.Context()); err != nil {
		t.Errorf("a Run after an earlier one's context ended: %v", err)
	}
}

// Close is the other way to stop a guest: it is what a caller without the
// context of the Run has.
func TestCloseStopsARunInProgress(t *testing.T) {
	var runner Runner
	closeRunner(t, &runner)
	started := make(chan struct{})
	runner.Add(host{Touch: func(context.Context) { close(started) }})
	runner.Set(loopingGuest())
	done := make(chan error, 1)
	go func() { done <- runner.Run(context.WithoutCancel(t.Context())) }()
	select {
	case <-started:
	case err := <-done:
		t.Fatalf("Run returned before its guest started: %v", err)
	case <-time.After(stopTimeout):
		t.Fatal("the guest never started")
	}
	if err := runner.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run returned %v from a closed Runner, want an error that is %v", err, context.Canceled)
		}
	case <-time.After(stopTimeout):
		t.Fatalf("the guest was still running %v after its Runner was closed", stopTimeout)
	}

	// Close leaves the Runner usable, as it does when nothing is running.
	runner.Set([]byte(emptyModule))
	if err := runner.Run(t.Context()); err != nil {
		t.Errorf("Run after Close: %v", err)
	}
}
