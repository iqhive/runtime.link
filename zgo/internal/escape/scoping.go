package escape

import "runtime.link/zgo/internal/source"

func isOuterScopeTo(node source.Node, value source.Node) bool {
	return true
}

func isGlobal(node source.Node) bool {
	return true
}

func isFunctionDefined(node source.Node) bool {
	return true
}
