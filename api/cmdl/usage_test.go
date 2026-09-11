package cmdl

import (
	"context"
	"reflect"
	"testing"

	"github.com/iqhive/runtime.link/api"
)

func TestUsageLine(t *testing.T) {
	t.Parallel()
	type opts struct {
		Flag bool `cmdl:"--flag"`
	}
	tests := []struct {
		name string
		fn   api.Function
		want string
	}{
		{
			name: "positional and flags",
			fn: api.Function{
				Type: reflect.TypeFor[func(ctx context.Context, name string, options opts)](),
				Args: []string{"name", "options"},
				Tags: `cmdl:"pos %[2]v %[1]v"`,
			},
			want: "pos <name> [--flag ...]",
		},
		{
			name: "literal only",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context)](),
				Tags: `cmdl:"something"`,
			},
			want: "something",
		},
		{
			name: "unnamed args omitted",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context, string, int)](),
				Args: []string{"", ""},
				Tags: `cmdl:"do %v %v"`,
			},
			want: "do",
		},
		{
			name: "empty tag",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context, string)](),
				Args: []string{"path"},
			},
			want: "<path>",
		},
		{
			name: "comma flags stripped from tag",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context)](),
				Tags: `cmdl:"run,json"`,
			},
			want: "run",
		},
		{
			name: "tag order ignored",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context, string, string)](),
				Args: []string{"src", "dst"},
				Tags: `cmdl:"copy %[2]v %[1]v"`,
			},
			want: "copy <src> <dst>",
		},
		{
			name: "flags only",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context, struct {
					Flag bool `cmdl:"--flag"`
				})](),
				Args: []string{"options"},
				Tags: `cmdl:"%v"`,
			},
			want: "[--flag ...]",
		},
		{
			name: "variadic named",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context, string, ...string)](),
				Args: []string{"sep", "parts"},
				Tags: `cmdl:"join %v %v"`,
			},
			want: "join <sep> <parts>",
		},
		{
			name: "no tag no names",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context, string)](),
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := usageLine(tt.fn); got != tt.want {
				t.Errorf("usageLine = %q, want %q", got, tt.want)
			}
		})
	}
}
