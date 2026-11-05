package ffi

import (
	"strconv"
	"strings"
	"text/scanner"
	"unicode"

	"github.com/iqhive/runtime.link/api/xray"
)

const (
	ErrTagMissingType errorString = "missing type information"
)

type errorString string

func (e errorString) Error() string { return string(e) }

// SyntaxError for a [Tag].
type SyntaxError struct {
	Tag string
	Pos int
	Err error
}

func (e SyntaxError) Error() string {
	var b strings.Builder
	b.WriteString("syntax error\n\n")
	b.WriteString(string(e.Tag))
	b.WriteString("\n")
	for i := 0; i < e.Pos-1; i++ {
		b.WriteString(" ")
	}
	b.WriteString("^ ")
	b.WriteString("\n")
	for i := 0; i < e.Pos-1; i++ {
		b.WriteString(" ")
	}
	b.WriteString("| ")
	b.WriteString("\n")
	for i := 0; i < e.Pos-1; i++ {
		b.WriteString(" ")
	}
	b.WriteString("└──")
	b.WriteString(e.Err.Error())
	return b.String()
}

// Type structure.
type Type struct {
	Name string

	Func *Type  // return type.
	Args []Type // arguments (if function)

	Hash bool       // immutablity marker, true if preceded by '#'
	Free rune       // ownership assertion, one of '$', '&', '*', '~', '+' or '-'
	Test Assertions // memory safety assertions
	Call Call       // symbol to lookup on failure (if function)
	More bool       // varaidic

	Maps int // index of the Go argument that is mapped to this value.
}

// Call represents a function to call on failure
// within a [Tag], to return information about why
// the assertion failed.
type Call struct {
	Name string     // symbol name
	Args []Argument // arguments to pass to the function
}

// Assertions for a [Type] within a [Tag].
type Assertions struct {
	Capacity bool     // []
	Inverted bool     // !
	Indirect int      // /
	Lifetime Argument // ^
	Overlaps Argument // ~
	SameType Argument // :
	Equality Argument // =
	MoreThan Argument // >
	LessThan Argument // <
	OfFormat Argument // f
}

// Argument for a [Type] assertion within a [Tag].
type Argument struct {
	Check bool   // should this assertion be checked?
	Index uint8  // of the argument being referred to. if greater than zero ignore const and value.
	Const string // C standard constant (or supported macro) name.
	Value int64  // integer value
}

// ParseTag returns a structured representation of
// the symbols and type defined in the tag.
func ParseTag(tag string) ([]string, Type, error) {
	symbols, stype, ok := strings.Cut(string(tag), " ")
	if !ok {
		return nil, Type{}, xray.New(ErrTagMissingType)
	}
	var scan scanner.Scanner
	scan.Init(strings.NewReader(stype))
	pos := strings.Index(string(tag), " ")
	ctype, err := parseType(tag, &scan, pos)
	if err != nil {
		return nil, Type{}, xray.New(err)
	}
	if !ignoredAfterType(skipSpace(&scan)) {
		return nil, Type{}, xray.New(SyntaxError{
			Tag: tag,
			Pos: pos + scan.Pos().Column + 1,
			Err: errorString("unexpected token after type"),
		})
	}
	return strings.Split(symbols, ","), ctype, nil
}

// ignoredAfterType reports whether the rest of a tag, which starts with r,
// is ignored. 840f71d stopped at a ',' or ')' after the type, and tags rely on it.
func ignoredAfterType(r rune) bool {
	return r == scanner.EOF || r == ',' || r == ')'
}

// skipSpace matters because [scanner.Scanner.Peek] looks at the next character
// before whitespace has been skipped, while the parser dispatches on it.
func skipSpace(scan *scanner.Scanner) rune {
	for scan.Peek() == ' ' || scan.Peek() == '\t' || scan.Peek() == '\n' || scan.Peek() == '\r' {
		scan.Next()
	}
	return scan.Peek()
}

// endOfType reports whether r can follow a complete type, so that
// no assertion needs parsing.
func endOfType(r rune) bool {
	return r == scanner.EOF || r == ',' || r == ')' || r == ';' || r == ']'
}

// startsArgument reports whether r begins an [Argument] with no operator before it.
func startsArgument(r rune) bool {
	return r == '@' || r == '_' || unicode.IsDigit(r) || unicode.IsLetter(r)
}

func argument(tag string, scan *scanner.Scanner, pos int) (Argument, error) {
	var arg Argument
	arg.Check = true
	tok := scan.Scan()
	negative := tok == '-'
	if negative {
		tok = scan.Scan()
		if tok != scanner.Int {
			return arg, xray.New(SyntaxError{
				Tag: tag,
				Pos: pos + scan.Pos().Column,
				Err: errorString("expected integer literal after '-'"),
			})
		}
	}
	switch tok {
	case '@':
		tok = scan.Scan()
		if tok != scanner.Int {
			return arg, SyntaxError{
				Tag: tag,
				Pos: pos + scan.Pos().Column,
				Err: errorString("expected integer literal"),
			}
		}
		value, err := strconv.ParseInt(scan.TokenText(), 10, 8)
		if err != nil {
			return arg, xray.New(SyntaxError{
				Tag: tag,
				Pos: pos + scan.Pos().Column,
				Err: errorString("expected integer literal"),
			})
		}
		arg.Index = uint8(value)
	case scanner.Ident:
		arg.Const = scan.TokenText()
	case scanner.Int:
		value, err := strconv.ParseInt(scan.TokenText(), 0, 64)
		if err != nil {
			return arg, xray.New(SyntaxError{
				Tag: tag,
				Pos: pos + scan.Pos().Column,
				Err: errorString("expected integer literal"),
			})
		}
		arg.Value = value
		if negative {
			arg.Value = -value
		}
	default:
		return arg, xray.New(SyntaxError{
			Tag: tag,
			Pos: pos + scan.Pos().Column,
			Err: errorString("unexpected token " + scan.TokenText() + " (expecting argument)"),
		})
	}
	return arg, nil
}

func parseType(tag string, scan *scanner.Scanner, pos int) (Type, error) {
	var (
	//err error
	)
	scan.Error = func(_ *scanner.Scanner, msg string) {}
	var (
		stype Type
	)
	tok := scan.Scan()
	switch tok {
	case '$', '&', '*', '~', '+', '-':
		stype.Free = tok
	case '#':
		stype.Hash = true
	case scanner.Ident:
		stype.Name = scan.TokenText()
	case scanner.EOF:
		return stype, SyntaxError{
			Tag: tag,
			Pos: pos + scan.Pos().Column,
			Err: errorString("unexpected end of tag, expected type"),
		}
	default:
		return stype, SyntaxError{
			Tag: tag,
			Pos: pos + scan.Pos().Column,
			Err: errorString("unexpected character " + string(scan.TokenText()) + " (expecting ownership assertion or type name)"),
		}
	}
	if stype.Name == "" {
		if !stype.Hash && scan.Peek() == '#' {
			stype.Hash = true
			scan.Scan()
		}
		tok = scan.Scan()
		if tok != scanner.Ident {
			return stype, SyntaxError{
				Tag: tag,
				Pos: pos + scan.Pos().Column,
				Err: errorString("expected type name"),
			}
		}
		stype.Name = scan.TokenText()
	}
	if stype.Name == "func" {
		tok = scan.Scan()
		if tok != '(' {
			return stype, SyntaxError{
				Tag: tag,
				Pos: pos + scan.Pos().Column,
				Err: errorString("expected '('"),
			}
		}
		var mapped int
		for {
			if skipSpace(scan) == ')' {
				scan.Scan()
				break
			}
			if scan.Peek() == scanner.EOF {
				return stype, SyntaxError{
					Tag: tag,
					Pos: pos + scan.Pos().Column,
					Err: errorString("unexpected end of tag, expected ')'"),
				}
			}
			var arg Type
			arg, err := parseType(tag, scan, pos)
			if err != nil {
				return stype, xray.New(err)
			}
			// a '-' parameter has no Go argument, so it must not shift the ones after it.
			if arg.Free != '-' {
				mapped++
				arg.Maps = mapped
			}
			stype.More = stype.More || arg.More
			stype.Args = append(stype.Args, arg)

			if skipSpace(scan) != ',' && scan.Peek() != ')' {
				return stype, SyntaxError{
					Tag: tag,
					Pos: pos + scan.Pos().Column + 1,
					Err: errorString("unexpected token, expected ',' or ')'"),
				}
			}
			if scan.Peek() == ',' {
				scan.Scan()
			}
		}
		if !endOfType(skipSpace(scan)) {
			ret, err := parseType(tag, scan, pos)
			if err != nil {
				return stype, xray.New(err)
			}
			stype.Func = &ret
		}
		if skipSpace(scan) == ';' {
			scan.Scan()
			if scan.Scan() != scanner.Ident {
				return stype, SyntaxError{
					Tag: tag,
					Pos: pos + scan.Pos().Column,
					Err: errorString("expected symbol name"),
				}
			}
			stype.Call.Name = scan.TokenText()
			tok = scan.Scan()
			if tok != '(' {
				return stype, SyntaxError{
					Tag: tag,
					Pos: pos + scan.Pos().Column,
					Err: errorString("expected '('"),
				}
			}
			for {
				if skipSpace(scan) == ')' {
					scan.Scan()
					break
				}
				if scan.Peek() == scanner.EOF {
					return stype, SyntaxError{
						Tag: tag,
						Pos: pos + scan.Pos().Column,
						Err: errorString("unexpected end of tag, expected ')'"),
					}
				}
				var arg Argument
				arg, err := argument(tag, scan, pos)
				if err != nil {
					return stype, xray.New(err)
				}
				stype.Call.Args = append(stype.Call.Args, arg)
				if skipSpace(scan) != ',' && scan.Peek() != ')' {
					return stype, SyntaxError{
						Tag: tag,
						Pos: pos + scan.Pos().Column + 1,
						Err: errorString("unexpected token, expected ',' or ')"),
					}
				}
				if scan.Peek() == ',' {
					scan.Scan()
				}
			}
		}
	}
	if err := parseAssertions(tag, scan, pos, &stype); err != nil {
		return stype, err
	}
	return stype, nil
}

// parseAssertions parses whatever follows a type's name. A type carries one
// of the suffixes '...', '*' and '[x]' and at most one assertion.
func parseAssertions(tag string, scan *scanner.Scanner, pos int, stype *Type) error {
	switch skipSpace(scan) {
	case '.':
		stype.More = true
		for i := 0; i < 3; i++ {
			if scan.Scan() != '.' {
				return SyntaxError{
					Tag: tag,
					Pos: pos + scan.Pos().Column,
					Err: errorString("expected '...'"),
				}
			}
		}
		return nil
	case '*':
		for scan.Peek() == '*' {
			scan.Scan()
			stype.Test.Indirect++
		}
		if startsArgument(skipSpace(scan)) {
			arg, err := argument(tag, scan, pos)
			if err != nil {
				return xray.New(err)
			}
			stype.Test.Equality = arg
			return nil
		}
	}
	if endOfType(skipSpace(scan)) {
		return nil
	}
	if scan.Peek() == '!' {
		stype.Test.Inverted = true
		scan.Scan()
	}
	if skipSpace(scan) != '[' {
		return parseAssertion(tag, scan, pos, stype)
	}
	scan.Scan()
	stype.Test.Capacity = true
	var err error
	switch {
	case startsArgument(skipSpace(scan)):
		stype.Test.MoreThan, err = argument(tag, scan, pos)
	default:
		err = parseAssertion(tag, scan, pos, stype)
	}
	if err != nil {
		return xray.New(err)
	}
	if scan.Scan() != ']' {
		return SyntaxError{
			Tag: tag,
			Pos: pos + scan.Pos().Column,
			Err: errorString("expected ']'"),
		}
	}
	return nil
}

func parseAssertion(tag string, scan *scanner.Scanner, pos int, stype *Type) error {
	if scan.Peek() == '!' {
		stype.Test.Inverted = !stype.Test.Inverted
		scan.Scan()
	}
	tok := scan.Scan()
	orEqual := (tok == '>' || tok == '<') && scan.Peek() == '='
	if orEqual {
		scan.Scan()
	}
	arg, err := argument(tag, scan, pos)
	if err != nil {
		return xray.New(err)
	}
	// 840f71d's form of "or equal" has the '=' after the argument, as in '>@1='.
	if (tok == '>' || tok == '<') && !orEqual && scan.Peek() == '=' {
		scan.Scan()
		orEqual = true
	}
	if orEqual {
		stype.Test.Equality = arg
	}
	switch tok {
	case '>':
		stype.Test.MoreThan = arg
	case '<':
		stype.Test.LessThan = arg
	case '=':
		stype.Test.Equality = arg
	case ':':
		stype.Test.SameType = arg
	case '|':
		stype.Test.Overlaps = arg
	case '~':
		stype.Test.Inverted = !stype.Test.Inverted
		stype.Test.Overlaps = arg
	case '^':
		stype.Test.Lifetime = arg
	case '?':
		stype.Test.OfFormat = arg
	default:
		return SyntaxError{
			Tag: tag,
			Pos: pos + scan.Pos().Column,
			Err: errorString("expected assertion"),
		}
	}
	return nil
}
