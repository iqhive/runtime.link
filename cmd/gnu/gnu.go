// Package gnu provides representations of the GNU coreutils.
package gnu

import (
	"context"

	"runtime.link/box"
	"runtime.link/ffi"
)

type Common struct {
	Help func() string `cmd:"--help"
		returns usage information. `
	Version func() string `cmd:"--version"
		returns the version number.`
}

// Arch command.
type Arch struct {
	host ffi.Host `cmd:"arch"`
	Common
	Get func() string `cmd:""
		prints the machine hardware name, and is equivalent to 'uname -m'.`
}

// Cat command.
type Cat struct {
	ffi.Documentation `
		concatenates files together and outputs the result.`

	host   ffi.Host `cmd:"cat"`
	darwin ffi.Host `cmd:"gcat"`

	Common

	Print  func(ctx context.Context, options *Concatenation, files ...string) error                     `cmd:"%v -- %v"`
	Chan   func(ctx context.Context, options *Concatenation, files ...string) (chan []byte, chan error) `cmd:"%v -- %v"`
	String func(ctx context.Context, options *Concatenation, files ...string) (string, error)           `cmd:"%v -- %v"`
	Bytes  func(ctx context.Context, options *Concatenation, files ...string) ([]byte, error)           `cmd:"%v -- %v"`
}

// Concatenation options for [Cat].
type Concatenation struct {
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

// Tac command.
type Tac struct {
	ffi.Documentation `
		concatenates files together and outputs the result in reverse line-order.`

	host   ffi.Host `cmd:"tac"`
	darwin ffi.Host `cmd:"gtac"`

	Common

	Print  func(ctx context.Context, options *ReverseConcatenation, files ...string) error                     `cmd:"%v -- %v"`
	Chan   func(ctx context.Context, options *ReverseConcatenation, files ...string) (chan []byte, chan error) `cmd:"%v -- %v"`
	String func(ctx context.Context, options *ReverseConcatenation, files ...string) (string, error)           `cmd:"%v -- %v"`
	Bytes  func(ctx context.Context, options *ReverseConcatenation, files ...string) ([]byte, error)           `cmd:"%v -- %v"`
}

// ReverseConcatenation for [Tac].
type ReverseConcatenation struct {
	PrefixSeperator bool `cmd:"--before"
		adds the seperator before instead of after.`
	LineSeperator string `cmd:"--separator=%v,omitempty"
		uses the given string to seperate 'lines'.`
	LineIsRegex bool `cmd:"--regex"
		interprets the seperator as a regular expression.`
}

// NumberLines command.
type NumberLines struct {
	ffi.Documentation `
		adds a line number column to files and outputs the result.
		
		Input files are split into pages and sections, by default 
		they are delimted by the following strings (exact line match):

			'\:\:\:' - header
			'\:\:'   - body
			'\:'     - footer

		These lines are not included in the output.`

	host   ffi.Host `cmd:"nl"`
	darwin ffi.Host `cmd:"gnl"`

	Print  func(ctx context.Context, options *LineNumbering, files ...string) error                     `cmd:"%v -- %v"`
	Chan   func(ctx context.Context, options *LineNumbering, files ...string) (chan []byte, chan error) `cmd:"%v -- %v"`
	String func(ctx context.Context, options *LineNumbering, files ...string) (string, error)           `cmd:"%v -- %v"`
	Bytes  func(ctx context.Context, options *LineNumbering, files ...string) ([]byte, error)           `cmd:"%v -- %v"`
}

type LineNumbering struct {
	StartAt int `cmd:"--starting-line-number=%v,omitempty"
		is the line number to begin from.`
	Padding LineNumberPadding `cmd:"--number-format=%v,omitempty"
		rule to apply to the line number column.`
	ColumnWidth int `cmd:"--number-width=%v,omitempty"
		is the width of the line number column.`

	Header LineNumberingRule `cmd:"--header-numbering=%v,omitempty"
		is the rule to apply to the header.`
	Body LineNumberingRule `cmd:"--body-numbering=%v,omitempty"
		is the rule to apply to the body.`
	Footer LineNumberingRule `cmd:"--footer-numbering=%v,omitempty"
		is the rule to apply to the footer.`

	IncrementBy int `cmd:"--line-increment=%v,omitempty"
		is the amount to increment the line number by for each line.`
	JoinEmpty bool `cmd:"--join-blank-lines=%v,omitempty"
		joins empty lines together as a single number.`

	SectionDelimiter string `cmd:"--section-delimiter=%v,omitempty"
		changes the string used for deliminating the header, body and 
		footer section, ie. the default is '\:\:'.`
	DisableReset bool `cmd:"--no-renumber"
		disables renumbering of line numbers for each page.`

	Indentation string `cmd:"--number-separator==%v,omitempty"
		is the string used to seperate the line number column from the line.`
}

type LineNumberingRule box.Variant[string, struct {
	Default   LineNumberingRule                   `text:""`
	Each      LineNumberingRule                   `text:"a"`
	SkipEmpty LineNumberingRule                   `text:"t"`
	Skip      LineNumberingRule                   `text:"n"`
	Regex     box.Vary[LineNumberingRule, string] `text:"p%v"`
}]

var LineNumbers = new(LineNumberingRule).Values()

type LineNumberPadding box.Variant[string, struct {
	Default                 LineNumberPadding `text:""`
	LeftWithNoLeadingZeros  LineNumberPadding `text:"ln"`
	RightWithNoLeadingZeros LineNumberPadding `text:"rn"`
	RightWithLeadingZeros   LineNumberPadding `text:"rz"`
}]

var PadLineNumbers = new(LineNumberPadding).Values()
