package email

import (
	"bytes"
	"errors"
	"net/url"
	"strings"
	"unicode"
)

// Address [txt.Format] as specified in RFC 5322 (sections 3.2.3 and 3.4.1) and RFC 5321.
// https://en.wikipedia.org/wiki/Email_address
type Address struct {
	_ struct{}
}

func (Address) UnmarshalText(text []byte) error {
	if bytes.HasPrefix(text, []byte{'.'}) {
		return errors.New("email address may not start with a dot")
	}
	if bytes.HasSuffix(text, []byte{'.'}) {
		return errors.New("email address may not end with a dot")
	}
	local, domain, ok := bytes.Cut(text, []byte{'@'})
	if !ok {
		return errors.New("email address must contain an @")
	}
	if len(local) < 1 {
		return errors.New("email address must contain a local part")
	}
	if len(local) > 64 {
		return errors.New("email address local part must be less than 64 characters")
	}
	quoted := local[0] == '"'
	if quoted {
		if len(local) == 1 {
			return errors.New("email address quoted local must end with a quote")
		}
		if local[len(local)-1] != '"' {
			return errors.New("email address quoted local must end with a quote")
		}
		local = local[1 : len(local)-1]
		for _, char := range local {
			if char == '\\' {
				return errors.New("email address quoted local may not contain a backslash")
			}
			if char == '"' {
				return errors.New("email address quoted local may not contain a quote")
			}
			if char > 127 {
				return errors.New("email address quoted local may not contain non-ASCII characters")
			}
			if !unicode.IsPrint(rune(char)) && char != '\t' {
				return errors.New("email address quoted local may not contain non-printable characters")
			}
		}
	} else {
		for i, char := range local {
			if char == '.' && i > 0 && local[i-1] == '.' {
				return errors.New("email address local may not contain consecutive dots")
			}
			if !strings.ContainsRune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!#$%&'*+-/=?^_{|}~.", rune(char)) {
				return errors.New("email address local contains invalid characters")
			}
		}
	}
	if bytes.ContainsRune(domain, '/') {
		return errors.New("email address domain may not contain a slash")
	}
	_, err := url.Parse(string(domain))
	if err != nil {
		return err
	}
	return err
}
