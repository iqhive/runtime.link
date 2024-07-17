package cmdl_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"runtime.link/api"
	"runtime.link/api/cmdl"
)

func TestCommandLine(T *testing.T) {
	type Options struct {
		Flag         bool `cmdl:"--flag"`
		FlagInverted bool `cmdl:"--flag-inverted,invert"`
		FlagFormat   bool `cmdl:"--flag-format=%v"`
	}
	type API struct {
		api.Specification

		Main func(context.Context, Options) (string, error) `cmdl:"%v"`

		DoSomething func(context.Context) (string, error) `cmdl:"something"`
	}
	program := API{
		Main: func(ctx context.Context, opts Options) (string, error) {
			if opts.Flag {
				return "flag", nil
			}
			if !opts.FlagInverted {
				return "flag-inverted", nil
			}
			if opts.FlagFormat {
				return "flag-format", nil
			}
			return "", errors.New("unrecognised main flag")
		},
		DoSomething: func(context.Context) (string, error) {
			return "DoSomething", nil
		},
	}
	exec := func(args string) cmdl.System {
		return cmdl.System{
			Args: strings.Split(args, " "),
		}
	}
	expect := func(b []byte, err error) func(string) {
		T.Helper()
		return func(s string) {
			T.Helper()
			if err != nil {
				T.Fatal(err)
			}
			if string(b) != s+"\n" {
				T.Fatalf("expected %q, got %q", s, string(b))
			}
		}
	}
	expect(exec("test --flag").Output(program))("flag")
	expect(exec("test --flag-inverted").Output(program))("flag-inverted")
	expect(exec("test --flag-format=true").Output(program))("flag-format")
	expect(exec("test something").Output(program))("DoSomething")
}
