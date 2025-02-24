package parser

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"runtime.link/zgo/internal/escape"
	"runtime.link/zgo/internal/source"
)

func TestEscapeAnalysis(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantKind escape.Kind
	}{
		{
			name: "channel send escapes to goroutine",
			code: `
				package test
				func f(ch chan int) {
					x := 42
					ch <- x
				}`,
			wantKind: escape.GoroutineEscape,
		},
		{
			name: "goroutine capture escapes",
			code: `
				package test
				func f() {
					x := 42
					go func() { println(x) }()
				}`,
			wantKind: escape.GoroutineEscape,
		},
		{
			name: "pointer escapes to heap",
			code: `
				package test
				func f() *int {
					x := 42
					return &x
				}`,
			wantKind: escape.HeapEscape,
		},
		{
			name: "interface conversion escapes to heap",
			code: `
				package test
				func f(x interface{}) int {
					return x.(int)
				}`,
			wantKind: escape.HeapEscape,
		},
		{
			name: "local value doesn't escape",
			code: `
				package test
				func f() {
					x := 42
					println(x)
				}`,
			wantKind: escape.NoEscape,
		},
	}

	fset := token.NewFileSet()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := parser.ParseFile(fset, "test.go", tt.code, 0)
			if err != nil {
				t.Fatal(err)
			}

			conf := types.Config{}
			info := &types.Info{
				Types: make(map[ast.Expr]types.TypeAndValue),
				Uses:  make(map[*ast.Ident]types.Object),
				Defs:  make(map[*ast.Ident]types.Object),
			}
			pkg, err := conf.Check("test", fset, []*ast.File{f}, info)
			if err != nil {
				t.Fatal(err)
			}

			spkg := &source.Package{
				Info:    *info,
				Name:    pkg.Name(),
				FileSet: fset,
			}

			// Find the first non-package declaration
			var expr ast.Expr
			ast.Inspect(f, func(n ast.Node) bool {
				if e, ok := n.(ast.Expr); ok && n != f {
					expr = e
					return false
				}
				return true
			})

			if expr == nil {
				t.Fatal("no expression found")
			}

			got := analyzeEscape(spkg, expr)
			if got.Kind != tt.wantKind {
				t.Errorf("analyzeEscape() = %v, want %v", got.Kind, tt.wantKind)
			}
		})
	}
}
