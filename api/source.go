package api

import (
	"bufio"
	"bytes"
	"context"
	"go/ast"
	"go/build/constraint"
	goparser "go/parser"
	"go/token"
	"io/fs"
	"reflect"
	"runtime"
	"strings"
	"sync"
)

// WithSource is implemented by API structures that ship the Go source of
// their package, enabling [Function.Args] and [Function.Outs]. Typically:
//
//	//go:embed *.go
//	var source embed.FS
//
//	func (API) Source() fs.FS { return source }
type WithSource interface {
	Source() fs.FS
}

// RegisterSource associates Go source with the package of T, so that
// any API structure declared in that package gains parameter names.
func RegisterSource[T any](files fs.FS) {
	t := reflect.TypeFor[T]()
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	registerSource(t.PkgPath(), files)
}

type packageSource struct {
	structs map[string]*ast.StructType // named struct types, by identifier
	funcs   map[string]*ast.FuncType   // named func types, by identifier
}

type sourceEntry struct {
	once sync.Once
	fs   fs.FS
	src  *packageSource
}

var sources sync.Map // reflect.Type.PkgPath() -> *sourceEntry

func registerSource(pkgPath string, files fs.FS) {
	if pkgPath == "" || files == nil {
		return
	}
	sources.LoadOrStore(pkgPath, &sourceEntry{fs: files})
}

func lookupSource(pkgPath string) *packageSource {
	if pkgPath == "" {
		return nil
	}
	v, ok := sources.Load(pkgPath)
	if !ok {
		return nil
	}
	e := v.(*sourceEntry)
	e.once.Do(func() {
		defer func() { recover() }()
		e.src = parsePackage(e.fs)
	})
	return e.src
}

func discoverSource(rvalue reflect.Value) {
	if !rvalue.IsValid() {
		return
	}
	if src, ok := rvalue.Interface().(WithSource); ok {
		registerSource(rvalue.Type().PkgPath(), src.Source())
		return
	}
	if rvalue.Kind() == reflect.Struct && rvalue.CanAddr() {
		if src, ok := rvalue.Addr().Interface().(WithSource); ok {
			registerSource(rvalue.Type().PkgPath(), src.Source())
		}
	}
}

func parsePackage(files fs.FS) *packageSource {
	src := &packageSource{
		structs: make(map[string]*ast.StructType),
		funcs:   make(map[string]*ast.FuncType),
	}
	if files == nil {
		return src
	}
	fset := token.NewFileSet()
	_ = fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != "." && (d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".")) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := fs.ReadFile(files, path)
		if err != nil {
			return nil
		}
		if !fileEnabled(path, data) {
			return nil
		}
		file, err := goparser.ParseFile(fset, path, data, goparser.SkipObjectResolution|goparser.ParseComments)
		if err != nil {
			return nil
		}
		indexFile(src, file)
		return nil
	})
	return src
}

func indexFile(src *packageSource, file *ast.File) {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name == nil {
				continue
			}
			switch t := ts.Type.(type) {
			case *ast.StructType:
				src.structs[ts.Name.Name] = t
			case *ast.FuncType:
				src.funcs[ts.Name.Name] = t
			}
		}
	}
}

func astFields(st *ast.StructType) map[string]ast.Expr {
	if st == nil || st.Fields == nil {
		return nil
	}
	fields := make(map[string]ast.Expr)
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			// Embedded field: key by the type identifier when present.
			switch t := field.Type.(type) {
			case *ast.Ident:
				fields[t.Name] = field.Type
			case *ast.StarExpr:
				if id, ok := t.X.(*ast.Ident); ok {
					fields[id.Name] = field.Type
				}
			}
			continue
		}
		for _, name := range field.Names {
			fields[name.Name] = field.Type
		}
	}
	return fields
}

func resolveFuncType(expr ast.Expr, src *packageSource) *ast.FuncType {
	for expr != nil {
		switch t := expr.(type) {
		case *ast.FuncType:
			return t
		case *ast.ParenExpr:
			expr = t.X
		case *ast.StarExpr:
			expr = t.X
		case *ast.Ident:
			if src == nil {
				return nil
			}
			return src.funcs[t.Name]
		default:
			return nil
		}
	}
	return nil
}

func namesFromFuncType(ft *ast.FuncType) (args, outs []string) {
	if ft == nil {
		return nil, nil
	}
	return namesFromFields(ft.Params), namesFromFields(ft.Results)
}

func namesFromFields(list *ast.FieldList) []string {
	if list == nil {
		return nil
	}
	var names []string
	for _, field := range list.List {
		if len(field.Names) == 0 {
			names = append(names, "")
			continue
		}
		for _, ident := range field.Names {
			if ident.Name == "_" {
				names = append(names, "")
			} else {
				names = append(names, ident.Name)
			}
		}
	}
	return names
}

func alignNames(args, outs []string, t reflect.Type) ([]string, []string) {
	if t == nil || t.Kind() != reflect.Func {
		return nil, nil
	}
	ctx := t.NumIn() > 0 && t.In(0) == contextType
	if ctx {
		if len(args) == 0 {
			return nil, nil
		}
		args = args[1:]
	}
	errOut := t.NumOut() > 0 && t.Out(t.NumOut()-1).Implements(errorType)
	if errOut {
		if len(outs) == 0 {
			return nil, nil
		}
		outs = outs[:len(outs)-1]
	}
	wantIn := t.NumIn()
	if ctx {
		wantIn--
	}
	wantOut := t.NumOut()
	if errOut {
		wantOut--
	}
	if len(args) != wantIn || len(outs) != wantOut {
		return nil, nil
	}
	if wantIn == 0 {
		args = nil
	}
	if wantOut == 0 {
		outs = nil
	}
	return args, outs
}

func attachNames(fn *Function, expr ast.Expr, src *packageSource) {
	ft := resolveFuncType(expr, src)
	if ft == nil {
		return
	}
	args, outs := namesFromFuncType(ft)
	fn.Args, fn.Outs = alignNames(args, outs, fn.Type)
}

func structAST(rtype reflect.Type, inline *ast.StructType) *ast.StructType {
	if inline != nil {
		return inline
	}
	if rtype.Name() == "" {
		return nil
	}
	src := lookupSource(rtype.PkgPath())
	if src == nil {
		return nil
	}
	return src.structs[baseName(rtype.Name())]
}

func baseName(name string) string {
	if i := strings.IndexByte(name, '['); i >= 0 {
		return name[:i]
	}
	return name
}

func fileEnabled(path string, src []byte) bool {
	if !matchFilename(path) {
		return false
	}
	expr := headerConstraint(src)
	if expr == nil {
		return true
	}
	ok := expr.Eval(func(tag string) bool {
		if tag == runtime.GOOS || tag == runtime.GOARCH {
			return true
		}
		if tag == "unix" {
			return unixOS[runtime.GOOS]
		}
		// cgo and any other tag are unknown → false.
		return false
	})
	return ok
}

func headerConstraint(src []byte) constraint.Expr {
	var (
		goBuild constraint.Expr
		plus    []constraint.Expr
	)
	scanner := bufio.NewScanner(bytes.NewReader(src))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if constraint.IsGoBuild(line) {
			expr, err := constraint.Parse(line)
			if err == nil {
				goBuild = expr
			}
			break
		}
		if constraint.IsPlusBuild(line) {
			expr, err := constraint.Parse(line)
			if err == nil {
				plus = append(plus, expr)
			}
			continue
		}
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		break
	}
	if goBuild != nil {
		return goBuild
	}
	if len(plus) == 0 {
		return nil
	}
	expr := plus[0]
	for _, next := range plus[1:] {
		expr = &constraint.AndExpr{X: expr, Y: next}
	}
	return expr
}

func matchFilename(path string) bool {
	name := path
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	if !strings.HasSuffix(name, ".go") {
		return false
	}
	name = name[:len(name)-len(".go")]
	if strings.HasSuffix(name, "_test") {
		return false
	}
	// name[_GOOS][_GOARCH]
	if i := strings.LastIndexByte(name, '_'); i >= 0 {
		suffix := name[i+1:]
		if knownGOARCH[suffix] {
			if suffix != runtime.GOARCH {
				return false
			}
			name = name[:i]
			if j := strings.LastIndexByte(name, '_'); j >= 0 {
				osName := name[j+1:]
				if knownGOOS[osName] && osName != runtime.GOOS {
					return false
				}
				if osName == "unix" && !unixOS[runtime.GOOS] {
					return false
				}
			}
			return true
		}
		if knownGOOS[suffix] {
			return suffix == runtime.GOOS
		}
		if suffix == "unix" {
			return unixOS[runtime.GOOS]
		}
	}
	return true
}

var (
	contextType = reflect.TypeFor[context.Context]()
	errorType   = reflect.TypeFor[error]()
)

// known GOOS/GOARCH values, matching go/build so that filename suffixes
// like link_linux.go are skipped on other platforms.
var knownGOOS = map[string]bool{
	"aix": true, "android": true, "darwin": true, "dragonfly": true,
	"freebsd": true, "hurd": true, "illumos": true, "ios": true,
	"js": true, "linux": true, "nacl": true, "netbsd": true,
	"openbsd": true, "plan9": true, "solaris": true, "wasip1": true,
	"windows": true, "zos": true,
}

var knownGOARCH = map[string]bool{
	"386": true, "amd64": true, "amd64p32": true, "arm": true, "armbe": true,
	"arm64": true, "arm64be": true, "loong64": true, "mips": true, "mipsle": true,
	"mips64": true, "mips64le": true, "mips64p32": true, "mips64p32le": true,
	"ppc": true, "ppc64": true, "ppc64le": true, "riscv": true, "riscv64": true,
	"s390": true, "s390x": true, "sparc": true, "sparc64": true, "wasm": true,
}

var unixOS = map[string]bool{
	"aix": true, "android": true, "darwin": true, "dragonfly": true,
	"freebsd": true, "hurd": true, "illumos": true, "ios": true,
	"linux": true, "netbsd": true, "openbsd": true, "solaris": true,
}
