package cmdl_test

import (
	"context"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/cmdl"
)

// mode is a string with a name for package fmt that is not its value.
type mode string

func (m mode) String() string { return "mode:" + string(m) }

// The text of a marshaler is asked for before its pointer is followed or its
// elements are passed one by one. A word that was only a placeholder used to
// pass a net.IP as an argument for each byte and a *big.Int as nothing.
func TestWholeWordPassesTheTextOfAMarshaler(t *testing.T) {
	type Fields struct {
		Verbose bool `cmdl:"-v"`
		Addr    net.IP
		Number  *big.Int
		Level   textLevel
	}
	A := api.Import[struct {
		api.Specification
		IP     func(context.Context, net.IP) (string, error)           `cmdl:"ip %v"`
		IPs    func(context.Context, []net.IP) (string, error)         `cmdl:"ips %v"`
		Many   func(context.Context, ...net.IP) (string, error)        `cmdl:"many %v"`
		Big    func(context.Context, *big.Int) (string, error)         `cmdl:"big %v"`
		Level  func(context.Context, textLevel) (string, error)        `cmdl:"level %v"`
		Lvlptr func(context.Context, *textLevel) (string, error)       `cmdl:"level %v"`
		Any    func(context.Context, any) (string, error)              `cmdl:"any %v"`
		At     func(context.Context, time.Time) (string, error)        `cmdl:"at %v"`
		Fields func(context.Context, Fields) (string, error)           `cmdl:"fields %v"`
		Inside func(context.Context, net.IP, *big.Int) (string, error) `cmdl:"in --ip=%v --n=%v"`
	}](cmdl.API, argvScript(t), nil)
	var (
		ctx   = context.Background()
		ip    = net.IPv4(127, 0, 0, 1).To4()
		other = net.ParseIP("::1")
		level = textLevel(1)
	)
	for want, call := range map[string]func() (string, error){
		"[ip][127.0.0.1]":              func() (string, error) { return A.IP(ctx, ip) },
		"[ip][]":                       func() (string, error) { return A.IP(ctx, nil) }, // the text of a nil net.IP is empty
		"[ips][127.0.0.1][::1]":        func() (string, error) { return A.IPs(ctx, []net.IP{ip, other}) },
		"[many][127.0.0.1][::1]":       func() (string, error) { return A.Many(ctx, ip, other) },
		"[many]":                       func() (string, error) { return A.Many(ctx) },
		"[big][42]":                    func() (string, error) { return A.Big(ctx, big.NewInt(42)) },
		"[big]":                        func() (string, error) { return A.Big(ctx, nil) },
		"[any][127.0.0.1]":             func() (string, error) { return A.Any(ctx, ip) },
		"[at][2024-01-02T03:04:05Z]":   func() (string, error) { return A.At(ctx, time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)) },
		"[in][--ip=127.0.0.1][--n=42]": func() (string, error) { return A.Inside(ctx, ip, big.NewInt(42)) },
		// A number with a text form has always been passed as the number.
		"[level][1]": func() (string, error) { return A.Level(ctx, level) },
		"[level][0]": func() (string, error) { return A.Lvlptr(ctx, new(textLevel)) },
		// Fields with no name have always passed an empty argument.
		"[fields][][][]":     func() (string, error) { return A.Fields(ctx, Fields{Addr: ip, Number: big.NewInt(42), Level: 1}) },
		"[fields][-v][][][]": func() (string, error) { return A.Fields(ctx, Fields{Verbose: true}) },
	} {
		if got, err := call(); err != nil || got != want {
			t.Errorf("got %q err=%v, want %q", got, err, want)
		}
	}
}

// joined is a list that says how it prints, in one piece.
// A link passes a slice parameter as an argument for each element, a []byte
// as a number for each byte, and prints a slice field whole into its one
// flag, as package fmt prints it. The commands that are linked today are
// passed exactly this, so it is kept, although a program of the same API
// reads a []byte as the text of one argument and a list of strings as a value
// for each time its flag is repeated.
func TestBytesAndListsArePassedAsTheyAlwaysWere(t *testing.T) {
	type Opts struct {
		Tags  []string `cmdl:"--tag=%v,omitempty"`
		Ports []int    `cmdl:"-p %v,omitempty"`
		Data  []byte   `cmdl:"--data=%v,omitempty"`
		Text  []byte   `cmdl:"--text=%s,omitempty"`
		Hex   []byte   `cmdl:"--hex=%x,omitempty"`
		Addr  net.IP   `cmdl:"--addr=%v,omitempty"`
		Kept  []string `cmdl:"--kept=%v"`
	}
	A := api.Import[struct {
		api.Specification
		Write   func(context.Context, []byte, string) (string, error) `cmdl:"write %v %v"`
		Chunks  func(context.Context, ...[]byte) (string, error)      `cmdl:"chunks %v"`
		Inside  func(context.Context, []byte) (string, error)         `cmdl:"in --data=%v"`
		Numbers func(context.Context, []int, string) (string, error)  `cmdl:"numbers %v %v"`
		Flags   func(context.Context, Opts) (string, error)           `cmdl:"flags %v"`
	}](cmdl.API, argvScript(t), nil)
	var (
		ctx  = context.Background()
		data = []byte("hi")
	)
	for want, call := range map[string]func() (string, error){
		"[write][104][105][name]":              func() (string, error) { return A.Write(ctx, data, "name") },
		"[write][name]":                        func() (string, error) { return A.Write(ctx, nil, "name") },
		"[chunks][97][98][99]":                 func() (string, error) { return A.Chunks(ctx, []byte("a"), []byte("bc")) },
		"[in][--data=[104 105]]":               func() (string, error) { return A.Inside(ctx, data) },
		"[numbers][1][2][name]":                func() (string, error) { return A.Numbers(ctx, []int{1, 2}, "name") },
		"[flags][--kept=[]]":                   func() (string, error) { return A.Flags(ctx, Opts{}) },
		"[flags][--tag=[a b]][--kept=[]]":      func() (string, error) { return A.Flags(ctx, Opts{Tags: []string{"a", "b"}}) },
		"[flags][--tag=[a]][--kept=[]]":        func() (string, error) { return A.Flags(ctx, Opts{Tags: []string{"a"}}) },
		"[flags][-p][[80 443]][--kept=[]]":     func() (string, error) { return A.Flags(ctx, Opts{Ports: []int{80, 443}}) },
		"[flags][--data=[104 105]][--kept=[]]": func() (string, error) { return A.Flags(ctx, Opts{Data: data}) },
		"[flags][--text=hi][--kept=[]]":        func() (string, error) { return A.Flags(ctx, Opts{Text: data}) },
		"[flags][--hex=6869][--kept=[]]":       func() (string, error) { return A.Flags(ctx, Opts{Hex: data}) },
		"[flags][--addr=10.0.0.1][--kept=[]]": func() (string, error) {
			return A.Flags(ctx, Opts{Addr: net.ParseIP("10.0.0.1")})
		},
		"[flags][--kept=[x y]]": func() (string, error) { return A.Flags(ctx, Opts{Kept: []string{"x", "y"}}) },
	} {
		if got, err := call(); err != nil || got != want {
			t.Errorf("got %q err=%v, want %q", got, err, want)
		}
	}
}

// A link inverts a bool, and passes a pointer to one as what it points to,
// 'invert' or not, as it always has. A program inverts the bool behind a
// pointer, so such a flag is not read back as it was passed: the link is kept
// as it is for the commands that are linked today.
func TestPointerToBoolIsPassedAsItPointsTo(t *testing.T) {
	type Opts struct {
		Plain   bool  `cmdl:"--plain=%v,invert"`
		Wrap    *bool `cmdl:"--wrap=%v,invert"`
		Letter  *bool `cmdl:"--letter=%t,invert"`
		Upright *bool `cmdl:"--upright=%v"`
	}
	A := api.Import[struct {
		api.Specification
		Run func(context.Context, Opts) (string, error) `cmdl:"run %v"`
	}](cmdl.API, argvScript(t), nil)
	yes, no := true, false
	for want, opts := range map[string]Opts{
		"[run][--plain=true]":                                 {},
		"[run][--plain=false][--wrap=true]":                   {Plain: true, Wrap: &yes},
		"[run][--plain=true][--wrap=false]":                   {Wrap: &no},
		"[run][--plain=true][--letter=true]":                  {Letter: &yes},
		"[run][--plain=true][--letter=false][--upright=true]": {Letter: &no, Upright: &yes},
	} {
		if got, err := A.Run(context.Background(), opts); err != nil || got != want {
			t.Errorf("got %q err=%v, want %q", got, err, want)
		}
	}
}

// A field tag is made safe for package fmt so that none of its error texts
// ends up in an argument. A word with a second verb still passed one,
// "--range=x:%!v(MISSING)". The value is printed for each verb of the word.
func TestEveryVerbOfAFlagWordPrintsTheValue(t *testing.T) {
	type Opts struct {
		Range   string   `cmdl:"--range=%v:%v,omitempty"`
		Twice   int      `cmdl:"-j%d -l%d,omitempty"`
		Shapes  int      `cmdl:"--n=%d/%x/%o,omitempty"`
		Indexed string   `cmdl:"--i=%[1]v+%[1]v,omitempty"`
		Mixed   string   `cmdl:"--m=%v+%[1]v,omitempty"`
		Percent int      `cmdl:"--p=%d%%of%d,omitempty"`
		Each    []string `cmdl:"--pair=%v=%v,omitempty"`
		Data    []byte   `cmdl:"--data=%v:%x,omitempty"`
		Pointer *int     `cmdl:"--ptr=%v-%v"`
	}
	A := api.Import[struct {
		api.Specification
		Run func(context.Context, Opts) (string, error) `cmdl:"run %v"`
	}](cmdl.API, argvScript(t), nil)
	seven := 7
	for want, opts := range map[string]Opts{
		"[run]":                         {},
		"[run][--range=x:x]":            {Range: "x"},
		"[run][-j4][-l4]":               {Twice: 4},
		"[run][--n=255/ff/377]":         {Shapes: 255},
		"[run][--i=x+x]":                {Indexed: "x"},
		"[run][--m=x+x]":                {Mixed: "x"},
		"[run][--p=5%of5]":              {Percent: 5},
		"[run][--pair=[a b]=[a b]]":     {Each: []string{"a", "b"}},
		"[run][--data=[104 105]:6869]":  {Data: []byte("hi")},
		"[run][--ptr=7-7]":              {Pointer: &seven},
		"[run][--range=a:a][--ptr=7-7]": {Range: "a", Pointer: &seven},
		"[run][--range=100%:100%]":      {Range: "100%"},
	} {
		if got, err := A.Run(context.Background(), opts); err != nil || got != want {
			t.Errorf("got %q err=%v, want %q", got, err, want)
		}
	}
}

// A program reads the flags of a group after the word that names it. A link
// used to pass the word and drop the flags. A nil group still passes its
// word, as it always has.
func TestNamedFlagGroupPassesItsFlags(t *testing.T) {
	type Group struct {
		Verbose bool   `cmdl:"-v"`
		Name    string `cmdl:"-name=%v,omitempty"`
		Colour  bool   `cmdl:"--no-colour,invert"`
	}
	type Opts struct {
		Quiet    bool   `cmdl:"-q"`
		Group    Group  `cmdl:"--group"`
		Optional *Group `cmdl:"--optional"`
		Omitted  Group  `cmdl:"--omitted,omitempty"`
		Words    *Group `cmdl:"remote add"`
		After    bool   `cmdl:"-a"`
	}
	A := api.Import[struct {
		api.Specification
		Run func(context.Context, Opts) (string, error) `cmdl:"%v"`
	}](cmdl.API, argvScript(t), nil)
	on := Group{Verbose: true, Name: "ada", Colour: true}
	for want, opts := range map[string]Opts{
		"[--group][--no-colour][--optional][remote][add]":                             {},
		"[-q][--group][-v][-name=ada][--optional][remote][add][-a]":                   {Quiet: true, Group: on, After: true},
		"[--group][--no-colour][--optional][--no-colour][remote][add]":                {Optional: &Group{}},
		"[--group][--no-colour][--optional][-v][-name=ada][remote][add]":              {Optional: &on},
		"[--group][--no-colour][--optional][--omitted][-v][--no-colour][remote][add]": {Omitted: Group{Verbose: true}},
		"[--group][--no-colour][--optional][remote][add][-name=x][--no-colour]":       {Words: &Group{Name: "x"}},
	} {
		if got, err := A.Run(context.Background(), opts); err != nil || got != want {
			t.Errorf("got %q err=%v, want %q", got, err, want)
		}
	}
}
