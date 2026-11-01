package api

import (
	"context"
	"go/ast"
	"go/build"
	"go/build/constraint"
	astparser "go/parser"
	"go/token"
	"io"
	"reflect"
	"runtime"
	"strconv"
	"strings"
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
	if !matchFilename("foo_unix.go") {
		t.Error("unix is a build tag, foo_unix.go is compiled everywhere")
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

// matchFileContext asks the go command's own logic whether it would compile the
// named file with the given contents.
func matchFileContext(t *testing.T, goos, goarch, name, src string) bool {
	t.Helper()
	ctx := build.Context{
		GOOS:     goos,
		GOARCH:   goarch,
		Compiler: runtime.Compiler,
		OpenFile: func(string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(src)), nil
		},
		ReleaseTags: build.Default.ReleaseTags,
	}
	match, err := ctx.MatchFile("dir", name)
	if err != nil {
		t.Fatal(err)
	}
	return match
}

func TestMatchFilenameAgreesWithGoBuild(t *testing.T) {
	t.Parallel()
	targets := []struct{ goos, goarch string }{
		{"linux", "amd64"}, {"windows", "amd64"}, {"darwin", "arm64"},
		{"android", "arm64"}, {"ios", "arm64"}, {"illumos", "amd64"}, {"plan9", "386"},
	}
	names := []string{
		"plain.go", "linux.go", "amd64.go", "linux_amd64.go", "foo_unix.go", "foo_unix_amd64.go",
		"foo_linux.go", "foo_windows.go", "foo_darwin.go", "foo_android.go", "foo_ios.go",
		"foo_solaris.go", "foo_illumos.go", "foo_plan9.go", "foo_amd64.go", "foo_arm64.go",
		"foo_linux_amd64.go", "foo_windows_amd64.go", "foo_darwin_arm64.go", "foo_android_arm64.go",
		"foo_bar_linux.go", "foo_linux_bar.go", "_foo.go", ".foo.go", "_linux.go",
	}
	for _, target := range targets {
		for _, name := range names {
			want := matchFileContext(t, target.goos, target.goarch, name, "package p\n")
			if got := matchFilenameOn(name, target.goos, target.goarch); got != want {
				t.Errorf("%s/%s: matchFilenameOn(%q) = %v, go/build says %v", target.goos, target.goarch, name, got, want)
			}
		}
	}
}

func TestTagEnabledAgreesWithGoBuild(t *testing.T) {
	t.Parallel()
	targets := []struct{ goos, goarch string }{
		{"linux", "amd64"}, {"windows", "amd64"}, {"darwin", "arm64"},
		{"android", "arm64"}, {"ios", "arm64"}, {"illumos", "amd64"}, {"plan9", "386"},
	}
	var newest string
	for _, tag := range build.Default.ReleaseTags {
		newest = tag
	}
	minor, err := strconv.Atoi(strings.TrimPrefix(newest, "go1."))
	if err != nil {
		t.Fatal(err)
	}
	tags := []string{
		"linux", "windows", "darwin", "android", "ios", "solaris", "illumos", "unix", "plan9",
		"amd64", "arm64", "386", "gc", "gccgo", "ignore", "custom",
		"go1.1", "go1.21", newest, "go1." + strconv.Itoa(minor+1), "go1.x", "go1.", "go2",
	}
	for _, target := range targets {
		for _, tag := range tags {
			want := matchFileContext(t, target.goos, target.goarch, "file.go", "//go:build "+tag+"\n\npackage p\n")
			if got := tagEnabled(tag, target.goos, target.goarch); got != want {
				t.Errorf("%s/%s: tagEnabled(%q) = %v, go/build says %v", target.goos, target.goarch, tag, got, want)
			}
		}
	}
}

func TestFileEnabledSkipsFilesForOlderOrOtherToolchains(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, src string
		want      bool
	}{
		{"released.go", "//go:build go1.21\n\npackage p\n", true},
		{"unreleased.go", "//go:build go1.999\n\npackage p\n", false},
		{"before.go", "//go:build !go1.21\n\npackage p\n", false},
		{"compiler.go", "//go:build " + runtime.Compiler + "\n\npackage p\n", true},
		{"notcompiler.go", "//go:build !" + runtime.Compiler + "\n\npackage p\n", false},
	}
	for _, tt := range tests {
		if got := fileEnabled(tt.name, []byte(tt.src)); got != tt.want {
			t.Errorf("fileEnabled(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestHeaderConstraintAfterBlockComment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, src string
		found     bool
	}{
		{"one line", "/* copyright */\n//go:build ignore\n\npackage p\n", true},
		{"several lines", "/*\n copyright\n*/\n\n//go:build ignore\n\npackage p\n", true},
		{"mixed", "// copyright\n/* more */ /* and more\n */\n//go:build ignore\n\npackage p\n", true},
		{"plus build", "/* copyright */\n// +build ignore\n\npackage p\n", true},
		{"indented", "  //go:build ignore\n\npackage p\n", true},
		{"in the comment", "/*\n//go:build ignore\n*/\n\npackage p\n", false},
		{"after code on the line", "/* copyright */ package p\n//go:build ignore\n", false},
		{"after the comment on its line", "/* copyright */ //go:build ignore\n\npackage p\n", false},
		{"never closed", "/* copyright\n//go:build ignore\n", false},
		{"not a tag", "//go:buildignore\n\npackage p\n", false},
	}
	for _, tt := range tests {
		if got := headerConstraint([]byte(tt.src)) != nil; got != tt.found {
			t.Errorf("%s: found constraint = %v, want %v", tt.name, got, tt.found)
		}
		if tt.found && tt.name != "plus build" {
			if matchFileContext(t, "linux", "amd64", "file.go", tt.src) {
				t.Errorf("%s: the test is wrong, go/build does not see the constraint", tt.name)
			}
		}
	}
	if fileEnabled("file.go", []byte("/* copyright */\n//go:build ignore\n\npackage p\n")) {
		t.Error("a block comment hid the constraint")
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
