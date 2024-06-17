package main

import "testing"

func TestIntegerLiterals(t *testing.T) {
	/*
		An integer literal is a sequence of digits representing an integer constant. An optional
		prefix sets a non-decimal base: 0b or 0B for binary, 0, 0o, or 0O for octal, and 0x or 0X
		for hexadecimal [Go 1.13]. A single 0 is considered a decimal zero. In hexadecimal literals,
		letters a through f and A through F represent values 10 through 15.

		For readability, an underscore character _ may appear after a base prefix or between
		successive digits; such underscores do not change the literal's value.
	*/
	const (
		_ = 42
		_ = 4_2
		_ = 0600
		_ = 0_600
		_ = 0o600
		_ = 0xBadFace
		_ = 0xBad_Face
		_ = 0x_67_7a_2f_cc_40_c6
		_ = 170141183460469231731687303715884105727
		_ = 170_141183_460469_231731_687303_715884_105727
	)
	_ = 0b1
	_ = 0b_11
}

func TestFloatingPointLiterals(t *testing.T) {
	/*
		A floating-point literal is a decimal or hexadecimal representation of a floating-point
		constant.

		A decimal floating-point literal consists of an integer part (decimal digits), a decimal
		point, a fractional part (decimal digits), and an exponent part (e or E followed by an
		optional sign and decimal digits). One of the integer part or the fractional part may be
		elided; one of the decimal point or the exponent part may be elided. An exponent value
		exp scales the mantissa (integer and fractional part) by 10exp.

		A hexadecimal floating-point literal consists of a 0x or 0X prefix, an integer part
		(hexadecimal digits), a radix point, a fractional part (hexadecimal digits), and an
		exponent part (p or P followed by an optional sign and decimal digits). One of the integer
		part or the fractional part may be elided; the radix point may be elided as well, but the
		exponent part is required. (This syntax matches the one given in IEEE 754-2008 §5.12.3.)
		An exponent value exp scales the mantissa (integer and fractional part) by 2exp [Go 1.13].

		For readability, an underscore character _ may appear after a base prefix or between
		successive digits; such underscores do not change the literal value.
	*/
	var (
		_ = 0.
		_ = 72.40
		_ = 072.40 // == 72.40
		_ = 2.71828
		_ = 1.e+0
		_ = 6.67428e-11
		_ = 1e6
		_ = .25
		_ = .12345e+5
		_ = 1_5.      // == 15.0
		_ = 0.15e+0_2 // == 15.0

		_ = 0x1p-2      // == 0.25
		_ = 0x2.p10     // == 2048.0
		_ = 0x1.Fp+0    // == 1.9375
		_ = 0x.8p-0     // == 0.5
		_ = 0x_1FFFp-16 // == 0.1249847412109375
		_ = 0x15e - 2   // == 0x15e - 2 (integer subtraction)
	)
}

func TestImaginaryLiterals(t *testing.T) {
	/*
		An imaginary literal represents the imaginary part of a complex constant. It
		consists of an integer or floating-point literal followed by the lowercase
		letter i. The value of an imaginary literal is the value of the respective
		integer or floating-point literal multiplied by the imaginary unit i [Go 1.13]

		For backward compatibility, an imaginary literal's integer part consisting
		entirely of decimal digits (and possibly underscores) is considered a decimal
		integer, even if it starts with a leading 0.
	*/
	var (
		_ = 0i
		_ = 0o123i // == 0o123 * 1i == 83i
		_ = 0xabci // == 0xabc * 1i == 2748i
		_ = 0.i
		_ = 2.71828i
		_ = 1.e+0i
		_ = 6.67428e-11i
		_ = 1e6i
		_ = .25i
		_ = .12345e+5i
		_ = 0x1p-2i // == 0x1p-2 * 1i == 0.25i
	)
}

func TestRuneLiterals(t *testing.T) {
	/*
		A rune literal represents a rune constant, an integer value identifying a Unicode code point.
		A rune literal is expressed as one or more characters enclosed in single quotes, as in 'x'
		or '\n'. Within the quotes, any character may appear except newline and unescaped single quote.
		A single quoted character represents the Unicode value of the character itself, while
		multi-character sequences beginning with a backslash encode values in various formats.

		The simplest form represents the single character within the quotes; since Go source text is
		Unicode characters encoded in UTF-8, multiple UTF-8-encoded bytes may represent a single
		integer value. For instance, the literal 'a' holds a single byte representing a literal a,
		Unicode U+0061, value 0x61, while 'ä' holds two bytes (0xc3 0xa4) representing a literal
		a-dieresis, U+00E4, value 0xe4.

		Several backslash escapes allow arbitrary values to be encoded as ASCII text. There are four
		ways to represent the integer value as a numeric constant: \x followed by exactly two
		hexadecimal digits; \u followed by exactly four hexadecimal digits; \U followed by exactly
		eight hexadecimal digits, and a plain backslash \ followed by exactly three octal digits.
		In each case the value of the literal is the value represented by the digits in the
		corresponding base.

		Although these representations all result in an integer, they have different valid ranges.
		Octal escapes must represent a value between 0 and 255 inclusive. Hexadecimal escapes satisfy
		this condition by construction. The escapes \u and \U represent Unicode code points so within
		them some values are illegal, in particular those above 0x10FFFF and surrogate halves.

		After a backslash, certain single-character escapes represent special values:

			\a   U+0007 alert or bell
			\b   U+0008 backspace
			\f   U+000C form feed
			\n   U+000A line feed or newline
			\r   U+000D carriage return
			\t   U+0009 horizontal tab
			\v   U+000B vertical tab
			\\   U+005C backslash
			\'   U+0027 single quote  (valid escape only within rune literals)
			\"   U+0022 double quote  (valid escape only within string literals)

		An unrecognized character following a backslash in a rune literal is illegal.
	*/
	var (
		_ = 'a'
		_ = 'ä'
		_ = '本'
		_ = '\t'
		_ = '\000'
		_ = '\007'
		_ = '\377'
		_ = '\x07'
		_ = '\xff'
		_ = '\u12e4'
		_ = '\U00101234'
	)
}

func TestStringLiterals(t *testing.T) {
	/*
		A string literal represents a string constant obtained from concatenating a sequence
		of characters. There are two forms: raw string literals and interpreted string literals.

		Raw string literals are character sequences between back quotes, as in `foo`. Within
		the quotes, any character may appear except back quote. The value of a raw string
		literal is the string composed of the uninterpreted (implicitly UTF-8-encoded) characters
		between the quotes; in particular, backslashes have no special meaning and the string
		may contain newlines. Carriage return characters ('\r') inside raw string literals are
		discarded from the raw string value.

		Interpreted string literals are character sequences between double quotes, as in "bar".
		Within the quotes, any character may appear except newline and unescaped double quote.
		The text between the quotes forms the value of the literal, with backslash escapes
		interpreted as they are in rune literals (except that \' is illegal and \" is legal),
		with the same restrictions. The three-digit octal (\nnn) and two-digit hexadecimal (\xnn)
		escapes represent individual bytes of the resulting string; all other escapes represent
		the (possibly multi-byte) UTF-8 encoding of individual characters. Thus inside a string
		literal \377 and \xFF represent a single byte of value 0xFF=255, while ÿ, \u00FF,
		\U000000FF and \xc3\xbf represent the two bytes 0xc3 0xbf of the UTF-8 encoding of
		character U+00FF.
	*/
	var (
		_ = `abc` // same as "abc"
		_ = `\n
\n` // same as "\\n\n\\n"
		_ = "\n"
		_ = "\"" // same as `"`
		_ = "Hello, world!\n"
		_ = "日本語"
		_ = "\u65e5本\U00008a9e"
		_ = "\xff\u00FF"
	)
	// These examples all represent the same string:
	var (
		_ = "日本語"                                  // UTF-8 input text
		_ = `日本語`                                  // UTF-8 input text as a raw literal
		_ = "\u65e5\u672c\u8a9e"                   // the explicit Unicode code points
		_ = "\U000065e5\U0000672c\U00008a9e"       // the explicit Unicode code points
		_ = "\xe6\x97\xa5\xe6\x9c\xac\xe8\xaa\x9e" // the explicit UTF-8 bytes
	)
	/*
		If the source code represents a character as two code points, such as a combining
		form involving an accent and a letter, the result will be an error if placed in a
		rune literal (it is not a single code point), and will appear as two code points
		if placed in a string literal.
	*/
}
