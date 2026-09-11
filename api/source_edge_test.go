package api

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"testing/fstest"
)

type namedCopy namedParams

type namedAlias = namedParams

type aliasEquals = myFunc

type aliasEqualsAPI struct {
	Hello aliasEquals
}

type hiddenAPI struct {
	hello func(secret string)
}

type mixedFieldsAPI struct {
	Do    any
	Ch    chan int
	Hello func(name string)
}

type myErr struct{}

func (myErr) Error() string { return "x" }

type customErrAPI struct {
	Fail func() (n int, err myErr)
}

type errMidAPI struct {
	Swap func() (err error, n int)
}

type variadicCtxAPI struct {
	Join func(ctx context.Context, sep string, parts ...string) string
}

type denyFileFS struct {
	inner fstest.MapFS
	deny  string
}

func (d denyFileFS) Open(name string) (fs.File, error) {
	if name == d.deny {
		return nil, os.ErrPermission
	}
	return d.inner.Open(name)
}

type failRootFS struct{}

func (failRootFS) Open(string) (fs.File, error) { return nil, os.ErrPermission }

func TestDefinedTypeHasNoNames(t *testing.T) {
	useSource(t, srcFile(`
type namedParams struct {
	Hello func(ctx context.Context, petId int, file string) (url string, err error)
}
`))
	fn := mustFn(t, &namedCopy{}, "Hello")
	if fn.Args != nil || fn.Outs != nil {
		t.Errorf("defined copy type should not inherit AST names, Args=%q Outs=%q", fn.Args, fn.Outs)
	}
}

func TestTypeAliasUsesUnderlyingName(t *testing.T) {
	useSource(t, srcFile(`
type namedParams struct {
	Hello func(ctx context.Context, petId int, file string) (url string, err error)
}
`))
	if reflect.TypeOf(namedAlias{}).Name() != "namedParams" {
		t.Fatalf("alias Name() = %q", reflect.TypeOf(namedAlias{}).Name())
	}
	fn := mustFn(t, &namedAlias{}, "Hello")
	if !reflect.DeepEqual(fn.Args, []string{"petId", "file"}) {
		t.Errorf("alias Args=%q", fn.Args)
	}
}

func TestFuncAliasEqualsDoesNotFollowIdent(t *testing.T) {
	useSource(t, srcFile(`
type myFunc func(a int)
type aliasEquals = myFunc
type aliasEqualsAPI struct {
	Hello aliasEquals
}
`))
	fn := mustFn(t, &aliasEqualsAPI{}, "Hello")
	if fn.Args != nil {
		t.Errorf("type T = otherFunc is not followed, Args=%q", fn.Args)
	}
}

func TestUnexportedFuncFieldNames(t *testing.T) {
	useSource(t, srcFile(`
type hiddenAPI struct {
	hello func(secret string)
}
`))
	fn := mustFn(t, &hiddenAPI{}, "hello")
	if !reflect.DeepEqual(fn.Args, []string{"secret"}) {
		t.Errorf("unexported field Args=%q", fn.Args)
	}
}

func TestMixedNonFuncFields(t *testing.T) {
	useSource(t, srcFile(`
type mixedFieldsAPI struct {
	Do    any
	Ch    chan int
	Hello func(name string)
}
`))
	s := StructureOf(&mixedFieldsAPI{})
	if len(s.Functions) != 1 {
		t.Fatalf("got %d functions", len(s.Functions))
	}
	if !reflect.DeepEqual(s.Functions[0].Args, []string{"name"}) {
		t.Errorf("Hello Args=%q", s.Functions[0].Args)
	}
}

func TestCustomErrorTypeStripped(t *testing.T) {
	useSource(t, srcFile(`
type myErr struct{}
type customErrAPI struct {
	Fail func() (n int, err myErr)
}
`))
	fn := mustFn(t, &customErrAPI{}, "Fail")
	if !reflect.DeepEqual(fn.Outs, []string{"n"}) || fn.Args != nil {
		t.Errorf("custom error Outs=%q Args=%q", fn.Outs, fn.Args)
	}
}

func TestErrorNotLastIsKept(t *testing.T) {
	useSource(t, srcFile(`
type errMidAPI struct {
	Swap func() (err error, n int)
}
`))
	fn := mustFn(t, &errMidAPI{}, "Swap")
	if !reflect.DeepEqual(fn.Outs, []string{"err", "n"}) {
		t.Errorf("error-not-last Outs=%q", fn.Outs)
	}
}

func TestVariadicWithContext(t *testing.T) {
	useSource(t, srcFile(`
type variadicCtxAPI struct {
	Join func(ctx context.Context, sep string, parts ...string) string
}
`))
	fn := mustFn(t, &variadicCtxAPI{}, "Join")
	if !reflect.DeepEqual(fn.Args, []string{"sep", "parts"}) {
		t.Errorf("Args=%q", fn.Args)
	}
}

func TestLastEnabledFileWins(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	RegisterSource[Structure](fstest.MapFS{
		"a.go": {Data: []byte("package api\ntype groupedParams struct { Add func(first int) }")},
		"z.go": {Data: []byte("package api\ntype groupedParams struct { Add func(a, b int) int }")},
	})
	fn := mustFn(t, &groupedParams{}, "Add")
	if !reflect.DeepEqual(fn.Args, []string{"a", "b"}) {
		t.Errorf("last file should win, Args=%q", fn.Args)
	}
}

func TestUnreadableFileSkipped(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	RegisterSource[Structure](denyFileFS{
		deny: "denied.go",
		inner: fstest.MapFS{
			"denied.go": {Data: []byte("package api\ntype groupedParams struct { Add func(wrong int) }")},
			"good.go":   {Data: []byte("package api\ntype groupedParams struct { Add func(a, b int) int }")},
		},
	})
	fn := mustFn(t, &groupedParams{}, "Add")
	if !reflect.DeepEqual(fn.Args, []string{"a", "b"}) {
		t.Errorf("denied file should be skipped, Args=%q", fn.Args)
	}
}

func TestParsePackageWalkError(t *testing.T) {
	t.Parallel()
	src := parsePackage(failRootFS{})
	if src == nil || len(src.structs) != 0 || len(src.funcs) != 0 {
		t.Errorf("walk error should yield an empty index: %+v", src)
	}
}

func TestParsePackageIgnoresNonTypeDecls(t *testing.T) {
	t.Parallel()
	src := parsePackage(srcFile(`
func Top(y int) {}
var X int
const C = 1
type groupedParams struct { Add func(a, b int) int }
`))
	if src.structs["groupedParams"] == nil {
		t.Fatal("struct type should still be indexed")
	}
	if _, ok := src.funcs["Top"]; ok {
		t.Error("func declarations must not be indexed as func types")
	}
}

func TestParsePackageSkipsNonGo(t *testing.T) {
	t.Parallel()
	src := parsePackage(fstest.MapFS{
		"readme.txt": {Data: []byte("type groupedParams struct { Add func(wrong int) }")},
		"api.go":     {Data: []byte("package api\ntype groupedParams struct { Add func(a, b int) int }")},
	})
	fields := astFields(src.structs["groupedParams"])
	ft := resolveFuncType(fields["Add"], src)
	args, _ := namesFromFuncType(ft)
	if !reflect.DeepEqual(args, []string{"a", "b"}) {
		t.Errorf("non-go files should be ignored, args=%q", args)
	}
}

func TestFilenameGOOSGOARCH(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	offOS := "windows"
	if runtime.GOOS == "windows" {
		offOS = "linux"
	}
	files := fstest.MapFS{
		"x_" + offOS + "_" + runtime.GOARCH + ".go":        {Data: []byte("package api\ntype constrainedAPI struct { Hello func(wrong string) }")},
		"x_" + runtime.GOOS + "_" + runtime.GOARCH + ".go": {Data: []byte("package api\ntype constrainedAPI struct { Hello func(ok string) }")},
	}
	RegisterSource[Structure](files)
	fn := mustFn(t, &constrainedAPI{}, "Hello")
	if !reflect.DeepEqual(fn.Args, []string{"ok"}) {
		t.Errorf("Args=%q, want [ok] on %s/%s", fn.Args, runtime.GOOS, runtime.GOARCH)
	}
}

func TestPlusBuildAndUnixTag(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	on := runtime.GOOS
	off := "windows"
	if on == "windows" {
		off = "linux"
	}
	files := fstest.MapFS{
		"and_off.go": {Data: []byte("// +build " + on + "\n// +build " + off + "\n\npackage api\ntype constrainedAPI struct { Hello func(off string) }\n")},
		"unix.go":    {Data: []byte("//go:build unix\n\npackage api\ntype constrainedAPI struct { Hello func(unixName string) }\n")},
		"plain.go":   {Data: []byte("package api\ntype constrainedAPI struct { Hello func(plain string) }\n")},
	}
	RegisterSource[Structure](files)
	fn := mustFn(t, &constrainedAPI{}, "Hello")
	// MapFS WalkDir is lexical: and_off.go, plain.go, unix.go
	want := "plain"
	if unixOS[runtime.GOOS] {
		want = "unixName"
	}
	if !reflect.DeepEqual(fn.Args, []string{want}) {
		t.Errorf("Args=%q, want %q (GOOS=%s)", fn.Args, want, runtime.GOOS)
	}
}

func TestRegisterSourceBuiltinIsNoop(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	RegisterSource[int](srcFile(`
type namedParams struct {
	Hello func(ctx context.Context, petId int, file string) (url string, err error)
}
`))
	if mustFn(t, &namedParams{}, "Hello").Args != nil {
		t.Error("RegisterSource[int] must not index this package")
	}
}

func TestStructureOfNonStruct(t *testing.T) {
	t.Parallel()
	s := StructureOf(42)
	if s.Name != "int" || len(s.Functions) != 0 {
		t.Errorf("int structure = %+v", s)
	}
	s = StructureOf((*int)(nil))
	if s.Name != "int" {
		t.Errorf("nil *int Name=%q", s.Name)
	}
}

func TestLookupSourceMissing(t *testing.T) {
	t.Parallel()
	if lookupSource("") != nil {
		t.Error("empty pkg path")
	}
	if lookupSource("runtime.link/api/does-not-exist") != nil {
		t.Error("missing package")
	}
}

func TestDiscoverSourceInvalid(t *testing.T) {
	t.Parallel()
	discoverSource(reflect.Value{})
}

func TestDoublePointerStructureOf(t *testing.T) {
	useSource(t, srcFile(`
type namedParams struct {
	Hello func(ctx context.Context, petId int, file string) (url string, err error)
}
`))
	p := &namedParams{}
	pp := &p
	fn := mustFn(t, pp, "Hello")
	if fn.InName(0) != "petId" {
		t.Errorf("**T Args InName(0)=%q", fn.InName(0))
	}
}

func TestCallAndMakePreserveNames(t *testing.T) {
	useSource(t, srcFile(`
type namedParams struct {
	Hello func(ctx context.Context, petId int, file string) (url string, err error)
}
`))
	var spec namedParams
	spec.Hello = func(_ context.Context, petId int, file string) (string, error) {
		return fmt.Sprintf("%d:%s", petId, file), nil
	}
	fn := mustFn(t, &spec, "Hello")
	outs, err := fn.Call(t.Context(), []reflect.Value{reflect.ValueOf(7), reflect.ValueOf("x")})
	if err != nil {
		t.Fatal(err)
	}
	if outs[0].String() != "7:x" {
		t.Errorf("Call = %q", outs[0].String())
	}
	if !reflect.DeepEqual(fn.Args, []string{"petId", "file"}) {
		t.Errorf("Call mutated Args=%q", fn.Args)
	}
	fn.Make(func(_ context.Context, petId int, file string) (string, error) {
		return "wrapped", nil
	})
	if !reflect.DeepEqual(fn.Args, []string{"petId", "file"}) {
		t.Errorf("Make mutated Args=%q", fn.Args)
	}
}

func TestConcurrentRegisterSourceFirstWins(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	a := srcFile(`
type namedParams struct {
	Hello func(ctx context.Context, petId int, file string) (url string, err error)
}
`)
	b := srcFile(`
type namedParams struct {
	Hello func(wrong int)
}
`)
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			RegisterSource[namedParams](a)
		}()
		go func() {
			defer wg.Done()
			RegisterSource[namedParams](b)
		}()
	}
	wg.Wait()
	fn := mustFn(t, &namedParams{}, "Hello")
	switch {
	case reflect.DeepEqual(fn.Args, []string{"petId", "file"}):
	case fn.Args == nil:
		// arity mismatch from the wrong file also degrades silently
	default:
		t.Errorf("unexpected Args=%q", fn.Args)
	}
}

func TestMethodsDoNotOverrideFields(t *testing.T) {
	useSource(t, srcFile(`
type namedParams struct {
	Hello func(ctx context.Context, petId int, file string) (url string, err error)
}
func (namedParams) Method(y int) {}
`))
	s := StructureOf(&namedParams{})
	if len(s.Functions) != 1 || s.Functions[0].Name != "Hello" {
		t.Fatalf("functions = %+v", s.Functions)
	}
	if s.Functions[0].InName(0) != "petId" {
		t.Errorf("method decl should not affect field names, Args=%q", s.Functions[0].Args)
	}
}
