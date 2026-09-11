package api

import (
	"context"
	"io/fs"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
)

func resetSources() {
	sources.Range(func(key, _ any) bool {
		sources.Delete(key)
		return true
	})
}

func useSource(t *testing.T, files fs.FS) {
	t.Helper()
	resetSources()
	RegisterSource[Structure](files)
	t.Cleanup(resetSources)
}

func srcFile(body string) fstest.MapFS {
	return fstest.MapFS{
		"api.go": &fstest.MapFile{Data: []byte("package api\n\n" + body)},
	}
}

func mustFn(t *testing.T, val any, name string) Function {
	t.Helper()
	s := StructureOf(val)
	for _, fn := range s.Functions {
		if fn.Name == name {
			return fn
		}
	}
	for _, ns := range s.Namespace {
		for _, fn := range ns.Functions {
			if fn.Name == name {
				return fn
			}
		}
	}
	t.Fatalf("function %s not found in %+v", name, s)
	return Function{}
}

type namedParams struct {
	Hello func(ctx context.Context, petId int, file string) (url string, err error)
}

type unnamedParams struct {
	Hello func(context.Context, int, string) (string, error)
}

type groupedParams struct {
	Add func(a, b int) int
}

type variadicParams struct {
	Join func(sep string, parts ...string) string
}

type blankParams struct {
	Ignore func(_ int, name string)
}

type namedResults struct {
	Coords func() (lat, lon float64)
}

type namedFuncTypeField struct {
	Hello myFunc
}

type myFunc func(a int)

type sharedTypeFields struct {
	A, B func(x int)
}

type genericBox[T any] struct {
	Get func(id T) T
}

type inlineNamespace struct {
	Nested struct {
		Hello func(name string)
	}
}

type hostWithNested struct {
	Nested nestedAPI
}

type nestedAPI struct {
	Hello func(name string)
}

type mismatchAPI struct {
	Hello func(a int)
}

type taggedName struct {
	Hello func(x int) `api:"World"`
}

type constrainedAPI struct {
	Hello func(x int)
}

type valueSourceAPI struct {
	Hello func(name string)
}

func (valueSourceAPI) Source() fs.FS {
	return srcFile(`
type valueSourceAPI struct {
	Hello func(name string)
}
`)
}

type pointerSourceAPI struct {
	Hello func(name string)
}

func (*pointerSourceAPI) Source() fs.FS {
	return srcFile(`
type pointerSourceAPI struct {
	Hello func(name string)
}
`)
}

func TestNamedParams(t *testing.T) {
	useSource(t, srcFile(`
type namedParams struct {
	Hello func(ctx context.Context, petId int, file string) (url string, err error)
}
`))
	fn := mustFn(t, &namedParams{}, "Hello")
	if !reflect.DeepEqual(fn.Args, []string{"petId", "file"}) {
		t.Errorf("Args = %q", fn.Args)
	}
	if !reflect.DeepEqual(fn.Outs, []string{"url"}) {
		t.Errorf("Outs = %q", fn.Outs)
	}
	if !fn.Named() {
		t.Error("Named() = false")
	}
	if fn.InName(0) != "petId" || fn.InName(1) != "file" {
		t.Errorf("InName = %q, %q", fn.InName(0), fn.InName(1))
	}
	if fn.OutName(0) != "url" {
		t.Errorf("OutName = %q", fn.OutName(0))
	}
	if fn.InName(-1) != "" || fn.InName(99) != "" || fn.OutName(99) != "" {
		t.Error("out-of-range names should be empty")
	}
}

func TestUnnamedParams(t *testing.T) {
	useSource(t, srcFile(`
type unnamedParams struct {
	Hello func(context.Context, int, string) (string, error)
}
`))
	fn := mustFn(t, &unnamedParams{}, "Hello")
	if !reflect.DeepEqual(fn.Args, []string{"", ""}) {
		t.Errorf("Args = %#v", fn.Args)
	}
	if !reflect.DeepEqual(fn.Outs, []string{""}) {
		t.Errorf("Outs = %#v", fn.Outs)
	}
	if fn.Named() {
		t.Error("Named() = true for unnamed params")
	}
}

func TestGroupedParams(t *testing.T) {
	useSource(t, srcFile(`
type groupedParams struct {
	Add func(a, b int) int
}
`))
	fn := mustFn(t, &groupedParams{}, "Add")
	if !reflect.DeepEqual(fn.Args, []string{"a", "b"}) {
		t.Errorf("Args = %q", fn.Args)
	}
}

func TestVariadicParams(t *testing.T) {
	useSource(t, srcFile(`
type variadicParams struct {
	Join func(sep string, parts ...string) string
}
`))
	fn := mustFn(t, &variadicParams{}, "Join")
	if !reflect.DeepEqual(fn.Args, []string{"sep", "parts"}) {
		t.Errorf("Args = %q", fn.Args)
	}
}

func TestBlankIdent(t *testing.T) {
	useSource(t, srcFile(`
type blankParams struct {
	Ignore func(_ int, name string)
}
`))
	fn := mustFn(t, &blankParams{}, "Ignore")
	if !reflect.DeepEqual(fn.Args, []string{"", "name"}) {
		t.Errorf("Args = %#v", fn.Args)
	}
	if fn.Named() {
		t.Error("Named() should be false when a name is blank")
	}
}

func TestNamedResults(t *testing.T) {
	useSource(t, srcFile(`
type namedResults struct {
	Coords func() (lat, lon float64)
}
`))
	fn := mustFn(t, &namedResults{}, "Coords")
	if !reflect.DeepEqual(fn.Outs, []string{"lat", "lon"}) {
		t.Errorf("Outs = %q", fn.Outs)
	}
	if len(fn.Args) != 0 {
		t.Errorf("Args = %q, want empty", fn.Args)
	}
	if !fn.Named() {
		t.Error("Named() = false for zero-argument function")
	}
}

func TestNamedFuncTypeField(t *testing.T) {
	useSource(t, srcFile(`
type myFunc func(a int)
type namedFuncTypeField struct {
	Hello myFunc
}
`))
	fn := mustFn(t, &namedFuncTypeField{}, "Hello")
	if !reflect.DeepEqual(fn.Args, []string{"a"}) {
		t.Errorf("Args = %q", fn.Args)
	}
}

func TestSharedTypeFields(t *testing.T) {
	useSource(t, srcFile(`
type sharedTypeFields struct {
	A, B func(x int)
}
`))
	s := StructureOf(&sharedTypeFields{})
	if len(s.Functions) != 2 {
		t.Fatalf("got %d functions", len(s.Functions))
	}
	for _, fn := range s.Functions {
		if !reflect.DeepEqual(fn.Args, []string{"x"}) {
			t.Errorf("%s Args = %q", fn.Name, fn.Args)
		}
	}
}

func TestGenericStructName(t *testing.T) {
	useSource(t, srcFile(`
type genericBox[T any] struct {
	Get func(id T) T
}
`))
	fn := mustFn(t, &genericBox[int]{}, "Get")
	if !reflect.DeepEqual(fn.Args, []string{"id"}) {
		t.Errorf("Args = %q (rtype.Name=%q)", fn.Args, reflect.TypeOf(genericBox[int]{}).Name())
	}
}

func TestInlineAnonymousNamespace(t *testing.T) {
	useSource(t, srcFile(`
type inlineNamespace struct {
	Nested struct {
		Hello func(name string)
	}
}
`))
	s := StructureOf(&inlineNamespace{})
	child, ok := s.Namespace["Nested"]
	if !ok {
		t.Fatal("missing Nested namespace")
	}
	if len(child.Functions) != 1 || !reflect.DeepEqual(child.Functions[0].Args, []string{"name"}) {
		t.Errorf("Nested.Hello Args = %v", child.Functions)
	}
}

func TestNamedNamespaceSecondFile(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	RegisterSource[Structure](fstest.MapFS{
		"api.go": &fstest.MapFile{Data: []byte(`package api
type hostWithNested struct {
	Nested nestedAPI
}
`)},
		"nested.go": &fstest.MapFile{Data: []byte(`package api
type nestedAPI struct {
	Hello func(name string)
}
`)},
	})
	s := StructureOf(&hostWithNested{})
	child := s.Namespace["Nested"]
	if len(child.Functions) != 1 || !reflect.DeepEqual(child.Functions[0].Args, []string{"name"}) {
		t.Errorf("Nested.Hello Args = %v", child.Functions)
	}
}

func TestSelectorExprNoNames(t *testing.T) {
	src := parsePackage(srcFile(`
import "other/pkg"
type Host struct {
	Child pkg.API
	Local func(x int)
}
`))
	st := src.structs["Host"]
	if st == nil {
		t.Fatal("Host not indexed")
	}
	fields := astFields(st)
	if resolveFuncType(fields["Child"], src) != nil {
		t.Error("selector should not resolve to a func type")
	}
	ft := resolveFuncType(fields["Local"], src)
	args, _ := namesFromFuncType(ft)
	if !reflect.DeepEqual(args, []string{"x"}) {
		t.Errorf("Local args = %q", args)
	}
}

func TestTestGoExcluded(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	RegisterSource[Structure](fstest.MapFS{
		"api.go": &fstest.MapFile{Data: []byte(`package api
type namedParams struct {
	Hello func(ctx context.Context, petId int, file string) (url string, err error)
}
`)},
		"api_test.go": &fstest.MapFile{Data: []byte(`package api
type namedParams struct {
	Hello func(wrong int)
}
`)},
	})
	fn := mustFn(t, &namedParams{}, "Hello")
	if !reflect.DeepEqual(fn.Args, []string{"petId", "file"}) {
		t.Errorf("Args = %q, _test.go should have been excluded", fn.Args)
	}
}

func TestBuildConstrainedDuplicates(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	RegisterSource[Structure](fstest.MapFS{
		"link_linux.go": &fstest.MapFile{Data: []byte(`//go:build linux

package api
type constrainedAPI struct {
	Hello func(linux string)
}
`)},
		"link_notlinux.go": &fstest.MapFile{Data: []byte(`//go:build !linux

package api
type constrainedAPI struct {
	Hello func(notLinux string)
}
`)},
	})
	fn := mustFn(t, &constrainedAPI{}, "Hello")
	want := "linux"
	if runtime.GOOS != "linux" {
		want = "notLinux"
	}
	if !reflect.DeepEqual(fn.Args, []string{want}) {
		t.Errorf("Args = %q, want %q on GOOS=%s", fn.Args, want, runtime.GOOS)
	}
}

func TestUnparsableFileSkipped(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	RegisterSource[Structure](fstest.MapFS{
		"bad.go": &fstest.MapFile{Data: []byte("this is not go")},
		"good.go": &fstest.MapFile{Data: []byte(`package api
type groupedParams struct {
	Add func(a, b int) int
}
`)},
	})
	fn := mustFn(t, &groupedParams{}, "Add")
	if !reflect.DeepEqual(fn.Args, []string{"a", "b"}) {
		t.Errorf("Args = %q", fn.Args)
	}
}

func TestArityMismatchDropsNames(t *testing.T) {
	useSource(t, srcFile(`
type mismatchAPI struct {
	Hello func(a, b int)
}
`))
	fn := mustFn(t, &mismatchAPI{}, "Hello")
	if fn.Args != nil || fn.Outs != nil {
		t.Errorf("mismatch should drop names, Args=%q Outs=%q", fn.Args, fn.Outs)
	}
}

func TestAPITagDoesNotAffectLookup(t *testing.T) {
	useSource(t, srcFile(`
type taggedName struct {
	Hello func(x int)
}
`))
	fn := mustFn(t, &taggedName{}, "World")
	if !reflect.DeepEqual(fn.Args, []string{"x"}) {
		t.Errorf("Args = %q", fn.Args)
	}
}

func TestAnonymousHasNoNames(t *testing.T) {
	useSource(t, srcFile(`type namedParams struct { Hello func(a int) }`))
	var API struct {
		Hello func(name string)
	}
	s := StructureOf(&API)
	if len(s.Functions) != 1 {
		t.Fatalf("got %d functions", len(s.Functions))
	}
	if s.Functions[0].Args != nil {
		t.Errorf("anonymous struct Args = %q, want nil", s.Functions[0].Args)
	}
}

func TestWithSourceValueReceiver(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	fn := mustFn(t, valueSourceAPI{}, "Hello")
	if !reflect.DeepEqual(fn.Args, []string{"name"}) {
		t.Errorf("Args = %q", fn.Args)
	}
}

func TestWithSourcePointerReceiver(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	fn := mustFn(t, &pointerSourceAPI{}, "Hello")
	if !reflect.DeepEqual(fn.Args, []string{"name"}) {
		t.Errorf("Args = %q", fn.Args)
	}
}

func TestRegisterSource(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	RegisterSource[namedParams](srcFile(`
type namedParams struct {
	Hello func(ctx context.Context, petId int, file string) (url string, err error)
}
`))
	fn := mustFn(t, &namedParams{}, "Hello")
	if fn.InName(0) != "petId" {
		t.Errorf("InName(0) = %q", fn.InName(0))
	}
}

type countingFS struct {
	inner fs.FS
	opens atomic.Int64
}

func (c *countingFS) Open(name string) (fs.File, error) {
	if len(name) >= 3 && name[len(name)-3:] == ".go" {
		c.opens.Add(1)
	}
	return c.inner.Open(name)
}

func TestConcurrentStructureOfParsesOnce(t *testing.T) {
	resetSources()
	t.Cleanup(resetSources)
	fsys := &countingFS{inner: srcFile(`
type namedParams struct {
	Hello func(ctx context.Context, petId int, file string) (url string, err error)
}
`)}
	RegisterSource[namedParams](fsys)
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = StructureOf(&namedParams{})
		}()
	}
	wg.Wait()
	if n := fsys.opens.Load(); n != 1 {
		t.Fatalf("parsed %d times, want 1", n)
	}
}

func TestNamedArgumentScanner(t *testing.T) {
	type pair struct {
		Left, Right string
	}
	left := reflect.ValueOf("alpha")
	right := reflect.ValueOf(pair{Left: "L", Right: "R"})
	scanner := NewNamedArgumentScanner(
		[]reflect.Value{left, right},
		[]string{"name", "pair"},
	)
	v, err := scanner.Scan("name")
	if err != nil || v.String() != "alpha" {
		t.Fatalf("name: %v %v", v, err)
	}
	v, err = scanner.Scan("Left")
	if err != nil || v.String() != "L" {
		t.Fatalf("Left field: %v %v", v, err)
	}
	// parameter names shadow struct fields
	shadow := NewNamedArgumentScanner(
		[]reflect.Value{reflect.ValueOf("param"), reflect.ValueOf(pair{Left: "field"})},
		[]string{"Left", "pair"},
	)
	v, err = shadow.Scan("Left")
	if err != nil || v.String() != "param" {
		t.Fatalf("shadow: %v %v", v, err)
	}
	pos := NewNamedArgumentScanner([]reflect.Value{left, right}, []string{"name", "pair"})
	v, err = pos.Scan("%v")
	if err != nil || v.String() != "alpha" {
		t.Fatalf("%%v: %v %v", v, err)
	}
}
