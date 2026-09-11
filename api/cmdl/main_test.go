package cmdl_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/cmdl"
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

type helpAPI struct {
	api.Specification `cmd:"example"
		is an example command.`
	WithPositional func(ctx context.Context, name string, options struct {
		Flag bool `cmdl:"--flag"`
	}) (string, error) `cmdl:"pos %[2]v %[1]v"`
}

type helpNestedAPI struct {
	api.Specification `cmd:"example"`
	Nested            struct {
		Greet func(ctx context.Context, name string) error `cmdl:"greet %v"`
	}
}

func registerCmdlSource(t *testing.T) {
	t.Helper()
	api.RegisterSource[helpAPI](fstest.MapFS{
		"api.go": &fstest.MapFile{Data: []byte(`package cmdl_test

import (
	"context"
	"github.com/iqhive/runtime.link/api"
)

type helpAPI struct {
	api.Specification
	WithPositional func(ctx context.Context, name string, options struct {
		Flag bool ` + "`cmdl:\"--flag\"`" + `
	}) (string, error) ` + "`cmdl:\"pos %[2]v %[1]v\"`" + `
}

type helpNestedAPI struct {
	api.Specification
	Nested struct {
		Greet func(ctx context.Context, name string) error ` + "`cmdl:\"greet %v\"`" + `
	}
}
`)},
	})
}

func TestHelpUsage(t *testing.T) {
	registerCmdlSource(t)
	out, err := cmdl.System{Args: []string{"example"}}.Output(helpAPI{
		WithPositional: func(context.Context, string, struct {
			Flag bool `cmdl:"--flag"`
		}) (string, error) {
			return "", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "pos <name> [--flag ...]") {
		t.Fatalf("usage missing from help:\n%s", out)
	}
}

func TestHelpNestedNamespace(t *testing.T) {
	registerCmdlSource(t)
	out, err := cmdl.System{Args: []string{"example"}}.Output(helpNestedAPI{
		Nested: struct {
			Greet func(ctx context.Context, name string) error `cmdl:"greet %v"`
		}{
			Greet: func(context.Context, string) error { return nil },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "greet <name>") {
		t.Fatalf("nested usage missing from help:\n%s", out)
	}
}

func TestPositionalStillRunsWithNames(t *testing.T) {
	registerCmdlSource(t)
	out, err := cmdl.System{Args: []string{"example", "pos", "--flag", "Ada"}}.Output(helpAPI{
		WithPositional: func(_ context.Context, name string, _ struct {
			Flag bool `cmdl:"--flag"`
		}) (string, error) {
			return name, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "Ada" {
		t.Fatalf("got %q", out)
	}
}

func TestHelpWithoutNames(t *testing.T) {
	var program struct {
		api.Specification `
			docs here`
		Run func(context.Context) error `cmdl:"run"`
	}
	program.Run = func(context.Context) error { return nil }
	out, err := cmdl.System{Args: []string{"example"}}.Output(program)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "docs here") {
		t.Fatalf("docs missing:\n%s", out)
	}
	if !strings.Contains(string(out), "run") {
		t.Fatalf("command missing:\n%s", out)
	}
}
