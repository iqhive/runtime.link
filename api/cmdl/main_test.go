package cmdl_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"runtime.link/api"
	"runtime.link/api/cmdl"
)

func TestCommandLine(T *testing.T) {
	type Options struct {
		Flag         bool   `cmdl:"--flag"`
		FlagInverted bool   `cmdl:"--flag-inverted,invert"`
		FlagFormat   bool   `cmdl:"--flag-format=%v"`
		FlagString   string `cmdl:"--flag-string=%v"`
		FlagInt      int    `cmdl:"--flag-int=%v"`
		FlagPointer  *uint  `cmdl:"--flag-pointer=%v"`
	}
	type API struct {
		api.Specification

		Main func(context.Context, Options) (string, error) `cmdl:"%v"`

		WithPositional func(context.Context, string, Options) (string, error) `cmdl:"pos %[2]v %[1]v"`

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
			if opts.FlagString != "" {
				return opts.FlagString, nil
			}
			if opts.FlagInt != 0 {
				return strconv.Itoa(opts.FlagInt), nil
			}
			if opts.FlagPointer != nil {
				return strconv.Itoa(int(*opts.FlagPointer)), nil
			}
			return "", errors.New("unrecognised main flag")
		},
		WithPositional: func(_ context.Context, pos string, _ Options) (string, error) {
			return pos, nil
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
	expect(exec("test --flag-string=hello").Output(program))("hello")
	expect(exec("test --flag-int=42").Output(program))("42")
	expect(exec("test --flag-pointer=0").Output(program))("0")
	expect(exec("test pos --flag hello").Output(program))("hello")
}
