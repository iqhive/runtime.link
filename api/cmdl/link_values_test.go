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
