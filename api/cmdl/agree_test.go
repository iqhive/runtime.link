package cmdl_test

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/cmdl"
)

// textLevel is a number whose text form is a word, as a log level is.
type textLevel int

func (l textLevel) MarshalText() ([]byte, error) {
	if l < 0 || l > 1 {
		return nil, errors.New("bad level")
	}
	return []byte([]string{"debug", "info"}[l]), nil
}

func (l *textLevel) UnmarshalText(text []byte) error {
	switch string(text) {
	case "debug":
		*l = 0
	case "info":
		*l = 1
	default:
		return errors.New("bad level")
	}
	return nil
}

// checkedID is a string that checks itself when read from text, as the ids of
// the key packages do.
type checkedID string

func (id *checkedID) UnmarshalText(text []byte) error {
	if !strings.HasPrefix(string(text), "id-") {
		return errors.New("bad id")
	}
	*id = checkedID(text)
	return nil
}

// colour is a number with a name that only package fmt knows about.
type colour int

func (c colour) String() string { return []string{"red", "green"}[c] }

// sentArguments turns the output of argvScript back into the arguments it was
// given, for a test whose values have no "][" in them.
func sentArguments(argv string) []string {
	if argv == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(strings.TrimPrefix(argv, "["), "]"), "][")
}

type agreeGroup struct {
	Verbose bool   `cmdl:"-v"`
	Name    string `cmdl:"-name=%v,omitempty"`
}

type agreeFlags struct {
	Quiet    bool           `cmdl:"-q"`
	Force    *bool          `cmdl:"--force"`
	Wrap     *bool          `cmdl:"--wrap=%v"`
	Count    *int           `cmdl:"--count=%v"`
	Timeout  time.Duration  `cmdl:"--timeout=%v,omitempty"`
	Wait     *time.Duration `cmdl:"--wait %v"`
	Nanos    time.Duration  `cmdl:"--nanos=%d,omitempty"`
	Tags     []string       `cmdl:"--tag=%v"`
	Ports    []int          `cmdl:"-p %v"`
	Option   string         `cmdl:"-o key=%v,omitempty"`
	Other    string         `cmdl:"-o other=%v,omitempty"`
	Done     bool           `cmdl:"100% done"`
	Quoted   string         `cmdl:"--quoted=%q,omitempty"`
	Hex      string         `cmdl:"--hex=%x,omitempty"`
	Text     string         `cmdl:"--text=%s,omitempty"`
	Data     []byte         `cmdl:"--data=%v,omitempty"`
	Raw      []byte         `cmdl:"--raw=%x,omitempty"`
	Addr     net.IP         `cmdl:"--addr=%v,omitempty"`
	Big      *big.Int       `cmdl:"--big=%v"`
	Level    textLevel      `cmdl:"--level=%v,omitempty"`
	Levels   []textLevel    `cmdl:"--also=%v"`
	ID       checkedID      `cmdl:"--id=%v,omitempty"`
	Colour   colour         `cmdl:"--colour=%v,omitempty"`
	Ratio    float32        `cmdl:"--ratio=%v,omitempty"`
	Number   string         `cmdl:"-n%v,omitempty"`
	Name     string         `cmdl:"-name=%v,omitempty"`
	Group    agreeGroup     `cmdl:"--group,omitempty"`
	Optional *agreeGroup    `cmdl:"--optional"`
}

type agreeAPI struct {
	api.Specification

	Flags    func(context.Context, agreeFlags) (string, error)               `cmdl:"flags %v"`
	Pointer  func(context.Context, *agreeFlags) (string, error)              `cmdl:"pointer %v"`
	Get      func(context.Context, *string, string) (string, error)          `cmdl:"get --rev=%v %v"`
	Between  func(context.Context, *int, *int, string) (string, error)       `cmdl:"between --from=%v --to=%v %v"`
	Sum      func(context.Context, ...int) (string, error)                   `cmdl:"sum %v"`
	Levels   func(context.Context, string, ...textLevel) (string, error)     `cmdl:"levels %v %v"`
	Wait     func(context.Context, time.Duration) (string, error)            `cmdl:"wait %v"`
	Within   func(context.Context, time.Duration) (string, error)            `cmdl:"within --timeout=%v"`
	Write    func(context.Context, []byte) (string, error)                   `cmdl:"write %v"`
	Address  func(context.Context, net.IP, *big.Int) (string, error)         `cmdl:"address %v %v"`
	Level    func(context.Context, textLevel, checkedID) (string, error)     `cmdl:"level %v %v"`
	Paint    func(context.Context, colour, float32) (string, error)          `cmdl:"paint %v --ratio=%v"`
	Trailing func(context.Context, string, agreeFlags) (string, error)       `cmdl:"trailing %[2]v %[1]v"`
	Grouped  func(context.Context, agreeFlags, string) (string, error)       `cmdl:"grouped %v %v"`
	Nothing  func(context.Context, *agreeFlags, string) (string, error)      `cmdl:"nothing %v %v"`
	Pair     func(context.Context, *textLevel, string) (string, error)       `cmdl:"pair %v %v"`
	Reversed func(context.Context, string, *checkedID) (string, error)       `cmdl:"reversed --id=%[2]v %[1]v"`
	Addrs    func(context.Context, string, ...net.IP) (string, error)        `cmdl:"addrs %v %v"`
	Sizes    func(context.Context, string, ...time.Duration) (string, error) `cmdl:"sizes %v %v"`
}

// agree calls a function of the API linked to a script that echoes its
// arguments, and runs a program with those arguments that records what its
// own function of that name was called with. A link and a program that agree
// on how a value is spelled give back what went in.
func agree(t *testing.T, name string, in ...any) {
	t.Helper()
	linked := api.Import[agreeAPI](cmdl.API, argvScript(t), nil)
	call := []reflect.Value{reflect.ValueOf(context.Background())}
	for _, arg := range in {
		call = append(call, reflect.ValueOf(arg))
	}
	function := reflect.ValueOf(linked).FieldByName(name)
	// A nil argument has no type of its own, it takes the parameter's.
	for i := range call {
		if !call[i].IsValid() {
			call[i] = reflect.Zero(function.Type().In(min(i, function.Type().NumIn()-1)))
		}
	}
	out := function.Call(call)
	if err, _ := out[1].Interface().(error); err != nil {
		t.Errorf("%s%v: link: %v", name, in, err)
		return
	}
	argv := out[0].String()

	var (
		program agreeAPI
		got     = []any{}
	)
	target := reflect.ValueOf(&program).Elem().FieldByName(name)
	target.Set(reflect.MakeFunc(target.Type(), func(args []reflect.Value) []reflect.Value {
		for _, arg := range args[1:] {
			if arg.Kind() != reflect.Slice || !target.Type().IsVariadic() || arg.Type().Elem().Kind() == reflect.Uint8 {
				got = append(got, arg.Interface())
				continue
			}
			for i := 0; i < arg.Len(); i++ {
				got = append(got, arg.Index(i).Interface())
			}
		}
		return []reflect.Value{reflect.ValueOf(""), reflect.Zero(reflect.TypeFor[error]())}
	}))
	if _, err := runProgram(program, sentArguments(argv)...); err != nil {
		t.Errorf("%s: the program refuses what the link sent, %s: %v", name, argv, err)
		return
	}
	want := make([]any, len(in))
	for i := range in {
		want[i] = call[i+1].Interface()
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: the link sent %s\nthe program read %s\nwant             %s", name, argv, spew(got), spew(want))
	}
}

// spew prints what is behind the pointers of values, which %+v leaves as
// addresses.
func spew(values []any) string {
	var text strings.Builder
	for _, value := range values {
		writeValue(&text, reflect.ValueOf(value))
		text.WriteByte(' ')
	}
	return text.String()
}

func writeValue(text *strings.Builder, value reflect.Value) {
	switch {
	case !value.IsValid():
		text.WriteString("nil")
	case value.Kind() == reflect.Pointer && value.IsNil():
		text.WriteString("nil")
	case value.Type() == reflect.TypeFor[*big.Int]():
		fmt.Fprint(text, value.Interface())
	case value.Kind() == reflect.Pointer:
		text.WriteByte('&')
		writeValue(text, value.Elem())
	case value.Kind() == reflect.Struct:
		text.WriteByte('{')
		for i := 0; i < value.NumField(); i++ {
			if value.Field(i).IsZero() {
				continue
			}
			text.WriteString(value.Type().Field(i).Name + ":")
			writeValue(text, value.Field(i))
			text.WriteByte(' ')
		}
		text.WriteByte('}')
	default:
		fmt.Fprintf(text, "%#v", value.Interface())
	}
}

type numberLists struct {
	Ports  []int           `cmdl:"-p %v,omitempty"`
	Hex    []int           `cmdl:"--hex=%x,omitempty"`
	Ratios []float64       `cmdl:"--ratio=%v,omitempty"`
	Wide   []int           `cmdl:"--wide=%5d,omitempty"`
	Flags  []bool          `cmdl:"--flag=%v,omitempty"`
	Waits  []time.Duration `cmdl:"--wait=%v,omitempty"`
	Kept   []int           `cmdl:"--kept=%v"`
	Tags   []string        `cmdl:"--tag=%v,omitempty"`
	Maybe  []*int          `cmdl:"--maybe=%v,omitempty"`
}

type numberListsAPI struct {
	api.Specification

	Run func(context.Context, numberLists) (string, error) `cmdl:"run %v"`
}

// describeNumbers prints flags with a nil list like an empty one, as that is
// all an argument can say, and each string quoted, to tell one from two.
func describeNumbers(flags numberLists) string {
	return fmt.Sprintf("ports=%v hex=%v ratios=%v wide=%v flags=%v waits=%v kept=%v tags=%q maybe=%d",
		flags.Ports, flags.Hex, flags.Ratios, flags.Wide, flags.Flags, flags.Waits, flags.Kept, flags.Tags, len(flags.Maybe))
}

// readNumberLists runs a program with args and returns the flags it was given.
func readNumberLists(args ...string) (numberLists, error) {
	var read numberLists
	program := numberListsAPI{Run: func(_ context.Context, flags numberLists) (string, error) {
		read = flags
		return "", nil
	}}
	_, err := runProgram(program, args...)
	return read, err
}

// A link prints a whole list into one flag, '-p [80 443]'. A program takes a
// value each time a flag is repeated, and refused the list of a link as no
// number. It reads a list of numbers or bools, which cannot be anything else.
func TestProgramReadsTheListsOfNumbersThatALinkWrites(t *testing.T) {
	linked := api.Import[numberListsAPI](cmdl.API, argvScript(t), nil)
	for want, flags := range map[string]numberLists{
		"[run][--kept=[]]":                         {},
		"[run][-p][[80 443]][--kept=[]]":           {Ports: []int{80, 443}},
		"[run][-p][[80]][--kept=[]]":               {Ports: []int{80}},
		"[run][--hex=[ff 10]][--kept=[]]":          {Hex: []int{255, 16}},
		"[run][--ratio=[1.5 -2 1e+21]][--kept=[]]": {Ratios: []float64{1.5, -2, 1e21}},
		"[run][--wide=[   80   443]][--kept=[]]":   {Wide: []int{80, 443}},
		"[run][--flag=[true false]][--kept=[]]":    {Flags: []bool{true, false}},
		"[run][--wait=[1.5s 2m0s]][--kept=[]]":     {Waits: []time.Duration{1500 * time.Millisecond, 2 * time.Minute}},
		"[run][--kept=[1 2 3]]":                    {Kept: []int{1, 2, 3}},
	} {
		argv, err := linked.Run(context.Background(), flags)
		if err != nil || argv != want {
			t.Errorf("a link passed %s err=%v, want %s", argv, err, want)
			continue
		}
		got, err := readNumberLists(sentArguments(argv)...)
		if err != nil || describeNumbers(got) != describeNumbers(flags) {
			t.Errorf("%s: a program read %+v err=%v, want %+v", argv, got, err, flags)
		}
	}
}

// What a program read before, it reads as it did: a value each time a flag is
// repeated, a list of strings in brackets as the one string it is, and
// anything in brackets that is not a list of numbers as an error.
func TestPrintedListLeavesOtherArgumentsAsTheyWere(t *testing.T) {
	for _, test := range []struct {
		args []string
		want numberLists
	}{
		{[]string{"run", "-p", "80", "-p", "443"}, numberLists{Ports: []int{80, 443}}},
		{[]string{"run", "-p", "80", "-p", "[443 8080]"}, numberLists{Ports: []int{80, 443, 8080}}},
		{[]string{"run", "--tag=[a b]"}, numberLists{Tags: []string{"[a b]"}}},
		{[]string{"run", "--tag=[]", "--tag=x"}, numberLists{Tags: []string{"[]", "x"}}},
		{[]string{"run", "--kept=[]"}, numberLists{Kept: []int{}}},
	} {
		if got, err := readNumberLists(test.args...); err != nil || describeNumbers(got) != describeNumbers(test.want) {
			t.Errorf("%q: got %+v err=%v, want %+v", test.args, got, err, test.want)
		}
	}
	for _, args := range [][]string{
		{"run", "-p", "[80 x]"}, {"run", "-p", "[80"}, {"run", "-p", "80]"}, {"run", "-p", "80 443"}, {"run", "-p", "[1.5]"},
		{"run", "--flag=[yes]"}, {"run", "--maybe=[1 2]"}, {"run", "--hex=[zz]"},
	} {
		if got, err := readNumberLists(args...); err == nil {
			t.Errorf("%q was read as %+v", args, got)
		}
	}
}
