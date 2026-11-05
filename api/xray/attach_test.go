package xray_test

import (
	"context"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/api/xray"
)

func TestAttachVariadicFunction(t *testing.T) {
	v := struct {
		Join func(ctx context.Context, parts ...string) string
	}{
		Join: func(_ context.Context, parts ...string) string { return strings.Join(parts, ",") },
	}
	xray.Attach(&v)
	ctx := xray.NewContext(context.Background())
	if got := v.Join(ctx, "a", "b"); got != "a,b" {
		t.Errorf("got %q", got)
	}
	if got := v.Join(ctx); got != "" {
		t.Errorf("no variadic arguments: got %q", got)
	}
	if call := xray.ContextGet[xray.Call](ctx); call.Name != "Join" {
		t.Errorf("the call was not recorded: %+v", call)
	}
}

type cycle struct {
	Self *cycle
	Call func(context.Context)
}

func TestAttachFollowsPointerCycle(t *testing.T) {
	var calls int
	v := &cycle{Call: func(context.Context) { calls++ }}
	v.Self = v
	xray.Attach(v)

	ctx := xray.NewContext(context.Background())
	v.Call(ctx)
	if calls != 1 {
		t.Errorf("the function ran %d times", calls)
	}
	if call := xray.ContextGet[xray.Call](ctx); call.Name != "Call" {
		t.Errorf("the call was not recorded: %+v", call)
	}
}

// A function reachable by two paths is one function, recording it twice would
// leave a phantom call in the trace.
func TestAttachWrapsSharedFunctionOnce(t *testing.T) {
	type service struct {
		Call func(context.Context)
	}
	shared := &service{Call: func(context.Context) {}}
	v := struct{ A, B *service }{shared, shared}
	xray.Attach(&v)

	ctx := xray.NewContext(context.Background())
	v.A.Call(ctx)
	xray.ContextGet[xray.Call](ctx)
	if xray.ContextHas[xray.Call](ctx) {
		t.Error("the call was recorded twice")
	}
}

// A struct and its first field share an address, but both must be followed.
func TestAttachFollowsPointerToFirstField(t *testing.T) {
	type inner struct {
		Call func(context.Context)
	}
	type outer struct {
		Inner inner
		Tail  func(context.Context)
	}
	value := outer{Inner: inner{Call: func(context.Context) {}}, Tail: func(context.Context) {}}
	v := struct {
		First *inner
		Whole *outer
	}{&value.Inner, &value}
	xray.Attach(&v)

	ctx := xray.NewContext(context.Background())
	v.Whole.Tail(ctx)
	if call := xray.ContextGet[xray.Call](ctx); call.Name != "Tail" {
		t.Errorf("the second path was not followed: %+v", call)
	}
}
