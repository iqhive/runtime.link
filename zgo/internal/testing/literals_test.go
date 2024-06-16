package main

import "testing"

/*
Integer literals

An integer literal is a sequence of digits representing an integer constant. An optional
prefix sets a non-decimal base: 0b or 0B for binary, 0, 0o, or 0O for octal, and 0x or 0X
for hexadecimal [Go 1.13]. A single 0 is considered a decimal zero. In hexadecimal literals,
letters a through f and A through F represent values 10 through 15.

For readability, an underscore character _ may appear after a base prefix or between
successive digits; such underscores do not change the literal's value.
*/
func TestIntegerLiterals(t *testing.T) {
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
}
