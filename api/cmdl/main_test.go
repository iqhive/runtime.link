package cmdl_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

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

type namedPlaceholderAPI struct {
	api.Specification
	Greet func(ctx context.Context, title, who string) (string, error) `cmdl:"greet {who} {title}"`
}

type helpOrderOptions struct {
	Flag bool `cmdl:"--flag"`
}

type helpOrderAPI struct {
	api.Specification
	Copy func(ctx context.Context, src, dst string) (string, error)                       `cmdl:"copy %[2]v %[1]v"`
	Pos  func(ctx context.Context, name string, options helpOrderOptions) (string, error) `cmdl:"pos %[2]v %[1]v"`
	Mix  func(ctx context.Context, a, b, c string) (string, error)                        `cmdl:"mix %[3]v %[1]v %[2]v"`
	Rel  func(ctx context.Context, a, b, c string) (string, error)                        `cmdl:"rel %v %[2]v %v"`
	Join func(ctx context.Context, sep string, parts ...string) (string, error)           `cmdl:"join %v %v"`
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

type namedPlaceholderAPI struct {
	api.Specification
	Greet func(ctx context.Context, title, who string) (string, error) ` + "`cmdl:\"greet {who} {title}\"`" + `
}

type helpOrderAPI struct {
	api.Specification
	Copy func(ctx context.Context, src, dst string) (string, error) ` + "`cmdl:\"copy %[2]v %[1]v\"`" + `
	Pos  func(ctx context.Context, name string, options helpOrderOptions) (string, error) ` + "`cmdl:\"pos %[2]v %[1]v\"`" + `
	Mix  func(ctx context.Context, a, b, c string) (string, error) ` + "`cmdl:\"mix %[3]v %[1]v %[2]v\"`" + `
	Rel  func(ctx context.Context, a, b, c string) (string, error) ` + "`cmdl:\"rel %v %[2]v %v\"`" + `
	Join func(ctx context.Context, sep string, parts ...string) (string, error) ` + "`cmdl:\"join %v %v\"`" + `
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
	if !strings.Contains(string(out), "pos [--flag ...] <name>") {
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

func TestValuedOptions(T *testing.T) {
	type Options struct {
		All  bool   `cmdl:"-all"`
		Days int    `cmdl:"-days %v"`
		Out  string `cmdl:"-out %v"`
		Name string `cmdl:"-name=%v"`
	}
	type API struct {
		api.Specification

		Cert func(context.Context, string, Options) (string, error) `cmdl:"cert %[2]v %[1]v"`
	}
	program := API{
		Cert: func(_ context.Context, who string, opts Options) (string, error) {
			return fmt.Sprintf("%s all=%v days=%d out=%s name=%s", who, opts.All, opts.Days, opts.Out, opts.Name), nil
		},
	}
	run := func(args string) (string, error) {
		out, err := cmdl.System{Args: strings.Split(args, " ")}.Output(program)
		return strings.TrimSuffix(string(out), "\n"), err
	}
	for args, want := range map[string]string{
		"test cert x":                           "x all=false days=0 out= name=",
		"test cert -days 7 x":                   "x all=false days=7 out= name=",
		"test cert -days 7 -out /tmp/c x":       "x all=false days=7 out=/tmp/c name=",
		"test cert -out /tmp/c -all -days 90 x": "x all=true days=90 out=/tmp/c name=",
		"test cert -name=bob x":                 "x all=false days=0 out= name=bob",
		"test cert -all -name=bob -days 1 x":    "x all=true days=1 out= name=bob",
	} {
		got, err := run(args)
		if err != nil {
			T.Errorf("%q: %v", args, err)
			continue
		}
		if got != want {
			T.Errorf("%q: got %q, want %q", args, got, want)
		}
	}
	if _, err := run("test cert -days"); err == nil {
		T.Error("a trailing -days with no value should be an error")
	}
}

func runProgram(program any, args ...string) (string, error) {
	out, err := cmdl.System{Args: append([]string{"prog"}, args...)}.Output(program)
	return strings.TrimSuffix(string(out), "\n"), err
}

func TestUnknownSubcommandNeverRunsASibling(t *testing.T) {
	var ran []string
	record := func(name string) func(context.Context) error {
		return func(context.Context) error {
			ran = append(ran, name)
			return nil
		}
	}
	type API struct {
		api.Specification
		Add    func(context.Context, string) error `cmdl:"remote add %v"`
		Drop   func(context.Context) error         `cmdl:"remote drop"`
		DBDrop func(context.Context) error         `cmdl:"db drop"`
	}
	program := API{
		Add: func(_ context.Context, name string) error {
			ran = append(ran, "add "+name)
			return nil
		},
		Drop:   record("remote drop"),
		DBDrop: record("db drop"),
	}
	for _, args := range [][]string{
		{"remote", "bogus"},
		{"remote"},
		{"db"},
		{"db", "status"},
		{"bogus"},
		{"bogus", "drop"},
	} {
		if _, err := runProgram(program, args...); err == nil {
			t.Errorf("%q: want an error", args)
		}
	}
	if len(ran) != 0 {
		t.Errorf("ran %q", ran)
	}
	if _, err := runProgram(program, "db", "drop"); err != nil || !slices.Equal(ran, []string{"db drop"}) {
		t.Errorf("ran %q, err=%v", ran, err)
	}
}

func TestNearMissDoesNotShadowACommandThatFits(t *testing.T) {
	type API struct {
		api.Specification
		Drop func(context.Context) (string, error)                 `cmdl:"db drop"`
		Pair func(context.Context, string, string) (string, error) `cmdl:"%v %v"`
	}
	program := API{
		Drop: func(context.Context) (string, error) { return "dropped", nil },
		Pair: func(_ context.Context, a, b string) (string, error) { return a + "+" + b, nil },
	}
	if got, err := runProgram(program, "db", "status"); err != nil || got != "db+status" {
		t.Errorf("got %q err=%v", got, err)
	}
	if got, err := runProgram(program, "db", "drop"); err != nil || got != "dropped" {
		t.Errorf("got %q err=%v", got, err)
	}

	type Words struct {
		api.Specification
		Flag func(context.Context, string) (string, error) `cmdl:"--a=%v"`
		Any  func(context.Context, string) (string, error) `cmdl:"%v"`
	}
	echo := func(prefix string) func(context.Context, string) (string, error) {
		return func(_ context.Context, s string) (string, error) { return prefix + s, nil }
	}
	words := Words{Flag: echo("flag:"), Any: echo("any:")}
	if got, err := runProgram(words, "--b=1"); err != nil || got != "any:--b=1" {
		t.Errorf("a word that does not fit must not beat one that does: got %q err=%v", got, err)
	}
	if got, err := runProgram(words, "--a=1"); err != nil || got != "flag:1" {
		t.Errorf("got %q err=%v", got, err)
	}
}

func TestUnknownWordIsNotSwallowedByARootCommand(t *testing.T) {
	type API struct {
		api.Specification
		Main func(context.Context, struct {
			Flag bool `cmdl:"--flag"`
		}) error `cmdl:"%v"`
	}
	var called bool
	program := API{Main: func(context.Context, struct {
		Flag bool `cmdl:"--flag"`
	}) error {
		called = true
		return nil
	}}
	if _, err := runProgram(program, "bogus"); err == nil || called {
		t.Errorf("called=%v err=%v", called, err)
	}
}

func TestLaterWordsSelectTheCommand(t *testing.T) {
	type API struct {
		api.Specification
		To   func(context.Context, string, string) (string, error) `cmdl:"copy %v to %v"`
		Into func(context.Context, string, string) (string, error) `cmdl:"copy %v into %v"`
	}
	join := func(verb string) func(context.Context, string, string) (string, error) {
		return func(_ context.Context, a, b string) (string, error) { return a + " " + verb + " " + b, nil }
	}
	program := API{To: join("to"), Into: join("into")}
	for args, want := range map[string]string{"copy a to b": "a to b", "copy a into b": "a into b"} {
		if got, err := runProgram(program, strings.Fields(args)...); err != nil || got != want {
			t.Errorf("%q: got %q err=%v", args, got, err)
		}
	}
	if got, err := runProgram(program, "copy", "a", "onto", "b"); err == nil {
		t.Errorf("onto ran: %q", got)
	}
}

func TestWordAfterFlagsIsChecked(t *testing.T) {
	type Opts struct {
		Verbose bool `cmdl:"-v"`
	}
	type API struct {
		api.Specification
		Status func(context.Context, Opts) (string, error) `cmdl:"%v status"`
	}
	program := API{Status: func(_ context.Context, o Opts) (string, error) {
		return fmt.Sprint(o.Verbose), nil
	}}
	if got, err := runProgram(program, "-v", "status"); err != nil || got != "true" {
		t.Errorf("got %q err=%v", got, err)
	}
	if got, err := runProgram(program, "status"); err != nil || got != "false" {
		t.Errorf("got %q err=%v", got, err)
	}
	if got, err := runProgram(program, "-v", "drop"); err == nil {
		t.Errorf("drop ran status: %q", got)
	}

	type Later struct {
		api.Specification
		Run func(context.Context, Opts, string) (string, error) `cmdl:"%v --mode=%v"`
	}
	later := Later{Run: func(_ context.Context, o Opts, mode string) (string, error) {
		return fmt.Sprint(o.Verbose, " ", mode), nil
	}}
	if got, err := runProgram(later, "-v", "--mode=fast"); err != nil || got != "true fast" {
		t.Errorf("got %q err=%v", got, err)
	}
	got, err := runProgram(later, "-v", "--other=fast")
	if err == nil {
		t.Errorf("a word that does not fit after flags ran: %q", got)
	}
}

func TestLeftoverArgumentsAreAnError(t *testing.T) {
	type Opts struct {
		Force bool `cmdl:"--force"`
	}
	type API struct {
		api.Specification
		Drop   func(context.Context) error               `cmdl:"drop"`
		Remove func(context.Context, string) error       `cmdl:"rm %v"`
		Run    func(context.Context, Opts) error         `cmdl:"run %v"`
		Move   func(context.Context, string, Opts) error `cmdl:"mv %[2]v %[1]v"`
	}
	var called int
	program := API{
		Drop:   func(context.Context) error { called++; return nil },
		Remove: func(context.Context, string) error { called++; return nil },
		Run:    func(context.Context, Opts) error { called++; return nil },
		Move:   func(context.Context, string, Opts) error { called++; return nil },
	}
	for _, args := range [][]string{
		{"drop", "--dry-run"},
		{"rm", "a", "b"},
		{"run", "--bogus"},
		{"mv", "a", "--force"},
	} {
		if _, err := runProgram(program, args...); err == nil {
			t.Errorf("%q: want an error", args)
		}
	}
	if called != 0 {
		t.Errorf("called %d times", called)
	}
}

func TestOptionOnALiteralCommand(t *testing.T) {
	type API struct {
		api.Specification
		Version func(context.Context) (map[string]string, error) `cmdl:"version,json"`
		List    func(context.Context) (string, error)            `cmdl:"list | json"`
	}
	program := API{
		Version: func(context.Context) (map[string]string, error) { return map[string]string{"v": "1"}, nil },
		List:    func(context.Context) (string, error) { return "listed", nil },
	}
	got, err := runProgram(program, "version")
	if err != nil || strings.Join(strings.Fields(got), "") != `{"v":"1"}` {
		t.Errorf("version: %q err=%v", got, err)
	}
	if got, err := runProgram(program, "list"); err != nil || got != "listed" {
		t.Errorf("list: %q err=%v", got, err)
	}
}

func TestMissingPositionalIsAnError(t *testing.T) {
	var called bool
	type API struct {
		api.Specification
		Remove func(context.Context, string) error    `cmdl:"rm %v"`
		Count  func(context.Context, int) error       `cmdl:"count %v"`
		At     func(context.Context, time.Time) error `cmdl:"at %v"`
		Open   func(context.Context, fs.File) error   `cmdl:"open %v"`
	}
	program := API{
		Remove: func(context.Context, string) error { called = true; return nil },
		Count:  func(context.Context, int) error { called = true; return nil },
		At:     func(context.Context, time.Time) error { called = true; return nil },
		Open:   func(context.Context, fs.File) error { called = true; return nil },
	}
	for _, command := range []string{"rm", "count", "at", "open"} {
		if _, err := runProgram(program, command); err == nil {
			t.Errorf("%s: want an error", command)
		}
	}
	if called {
		t.Error("the function ran without its argument")
	}
}

func TestOptionalPositionals(t *testing.T) {
	type API struct {
		api.Specification
		Show func(context.Context, *int) (string, error)              `cmdl:"show %v"`
		Join func(context.Context, string, ...string) (string, error) `cmdl:"join %v %v"`
	}
	program := API{
		Show: func(_ context.Context, n *int) (string, error) {
			if n == nil {
				return "unset", nil
			}
			return strconv.Itoa(*n), nil
		},
		Join: func(_ context.Context, sep string, parts ...string) (string, error) {
			return "[" + strings.Join(parts, sep) + "]", nil
		},
	}
	for args, want := range map[string]string{
		"show":       "unset",
		"show 3":     "3",
		"join +":     "[]",
		"join + a":   "[a]",
		"join + a b": "[a+b]",
	} {
		if got, err := runProgram(program, strings.Fields(args)...); err != nil || got != want {
			t.Errorf("%q: got %q err=%v", args, got, err)
		}
	}
	if got, err := runProgram(program, "join"); err == nil {
		t.Errorf("join without its first argument: %q", got)
	}
}

func TestFileArgument(t *testing.T) {
	type API struct {
		api.Specification
		Read func(context.Context, fs.File) (string, error) `cmdl:"read %v"`
	}
	program := API{Read: func(_ context.Context, file fs.File) (string, error) {
		data, err := io.ReadAll(file)
		return string(data), err
	}}
	system := cmdl.System{
		Args: []string{"prog", "read", "a.txt"},
		FS:   fstest.MapFS{"a.txt": &fstest.MapFile{Data: []byte("contents")}},
	}
	got, err := system.Output(program)
	if err != nil || string(got) != "contents\n" {
		t.Errorf("got %q err=%v", got, err)
	}
}

func TestInvertDefaultAppliesWithoutArguments(t *testing.T) {
	type Opts struct {
		Color bool `cmdl:"--no-color,invert"`
	}
	type API struct {
		api.Specification
		Run func(context.Context, Opts) (string, error) `cmdl:"run %v"`
	}
	program := API{Run: func(_ context.Context, o Opts) (string, error) {
		return fmt.Sprintf("color=%v", o.Color), nil
	}}
	if got, err := runProgram(program, "run"); err != nil || got != "color=true" {
		t.Errorf("run: %q err=%v", got, err)
	}
	if got, err := runProgram(program, "run", "--no-color"); err != nil || got != "color=false" {
		t.Errorf("run --no-color: %q err=%v", got, err)
	}
}

func TestStructValuedFlags(t *testing.T) {
	type Opts struct {
		Since   time.Time  `cmdl:"--since=%v"`
		Until   *time.Time `cmdl:"--until %v"`
		Verbose bool       `cmdl:"-v"`
	}
	type API struct {
		api.Specification
		Run func(context.Context, string, Opts) (string, error) `cmdl:"run %[2]v %[1]v"`
	}
	program := API{Run: func(_ context.Context, name string, o Opts) (string, error) {
		until := 0
		if o.Until != nil {
			until = o.Until.Year()
		}
		return fmt.Sprintf("%s since=%d until=%d verbose=%v", name, o.Since.Year(), until, o.Verbose), nil
	}}
	for args, want := range map[string]string{
		"run --since=2024-01-02T03:04:05Z x":                                 "x since=2024 until=0 verbose=false",
		"run --since=2024-01-02T03:04:05Z -v x":                              "x since=2024 until=0 verbose=true",
		"run -v --until 2030-01-02T03:04:05Z --since=2024-01-02T03:04:05Z x": "x since=2024 until=2030 verbose=true",
	} {
		if got, err := runProgram(program, strings.Fields(args)...); err != nil || got != want {
			t.Errorf("%q: got %q err=%v", args, got, err)
		}
	}
	if got, err := runProgram(program, "run", "--since=yesterday", "x"); err == nil {
		t.Errorf("bad time accepted: %q", got)
	}
}

func TestStructPositional(t *testing.T) {
	type API struct {
		api.Specification
		At func(context.Context, time.Time) (string, error) `cmdl:"at %v"`
	}
	program := API{At: func(_ context.Context, at time.Time) (string, error) { return fmt.Sprint(at.Year()), nil }}
	if got, err := runProgram(program, "at", "2024-01-02T03:04:05Z"); err != nil || got != "2024" {
		t.Errorf("got %q err=%v", got, err)
	}
}

func TestNestedFlagGroup(t *testing.T) {
	type Group struct {
		Verbose bool   `cmdl:"-v"`
		Name    string `cmdl:"-name=%v"`
	}
	type Opts struct {
		Quiet bool  `cmdl:"-q"`
		Group Group `cmdl:"--group"`
	}
	type API struct {
		api.Specification
		Run func(context.Context, string, Opts) (string, error) `cmdl:"run %[2]v %[1]v"`
	}
	program := API{Run: func(_ context.Context, arg string, o Opts) (string, error) {
		return fmt.Sprintf("%s quiet=%v verbose=%v name=%s", arg, o.Quiet, o.Group.Verbose, o.Group.Name), nil
	}}
	got, err := runProgram(program, "run", "--group", "-v", "-name=ada", "-q", "x")
	if want := "x quiet=true verbose=true name=ada"; err != nil || got != want {
		t.Errorf("got %q err=%v, want %q", got, err, want)
	}
}

func TestValuesParseStrictly(t *testing.T) {
	type Opts struct {
		Count int     `cmdl:"--count=%v"`
		Small int8    `cmdl:"--small=%v"`
		Size  uint    `cmdl:"--size=%v"`
		Ratio float64 `cmdl:"--ratio=%v"`
		On    bool    `cmdl:"--on=%v"`
		Hex   int     `cmdl:"--hex=%x"`
		Dec   int     `cmdl:"--dec=%d"`
		Word  *string `cmdl:"--word=%v"`
	}
	type API struct {
		api.Specification
		Run func(context.Context, Opts) (string, error) `cmdl:"run %v"`
		Pos func(context.Context, int) (string, error)  `cmdl:"pos %v"`
	}
	program := API{
		Run: func(_ context.Context, o Opts) (string, error) {
			word := ""
			if o.Word != nil {
				word = *o.Word
			}
			return fmt.Sprintf("%v %v %v %v %v %v %v %q", o.Count, o.Small, o.Size, o.Ratio, o.On, o.Hex, o.Dec, word), nil
		},
		Pos: func(_ context.Context, n int) (string, error) { return strconv.Itoa(n), nil },
	}
	for _, args := range [][]string{
		{"run", "--count=12abc"},
		{"run", "--count= 12"},
		{"run", "--count="},
		{"run", "--small=300"},
		{"run", "--size=-1"},
		{"run", "--ratio=1.5x"},
		{"run", "--on=truex"},
		{"run", "--hex=ffzz"},
		{"run", "--dec=12abc"},
		{"pos", "7days"},
		{"pos", "1.5"},
	} {
		if got, err := runProgram(program, args...); err == nil {
			t.Errorf("%q accepted as %q", args, got)
		}
	}
	for args, want := range map[string]string{
		"run --count=-12":  `-12 0 0 0 false 0 0 ""`,
		"run --count=0x10": `16 0 0 0 false 0 0 ""`,
		"run --small=127":  `0 127 0 0 false 0 0 ""`,
		"run --size=7":     `0 0 7 0 false 0 0 ""`,
		"run --ratio=1.5":  `0 0 0 1.5 false 0 0 ""`,
		"run --on=true":    `0 0 0 0 true 0 0 ""`,
		"run --hex=ff":     `0 0 0 0 false 255 0 ""`,
		"run --dec=010":    `0 0 0 0 false 0 10 ""`,
		"pos 7":            "7",
	} {
		if got, err := runProgram(program, strings.Fields(args)...); err != nil || got != want {
			t.Errorf("%q: got %q err=%v, want %q", args, got, err, want)
		}
	}
	if got, err := runProgram(program, "run", "--word=two words"); err != nil || got != `0 0 0 0 false 0 0 "two words"` {
		t.Errorf("a string takes the whole value: %q err=%v", got, err)
	}
}

func TestIndexedPlaceholderOnANonString(t *testing.T) {
	type API struct {
		api.Specification
		Sub func(context.Context, int, int) (string, error) `cmdl:"sub %[1]v %[2]v"`
	}
	program := API{Sub: func(_ context.Context, a, b int) (string, error) { return strconv.Itoa(a - b), nil }}
	if got, err := runProgram(program, "sub", "9", "4"); err != nil || got != "5" {
		t.Errorf("got %q err=%v", got, err)
	}
}

func TestNamedPlaceholder(t *testing.T) {
	registerCmdlSource(t)
	program := namedPlaceholderAPI{Greet: func(_ context.Context, title, who string) (string, error) {
		return title + " " + who, nil
	}}
	if got, err := runProgram(program, "greet", "ada", "dr"); err != nil || got != "dr ada" {
		t.Errorf("got %q err=%v", got, err)
	}
	help, err := runProgram(program)
	if err != nil || strings.Contains(help, "{") {
		t.Errorf("a placeholder is not a word of the command:\n%s\nerr=%v", help, err)
	}
}

func TestNestedCommandsRun(t *testing.T) {
	type API struct {
		api.Specification
		Nested struct {
			Greet func(context.Context, string) (string, error) `cmdl:"greet %v"`
		}
		Git struct {
			Remote struct {
				List func(context.Context) (string, error) `cmdl:"list"`
			} `cmdl:"remote"`
		} `cmdl:"git"`
	}
	var program API
	program.Nested.Greet = func(_ context.Context, s string) (string, error) { return "hi " + s, nil }
	program.Git.Remote.List = func(context.Context) (string, error) { return "listed", nil }
	if got, err := runProgram(program, "greet", "ada"); err != nil || got != "hi ada" {
		t.Errorf("greet: %q err=%v", got, err)
	}
	if got, err := runProgram(program, "git", "remote", "list"); err != nil || got != "listed" {
		t.Errorf("git remote list: %q err=%v", got, err)
	}
	if got, err := runProgram(program, "list"); err == nil {
		t.Errorf("a namespaced command ran without its prefix: %q", got)
	}
}

func TestHelpListsOnlyWhatRuns(t *testing.T) {
	var program helpRunnableAPI
	help, err := runProgram(program)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(help, "git remote list") {
		t.Errorf("namespace missing from help:\n%s", help)
	}
	if !slices.Contains(strings.Split(help, "\n"), "git") {
		t.Errorf("help leaves out a command that runs:\n%s", help)
	}
	if strings.Contains(help, "<path>") {
		t.Errorf("help lists a function with no command:\n%s", help)
	}
}

type helpRunnableAPI struct {
	api.Specification
	Untagged func(ctx context.Context, path string) error
	Git      struct {
		Status func(context.Context) (string, error)
		Remote struct {
			List func(context.Context) (string, error) `cmdl:"list"`
		} `cmdl:"remote"`
	} `cmdl:"git"`
}

func TestNoArgumentsAtAllShowsHelp(t *testing.T) {
	type API struct {
		api.Specification `
			docs here`
		Run func(context.Context) error `cmdl:"run"`
	}
	out, err := cmdl.System{}.Output(API{})
	if err != nil || !strings.Contains(string(out), "docs here") {
		t.Errorf("got %q err=%v", out, err)
	}
}

func TestArgumentsCannotReachUnflaggedFields(t *testing.T) {
	type Opts struct {
		hidden string `cmdl:"--hidden=%v"`
		Plain  string
		Skip   string `cmdl:"-"`
	}
	type API struct {
		api.Specification
		Run func(context.Context, string, Opts) (string, error) `cmdl:"run %[2]v %[1]v"`
	}
	program := API{Run: func(_ context.Context, pos string, o Opts) (string, error) {
		return fmt.Sprintf("pos=%q hidden=%q plain=%q skip=%q", pos, o.hidden, o.Plain, o.Skip), nil
	}}
	for args, want := range map[string]string{
		"run -":          `pos="-" hidden="" plain="" skip=""`,
		"run --hidden=x": `pos="--hidden=x" hidden="" plain="" skip=""`,
	} {
		if got, err := runProgram(program, strings.Fields(args)...); err != nil || got != want {
			t.Errorf("%q: got %q err=%v, want %q", args, got, err, want)
		}
	}
	if got, err := runProgram(program, "run", ""); err != nil || got != `pos="" hidden="" plain="" skip=""` {
		t.Errorf("empty argument: got %q err=%v", got, err)
	}
}

func TestBytesResultIsWrittenAsIs(t *testing.T) {
	type API struct {
		api.Specification
		Cat func(context.Context) ([]byte, error) `cmdl:"cat"`
	}
	got, err := cmdl.System{Args: []string{"prog", "cat"}}.Output(API{
		Cat: func(context.Context) ([]byte, error) { return []byte("raw\n"), nil },
	})
	if err != nil || string(got) != "raw\n" {
		t.Errorf("got %q err=%v", got, err)
	}
}

func TestPlaceholderInsideAWordRuns(t *testing.T) {
	type API struct {
		api.Specification

		Greet func(context.Context, string) (string, error)         `cmdl:"greet --name=%v"`
		Count func(context.Context, int) (string, error)            `cmdl:"count --n=%v"`
		Span  func(context.Context, int, int) (string, error)       `cmdl:"span --range=%v-%v"`
		Swap  func(context.Context, string, string) (string, error) `cmdl:"swap --b=%[2]v --a=%[1]v"`
		Note  func(context.Context, string) (string, error)         `cmdl:"note %v.txt"`
		Maybe func(context.Context, *int) (string, error)           `cmdl:"maybe --n=%v"`
		Twice func(context.Context, string) (string, error)         `cmdl:"--mode=%v status"`
	}
	echo := func(_ context.Context, s string) (string, error) { return s, nil }
	number := func(_ context.Context, n int) (string, error) { return strconv.Itoa(n), nil }
	program := API{
		Greet: echo,
		Count: number,
		Span:  func(_ context.Context, a, b int) (string, error) { return strconv.Itoa(a + b), nil },
		Swap:  func(_ context.Context, a, b string) (string, error) { return a + "," + b, nil },
		Note:  echo,
		Maybe: func(_ context.Context, n *int) (string, error) {
			if n == nil {
				return "unset", nil
			}
			return strconv.Itoa(*n), nil
		},
		Twice: echo,
	}
	for args, want := range map[string]string{
		"greet --name=ada":   "ada",
		"greet --name=":      "",
		"count --n=12":       "12",
		"span --range=1-5":   "6",
		"swap --b=B --a=A":   "A,B",
		"note todo.txt":      "todo",
		"maybe":              "unset",
		"maybe --n=3":        "3",
		"--mode=fast status": "fast",
	} {
		if got, err := runProgram(program, strings.Fields(args)...); err != nil || got != want {
			t.Errorf("%q: got %q err=%v, want %q", args, got, err, want)
		}
	}
	for _, args := range []string{
		"greet",
		"greet ada",
		"greet --other=ada",
		"count --n=12abc",
		"span --range=15",
		"swap --a=A --b=B",
		"note todo.md",
		"--mode=fast drop",
	} {
		if got, err := runProgram(program, strings.Fields(args)...); err == nil {
			t.Errorf("%q ran: %q", args, got)
		}
	}
}

func TestPlaceholderInsideAWordOpensAFile(t *testing.T) {
	type API struct {
		api.Specification
		Read func(context.Context, fs.File) (string, error) `cmdl:"read --file=%v"`
	}
	program := API{Read: func(_ context.Context, file fs.File) (string, error) {
		data, err := io.ReadAll(file)
		return string(data), err
	}}
	system := cmdl.System{
		Args: []string{"prog", "read", "--file=a.txt"},
		FS:   fstest.MapFS{"a.txt": &fstest.MapFile{Data: []byte("contents")}},
	}
	got, err := system.Output(program)
	if err != nil || string(got) != "contents\n" {
		t.Errorf("got %q err=%v", got, err)
	}
}

func TestPercentThatIsNotAPlaceholderIsACommandWord(t *testing.T) {
	type API struct {
		api.Specification

		Date   func(context.Context) (string, error) `cmdl:"date +%Y"`
		Escape func(context.Context) (string, error) `cmdl:"escape %%v"`
	}
	done := func(context.Context) (string, error) { return "ran", nil }
	program := API{Date: done, Escape: done}
	for _, args := range [][]string{{"date", "+%Y"}, {"escape", "%v"}} {
		if got, err := runProgram(program, args...); err != nil || got != "ran" {
			t.Errorf("%q: got %q err=%v", args, got, err)
		}
	}
	for _, args := range [][]string{{"date", "+%M"}, {"date"}, {"escape", "x"}, {"escape", "%%v"}} {
		if got, err := runProgram(program, args...); err == nil {
			t.Errorf("%q ran: %q", args, got)
		}
	}
}

func TestCommandWhoseValuesTouchIsNotOffered(t *testing.T) {
	type API struct {
		api.Specification
		Touch func(context.Context, string, string) (string, error) `cmdl:"touching %v%v"`
		Other func(context.Context) (string, error)                 `cmdl:"other"`
	}
	program := API{
		Touch: func(_ context.Context, a, b string) (string, error) { return a + "|" + b, nil },
		Other: func(context.Context) (string, error) { return "other", nil },
	}
	if got, err := runProgram(program, "touching", "ab"); err == nil {
		t.Errorf("no way to tell where the first value ends, yet it ran: %q", got)
	}
	help, err := runProgram(program)
	if err != nil || strings.Contains(help, "touching") || !strings.Contains(help, "other") {
		t.Errorf("help must only list what runs:\n%s\nerr=%v", help, err)
	}
}

func TestCommaThatIsNotTheJSONOptionIsACommandWord(t *testing.T) {
	type API struct {
		api.Specification

		List    func(context.Context) (string, error) `cmdl:"-o pid,comm"`
		Longer  func(context.Context) (string, error) `cmdl:"list,jsonish"`
		Version func(context.Context) (string, error) `cmdl:"version,json"`
	}
	program := API{
		List:    func(context.Context) (string, error) { return "listed", nil },
		Longer:  func(context.Context) (string, error) { return "longer", nil },
		Version: func(context.Context) (string, error) { return "1", nil },
	}
	for args, want := range map[string]string{
		"-o pid,comm":  "listed",
		"list,jsonish": "longer",
		"version":      `"1"`,
	} {
		if got, err := runProgram(program, strings.Fields(args)...); err != nil || got != want {
			t.Errorf("%q: got %q err=%v, want %q", args, got, err, want)
		}
	}
	for _, args := range []string{"-o pid", "list", "version,json"} {
		if got, err := runProgram(program, strings.Fields(args)...); err == nil {
			t.Errorf("%q ran: %q", args, got)
		}
	}
}

type CommonFlags struct {
	Verbose bool `cmdl:"-v"`
	Color   bool `cmdl:"--no-color,invert"`
}

type hiddenFlags struct {
	Name string `cmdl:"--name=%v"`
}

type DeepFlags struct {
	CommonFlags
	Depth int `cmdl:"--depth=%v"`
}

func TestEmbeddedFlags(t *testing.T) {
	type Opts struct {
		DeepFlags
		hiddenFlags
		Own string `cmdl:"--own=%v"`
	}
	type API struct {
		api.Specification
		Run func(context.Context, string, Opts) (string, error) `cmdl:"run %[2]v %[1]v"`
	}
	program := API{Run: func(_ context.Context, pos string, o Opts) (string, error) {
		return fmt.Sprintf("%s v=%v color=%v depth=%d name=%s own=%s", pos, o.Verbose, o.Color, o.Depth, o.Name, o.Own), nil
	}}
	for args, want := range map[string]string{
		"run x":                                 "x v=false color=true depth=0 name= own=",
		"run -v x":                              "x v=true color=true depth=0 name= own=",
		"run --no-color x":                      "x v=false color=false depth=0 name= own=",
		"run --depth=3 --name=ada --own=o -v x": "x v=true color=true depth=3 name=ada own=o",
		"run --own=o --name=ada --depth=3 x":    "x v=false color=true depth=3 name=ada own=o",
	} {
		if got, err := runProgram(program, strings.Fields(args)...); err != nil || got != want {
			t.Errorf("%q: got %q err=%v, want %q", args, got, err, want)
		}
	}
	if got, err := runProgram(program, "run", "--depth=3x", "x"); err == nil {
		t.Errorf("embedded values parse as strictly as any other: %q", got)
	}
}

func TestStructOfOnlyEmbeddedFlags(t *testing.T) {
	type Opts struct {
		CommonFlags
	}
	type API struct {
		api.Specification
		Run func(context.Context, Opts) (string, error) `cmdl:"run %v"`
	}
	program := API{Run: func(_ context.Context, o Opts) (string, error) {
		return fmt.Sprintf("v=%v color=%v", o.Verbose, o.Color), nil
	}}
	if got, err := runProgram(program, "run", "-v"); err != nil || got != "v=true color=true" {
		t.Errorf("got %q err=%v", got, err)
	}
	if got, err := runProgram(program, "run"); err != nil || got != "v=false color=true" {
		t.Errorf("got %q err=%v", got, err)
	}
}

func TestEmbeddedPointerFlags(t *testing.T) {
	type Opts struct {
		*CommonFlags
		Own string `cmdl:"--own=%v"`
	}
	type API struct {
		api.Specification
		Run func(context.Context, Opts) (string, error) `cmdl:"run %v"`
	}
	program := API{Run: func(_ context.Context, o Opts) (string, error) {
		return fmt.Sprintf("v=%v color=%v own=%s", o.Verbose, o.Color, o.Own), nil
	}}
	for args, want := range map[string]string{
		"run":            "v=false color=true own=",
		"run -v --own=o": "v=true color=true own=o",
		"run --no-color": "v=false color=false own=",
	} {
		if got, err := runProgram(program, strings.Fields(args)...); err != nil || got != want {
			t.Errorf("%q: got %q err=%v, want %q", args, got, err, want)
		}
	}
}

type LoopFlags struct {
	*LoopFlags
	Value int `cmdl:"--value=%v"`
}

func TestEmbeddingItselfDoesNotRecurseForever(t *testing.T) {
	type API struct {
		api.Specification
		Run func(context.Context, LoopFlags) (string, error) `cmdl:"run %v"`
	}
	program := API{Run: func(_ context.Context, o LoopFlags) (string, error) { return strconv.Itoa(o.Value), nil }}
	if got, err := runProgram(program, "run", "--value=7"); err != nil || got != "7" {
		t.Errorf("got %q err=%v", got, err)
	}
}

func TestEmbeddedValueWithItsOwnFlag(t *testing.T) {
	type Since struct {
		time.Time `cmdl:"--since=%v"`
	}
	type API struct {
		api.Specification
		Run func(context.Context, Since) (string, error) `cmdl:"run %v"`
	}
	program := API{Run: func(_ context.Context, o Since) (string, error) { return strconv.Itoa(o.Year()), nil }}
	if got, err := runProgram(program, "run", "--since=2024-01-02T03:04:05Z"); err != nil || got != "2024" {
		t.Errorf("got %q err=%v", got, err)
	}
}

func TestEmbeddedFlagsMatchALink(t *testing.T) {
	type Opts struct {
		Named  CommonFlags `cmdl:"--group"`
		Tagged struct {
			CommonFlags `cmdl:"--ignored"`
		}
	}
	type Skipped struct {
		CommonFlags `cmdl:"-"`
		Own         string `cmdl:"--own=%v"`
	}
	type API struct {
		api.Specification
		Run  func(context.Context, Opts) (string, error)    `cmdl:"run %v"`
		Skip func(context.Context, Skipped) (string, error) `cmdl:"skip %v"`
	}
	program := API{
		Run: func(_ context.Context, o Opts) (string, error) {
			return fmt.Sprintf("group=%v", o.Named.Verbose), nil
		},
		Skip: func(_ context.Context, o Skipped) (string, error) {
			return fmt.Sprintf("v=%v own=%s", o.Verbose, o.Own), nil
		},
	}
	if got, err := runProgram(program, "run", "--group", "-v"); err != nil || got != "group=true" {
		t.Errorf("a named group still takes its word: %q err=%v", got, err)
	}
	if got, err := runProgram(program, "skip", "--own=o"); err != nil || got != "v=false own=o" {
		t.Errorf("got %q err=%v", got, err)
	}
	if got, err := runProgram(program, "skip", "-v"); err == nil {
		t.Errorf("a field tagged - cannot be set: %q", got)
	}

	type Embedded struct {
		CommonFlags `cmdl:"--group"`
	}
	type Promoted struct {
		api.Specification
		Run func(context.Context, Embedded) (string, error) `cmdl:"run %v"`
	}
	promoted := Promoted{Run: func(_ context.Context, o Embedded) (string, error) {
		return fmt.Sprintf("v=%v", o.Verbose), nil
	}}
	if got, err := runProgram(promoted, "run", "-v"); err != nil || got != "v=true" {
		t.Errorf("a link passes an embedded struct's flags on their own: %q err=%v", got, err)
	}
}

func TestPercentInAFlagTagParsesBack(t *testing.T) {
	type Flags struct {
		Stamp  bool   `cmdl:"+%Y"`
		Escape bool   `cmdl:"+%%d"`
		Mixed  string `cmdl:"--f=%Y-%v"`
		Hex    int    `cmdl:"--hex=%x"`
		Pad    int    `cmdl:"--pad=%03d"`
		On     bool   `cmdl:"--on=%t"`
		Suffix string `cmdl:"--file=%v.txt"`
		Size   int    `cmdl:"-size %vMB"`
	}
	type API struct {
		api.Specification
		Run func(context.Context, Flags) (string, error) `cmdl:"run %v"`
	}
	program := API{Run: func(_ context.Context, f Flags) (string, error) {
		return fmt.Sprintf("%+v", f), nil
	}}
	const zero = "Stamp:false Escape:false Mixed: Hex:0 Pad:0 On:false Suffix: Size:0"
	replace := func(from, to string) string { return "{" + strings.Replace(zero, from, to, 1) + "}" }
	for args, want := range map[string]string{
		"run +%Y":          replace("Stamp:false", "Stamp:true"),
		"run +%d":          replace("Escape:false", "Escape:true"),
		"run --f=%Y-x":     replace("Mixed:", "Mixed:x"),
		"run --hex=ff":     replace("Hex:0", "Hex:255"),
		"run --pad=007":    replace("Pad:0", "Pad:7"),
		"run --on=true":    replace("On:false", "On:true"),
		"run --file=a.txt": replace("Suffix:", "Suffix:a"),
		"run -size 5MB":    replace("Size:0", "Size:5"),
	} {
		if got, err := runProgram(program, strings.Fields(args)...); err != nil || got != want {
			t.Errorf("%q: got %q err=%v, want %q", args, got, err, want)
		}
	}
	for _, args := range []string{"run +%M", "run --file=a.md", "run --f=x", "run --hex=ffzz", "run -size 5GB", "run +%%d"} {
		if got, err := runProgram(program, strings.Fields(args)...); err == nil {
			t.Errorf("%q ran: %q", args, got)
		}
	}

	type Text struct {
		Name string `cmdl:"-name %v.txt"`
		Tied string `cmdl:"ab%vba"`
	}
	type TextAPI struct {
		api.Specification
		Run func(context.Context, Text) (string, error) `cmdl:"run %v"`
	}
	text := TextAPI{Run: func(_ context.Context, f Text) (string, error) {
		return f.Name + "|" + f.Tied, nil
	}}
	for args, want := range map[string]string{"run -name a.txt": "a|", "run abxba": "|x", "run abba": "|"} {
		if got, err := runProgram(text, strings.Fields(args)...); err != nil || got != want {
			t.Errorf("%q: got %q err=%v, want %q", args, got, err, want)
		}
	}
	for _, args := range []string{"run -name a.md", "run aba"} {
		if got, err := runProgram(text, strings.Fields(args)...); err == nil {
			t.Errorf("%q ran: %q", args, got)
		}
	}
}

// typedFromHelp reads the help line of command the way someone at a terminal
// would, filling in each label, so a test can run exactly what help promises.
func typedFromHelp(t *testing.T, help, command string, fill *strings.Replacer) []string {
	t.Helper()
	for _, line := range strings.Split(help, "\n") {
		if words := strings.Fields(fill.Replace(line)); len(words) > 0 && words[0] == command {
			return words
		}
	}
	t.Fatalf("no help line for %s in:\n%s", command, help)
	return nil
}

func TestFollowingHelpLiterally(t *testing.T) {
	registerCmdlSource(t)
	abc := func(_ context.Context, a, b, c string) (string, error) {
		return fmt.Sprintf("a=%s b=%s c=%s", a, b, c), nil
	}
	program := helpOrderAPI{
		Copy: func(_ context.Context, src, dst string) (string, error) {
			return fmt.Sprintf("src=%s dst=%s", src, dst), nil
		},
		Pos: func(_ context.Context, name string, o helpOrderOptions) (string, error) {
			return fmt.Sprintf("name=%s flag=%v", name, o.Flag), nil
		},
		Mix: abc,
		Rel: abc,
		Join: func(_ context.Context, sep string, parts ...string) (string, error) {
			return fmt.Sprintf("sep=%s parts=%v", sep, parts), nil
		},
	}
	help, err := runProgram(program)
	if err != nil {
		t.Fatal(err)
	}
	fill := strings.NewReplacer(
		"<src>", "from", "<dst>", "to", "<name>", "Ada", "[--flag ...]", "--flag",
		"<a>", "1", "<b>", "2", "<c>", "3", "<sep>", "+", "<parts>", "p",
	)
	for command, want := range map[string]string{
		"copy": "src=from dst=to",
		"pos":  "name=Ada flag=true",
		"mix":  "a=1 b=2 c=3",
		"rel":  "a=1 b=2 c=3",
		"join": "sep=+ parts=[p]",
	} {
		typed := typedFromHelp(t, help, command, fill)
		if got, err := runProgram(program, typed...); err != nil || got != want {
			t.Errorf("%q from help: got %q err=%v, want %q\nhelp:\n%s", typed, got, err, want, help)
		}
	}
}
