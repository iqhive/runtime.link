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
		
		// Check for channel operations that require immutability
		if obj := pkg.Uses[node.Sel]; obj != nil {
			if _, ok := obj.Type().Underlying().(*types.Chan); ok {
				info.Kind = escape.GoroutineEscape
				info.Reason = "value used with channel (must remain immutable)"
			}
		}
	}

	return info
}

// Helper functions to check context
// isInDeferredFunc checks if a node is within a deferred function call
func isInDeferredFunc(pkg *source.Package, node ast.Node) bool {
	var inDefer bool
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			return true
		}
		if d, ok := n.(*ast.DeferStmt); ok {
			if d.Call.Fun == node || containsNode(d.Call.Fun, node) {
				inDefer = true
				return false
			}
		}
		return true
	})
	return inDefer
}

// isGlobalVar checks if a selector expression refers to a global variable
func isGlobalVar(pkg *source.Package, node *ast.SelectorExpr) bool {
	if obj := pkg.Uses[node.Sel]; obj != nil {
		if v, ok := obj.(*types.Var); ok {
			return v.Parent() != nil && v.Parent().Parent() == types.Universe
		}
	}
	return false
}

// isInInitFunc checks if a node is within an init function
func isInInitFunc(pkg *source.Package, node ast.Node) bool {
	var inInit bool
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			return true
		}
		if f, ok := n.(*ast.FuncDecl); ok {
			if f.Name.Name == "init" {
				inInit = true
				return false
			}
		}
		return true
	})
	return inInit
}

// containsNode checks if parent contains child node
func containsNode(parent, child ast.Node) bool {
	var found bool
	ast.Inspect(parent, func(n ast.Node) bool {
		if n == child {
			found = true
			return false
		}
		return true
	})
	return found
}

// updateEscapeInfo updates the escape info for a typed node
func updateEscapeInfo(pkg *source.Package, node ast.Expr, typed *source.Typed) {
	info := analyzeEscape(pkg, node)
	if info.Kind > typed.EscapeInfo.Kind {
		typed.EscapeInfo = info
	}
}
