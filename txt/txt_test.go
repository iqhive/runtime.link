package txt_test

import (
	"testing"

	"runtime.link/has"
	"runtime.link/may"
	"runtime.link/not"
	"runtime.link/txt"
)

func TestEmailAddress(t *testing.T) {
	type EmailAddress txt.Pattern[struct {
		not.Prefix `.`
		not.Suffix `.`

		Local txt.Scanner[struct {
			txt.Min `1`
			txt.Max `64`

			Quoted has.Prefix[
				has.This[struct {
					txt.Suffix `"`
					not.Middle `"\`
					txt.ASCII
				}],
				has.Else[struct {
					may.Backtick[may.Contain] `0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!#$%&'*+-/=?^_{|}~.`
				}],
			] `"`
		}] `@`
		Domain txt.Divide[struct {
			txt.Min     `1`
			txt.Max     `63`
			not.Prefix  `-`
			not.Suffix  `-`
			may.Contain `0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-`

			txt.Final[not.Only] `0123456789`
		}] `.`
	}]

	var email = txt.New[EmailAddress]("user@example.com")

	s, ok := email.Get()
	if !ok || s != "user@example.com" {
		t.Fatal("unexpected value!", email.Err())
	}

	if email.Format().Local.String() != "user" {
		t.Fatal("expected local")
	}
	domain := email.Format().Domain
	if domain.String() != "example.com" {
		t.Fatal("unexpected domain")
	}
	if len(domain.Values) != 2 {
		t.Fatal("unexpected domain")
	}
	if domain.Values[0].String() != "example" {
		t.Fatal("unexpected domain")
	}
	if domain.Values[1].String() != "com" {
		t.Fatal("unexpected domain")
	}

	var quoted = txt.New[EmailAddress](`"user spaced"@example.com`)
	s, ok = quoted.Get()
	if !ok || s != `"user spaced"@example.com` {
		t.Fatal("unexpected value!", quoted.Err())
	}
}

func TestMobileNumber(t *testing.T) {
	type MobileNumber txt.Pattern[struct {
		txt.Max `15`
		Plus    may.Prefix  `+`
		Digits  may.Contain `0123456789`
	}]

	var mobile = txt.New[MobileNumber]("1234567890")

	s, ok := mobile.Get()
	if !ok || s != "1234567890" {
		t.Fatal("unexpected value!", mobile.Err())
	}
	if mobile.Format().Plus {
		t.Fatal("unexpected value")
	}
	if mobile.Format().Digits.String() != "1234567890" {
		t.Fatal("unexpected value")
	}

	mobile = txt.New[MobileNumber]("+1234567890")

	s, ok = mobile.Get()
	if !ok || s != "+1234567890" {
		t.Fatal("unexpected value!", mobile.Err())
	}
	if !mobile.Format().Plus {
		t.Fatal("unexpected value")
	}

	mobile = txt.New[MobileNumber]("1234567890a")

	s, ok = mobile.Get()
	if ok {
		t.Fatal("expected failure!")
	}

}
