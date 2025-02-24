package parser

import (
	"go/ast"
	"go/token"
	"go/types"

	"runtime.link/zgo/internal/escape"
	"runtime.link/zgo/internal/source"
)

// analyzeEscape determines if a value escapes its current scope
func analyzeEscape(pkg *source.Package, expr ast.Expr) escape.Info {
	info := escape.Info{Kind: escape.NoEscape}

	switch node := expr.(type) {
	case *ast.UnaryExpr:
		// Check for channel operations
		if node.Op == token.ARROW {
			info.Kind = escape.GoroutineEscape
			info.Reason = "value passed through channel (must remain immutable)"
		}

	case *ast.CallExpr:
		// Check for goroutine spawning
		if sel, ok := node.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "go" {
			info.Kind = escape.GoroutineEscape
			info.Reason = "value captured by goroutine (must remain immutable)"
		}
		
		// Check if this is a channel send operation
		if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
			if sel.Sel.Name == "Send" || sel.Sel.Name == "Chan" {
				info.Kind = escape.GoroutineEscape
				info.Reason = "value sent through channel (must remain immutable)"
			}
		}

	case *ast.FuncLit:
		// Function literals may capture values
		info.Kind = escape.HeapEscape
		info.Reason = "function closure may capture values"

	case *ast.StarExpr:
		// Pointer operations require heap allocation
		info.Kind = escape.HeapEscape
		info.Reason = "value referenced by pointer"

	case *ast.TypeAssertExpr:
		// Interface values may require heap allocation
		if _, ok := pkg.TypeOf(node.X).Underlying().(*types.Interface); ok {
			info.Kind = escape.HeapEscape
			info.Reason = "value boxed in interface"
		}
	}

	return info
}

// updateEscapeInfo updates the escape info for a typed node
func updateEscapeInfo(pkg *source.Package, node ast.Expr, typed *source.Typed) {
	info := analyzeEscape(pkg, node)
	if info.Kind > typed.EscapeInfo.Kind {
		typed.EscapeInfo = info
	}
}
