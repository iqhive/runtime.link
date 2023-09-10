// Package rtags provides methods for reading a rest.Tag, do not make this public, instead extend the rest.Tag with methods if required.
package rtags

import (
	"regexp"
	"strings"
)

var InPathRegex = regexp.MustCompile(`\{((?:.)+?)=%(?:\[(?:\d)+?\])?v\}`)

var InStructRegex = regexp.MustCompile(`\{((?:[^=])+?)\}`)

var InQueryRegex = regexp.MustCompile(`[?&]((?:.)+?)=%(?:\[(?:\d)+?\])?v`)

var FormatParamRegex = regexp.MustCompile(`=%(?:\[(?:\d)+?\])?v`)

// Decode returns the method, path & query of the tag.
func Decode(t string) (method, path, query string) {
	splits := strings.Split(PathOf(t), "?")
	path = splits[0]
	if len(splits) > 1 {
		query = splits[1]
	}
	method = MethodOf(t)

	return
}

// MethodOf returns the method of the tag.
func MethodOf(t string) string {
	if t == "" {
		return "GET"
	}
	splits := strings.Split(string(t), " ")

	switch method := splits[0]; method {
	case "PUT", "POST", "DELETE", "HEAD", "OPTIONS", "PATCH", "CONNECT":
		return method
	}

	return "GET"
}

// Path returns the path of the tag.
func PathOf(t string) string {
	splits := strings.Split(string(t), " ")
	if len(splits) > 1 {
		return splits[1]
	}
	return ""
}

// ResultRules returns the result rules of the tag.
func ResultRulesOf(t string) []string {
	splits := strings.Split(string(t), " ")
	rules := ""
	if len(splits) > 2 {
		if splits[2][0] == '(' {
			if len(splits) > 3 {
				rules = splits[3]
			} else {
				return nil
			}
		} else {
			rules = splits[2]
		}
	} else {
		return nil
	}

	return strings.Split(rules, ",")
}

// ArgumentRulesOf returns the argument rules of the tag.
func ArgumentRulesOf(t string) []string {
	splits := strings.Split(string(t), " ")
	rules := ""
	if len(splits) > 2 {
		if splits[2][0] != '(' {
			return nil
		}
		rules = splits[2]
	} else {
		return nil
	}

	return strings.Split(rules[1:len(rules)-1], ",")
}

// CleanupPattern removes %v values from the path of the pattern.
func CleanupPattern(pattern string) string {
	return FormatParamRegex.ReplaceAllString(pattern, "")
}
