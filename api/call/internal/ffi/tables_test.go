package ffi_test

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"runtime"
	"strconv"
	"testing"

	"github.com/iqhive/runtime.link/api/call/internal/ffi"
)

// declarations returns the C constants that a tag can name on a platform, as the
// source text of their values, and how often each function is declared, from
// the files that are selected for the platform: only one can be run by a test.
func declarations(t *testing.T, goos, goarch string) (constants map[string]string, funcs map[string]int) {
	t.Helper()
	ctx := build.Default
	ctx.GOOS, ctx.GOARCH, ctx.CgoEnabled = goos, goarch, true
	pkg, err := ctx.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("%s/%s: %v", goos, goarch, err)
	}
	constants, funcs = map[string]string{}, map[string]int{}
	fset := token.NewFileSet()
	for _, name := range pkg.GoFiles {
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				funcs[decl.Name.Name]++
			case *ast.GenDecl:
				if decl.Tok != token.CONST {
					continue
				}
				for _, spec := range decl.Specs {
					value := spec.(*ast.ValueSpec)
					for i, name := range value.Names {
						if _, twice := constants[name.Name]; twice {
							t.Errorf("%s/%s: %s is declared twice, in %v", goos, goarch, name.Name, pkg.GoFiles)
						}
						if i < len(value.Values) {
							constants[name.Name] = types.ExprString(value.Values[i])
						}
					}
				}
			}
		}
	}
	return constants, funcs
}

// The values of the constants belong to the C library of the operating system and
// to the C compiler's types, so a table is for one platform, and not for an
// architecture: on amd64 there used to be Linux's for every operating system.
func TestConstantsAreThoseOfThePlatform(t *testing.T) {
	for _, c := range []struct {
		goos, goarch string
		want         map[string]string
	}{
		{"linux", "amd64", map[string]string{"c_CHAR_MIN": "-128", "c_LONG_MAX": "9223372036854775807", "c_WCHAR_MAX": "2147483647", "c_EILSEQ": "84", "c_BUFSIZ": "8192", "c_L_tmpnam": "20", "c_FILENAME_MAX": "4096", "c_FOPEN_MAX": "16", "c_TMP_MAX": "238328"}},
		{"linux", "arm64", map[string]string{"c_CHAR_MIN": "0", "c_LONG_MAX": "9223372036854775807", "c_WCHAR_MAX": "4294967295", "c_EILSEQ": "84", "c_BUFSIZ": "8192", "c_L_tmpnam": "20", "c_FILENAME_MAX": "4096", "c_FOPEN_MAX": "16", "c_TMP_MAX": "238328"}},
		{"android", "arm64", map[string]string{"c_CHAR_MIN": "0", "c_LONG_MAX": "9223372036854775807", "c_WCHAR_MAX": "4294967295", "c_EILSEQ": "84", "c_BUFSIZ": "1024", "c_L_tmpnam": "4096"}},
		{"darwin", "amd64", map[string]string{"c_CHAR_MIN": "-128", "c_LONG_MAX": "9223372036854775807", "c_WCHAR_MAX": "2147483647", "c_EILSEQ": "92", "c_BUFSIZ": "1024", "c_L_tmpnam": "1024", "c_FILENAME_MAX": "1024", "c_FOPEN_MAX": "20", "c_TMP_MAX": "308915776"}},
		{"darwin", "arm64", map[string]string{"c_CHAR_MIN": "-128", "c_LONG_MAX": "9223372036854775807", "c_WCHAR_MAX": "2147483647", "c_EILSEQ": "92", "c_BUFSIZ": "1024", "c_L_tmpnam": "1024", "c_FILENAME_MAX": "1024", "c_FOPEN_MAX": "20", "c_TMP_MAX": "308915776"}},
		// a long is 32 bits wide on Windows, and a wchar_t is 16.
		{"windows", "amd64", map[string]string{"c_CHAR_MIN": "-128", "c_LONG_MAX": "2147483647", "c_ULONG_MAX": "4294967295", "c_LLONG_MAX": "9223372036854775807", "c_SIZE_MAX": "18446744073709551615", "c_WCHAR_MAX": "65535", "c_EILSEQ": "42", "c_BUFSIZ": "512", "c_FILENAME_MAX": "260", "c_FOPEN_MAX": "20"}},
	} {
		constants, funcs := declarations(t, c.goos, c.goarch)
		if len(constants) < 170 || funcs["Const"] != 1 {
			t.Errorf("%s/%s: %d constants and %d declarations of Const", c.goos, c.goarch, len(constants), funcs["Const"])
			continue
		}
		for name, want := range c.want {
			if constants[name] != want {
				t.Errorf("%s/%s: %s = %s, want %s", c.goos, c.goarch, name[2:], constants[name], want)
			}
		}
	}
}

// macOS has one C library for both of its architectures, as Linux does.
func TestConstantsOfALibraryDoNotDependOnTheArchitecture(t *testing.T) {
	for _, goos := range []string{"darwin", "linux"} {
		intel, _ := declarations(t, goos, "amd64")
		arm, _ := declarations(t, goos, "arm64")
		for _, name := range []string{
			"c_EDOM", "c_ERANGE", "c_EILSEQ", "c_SIGTERM", "c_SIGSEGV", "c_SIGINT", "c_SIGILL", "c_SIGABRT", "c_SIGFPE",
			"c_LC_ALL", "c_LC_COLLATE", "c_LC_CTYPE", "c_LC_MONETARY", "c_LC_NUMERIC", "c_LC_TIME",
			"c_EOF", "c_FOPEN_MAX", "c_FILENAME_MAX", "c_L_tmpnam", "c_TMP_MAX", "c__IOFBF", "c__IOLBF", "c__IONBF", "c_BUFSIZ",
			"c_SEEK_SET", "c_SEEK_CUR", "c_SEEK_END", "c_CLOCKS_PER_SEC", "c_MB_LEN_MAX", "c_EXIT_FAILURE",
		} {
			if intel[name] == "" || intel[name] != arm[name] {
				t.Errorf("%s: %s = %s on amd64 and %s on arm64", goos, name[2:], intel[name], arm[name])
			}
		}
	}
}

// A platform without a table of its own has no constants, and is not given the
// numbers of another: it still builds, and a tag can be parsed there.
func TestOtherPlatformsHaveNoConstants(t *testing.T) {
	for _, target := range [][2]string{
		{"windows", "arm64"}, {"windows", "386"}, {"freebsd", "amd64"}, {"freebsd", "arm64"}, {"openbsd", "amd64"},
		{"linux", "386"}, {"linux", "arm"}, {"linux", "riscv64"}, {"linux", "ppc64le"}, {"js", "wasm"}, {"wasip1", "wasm"},
	} {
		constants, funcs := declarations(t, target[0], target[1])
		for name := range constants {
			if name[:2] == "c_" {
				t.Errorf("%s/%s: has the C constants of another platform, such as %s", target[0], target[1], name[2:])
				break
			}
		}
		if funcs["Const"] != 1 || funcs["ParseTag"] != 1 {
			t.Errorf("%s/%s: Const is declared %d times and ParseTag %d times, want once each", target[0], target[1], funcs["Const"], funcs["ParseTag"])
		}
	}
}

// sameFormat reports whether a table gives long double the format of a double.
func sameFormat(constants map[string]string) bool {
	for _, limit := range []string{"MANT_DIG", "MIN_EXP", "MAX_EXP"} {
		if constants["c_LDBL_"+limit] != constants["c_DBL_"+limit] {
			return false
		}
	}
	return true
}

// A long double that is wider than a double has a smallest value that a float64
// rounds to 0 and a largest that it cannot hold, so neither can be compared with
// and neither is a constant. Where it is a double they are the limits of one.
func TestLimitsOfALongDouble(t *testing.T) {
	limits := []string{"MIN", "TRUE_MIN", "MAX"}
	for _, target := range [][2]string{{"linux", "amd64"}, {"linux", "arm64"}, {"android", "arm64"}, {"darwin", "amd64"}, {"darwin", "arm64"}, {"windows", "amd64"}} {
		constants, _ := declarations(t, target[0], target[1])
		if want := target == [2]string{"darwin", "arm64"}; sameFormat(constants) != want {
			t.Errorf("%s/%s: long double has the format of a double: %v, want %v", target[0], target[1], !want, want)
		}
		if !sameFormat(constants) {
			continue
		}
		for _, limit := range limits {
			long, err := strconv.ParseFloat(constants["c_LDBL_"+limit], 64)
			if err != nil {
				t.Errorf("%s/%s: LDBL_%s: %v", target[0], target[1], limit, err)
			}
			if double, _ := strconv.ParseFloat(constants["c_DBL_"+limit], 64); long != double {
				t.Errorf("%s/%s: LDBL_%s = %v, which is not DBL_%s = %v", target[0], target[1], limit, long, limit, double)
			}
		}
	}
	constants, _ := declarations(t, runtime.GOOS, runtime.GOARCH)
	if _, known := constants["c_INT_MAX"]; !known {
		t.Skip("no table for this platform")
	}
	for _, limit := range limits {
		want := ""
		if sameFormat(constants) {
			want = ffi.Const("DBL_" + limit)
		}
		if got := ffi.Const("LDBL_" + limit); got != want {
			t.Errorf("LDBL_%s = %q, want %q", limit, got, want)
		}
	}
	// what a wider long double has that a float64 can hold is still a constant.
	for _, name := range []string{"LDBL_EPSILON", "LDBL_MANT_DIG", "LDBL_MAX_EXP", "DBL_MIN", "DBL_TRUE_MIN", "DBL_MAX"} {
		if got := ffi.Const(name); got == "" || got == "0" {
			t.Errorf("%s = %q", name, got)
		}
	}
}

func TestConstOfThisPlatform(t *testing.T) {
	constants, _ := declarations(t, runtime.GOOS, runtime.GOARCH)
	if _, known := constants["c_INT_MAX"]; !known {
		if got := ffi.Const("INT_MAX"); got != "" {
			t.Errorf("INT_MAX = %q on a platform without a table", got)
		}
		return
	}
	for name, want := range map[string]string{
		"INT_MAX": "2147483647", "INT_MIN": "-2147483648", "UINT64_MAX": "18446744073709551615", "CHAR_BIT": "8",
		"NULL": "0", "true": "1", "false": "0", "NO_SUCH_CONSTANT": "",
		"LONG_MAX": constants["c_LONG_MAX"], "L_tmpnam": constants["c_L_tmpnam"], "EILSEQ": constants["c_EILSEQ"],
	} {
		if got := ffi.Const(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}
