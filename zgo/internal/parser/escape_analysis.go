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
		if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
			switch sel.Sel.Name {
			case "go":
				info.Kind = escape.GoroutineEscape
				info.Reason = "value captured by goroutine (must remain immutable)"
			case "Send", "Chan":
				info.Kind = escape.GoroutineEscape
				info.Reason = "value sent through channel (must remain immutable)"
			case "recover":
				// Rule 4: recover only valid in deferred functions in goroutines
				if !isInDeferredFunc(pkg, node) {
					info.Kind = escape.HeapEscape
					info.Reason = "recover called outside deferred function"
				}
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

	case *ast.SelectorExpr:
		// Rule 1: Global variables cannot be mutated after init
		if isGlobalVar(pkg, node) && !isInInitFunc(pkg, node) {
			info.Kind = escape.HeapEscape
			info.Reason = "global variable mutation outside init"
		}
	}

	return info
}

// Helper functions to check context
func isInDeferredFunc(pkg *source.Package, node ast.Node) bool {
	// TODO: Implement checking if node is within a deferred function
	return false
}

func isGlobalVar(pkg *source.Package, node *ast.SelectorExpr) bool {
	// TODO: Implement checking if selector refers to a global variable
	return false
}

func isInInitFunc(pkg *source.Package, node ast.Node) bool {
	// TODO: Implement checking if node is within an init function
	return false
}

// updateEscapeInfo updates the escape info for a typed node
func updateEscapeInfo(pkg *source.Package, node ast.Expr, typed *source.Typed) {
	info := analyzeEscape(pkg, node)
	if info.Kind > typed.EscapeInfo.Kind {
		typed.EscapeInfo = info
	}
}
