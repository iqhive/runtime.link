package api

import (
	"context"
	"io/fs"
	"reflect"
	"runtime"
	"testing"
	"testing/fstest"
)

type onlyCtxAPI struct {
	Ping func(ctx context.Context)
}

type onlyErrAPI struct {
	Fail func() error
}

type noResultAPI struct {
	Fire func()
}

type deepInline struct {
	Outer struct {
		Inner struct {
			Hello func(name string)
		}
	}
}

type embedHost struct {
	nestedAPI
}

type parenFuncAPI struct {
	Hello func(y int)
}

type aliasFunc func(a int)

type aliasFieldAPI struct {
	Hello aliasFunc
}

type nilSourceAPI struct {
	Hello func(name string)
}

func (nilSourceAPI) Source() fs.FS { return nil }

type explodingFS struct{}

func (explodingFS) Open(string) (fs.File, error) { panic("boom") }

type explodingSourceAPI struct {
	Hello func(name string)
}

func (explodingSourceAPI) Source() fs.FS { return explodingFS{} }

func TestOnlyContextAndError(t *testing.T) {
	useSource(t, srcFile(`
type onlyCtxAPI struct { Ping func(ctx context.Context) }
type onlyErrAPI struct { Fail func() error }
type noResultAPI struct { Fire func() }
`))
	ping := mustFn(t, &onlyCtxAPI{}, "Ping")
	if ping.Args != nil || ping.Outs != nil {
		t.Errorf("Ping Args=%q Outs=%q", ping.Args, ping.Outs)
	}
	if !ping.Named() {
		t.Error("zero-arg Ping should be Named")
	}
	fail := mustFn(t, &onlyErrAPI{}, "Fail")
	if fail.Args != nil || fail.Outs != nil {
		t.Errorf("Fail Args=%q Outs=%q", fail.Args, fail.Outs)
	}
	fire := mustFn(t, &noResultAPI{}, "Fire")
	if fire.Args != nil || fire.Outs != nil {
		t.Errorf("Fire Args=%q Outs=%q", fire.Args, fire.Outs)
	}
}

func TestDeepInlineNamespace(t *testing.T) {
	useSource(t, srcFile(`
type deepInline struct {
	Outer struct {
		Inner struct {
			Hello func(name string)
		}
	}
}
`))
	s := StructureOf(&deepInline{})
	inner := s.Namespace["Outer"].Namespace["Inner"]
	if len(inner.Functions) != 1 || !reflect.DeepEqual(inner.Functions[0].Args, []string{"name"}) {
		t.Errorf("deep inline: %+v", inner.Functions)
	}
}

func TestEmbeddedNamespace(t *testing.T) {
	useSource(t, srcFile(`
type nestedAPI struct { Hello func(name string) }
type embedHost struct { nestedAPI }
`))
	s := StructureOf(&embedHost{})
	child := s.Namespace["nestedAPI"]
	if len(child.Functions) != 1 || child.Functions[0].InName(0) != "name" {
		t.Errorf("embedded namespace: %+v", child.Functions)
	}
}

func TestParenthesizedAndAliasFuncTypes(t *testing.T) {
	useSource(t, srcFile(`
type aliasFunc func(a int)
type parenFuncAPI struct {
	Hello func(y int)
}
type aliasFieldAPI struct {
	Hello aliasFunc
}
`))
	if got := mustFn(t, &parenFuncAPI{}, "Hello").Args; !reflect.DeepEqual(got, []string{"y"}) {
		t.Errorf("parens Args=%q", got)
	}
	if got := mustFn(t, &aliasFieldAPI{}, "Hello").Args; !reflect.DeepEqual(got, []string{"a"}) {
		t.Errorf("alias Args=%q", got)
	}
}

func TestFunctionCopyPreservesNames(t *testing.T) {
	useSource(t, srcFile(`
type namedParams struct {
	Hello func(ctx context.Context, petId int, file string) (url string, err error)
}
`))
	var spec namedParams
	spec.Hello = func(context.Context, int, string) (string, error) { return "", nil }
	fn := mustFn(t, &spec, "Hello")
	cp := fn.Copy()
	if !reflect.DeepEqual(cp.Args, fn.Args) || !reflect.DeepEqual(cp.Outs, fn.Outs) {
		t.Errorf("Copy Args=%q Outs=%q", cp.Args, cp.Outs)
	}
	if !cp.Named() {
		t.Error("copy should stay Named")
	}
}

func TestStructureOfStructurePreservesNames(t *testing.T) {
	useSource(t, srcFile(`
type groupedParams struct {
	Add func(a, b int) int
}
`))
	s := StructureOf(&groupedParams{})
	again := StructureOf(s)
	if !reflect.DeepEqual(again.Functions[0].Args, []string{"a", "b"}) {
		t.Errorf("Structure passthrough Args=%q", again.Functions[0].Args)
	}
}

func TestIterCarriesNames(t *testing.T) {
	useSource(t, srcFile(`
type hostWithNested struct { Nested nestedAPI }
type nestedAPI struct { Hello func(name string) }
`))
	var found []string
	for fn := range StructureOf(&hostWithNested{}).Iter() {
		found = append(found, fn.Name+":"+fn.InName(0))
	}
	if !reflect.DeepEqual(found, []string{"Hello:name"}) {
		t.Errorf("Iter = %q", found)
	}
}

func TestRegisterSourcePointerAndFirstWins(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	RegisterSource[*namedParams](srcFile(`
type namedParams struct {
	Hello func(ctx context.Context, petId int, file string) (url string, err error)
}
`))
	if mustFn(t, &namedParams{}, "Hello").InName(0) != "petId" {
		t.Fatal("pointer RegisterSource should key by package")
	}
	RegisterSource[namedParams](srcFile(`
type namedParams struct {
	Hello func(wrong int)
}
`))
	if mustFn(t, &namedParams{}, "Hello").InName(0) != "petId" {
		t.Fatal("second RegisterSource must be a no-op")
	}
}

func TestRegisterSourceNilIsNoop(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	RegisterSource[namedParams](nil)
	if mustFn(t, &namedParams{}, "Hello").Args != nil {
		t.Error("nil FS should not invent names")
	}
}

func TestNilSourceMethod(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	if mustFn(t, nilSourceAPI{}, "Hello").Args != nil {
		t.Error("nil Source() should degrade to unknown names")
	}
}

func TestParsePanicDoesNotEscape(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	var fn Function
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				t.Fatalf("StructureOf panicked: %v", rec)
			}
		}()
		fn = mustFn(t, explodingSourceAPI{}, "Hello")
	}()
	if fn.Args != nil {
		t.Errorf("panic during parse should drop names, Args=%q", fn.Args)
	}
}

func TestWithSourcePointerReceiverOnValue(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	fn := mustFn(t, pointerSourceAPI{}, "Hello")
	if !reflect.DeepEqual(fn.Args, []string{"name"}) {
		t.Errorf("value of pointer-receiver type Args=%q", fn.Args)
	}
}

func TestFilenameSuffixSelectsOS(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	files := fstest.MapFS{
		"x_windows.go":              {Data: []byte("package api\ntype constrainedAPI struct { Hello func(windows string) }")},
		"x_" + runtime.GOOS + ".go": {Data: []byte("package api\ntype constrainedAPI struct { Hello func(" + runtime.GOOS + " string) }")},
	}
	RegisterSource[Structure](files)
	fn := mustFn(t, &constrainedAPI{}, "Hello")
	if !reflect.DeepEqual(fn.Args, []string{runtime.GOOS}) {
		t.Errorf("Args = %q, want %q", fn.Args, []string{runtime.GOOS})
	}
}
