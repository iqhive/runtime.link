package api

import (
	"context"
	"go/ast"
	"go/build/constraint"
	astparser "go/parser"
	"go/token"
	"reflect"
	"runtime"
	"testing"
	"testing/fstest"
)

func TestBaseName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, want string
	}{
		{"Box", "Box"},
		{"Box[int]", "Box"},
		{"Box[github.com/foo.Bar]", "Box"},
		{"genericBox[int, string]", "genericBox"},
		{"", ""},
		{"A[B[C]]", "A"},
	}
	for _, tt := range tests {
		if got := baseName(tt.in); got != tt.want {
			t.Errorf("baseName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMatchFilename(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		want bool
	}{
		{"api.go", true},
		{"foo_test.go", false},
		{"dir/foo_test.go", false},
		{"readme.txt", false},
		{"dir/api.go", true},
		{"foo_" + runtime.GOOS + ".go", true},
		{"foo_" + runtime.GOARCH + ".go", true},
		{"foo_" + runtime.GOOS + "_" + runtime.GOARCH + ".go", true},
		{`dir\api.go`, true},
	}
	for _, tt := range tests {
		if got := matchFilename(tt.name); got != tt.want {
			t.Errorf("matchFilename(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
	if matchFilename("foo_windows.go") && runtime.GOOS != "windows" {
		t.Error("foo_windows.go should be skipped off windows")
	}
	if matchFilename("foo_js.go") && runtime.GOOS != "js" {
		t.Error("foo_js.go should be skipped off js")
	}
	if matchFilename("foo_wasm.go") && runtime.GOARCH != "wasm" {
		t.Error("foo_wasm.go should be skipped off wasm")
	}
	if unixOS[runtime.GOOS] && !matchFilename("foo_unix.go") {
		t.Error("foo_unix.go should match on a unix GOOS")
	}
	if !unixOS[runtime.GOOS] && matchFilename("foo_unix.go") {
		t.Error("foo_unix.go should be skipped off unix")
	}
	offOS := "windows"
	if runtime.GOOS == "windows" {
		offOS = "linux"
	}
	if matchFilename("foo_" + offOS + "_" + runtime.GOARCH + ".go") {
		t.Errorf("foo_%s_%s.go should be skipped on GOOS=%s", offOS, runtime.GOARCH, runtime.GOOS)
	}
	if runtime.GOARCH != "wasm" && matchFilename("foo_"+runtime.GOOS+"_wasm.go") {
		t.Error("wrong GOARCH suffix should be skipped")
	}
}

func TestFileEnabledBuildTags(t *testing.T) {
	t.Parallel()
	on := runtime.GOOS
	off := "windows"
	if on == "windows" {
		off = "linux"
	}
	tests := []struct {
		name, src string
		want      bool
	}{
		{"ok.go", "package p\n", true},
		{"ignore.go", "//go:build ignore\n\npackage p\n", false},
		{"cgo.go", "//go:build cgo\n\npackage p\n", false},
		{"os.go", "//go:build " + on + "\n\npackage p\n", true},
		{"otheros.go", "//go:build " + off + "\n\npackage p\n", false},
		{"notos.go", "//go:build !" + off + "\n\npackage p\n", true},
		{"plus.go", "// +build " + on + "\n\npackage p\n", true},
		{"plusoff.go", "// +build " + off + "\n\npackage p\n", false},
		{"plusand.go", "// +build " + on + "\n// +build " + on + "\n\npackage p\n", true},
		{"plusandoff.go", "// +build " + on + "\n// +build " + off + "\n\npackage p\n", false},
		{"unix.go", "//go:build unix\n\npackage p\n", unixOS[runtime.GOOS]},
		{"goBuildWins.go", "//go:build " + on + "\n// +build " + off + "\n\npackage p\n", true},
		{"commentThenCode.go", "// copyright\n\npackage p\n", true},
		{"blankThenBuild.go", "\n\n//go:build " + on + "\n\npackage p\n", true},
		{"suffix_" + off + ".go", "package p\n", false},
	}
	for _, tt := range tests {
		if got := fileEnabled(tt.name, []byte(tt.src)); got != tt.want {
			t.Errorf("fileEnabled(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestNamesFromFields(t *testing.T) {
	t.Parallel()
	parse := func(sig string) *ast.FuncType {
		t.Helper()
		src := "package p\ntype T " + sig
		f, err := astparser.ParseFile(token.NewFileSet(), "t.go", src, 0)
		if err != nil {
			t.Fatal(err)
		}
		ts := f.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
		return ts.Type.(*ast.FuncType)
	}
	args, outs := namesFromFuncType(parse("func(a int, b string)"))
	if !reflect.DeepEqual(args, []string{"a", "b"}) || outs != nil {
		t.Errorf("named params: args=%q outs=%q", args, outs)
	}
	args, outs = namesFromFuncType(parse("func(int, string)"))
	if !reflect.DeepEqual(args, []string{"", ""}) {
		t.Errorf("unnamed params: %q", args)
	}
	args, outs = namesFromFuncType(parse("func(a, b int)"))
	if !reflect.DeepEqual(args, []string{"a", "b"}) {
		t.Errorf("grouped: %q", args)
	}
	args, _ = namesFromFuncType(parse("func(_ int, name string)"))
	if !reflect.DeepEqual(args, []string{"", "name"}) {
		t.Errorf("blank: %q", args)
	}
	args, outs = namesFromFuncType(parse("func() (lat, lon float64)"))
	if args != nil || !reflect.DeepEqual(outs, []string{"lat", "lon"}) {
		t.Errorf("results: args=%q outs=%q", args, outs)
	}
	args, outs = namesFromFuncType(parse("func()"))
	if args != nil || outs != nil {
		t.Errorf("empty: args=%q outs=%q", args, outs)
	}
	args, outs = namesFromFuncType(parse("func(parts ...string)"))
	if !reflect.DeepEqual(args, []string{"parts"}) {
		t.Errorf("variadic: %q", args)
	}
	_, outs = namesFromFuncType(parse("func() (int, error)"))
	if !reflect.DeepEqual(outs, []string{"", ""}) {
		t.Errorf("unnamed results: %q", outs)
	}
	_, outs = namesFromFuncType(parse("func() (a int, b, c string)"))
	if !reflect.DeepEqual(outs, []string{"a", "b", "c"}) {
		t.Errorf("grouped results: %q", outs)
	}
	if namesFromFields(nil) != nil {
		t.Error("nil field list should be nil")
	}
	if a, o := namesFromFuncType(nil); a != nil || o != nil {
		t.Error("nil func type should be nil")
	}
}

func TestAlignNames(t *testing.T) {
	t.Parallel()
	type fnc func(ctx context.Context, petId int, file string) (url string, err error)
	type noctx func(a, b int) int
	type onlyErr func() error
	type onlyCtx func(context.Context)
	type unnamedErr func(int) error
	type badCtx func(a int, ctx context.Context)
	type twoOut func() (lat, lon float64)
	type mismatch func(a int)
	type errThenInt func() (err error, n int)

	tests := []struct {
		name     string
		args     []string
		outs     []string
		typ      reflect.Type
		wantArgs []string
		wantOuts []string
	}{
		{
			name:     "ctx and error stripped",
			args:     []string{"ctx", "petId", "file"},
			outs:     []string{"url", "err"},
			typ:      reflect.TypeFor[fnc](),
			wantArgs: []string{"petId", "file"},
			wantOuts: []string{"url"},
		},
		{
			name:     "no ctx no error",
			args:     []string{"a", "b"},
			outs:     []string{""},
			typ:      reflect.TypeFor[noctx](),
			wantArgs: []string{"a", "b"},
			wantOuts: []string{""},
		},
		{
			name:     "only error",
			args:     nil,
			outs:     []string{"err"},
			typ:      reflect.TypeFor[onlyErr](),
			wantArgs: nil,
			wantOuts: nil,
		},
		{
			name:     "only context",
			args:     []string{"ctx"},
			outs:     nil,
			typ:      reflect.TypeFor[onlyCtx](),
			wantArgs: nil,
			wantOuts: nil,
		},
		{
			name:     "arity mismatch drops both",
			args:     []string{"a", "b"},
			outs:     nil,
			typ:      reflect.TypeFor[mismatch](),
			wantArgs: nil,
			wantOuts: nil,
		},
		{
			name:     "ctx not first is not stripped",
			args:     []string{"a", "ctx"},
			outs:     nil,
			typ:      reflect.TypeFor[badCtx](),
			wantArgs: []string{"a", "ctx"},
			wantOuts: nil,
		},
		{
			name:     "two named results",
			args:     nil,
			outs:     []string{"lat", "lon"},
			typ:      reflect.TypeFor[twoOut](),
			wantArgs: nil,
			wantOuts: []string{"lat", "lon"},
		},
		{
			name:     "nil type",
			args:     []string{"a"},
			outs:     nil,
			typ:      nil,
			wantArgs: nil,
			wantOuts: nil,
		},
		{
			name:     "non-func type",
			args:     []string{"a"},
			outs:     nil,
			typ:      reflect.TypeFor[int](),
			wantArgs: nil,
			wantOuts: nil,
		},
		{
			name:     "ctx present but args empty",
			args:     nil,
			outs:     nil,
			typ:      reflect.TypeFor[onlyCtx](),
			wantArgs: nil,
			wantOuts: nil,
		},
		{
			name:     "error present but outs empty",
			args:     []string{""},
			outs:     nil,
			typ:      reflect.TypeFor[unnamedErr](),
			wantArgs: nil,
			wantOuts: nil,
		},
		{
			name:     "error not last is not stripped",
			args:     nil,
			outs:     []string{"err", "n"},
			typ:      reflect.TypeFor[errThenInt](),
			wantArgs: nil,
			wantOuts: []string{"err", "n"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, outs := alignNames(tt.args, tt.outs, tt.typ)
			if !reflect.DeepEqual(args, tt.wantArgs) || !reflect.DeepEqual(outs, tt.wantOuts) {
				t.Errorf("got args=%q outs=%q, want args=%q outs=%q", args, outs, tt.wantArgs, tt.wantOuts)
			}
		})
	}
}

func TestParsePackageSkipsTestdataAndHidden(t *testing.T) {
	t.Parallel()
	src := parsePackage(fstest.MapFS{
		"api.go":          {Data: []byte("package api\ntype keep struct { Hello func(ok string) }")},
		"testdata/bad.go": {Data: []byte("package api\ntype keep struct { Hello func(wrong string) }")},
		".hidden/bad.go":  {Data: []byte("package api\ntype keep struct { Hello func(hidden string) }")},
		"subdir/more.go":  {Data: []byte("package api\ntype extra struct { Run func(n int) }")},
	})
	if src.structs["keep"] == nil {
		t.Fatal("keep not indexed")
	}
	fields := astFields(src.structs["keep"])
	ft := resolveFuncType(fields["Hello"], src)
	args, _ := namesFromFuncType(ft)
	if !reflect.DeepEqual(args, []string{"ok"}) {
		t.Errorf("testdata/hidden should not override, args=%q", args)
	}
	if src.structs["extra"] == nil {
		t.Error("subdir .go files should be indexed")
	}
}

func TestParsePackageNilAndEmpty(t *testing.T) {
	t.Parallel()
	if src := parsePackage(nil); src == nil || src.structs == nil {
		t.Fatal("nil fs should still return an empty index")
	}
	if src := parsePackage(fstest.MapFS{}); len(src.structs) != 0 {
		t.Errorf("empty fs: %+v", src.structs)
	}
}

func TestResolveFuncTypeShapes(t *testing.T) {
	t.Parallel()
	src := parsePackage(srcFile(`
type myFunc func(a int)
type wrapped struct {
	Direct func(x int)
	Named  myFunc
	Parens (func(y int))
	Star   *myFunc
	Other  pkg.API
}
`))
	fields := astFields(src.structs["wrapped"])
	if got := resolveFuncType(fields["Direct"], src); got == nil {
		t.Error("direct func")
	}
	if got := resolveFuncType(fields["Named"], src); got == nil {
		t.Error("named func type")
	}
	if got := resolveFuncType(fields["Parens"], src); got == nil {
		t.Error("parenthesized func type")
	}
	if got := resolveFuncType(fields["Star"], src); got == nil {
		t.Error("pointer to named func type")
	}
	if got := resolveFuncType(fields["Other"], src); got != nil {
		t.Error("selector should not resolve")
	}
	if resolveFuncType(nil, src) != nil {
		t.Error("nil expr")
	}
	if resolveFuncType(&ast.Ident{Name: "myFunc"}, nil) != nil {
		t.Error("ident without src")
	}
}

func TestAstFieldsEmbedded(t *testing.T) {
	t.Parallel()
	src := parsePackage(srcFile(`
type inner struct { Hello func(name string) }
type host struct {
	inner
	*nestedAPI
	A, B func(x int)
}
`))
	fields := astFields(src.structs["host"])
	if _, ok := fields["inner"]; !ok {
		t.Error("embedded inner")
	}
	if _, ok := fields["nestedAPI"]; !ok {
		t.Error("embedded *nestedAPI")
	}
	if fields["A"] == nil || fields["B"] == nil {
		t.Error("grouped names")
	}
}

func TestAstFieldsNil(t *testing.T) {
	t.Parallel()
	if astFields(nil) != nil {
		t.Error("nil struct")
	}
	if astFields(&ast.StructType{}) != nil {
		t.Error("nil field list")
	}
}

func TestHeaderConstraintPlusBuildAnd(t *testing.T) {
	t.Parallel()
	src := []byte("// +build linux\n// +build amd64\n\npackage p\n")
	expr := headerConstraint(src)
	if expr == nil {
		t.Fatal("expected AND of plus-build lines")
	}
	and, ok := expr.(*constraint.AndExpr)
	if !ok {
		t.Fatalf("got %T", expr)
	}
	if and.X == nil || and.Y == nil {
		t.Fatal("AND operands")
	}
}

func TestHeaderConstraintIgnoresBodyComments(t *testing.T) {
	t.Parallel()
	src := []byte("package p\n\n//go:build windows\nfunc F() {}\n")
	if expr := headerConstraint(src); expr != nil {
		t.Error("go:build after package clause must not constrain the file")
	}
}
