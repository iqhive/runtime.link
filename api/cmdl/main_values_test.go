package cmdl_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/cmdl"
)

// A named placeholder is read by the type of its parameter. What is between
// its braces used to be taken for a format, which only a string got past.
func TestNamedPlaceholderOnANonString(t *testing.T) {
	registerCmdlSource(t)
	program := namedNumberAPI{Repeat: func(_ context.Context, times int, what string) (string, error) {
		return strings.Repeat(what, times), nil
	}}
	if got, err := runProgram(program, "repeat", "ab", "3"); err != nil || got != "ababab" {
		t.Errorf("got %q err=%v", got, err)
	}
	if got, err := runProgram(program, "repeat", "ab", "3x"); err == nil {
		t.Errorf("3x accepted as a number: %q", got)
	}
}

type envEmbedded struct {
	Region string `cmdl:"APP_REGION,env"`
}

// A pointer is set to what the flag says, as that is what a link passes for
// it. A *bool with no placeholder used to fail inside package fmt.
func TestPointerFlagsAreRead(t *testing.T) {
	type Group struct {
		Verbose bool `cmdl:"-v"`
	}
	type Opts struct {
		Force *bool    `cmdl:"--force"`
		Wrap  *bool    `cmdl:"--wrap=%v"`
		Trim  *bool    `cmdl:"--trim %t"`
		Count *int     `cmdl:"--count=%v"`
		Name  *string  `cmdl:"-name %v"`
		Label *string  `cmdl:"--label"`
		Group *Group   `cmdl:"--group"`
		Deep  **string `cmdl:"--deep=%v"`
	}
	type API struct {
		api.Specification
		Run func(context.Context, Opts) (string, error) `cmdl:"run %v"`
	}
	program := API{Run: func(_ context.Context, o Opts) (string, error) {
		var out []string
		if o.Force != nil {
			out = append(out, fmt.Sprint("force=", *o.Force))
		}
		if o.Wrap != nil {
			out = append(out, fmt.Sprint("wrap=", *o.Wrap))
		}
		if o.Trim != nil {
			out = append(out, fmt.Sprint("trim=", *o.Trim))
		}
		if o.Count != nil {
			out = append(out, fmt.Sprint("count=", *o.Count))
		}
		if o.Name != nil {
			out = append(out, "name="+*o.Name)
		}
		if o.Label != nil {
			out = append(out, "label="+*o.Label)
		}
		if o.Group != nil {
			out = append(out, fmt.Sprint("group=", o.Group.Verbose))
		}
		if o.Deep != nil {
			out = append(out, "deep="+**o.Deep)
		}
		return strings.Join(out, " "), nil
	}}
	for args, want := range map[string]string{
		"run":                    "",
		"run --force":            "force=true",
		"run --wrap=true":        "wrap=true",
		"run --wrap=false":       "wrap=false",
		"run --trim false":       "trim=false",
		"run --count=0":          "count=0",
		"run -name ada":          "name=ada",
		"run --label":            "label=--label",
		"run --group":            "group=false",
		"run --group -v":         "group=true",
		"run --deep=x":           "deep=x",
		"run --group -v --force": "force=true group=true",
	} {
		if got, err := runProgram(program, strings.Fields(args)...); err != nil || got != want {
			t.Errorf("%q: got %q err=%v, want %q", args, got, err, want)
		}
	}
	for _, args := range []string{"run --wrap=maybe", "run --wrap=", "run --count=x", "run -name", "run --trim"} {
		if got, err := runProgram(program, strings.Fields(args)...); err == nil {
			t.Errorf("%q ran: %q", args, got)
		}
	}
}

// A link writes the flags of what a pointer points to. A program did not see
// a pointer to a struct as flags at all.
func TestPointerToFlagsIsParsed(t *testing.T) {
	type Opts struct {
		Verbose bool   `cmdl:"-v"`
		Colour  bool   `cmdl:"--no-colour,invert"`
		Name    string `cmdl:"--name=%v"`
	}
	type API struct {
		api.Specification
		Run  func(context.Context, *Opts) (string, error)         `cmdl:"run %v"`
		Tail func(context.Context, *Opts, string) (string, error) `cmdl:"tail %v %v"`
		Head func(context.Context, string, *Opts) (string, error) `cmdl:"head %[2]v %[1]v"`
	}
	show := func(o *Opts) string {
		if o == nil {
			return "nil"
		}
		return fmt.Sprintf("v=%v colour=%v name=%s", o.Verbose, o.Colour, o.Name)
	}
	program := API{
		Run:  func(_ context.Context, o *Opts) (string, error) { return show(o), nil },
		Tail: func(_ context.Context, o *Opts, name string) (string, error) { return show(o) + " " + name, nil },
		Head: func(_ context.Context, name string, o *Opts) (string, error) { return show(o) + " " + name, nil },
	}
	for args, want := range map[string]string{
		"run":               "nil",
		"run -v":            "v=true colour=true name=",
		"run --no-colour":   "v=false colour=false name=",
		"run --name=ada -v": "v=true colour=true name=ada",
		"tail x":            "nil x",
		"tail -v x":         "v=true colour=true name= x",
		"head x":            "nil x",
		"head --name=ada x": "v=false colour=true name=ada x",
	} {
		if got, err := runProgram(program, strings.Fields(args)...); err != nil || got != want {
			t.Errorf("%q: got %q err=%v, want %q", args, got, err, want)
		}
	}
}

// A link repeats the flag of a slice for each element, and a program appends
// each one it reads. A []byte is one value, as is a type with a text form.
func TestSliceFlagTakesEachValue(t *testing.T) {
	type Opts struct {
		Tags   []string    `cmdl:"--tag=%v"`
		Ports  []int       `cmdl:"-p %v"`
		Levels []textLevel `cmdl:"--level=%v"`
		IDs    []checkedID `cmdl:"--id=%v"`
		Data   []byte      `cmdl:"--data=%v"`
		Hex    []byte      `cmdl:"--hex=%x"`
		Addr   net.IP      `cmdl:"--addr=%v"`
		Maybe  []*int      `cmdl:"--maybe=%v"`
	}
	type API struct {
		api.Specification
		Run func(context.Context, Opts) (string, error) `cmdl:"run %v"`
	}
	program := API{Run: func(_ context.Context, o Opts) (string, error) {
		maybe := make([]int, 0, len(o.Maybe))
		for _, n := range o.Maybe {
			maybe = append(maybe, *n)
		}
		return fmt.Sprintf("%q %v %v %v %q %q %v %v", o.Tags, o.Ports, o.Levels, o.IDs, o.Data, o.Hex, o.Addr, maybe), nil
	}}
	const none = `[] [] [] [] "" "" <nil> []`
	for _, test := range []struct {
		args []string
		want string
	}{
		{nil, none},
		{[]string{"--tag=a"}, `["a"] [] [] [] "" "" <nil> []`},
		{[]string{"--tag=a", "--tag=b c", "--tag="}, `["a" "b c" ""] [] [] [] "" "" <nil> []`},
		{[]string{"-p", "80", "--tag=a", "-p", "443"}, `["a"] [80 443] [] [] "" "" <nil> []`},
		{[]string{"--level=info", "--level=debug"}, `[] [] [1 0] [] "" "" <nil> []`},
		{[]string{"--id=id-1", "--id=id-2"}, `[] [] [] [id-1 id-2] "" "" <nil> []`},
		{[]string{"--data=two words"}, `[] [] [] [] "two words" "" <nil> []`},
		{[]string{"--data=a", "--data=b"}, `[] [] [] [] "b" "" <nil> []`},
		{[]string{"--hex=6869"}, `[] [] [] [] "" "hi" <nil> []`},
		{[]string{"--addr=10.0.0.1"}, `[] [] [] [] "" "" 10.0.0.1 []`},
		{[]string{"--maybe=1", "--maybe=2"}, `[] [] [] [] "" "" <nil> [1 2]`},
	} {
		if got, err := runProgram(program, append([]string{"run"}, test.args...)...); err != nil || got != test.want {
			t.Errorf("%q:\ngot  %q err=%v\nwant %q", test.args, got, err, test.want)
		}
	}
	for _, args := range []string{"run -p x", "run -p 1 -p", "run --level=loud", "run --id=bogus", "run --addr=nowhere", "run --hex=zz", "run --maybe=x"} {
		if got, err := runProgram(program, strings.Fields(args)...); err == nil {
			t.Errorf("%q ran: %q", args, got)
		}
	}
}

// A variadic parameter used to take strings and nothing else.
func TestVariadicValuesAreReadByTheirType(t *testing.T) {
	type API struct {
		api.Specification
		Sum    func(context.Context, ...int) (string, error)               `cmdl:"sum %v"`
		Levels func(context.Context, string, ...textLevel) (string, error) `cmdl:"levels %v %v"`
		Waits  func(context.Context, ...time.Duration) (string, error)     `cmdl:"waits %v"`
		Maybe  func(context.Context, ...*int) (string, error)              `cmdl:"maybe %v"`
		Chunks func(context.Context, ...[]byte) (string, error)            `cmdl:"chunks %v"`
		Join   func(context.Context, ...string) (string, error)            `cmdl:"join %v"`
	}
	var calls int
	program := API{
		Sum: func(_ context.Context, n ...int) (string, error) {
			calls++
			return fmt.Sprint(n), nil
		},
		Levels: func(_ context.Context, of string, levels ...textLevel) (string, error) {
			calls++
			return fmt.Sprint(of, levels), nil
		},
		Waits: func(_ context.Context, waits ...time.Duration) (string, error) { return fmt.Sprint(waits), nil },
		Maybe: func(_ context.Context, maybe ...*int) (string, error) {
			total := 0
			for _, n := range maybe {
				total += *n
			}
			return fmt.Sprint(total), nil
		},
		Chunks: func(_ context.Context, chunks ...[]byte) (string, error) { return fmt.Sprintf("%q", chunks), nil },
		Join:   func(_ context.Context, parts ...string) (string, error) { return fmt.Sprintf("%q", parts), nil },
	}
	for args, want := range map[string]string{
		"sum":                  "[]",
		"sum 1":                "[1]",
		"sum 1 -2 0x10":        "[1 -2 16]",
		"levels of":            "of[]",
		"levels of info debug": "of[1 0]",
		"waits 1.5s 2m 10":     "[1.5s 2m0s 10ns]",
		"maybe 1 2":            "3",
		"chunks a bc":          `["a" "bc"]`,
		"join a b":             `["a" "b"]`,
		"join":                 "[]",
	} {
		if got, err := runProgram(program, strings.Fields(args)...); err != nil || got != want {
			t.Errorf("%q: got %q err=%v, want %q", args, got, err, want)
		}
	}
	calls = 0
	for _, args := range []string{"sum 1 x", "sum 1.5", "levels of info loud", "waits soon", "maybe x"} {
		if got, err := runProgram(program, strings.Fields(args)...); err == nil {
			t.Errorf("%q ran: %q", args, got)
		}
	}
	if calls != 0 {
		t.Errorf("ran %d times with a value that does not parse", calls)
	}
	if got, err := runProgram(program, "join", "a", ""); err != nil || got != `["a" ""]` {
		t.Errorf("an empty argument is still a value: %q err=%v", got, err)
	}
}

// A []byte is read as the text of its argument. Any other slice has no one
// argument to be read from.
func TestBytesArgumentIsReadAsText(t *testing.T) {
	type API struct {
		api.Specification
		Write  func(context.Context, []byte) (string, error)     `cmdl:"write %v"`
		Named  func(context.Context, namedBytes) (string, error) `cmdl:"named %v"`
		Inside func(context.Context, []byte) (string, error)     `cmdl:"in --data=%v"`
		Addr   func(context.Context, net.IP) (string, error)     `cmdl:"addr %v"`
		Nums   func(context.Context, []int) (string, error)      `cmdl:"nums %v"`
	}
	program := API{
		Write:  func(_ context.Context, data []byte) (string, error) { return fmt.Sprintf("%q", data), nil },
		Named:  func(_ context.Context, data namedBytes) (string, error) { return fmt.Sprintf("%q", []byte(data)), nil },
		Inside: func(_ context.Context, data []byte) (string, error) { return fmt.Sprintf("%q", data), nil },
		Addr:   func(_ context.Context, ip net.IP) (string, error) { return ip.String(), nil },
		Nums:   func(_ context.Context, n []int) (string, error) { return fmt.Sprint(n), nil },
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"write", "hi"}, `"hi"`},
		{[]string{"write", "two words"}, `"two words"`},
		{[]string{"write", ""}, `""`},
		{[]string{"named", "hi"}, `"hi"`},
		{[]string{"in", "--data=hi"}, `"hi"`},
		{[]string{"addr", "10.0.0.1"}, "10.0.0.1"},
	} {
		if got, err := runProgram(program, test.args...); err != nil || got != test.want {
			t.Errorf("%q: got %q err=%v, want %q", test.args, got, err, test.want)
		}
	}
	if got, err := runProgram(program, "nums", "1"); err == nil || err.Error() != "cannot set []int to 1" {
		t.Errorf("nums 1: got %q err=%v", got, err)
	}
	for _, args := range []string{"addr nowhere"} {
		if got, err := runProgram(program, strings.Fields(args)...); err == nil {
			t.Errorf("%q ran: %q", args, got)
		}
	}
}

// A link passes a time.Duration as it prints, which a program used to refuse:
// it read any kind of int as a plain number.
func TestDurationIsReadAsItPrints(t *testing.T) {
	type timeout time.Duration
	type Opts struct {
		Timeout time.Duration  `cmdl:"--timeout=%v"`
		Wait    *time.Duration `cmdl:"--wait %v"`
		Nanos   time.Duration  `cmdl:"--nanos=%d"`
		Named   timeout        `cmdl:"--named=%v"`
	}
	type API struct {
		api.Specification
		Run  func(context.Context, Opts) (string, error)          `cmdl:"run %v"`
		Wait func(context.Context, time.Duration) (string, error) `cmdl:"wait %v"`
	}
	program := API{
		Run: func(_ context.Context, o Opts) (string, error) {
			wait := time.Duration(-1)
			if o.Wait != nil {
				wait = *o.Wait
			}
			return fmt.Sprint(o.Timeout, wait, o.Nanos, int64(o.Named)), nil
		},
		Wait: func(_ context.Context, d time.Duration) (string, error) { return d.String(), nil },
	}
	for args, want := range map[string]string{
		"run --timeout=1.5s":       "1.5s -1ns 0s 0",
		"run --timeout=1h2m3s":     "1h2m3s -1ns 0s 0",
		"run --timeout=-90s":       "-1m30s -1ns 0s 0",
		"run --timeout=0":          "0s -1ns 0s 0",
		"run --timeout=0s":         "0s -1ns 0s 0",
		"run --timeout=1500":       "1.5µs -1ns 0s 0",
		"run --wait 250ms":         "0s 250ms 0s 0",
		"run --nanos=1500":         "0s -1ns 1.5µs 0",
		"run --named=1500":         "0s -1ns 0s 1500",
		"wait 1.5s":                "1.5s",
		"wait 2562047h47m16.8s":    "2562047h47m16.8s",
		"wait 9223372036854775807": "2562047h47m16.854775807s",
	} {
		if got, err := runProgram(program, strings.Fields(args)...); err != nil || got != want {
			t.Errorf("%q: got %q err=%v, want %q", args, got, err, want)
		}
	}
	for _, args := range []string{
		"run --timeout=1.5",
		"run --timeout=soon",
		"run --timeout=1.5sx",
		"run --timeout=",
		"run --timeout= 1s",
		"run --wait",
		"run --nanos=1.5s",
		"run --named=1.5s",
		"wait 9223372036854775808",
		"wait 1e6h",
	} {
		if got, err := runProgram(program, strings.Fields(args)...); err == nil {
			t.Errorf("%q accepted as %q", args, got)
		}
	}
	if got, err := runProgram(program, "wait", " 1s"); err == nil {
		t.Errorf("a duration after a space was accepted as %q", got)
	}
}

type anything interface{}

// A link passes what an any holds as a word. A program takes that word for the
// name of a file to open, because a file implements any too, but without a
// file system to open it from, it once panicked: the any is given the word.
func TestAnyParameterIsGivenTheWord(t *testing.T) {
	type API struct {
		api.Specification
		Show   func(context.Context, any) (string, error)       `cmdl:"show %v"`
		Named  func(context.Context, anything) (string, error)  `cmdl:"named %v"`
		Inside func(context.Context, any, any) (string, error)  `cmdl:"inside --from=%v:%v"`
		All    func(context.Context, ...any) (string, error)    `cmdl:"all %v"`
		Read   func(context.Context, io.Reader) (string, error) `cmdl:"read %v"`
	}
	show := func(values ...any) (string, error) { return fmt.Sprintf("%#v", values), nil }
	program := API{
		Show:   func(_ context.Context, value any) (string, error) { return show(value) },
		Named:  func(_ context.Context, value anything) (string, error) { return show(value) },
		Inside: func(_ context.Context, from, to any) (string, error) { return show(from, to) },
		All:    func(_ context.Context, values ...any) (string, error) { return show(values...) },
		Read: func(_ context.Context, file io.Reader) (string, error) {
			data, err := io.ReadAll(file)
			return string(data), err
		},
	}
	files := map[string]fs.FS{
		"with the file":    fstest.MapFS{"notes": &fstest.MapFile{Data: []byte("secret")}},
		"without the file": fstest.MapFS{},
	}
	for args, want := range map[string]string{
		"show notes":        `[]interface {}{"notes"}`,
		"show 7":            `[]interface {}{"7"}`,
		"named notes":       `[]interface {}{"notes"}`,
		"inside --from=a:b": `[]interface {}{"a", "b"}`,
		"all notes other 7": `[]interface {}{"notes", "other", "7"}`,
		"all":               `[]interface {}{}`,
	} {
		out, err := cmdl.System{Args: append([]string{"prog"}, strings.Fields(args)...)}.Output(program)
		if got := strings.TrimSuffix(string(out), "\n"); err != nil || got != want {
			t.Errorf("no file system, %q: got %q err=%v, want %q", args, got, err, want)
		}
	}
	if _, err := (cmdl.System{Args: []string{"prog", "read", "notes"}}).Output(program); err == nil {
		t.Error("read without a file system: no error")
	}
	// An interface that asks for something a file can do is still given the
	// file that the word names.
	out, err := cmdl.System{Args: []string{"prog", "read", "notes"}, FS: files["with the file"]}.Output(program)
	if err != nil || string(out) != "secret\n" {
		t.Errorf("read notes: got %q err=%v", out, err)
	}
	if out, err := (cmdl.System{Args: []string{"prog", "read", "notes"}, FS: files["without the file"]}).Output(program); err == nil {
		t.Errorf("read of a file that is not there: got %q", out)
	}

	// What a link sends for an any is what the program is given.
	linked := api.Import[API](cmdl.API, argvScript(t), nil)
	argv, err := linked.Show(context.Background(), "notes")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := runProgram(program, sentArguments(argv)...); err != nil || got != `[]interface {}{"notes"}` {
		t.Errorf("the link sent %s, the program read %q err=%v", argv, got, err)
	}
}

type streamFields struct {
	In      io.Reader
	Out     io.Writer
	Log     io.Writer `cmdl:",stderr"`
	Verbose bool      `cmdl:"-v"`
}

type EmbeddedStreams struct {
	In  io.Reader
	Log io.Writer `cmdl:",stderr"`
}

// What a field is given, or "nil".
func given(streams ...any) string {
	var text []string
	for _, stream := range streams {
		switch stream := stream.(type) {
		case nil:
			text = append(text, "nil")
		case namedStream:
			text = append(text, string(stream))
		default:
			text = append(text, fmt.Sprintf("%T", stream))
		}
	}
	return strings.Join(text, " ")
}

type namedStream string

func (name namedStream) Read([]byte) (int, error)    { return 0, io.EOF }
func (name namedStream) Write(p []byte) (int, error) { return len(p), nil }

type onlyReader struct{ io.Reader }

// What a link does with the same fields, for the program above to agree with.
func TestStreamFieldsAgreeWithALink(t *testing.T) {
	linked := api.Import[struct {
		api.Specification
		Run func(context.Context, string, streamFields) (string, error) `cmdl:"-c %v %v"`
	}](cmdl.API, "sh", nil)
	var stdout, stderr bytes.Buffer
	got, err := linked.Run(context.Background(), `printf '[%s]' "$0"; tr a-z A-Z; echo read >&2`, streamFields{
		In: strings.NewReader("hello"), Out: &stdout, Log: &stderr, Verbose: true,
	})
	if err != nil || got != "[-v]HELLO" || stdout.String() != "[-v]HELLO" || stderr.String() != "read\n" {
		t.Errorf("got %q err=%v, with %q on its writer and %q on the one for stderr", got, err, stdout.String(), stderr.String())
	}
}

type verbFlags struct {
	Price   float64  `cmdl:"--price=%.2f,omitempty"`
	Offset  int      `cmdl:"--offset=%+d,omitempty"`
	Mask    int      `cmdl:"--mask=%#x,omitempty"`
	Upper   uint8    `cmdl:"--upper=%#X,omitempty"`
	Octal   int      `cmdl:"--octal=%#o,omitempty"`
	Binary  int      `cmdl:"--binary=%#b,omitempty"`
	Go      int      `cmdl:"--go=%O,omitempty"`
	Serial  int      `cmdl:"--serial=%05d,omitempty"`
	Left    int      `cmdl:"--left=%-6d,omitempty"`
	Science float64  `cmdl:"--science=%+.3e,omitempty"`
	Text    []byte   `cmdl:"--text=%s,omitempty"`
	Short   []byte   `cmdl:"--short=%.3s,omitempty"`
	Names   []string `cmdl:"--name=%5s,omitempty"`
	Hex     []byte   `cmdl:"--hex=%x,omitempty"`
	Count   int      `cmdl:"--count=%d,omitempty"`
	Next    int      `cmdl:"-n %+d,omitempty"`
	Syntax  int      `cmdl:"--syntax=%#v,omitempty"`
}

type verbAPI struct {
	api.Specification

	Run func(context.Context, verbFlags) (string, error) `cmdl:"run %v"`
}

// readVerbFlags runs a program with args and returns the flags it was given.
func readVerbFlags(args ...string) (verbFlags, error) {
	var read verbFlags
	program := verbAPI{Run: func(_ context.Context, flags verbFlags) (string, error) {
		read = flags
		return "", nil
	}}
	_, err := runProgram(program, append([]string{"run"}, args...)...)
	return read, err
}

// A field tag takes any verb of package fmt, and a link prints with all of
// it. A program used to scan with the same verb, and package fmt scans with
// no flag and no precision, and with a width for the most it will read, so
// that a program failed on what a link to the same API had written.
func TestVerbsThatALinkPrintsWithAreRead(t *testing.T) {
	for _, test := range []struct {
		args []string
		want verbFlags
	}{
		{[]string{"--price=1.50"}, verbFlags{Price: 1.5}},
		{[]string{"--price=2"}, verbFlags{Price: 2}},
		{[]string{"--offset=+3"}, verbFlags{Offset: 3}},
		{[]string{"--offset=-3"}, verbFlags{Offset: -3}},
		{[]string{"--offset=3"}, verbFlags{Offset: 3}},
		{[]string{"--mask=0xff"}, verbFlags{Mask: 255}},
		{[]string{"--mask=-0x10"}, verbFlags{Mask: -16}},
		{[]string{"--upper=0XFF"}, verbFlags{Upper: 255}},
		{[]string{"--octal=0377"}, verbFlags{Octal: 255}},
		{[]string{"--binary=0b101"}, verbFlags{Binary: 5}},
		{[]string{"--go=0o17"}, verbFlags{Go: 15}},
		{[]string{"--serial=00042"}, verbFlags{Serial: 42}},
		{[]string{"--serial=123456"}, verbFlags{Serial: 123456}},
		{[]string{"--left=42    "}, verbFlags{Left: 42}},
		{[]string{"--science=+1.500e+00"}, verbFlags{Science: 1.5}},
		{[]string{"--text=one"}, verbFlags{Text: []byte("one")}},
		{[]string{"--text=a b"}, verbFlags{Text: []byte("a b")}},
		{[]string{"--text="}, verbFlags{Text: []byte{}}},
		{[]string{"--short=abc"}, verbFlags{Short: []byte("abc")}},
		{[]string{"--name=ab", "--name=abcdefgh"}, verbFlags{Names: []string{"ab", "abcdefgh"}}},
		{[]string{"--hex=6869"}, verbFlags{Hex: []byte("hi")}},
		{[]string{"--count=12"}, verbFlags{Count: 12}},
		{[]string{"-n", "+7"}, verbFlags{Next: 7}},
		// The prefix of a base is what a link writes. Typed without it, a
		// number still reads in the base of its verb.
		{[]string{"--mask=ff"}, verbFlags{Mask: 255}},
		{[]string{"--mask=10"}, verbFlags{Mask: 16}},
		{[]string{"--binary=101"}, verbFlags{Binary: 5}},
		{[]string{"--go=17"}, verbFlags{Go: 15}},
		{[]string{"--octal=17"}, verbFlags{Octal: 15}},
	} {
		if got, err := readVerbFlags(test.args...); err != nil || !reflect.DeepEqual(got, test.want) {
			t.Errorf("%q: got %+v err=%v, want %+v", test.args, got, err, test.want)
		}
	}
	// A value is still read all of it, and by its verb.
	for _, arg := range []string{
		"--price=abc", "--price=1.5x", "--price=", "--offset=1x", "--offset=+", "--mask=0xzz", "--mask=zz", "--upper=0x100",
		"--octal=9", "--binary=102", "--go=0o9", "--serial=12abc", "--left=4 2", "--science=fast", "--hex=zz",
		"--count=0x10", "--count=+", "--syntax=3",
	} {
		if got, err := readVerbFlags(arg); err == nil {
			t.Errorf("%q was read as %+v", arg, got)
		}
	}
	if _, err := readVerbFlags("-n"); err == nil {
		t.Error("a flag with no value was read")
	}
}

// What a link prints for those flags is what a program reads back.
func TestVerbsAgreeWithALink(t *testing.T) {
	linked := api.Import[verbAPI](cmdl.API, argvScript(t), nil)
	for want, flags := range map[string]verbFlags{
		"[run][--price=1.50]":         {Price: 1.5},
		"[run][--offset=+3]":          {Offset: 3},
		"[run][--offset=-3]":          {Offset: -3},
		"[run][--mask=0xff]":          {Mask: 255},
		"[run][--mask=-0x10]":         {Mask: -16},
		"[run][--upper=0XFF]":         {Upper: 255},
		"[run][--octal=0377]":         {Octal: 255},
		"[run][--binary=0b101]":       {Binary: 5},
		"[run][--go=0o17]":            {Go: 15},
		"[run][--serial=00042]":       {Serial: 42},
		"[run][--serial=123456]":      {Serial: 123456},
		"[run][--left=42    ]":        {Left: 42},
		"[run][--science=+1.500e+00]": {Science: 1.5},
		"[run][--text=a b]":           {Text: []byte("a b")},
		"[run][--short=abc]":          {Short: []byte("abc")},
		"[run][--hex=6869]":           {Hex: []byte("hi")},
		"[run][--count=12]":           {Count: 12},
		"[run][-n][+7]":               {Next: 7},
	} {
		argv, err := linked.Run(context.Background(), flags)
		if err != nil || argv != want {
			t.Errorf("a link passed %s err=%v, want %s", argv, err, want)
			continue
		}
		if got, err := readVerbFlags(sentArguments(argv)[1:]...); err != nil || !reflect.DeepEqual(got, flags) {
			t.Errorf("%s: a program read %+v err=%v, want %+v", argv, got, err, flags)
		}
	}
}

// A variadic parameter takes an argument for each of its values, and an
// io.Reader is the file that its argument names, as it is for a parameter by
// itself. A program used to refuse it as a value that cannot be scanned.
func TestVariadicFilesAreOpened(t *testing.T) {
	type API struct {
		api.Specification
		Cat    func(context.Context, ...io.Reader) (string, error)             `cmdl:"cat %v"`
		Into   func(context.Context, string, ...io.ReadCloser) (string, error) `cmdl:"into %v %v"`
		Files  func(context.Context, ...fs.File) (string, error)               `cmdl:"files %v"`
		Words  func(context.Context, ...any) (string, error)                   `cmdl:"words %v"`
		Titles func(context.Context, ...fmt.Stringer) (string, error)          `cmdl:"titles %v"`
	}
	join := func(files []io.Reader) (string, error) {
		var text strings.Builder
		for _, file := range files {
			if _, err := io.Copy(&text, file); err != nil {
				return "", err
			}
		}
		return text.String(), nil
	}
	program := API{
		Cat: func(_ context.Context, files ...io.Reader) (string, error) { return join(files) },
		Into: func(_ context.Context, name string, files ...io.ReadCloser) (string, error) {
			readers := make([]io.Reader, len(files))
			for i, file := range files {
				readers[i] = file
			}
			text, err := join(readers)
			return name + ":" + text, err
		},
		Files: func(_ context.Context, files ...fs.File) (string, error) {
			readers := make([]io.Reader, len(files))
			for i, file := range files {
				readers[i] = file
			}
			return join(readers)
		},
		Words:  func(_ context.Context, words ...any) (string, error) { return fmt.Sprintf("%#v", words), nil },
		Titles: func(_ context.Context, titles ...fmt.Stringer) (string, error) { return fmt.Sprint(len(titles)), nil },
	}
	files := fstest.MapFS{
		"a.txt": &fstest.MapFile{Data: []byte("A")},
		"b.txt": &fstest.MapFile{Data: []byte("B")},
	}
	run := func(args ...string) (string, error) {
		out, err := cmdl.System{Args: append([]string{"prog"}, args...), FS: files}.Output(program)
		return strings.TrimSuffix(string(out), "\n"), err
	}
	for args, want := range map[string]string{
		"cat a.txt b.txt":      "AB",
		"cat b.txt":            "B",
		"cat":                  "",
		"into out a.txt b.txt": "out:AB",
		"files a.txt a.txt":    "AA",
		// An any is given the word, as it was, and not a file of that name.
		"words a.txt b.txt": `[]interface {}{"a.txt", "b.txt"}`,
	} {
		if got, err := run(strings.Fields(args)...); err != nil || got != want {
			t.Errorf("%q: got %q err=%v, want %q", args, got, err, want)
		}
	}
	if got, err := run("cat", "a.txt", "missing.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a file that is not there: got %q err=%v", got, err)
	}
	// An interface that no file fits is still a value that cannot be read.
	if got, err := run("titles", "a.txt"); err == nil {
		t.Errorf("titles ran: %q", got)
	}
}
