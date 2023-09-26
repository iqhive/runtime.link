// Package gnu provides options for GNU-extended POSIX commands.
package gnu

import (
	"runtime.link/cmd/posix"
)

// CommonFunctions supported by all GNU-extended commands.
type CommonFunctions struct {
	Help bool `cmd:"--help"
		returns usage information. `
	Version bool `cmd:"--version"
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

	LineNumbers bool `cmd:"--number"
		are added to the beginning of each line.`
	LineNumbersSkipEmpty bool `cmd:"--number-nonblank"
		adds lines numbers to each line, unless the line 
		is empty.`
	ShowEnds bool `cmd:"--show-ends"
		adds a '$' to the end of each line.`
	Squeeze bool `cmd:"--squeeze-blank"
		adjacent empty lines into one.`
	EscapeTabs bool `cmd:"--show-tabs"
		replaces tabs with '^I'.`
	EscapeBinary bool `cmd:"--show-nonprinting"
		uses ^ and M- notation, except for LFD and TAB.`
}
