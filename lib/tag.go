package lib

import (
	"strconv"
	"strings"
	"text/scanner"
)

// Tag includes a symbol name along with the type of the symbol.
// Format specification is documented in the package documentation.
type Tag string

const (
	ErrTagMissingType errorString = "missing type information"
)

type errorString string

func (e errorString) Error() string { return string(e) }

// SyntaxError for a [Tag].
type SyntaxError struct {
	Tag Tag
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

// Type within a [Tag].
type Type struct {
	Name string

	Func *Type  // return type.
	Args []Type // arguments (if function)

	Hash bool       // immutablity marker, true if preceded by '#'
	Free rune       // ownership assertion, one of '$', '&', '*', '+' or '-'
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

// Parse returns a structured representation of
// the symbols and type defined in the tag.
func (tag Tag) Parse() ([]string, Type, error) {
	symbols, stype, ok := strings.Cut(string(tag), " ")
	if !ok {
		return nil, Type{}, ErrTagMissingType
	}
	var scan scanner.Scanner
	scan.Init(strings.NewReader(stype))
	ctype, err := tag.parseType(&scan, strings.Index(string(tag), " "))
	if err != nil {
		return nil, Type{}, err
	}
	return strings.Split(symbols, ","), ctype, nil
}

func (tag Tag) argument(scan *scanner.Scanner, pos int) (Argument, error) {
	var arg Argument
	arg.Check = true
	tok := scan.Scan()
	switch tok {
	case '@':
		tok = scan.Scan()
		if tok != scanner.Int {
			return arg, SyntaxError{
				Tag: Tag(tag),
				Pos: pos + scan.Pos().Column,
				Err: errorString("expected integer literal"),
			}
		}
		value, err := strconv.ParseInt(scan.TokenText(), 10, 8)
		if err != nil {
			return arg, SyntaxError{
				Tag: Tag(tag),
				Pos: pos + scan.Pos().Column,
				Err: errorString("expected integer literal"),
			}
		}
		arg.Index = uint8(value)
	case scanner.Ident:
		arg.Const = scan.TokenText()
	case scanner.Int:
		value, err := strconv.ParseInt(scan.TokenText(), 10, 64)
		if err != nil {
			return arg, SyntaxError{
				Tag: Tag(tag),
				Pos: pos + scan.Pos().Column,
				Err: errorString("expected integer literal"),
			}
		}
		arg.Value = value
	default:
		return arg, SyntaxError{
			Tag: Tag(tag),
			Pos: pos + scan.Pos().Column,
			Err: errorString("unexpected token " + scan.TokenText() + " (expecting argument)"),
		}
	}
	return arg, nil
}

func (tag Tag) parseType(scan *scanner.Scanner, pos int) (Type, error) {
	var (
	//err error
	)
	scan.Error = func(_ *scanner.Scanner, msg string) {}
	var (
		stype Type
	)
	tok := scan.Scan()
	switch tok {
	case '$', '&', '*', '+', '-':
		stype.Free = tok
	case '#':
		stype.Hash = true
	case scanner.Ident:
		stype.Name = scan.TokenText()
	case scanner.EOF:
		return stype, SyntaxError{
			Tag: Tag(tag),
			Pos: pos + scan.Pos().Column,
			Err: errorString("unexpected end of tag, expected type"),
		}
	default:
		return stype, SyntaxError{
			Tag: Tag(tag),
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
				Tag: Tag(tag),
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
				Tag: Tag(tag),
				Pos: pos + scan.Pos().Column,
				Err: errorString("expected '('"),
			}
		}
		for {
			if scan.Peek() == ')' {
				scan.Scan()
				break
			}
			if scan.Peek() == scanner.EOF {
				return stype, SyntaxError{
					Tag: Tag(tag),
					Pos: pos + scan.Pos().Column,
					Err: errorString("unexpected end of tag, expected ')'"),
				}
			}
			var arg Type
			arg, err := tag.parseType(scan, pos)
			if err != nil {
				return stype, err
			}
			arg.Maps = len(stype.Args) + 1
			stype.Args = append(stype.Args, arg)

			if scan.Peek() != ',' && scan.Peek() != ')' {
				return stype, SyntaxError{
					Tag: Tag(tag),
					Pos: pos + scan.Pos().Column + 1,
					Err: errorString("unexpected token, expected ',' or ')'"),
				}
			}
			if scan.Peek() == ',' {
				scan.Scan()
			}
		}
		ret, err := tag.parseType(scan, pos)
		if err != nil {
			return stype, err
		}
		stype.Func = &ret

		if scan.Peek() == ';' {
			scan.Scan()
			if scan.Scan() != scanner.Ident {
				return stype, SyntaxError{
					Tag: Tag(tag),
					Pos: pos + scan.Pos().Column,
					Err: errorString("expected symbol name"),
				}
			}
			stype.Call.Name = scan.TokenText()
			tok = scan.Scan()
			if tok != '(' {
				return stype, SyntaxError{
					Tag: Tag(tag),
					Pos: pos + scan.Pos().Column,
					Err: errorString("expected '('"),
				}
			}
			for {
				if scan.Peek() == ')' {
					scan.Scan()
					break
				}
				if scan.Peek() == scanner.EOF {
					return stype, SyntaxError{
						Tag: Tag(tag),
						Pos: pos + scan.Pos().Column,
						Err: errorString("unexpected end of tag, expected ')'"),
					}
				}
				var arg Argument
				arg, err := tag.argument(scan, pos)
				if err != nil {
					return stype, err
				}
				stype.Call.Args = append(stype.Call.Args, arg)
				if scan.Peek() != ',' && scan.Peek() != ')' {
					return stype, SyntaxError{
						Tag: Tag(tag),
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
	switch scan.Peek() {
	case scanner.EOF, ',', ')':
		return stype, nil
	case '[':
		stype.Test.Capacity = true
		scan.Scan()
	case '*':
		for scan.Peek() == '*' {
			scan.Scan()
			stype.Test.Indirect++
		}
	case '.':
		stype.More = true
		for i := 0; i < 3; i++ {
			if scan.Scan() != '.' {
				return stype, SyntaxError{
					Tag: Tag(tag),
					Pos: pos + scan.Pos().Column,
					Err: errorString("expected '...'"),
				}
			}
		}
	}
	if scan.Peek() == '!' {
		stype.Test.Inverted = true
		scan.Scan()
	}
	tok = scan.Scan()
	//tokPos := pos + scan.Pos().Column
	arg, err := tag.argument(scan, pos)
	if err != nil {
		return stype, err
	}
	switch tok {
	case '>', '<':
		if scan.Peek() == '=' {
			stype.Test.Equality = arg
			scan.Scan()
		}
		if tok == '>' {
			stype.Test.MoreThan = arg
		} else {
			stype.Test.LessThan = arg
		}
	case '=':
		stype.Test.Equality = arg
	case ':':
		stype.Test.SameType = arg
	case '~':
		stype.Test.Inverted = !stype.Test.Inverted
		stype.Test.Overlaps = arg
	case '^':
		stype.Test.Lifetime = arg
	case '?':
		stype.Test.OfFormat = arg
	default:
		return stype, SyntaxError{
			Tag: Tag(tag),
			Pos: pos + scan.Pos().Column,
			Err: errorString("expected assertion"),
		}
	}
	if stype.Test.Capacity && scan.Scan() != ']' {
		return stype, SyntaxError{
			Tag: Tag(tag),
			Pos: pos + scan.Pos().Column,
			Err: errorString("expected ']'"),
		}
	}
	return stype, nil
}
