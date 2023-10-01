// Package gnu provides options for GNU-extended POSIX commands.
package gnu

import (
	"runtime.link/cmd/std/posix"
)

// CommonFunctions supported by all GNU-extended commands.
type CommonFunctions struct {
	Help bool `args:"--help"
		returns usage information. `
	Version bool `args:"--version"
		returns the version number.`
}

// Cat defined by the GNU standard.
type Cat struct {
	posix.StandardCatCommand[CatOptions]
	CommonFunctions
}

// CatOptions for the [cat.Command].
type CatOptions struct {
	posix.CatOptions

	CommonFunctions

	LineNumbers bool `args:"--number"
		are added to the beginning of each line.`
	LineNumbersSkipEmpty bool `args:"--number-nonblank"
		adds lines numbers to each line, unless the line 
		is empty.`
	ShowEnds bool `args:"--show-ends"
		adds a '$' to the end of each line.`
	Squeeze bool `args:"--squeeze-blank"
		adjacent empty lines into one.`
	EscapeTabs bool `args:"--show-tabs"
		replaces tabs with '^I'.`
	EscapeBinary bool `args:"--show-nonprinting"
		uses ^ and M- notation, except for LFD and TAB.`
}
