//go:build !wasm

package wasm

import (
	"context"
	"testing"

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
