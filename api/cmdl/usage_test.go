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
			want: "pos [--flag ...] <name>",
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
			name: "unnamed args keep their place",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context, string, int)](),
				Args: []string{"", ""},
				Tags: `cmdl:"do %v %v"`,
			},
			want: "do <value> <value>",
		},
		{
			name: "empty tag has nothing to type",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context, string)](),
				Args: []string{"path"},
			},
			want: "",
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
			name: "tag order kept",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context, string, string)](),
				Args: []string{"src", "dst"},
				Tags: `cmdl:"copy %[2]v %[1]v"`,
			},
			want: "copy <dst> <src>",
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
			name: "placeholder inside a word",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context, string)](),
				Args: []string{"name"},
				Tags: `cmdl:"greet --name=%v"`,
			},
			want: "greet --name=<name>",
		},
		{
			name: "placeholder inside a word with no name",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context, string)](),
				Tags: `cmdl:"greet --name=%v"`,
			},
			want: "greet --name=<value>",
		},
		{
			name: "several placeholders inside a word",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context, int, int)](),
				Args: []string{"lo", "hi"},
				Tags: `cmdl:"span --range=%v-%v"`,
			},
			want: "span --range=<lo>-<hi>",
		},
		{
			name: "word after a placeholder",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context, string, string)](),
				Args: []string{"src", "dst"},
				Tags: `cmdl:"copy %v to %v"`,
			},
			want: "copy <src> to <dst>",
		},
		{
			name: "flags before a word",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context, opts)](),
				Args: []string{"options"},
				Tags: `cmdl:"%v status"`,
			},
			want: "[--flag ...] status",
		},
		{
			name: "percent that is not a placeholder",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context)](),
				Tags: `cmdl:"date +%Y"`,
			},
			want: "date +%Y",
		},
		{
			name: "escaped placeholder",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context)](),
				Tags: `cmdl:"escape %%v"`,
			},
			want: "escape %v",
		},
		{
			name: "parameter the tag never mentions",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context, string)](),
				Args: []string{"unused"},
				Tags: `cmdl:"run"`,
			},
			want: "run",
		},
		{
			name: "placeholder for a struct field",
			fn: api.Function{
				Type: reflect.TypeFor[func(context.Context, struct{ Name string })](),
				Args: []string{"options"},
				Tags: `cmdl:"greet {Name}"`,
			},
			want: "greet <value>",
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
