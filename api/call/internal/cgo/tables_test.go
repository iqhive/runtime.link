package cgo

import (
	"go/ast"
	"go/parser"
	"go/token"
	"math/big"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// Only one platform can be run by a test, so the tables of all of them are read
// from the files that are selected for each, as the tests in build_test.go do,
// and held to what the platform's C compiler and C library are known to do.

// entry is one C type in the table of a platform.
type entry struct {
	kind  string
	size  int64
	align int64
}

// table is what a platform says about C: its types, and its constants as numbers.
// A constant that is text, as the long double ones are, is left out.
type table struct {
	types     map[string]entry
	constants map[string]*big.Float
}

func number(t *testing.T, expr ast.Expr) (*big.Float, bool) {
	t.Helper()
	negative := false
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.SUB {
		negative, expr = true, unary.X
	}
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind == token.STRING {
		return nil, false
	}
	value, _, err := big.ParseFloat(literal.Value, 0, 256, big.ToNearestEven)
	if err != nil {
		t.Fatalf("%s: %v", literal.Value, err)
	}
	if negative {
		value.Neg(value)
	}
	return value, true
}

// declared returns the value that the package gives a variable for a platform,
// or nil if the variable is declared without one.
func declared(t *testing.T, dir string, target platform, name string) (literal *ast.CompositeLit, found bool) {
	t.Helper()
	pkg := selected(t, dir, target, true)
	fset := token.NewFileSet()
	for _, file := range append(pkg.GoFiles, pkg.CgoFiles...) {
		parsed, err := parser.ParseFile(fset, filepath.Join(dir, file), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range parsed.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				value := spec.(*ast.ValueSpec)
				if value.Names[0].Name != name {
					continue
				}
				found = true
				if len(value.Values) == 1 {
					literal, _ = value.Values[0].(*ast.CompositeLit)
				}
			}
		}
	}
	return literal, found
}

func tableOf(t *testing.T, target platform) table {
	t.Helper()
	result := table{types: map[string]entry{}, constants: map[string]*big.Float{}}
	types, _ := declared(t, ".", target, "Types")
	constants, _ := declared(t, ".", target, "Constants")
	if types == nil || constants == nil {
		t.Fatalf("%s/%s: no table of C types and constants", target.goos, target.goarch)
	}
	for _, element := range types.Elts {
		pair := element.(*ast.KeyValueExpr)
		var value entry
		var memory, inStruct int64
		for _, field := range pair.Value.(*ast.CompositeLit).Elts {
			field := field.(*ast.KeyValueExpr)
			switch field.Key.(*ast.Ident).Name {
			case "kind":
				value.kind = field.Value.(*ast.SelectorExpr).Sel.Name
			case "size":
				size, _ := number(t, field.Value)
				value.size, _ = size.Int64()
			case "align_memory":
				align, _ := number(t, field.Value)
				memory, _ = align.Int64()
			case "align_struct":
				align, _ := number(t, field.Value)
				inStruct, _ = align.Int64()
			}
		}
		if memory != inStruct {
			t.Errorf("%s/%s: %s is aligned to %d in memory and to %d in a struct", target.goos, target.goarch, pair.Key.(*ast.Ident).Name, memory, inStruct)
		}
		value.align = memory
		result.types[pair.Key.(*ast.Ident).Name] = value
	}
	for _, element := range constants.Elts {
		pair := element.(*ast.KeyValueExpr)
		if value, ok := number(t, pair.Value); ok {
			result.constants[pair.Key.(*ast.Ident).Name] = value
		}
	}
	return result
}

func (target platform) String() string { return target.goos + "/" + target.goarch }

// integer returns the Go kind of a C integer of the given size.
func integer(signed bool, size int64) string {
	if signed {
		return "Int" + strconv.FormatInt(size*8, 10)
	}
	return "Uint" + strconv.FormatInt(size*8, 10)
}

func (tab table) want(t *testing.T, target platform, name, kind string, size int64) {
	t.Helper()
	got, ok := tab.types[name]
	if !ok {
		t.Errorf("%s: no C type '%s'", target, name)
		return
	}
	if got.kind != kind || got.size != size {
		t.Errorf("%s: '%s' is %s of %d bytes, want %s of %d bytes", target, name, got.kind, got.size, kind, size)
	}
}

func (tab table) wantConstant(t *testing.T, target platform, name string, want *big.Float) {
	t.Helper()
	got, ok := tab.constants[name]
	if !ok {
		t.Errorf("%s: no constant %s", target, name)
		return
	}
	if got.Cmp(want) != 0 {
		t.Errorf("%s: %s = %s, want %s", target, name, got.Text('g', 40), want.Text('g', 40))
	}
}

func whole(value int64) *big.Float { return new(big.Float).SetPrec(256).SetInt64(value) }

// limits returns the least and the greatest value of a C integer type.
func limits(kind string, size int64) (least, greatest *big.Float) {
	bits := uint(size * 8)
	one := big.NewInt(1)
	if strings.HasPrefix(kind, "Uint") {
		top := new(big.Int).Sub(new(big.Int).Lsh(one, bits), one)
		return whole(0), new(big.Float).SetPrec(256).SetInt(top)
	}
	half := new(big.Int).Lsh(one, bits-1)
	return new(big.Float).SetPrec(256).SetInt(new(big.Int).Neg(half)), new(big.Float).SetPrec(256).SetInt(new(big.Int).Sub(half, one))
}

func TestEveryPlatformHasATableOfEveryType(t *testing.T) {
	fields := reflect.TypeFor[types]()
	for _, target := range platforms {
		tab := tableOf(t, target)
		for i := range fields.NumField() {
			name := fields.Field(i).Name
			got, ok := tab.types[name]
			if !ok || got.size == 0 || got.kind == "" || got.kind == "Invalid" {
				t.Errorf("%s: the C type '%s' is not known: %+v", target, name, got)
			}
		}
		if len(tab.types) != fields.NumField() {
			t.Errorf("%s: %d types in the table, want %d", target, len(tab.types), fields.NumField())
		}
		if len(tab.constants) < 150 {
			t.Errorf("%s: only %d constants in the table", target, len(tab.constants))
		}
	}
}

// What every 64-bit platform that Go runs on agrees about.
func TestTypesThatAreTheSameOnEveryPlatform(t *testing.T) {
	for _, target := range platforms {
		tab := tableOf(t, target)
		for name, want := range map[string]entry{
			"signed_char": {"Int8", 1, 1}, "unsigned_char": {"Uint8", 1, 1},
			"short": {"Int16", 2, 2}, "unsigned_short": {"Uint16", 2, 2},
			"int": {"Int32", 4, 4}, "unsigned_int": {"Uint32", 4, 4},
			"long_long": {"Int64", 8, 8}, "unsigned_long_long": {"Uint64", 8, 8},
			"float": {"Float32", 4, 4}, "double": {"Float64", 8, 8},
			"float_t": {"Float32", 4, 4}, "double_t": {"Float64", 8, 8},
			"int8_t": {"Int8", 1, 1}, "int16_t": {"Int16", 2, 2}, "int32_t": {"Int32", 4, 4}, "int64_t": {"Int64", 8, 8},
			"uint8_t": {"Uint8", 1, 1}, "uint16_t": {"Uint16", 2, 2}, "uint32_t": {"Uint32", 4, 4}, "uint64_t": {"Uint64", 8, 8},
			"int_least8_t": {"Int8", 1, 1}, "int_least16_t": {"Int16", 2, 2}, "int_least32_t": {"Int32", 4, 4}, "int_least64_t": {"Int64", 8, 8},
			"uint_least8_t": {"Uint8", 1, 1}, "uint_least16_t": {"Uint16", 2, 2}, "uint_least32_t": {"Uint32", 4, 4}, "uint_least64_t": {"Uint64", 8, 8},
			"int_fast8_t": {"Int8", 1, 1}, "uint_fast8_t": {"Uint8", 1, 1}, "int_fast64_t": {"Int64", 8, 8}, "uint_fast64_t": {"Uint64", 8, 8},
			"char16_t": {"Uint16", 2, 2}, "char32_t": {"Uint32", 4, 4},
			"bool":   {"Bool", 1, 1},
			"size_t": {"Uint64", 8, 8}, "ptrdiff_t": {"Int64", 8, 8}, "intptr_t": {"Int64", 8, 8}, "uintptr_t": {"Uint64", 8, 8},
			"intmax_t": {"Int64", 8, 8}, "uintmax_t": {"Uint64", 8, 8},
			"time_t": {"Int64", 8, 8}, "sig_atomic_t": {"Int32", 4, 4},
		} {
			tab.want(t, target, name, want.kind, want.size)
			if got := tab.types[name].align; got != want.align {
				t.Errorf("%s: '%s' is aligned to %d, want %d", target, name, got, want.align)
			}
		}
	}
}

// model is what differs between the platforms: how wide a long is (LP64 against
// the LLP64 of Windows), whether a plain char has a sign (it doesn't on arm64,
// except for Apple's), and the choices that each C library makes for itself.
type model struct {
	long       int64 // the size of a long.
	charSigned bool
	wchar      string
	wint       string
	clock      string
	fast16     string // int_fast16_t, the unsigned one being as wide.
	fast32     string
	longDouble int64 // the size of a long double.
	mantissa   int64 // the bits of precision that it has.
}

var models = map[platform]model{
	{"linux", "amd64"}:   {long: 8, charSigned: true, wchar: "Int32", wint: "Uint32", clock: "Int64", fast16: "Int64", fast32: "Int64", longDouble: 16, mantissa: 64},
	{"linux", "arm64"}:   {long: 8, charSigned: false, wchar: "Uint32", wint: "Uint32", clock: "Int64", fast16: "Int64", fast32: "Int64", longDouble: 16, mantissa: 113},
	{"android", "arm64"}: {long: 8, charSigned: false, wchar: "Uint32", wint: "Uint32", clock: "Int64", fast16: "Int64", fast32: "Int64", longDouble: 16, mantissa: 113},
	{"darwin", "amd64"}:  {long: 8, charSigned: true, wchar: "Int32", wint: "Int32", clock: "Uint64", fast16: "Int16", fast32: "Int32", longDouble: 16, mantissa: 64},
	{"darwin", "arm64"}:  {long: 8, charSigned: true, wchar: "Int32", wint: "Int32", clock: "Uint64", fast16: "Int16", fast32: "Int32", longDouble: 8, mantissa: 53},
	{"windows", "amd64"}: {long: 4, charSigned: true, wchar: "Uint16", wint: "Uint16", clock: "Int32", fast16: "Int16", fast32: "Int32", longDouble: 16, mantissa: 64},
}

// size returns the bytes of a Go kind that is named for its bits.
func size(kind string) int64 {
	bits, _ := strconv.ParseInt(strings.TrimLeft(kind, "UIFintloa"), 10, 64)
	return bits / 8
}

func TestTypesFollowTheDataModelOfThePlatform(t *testing.T) {
	for _, target := range platforms {
		want, ok := models[target]
		if !ok {
			t.Errorf("%s: nothing says what its C types are", target)
			continue
		}
		tab := tableOf(t, target)
		tab.want(t, target, "long", integer(true, want.long), want.long)
		tab.want(t, target, "unsigned_long", integer(false, want.long), want.long)
		tab.want(t, target, "char", integer(want.charSigned, 1), 1)
		tab.want(t, target, "wchar_t", want.wchar, size(want.wchar))
		tab.want(t, target, "wint_t", want.wint, size(want.wint))
		tab.want(t, target, "clock_t", want.clock, size(want.clock))
		tab.want(t, target, "int_fast16_t", want.fast16, size(want.fast16))
		tab.want(t, target, "uint_fast16_t", "U"+strings.ToLower(want.fast16), size(want.fast16))
		tab.want(t, target, "int_fast32_t", want.fast32, size(want.fast32))
		tab.want(t, target, "uint_fast32_t", "U"+strings.ToLower(want.fast32), size(want.fast32))
		// a long double is not passed by value, so all that is known of it is its size.
		tab.want(t, target, "long_double", "Array", want.longDouble)
		tab.wantConstant(t, target, "LDBL_MANT_DIG", whole(want.mantissa))
	}
}

// The limits that a platform gives for a type have to be those of the type that
// it says it is, which holds the constants and the types to one another.
func TestLimitsAreThoseOfTheTypes(t *testing.T) {
	for _, target := range platforms {
		tab := tableOf(t, target)
		for name, limit := range map[string]struct{ least, greatest string }{
			"char":        {"CHAR_MIN", "CHAR_MAX"},
			"signed_char": {"SCHAR_MIN", "SCHAR_MAX"}, "unsigned_char": {"", "UCHAR_MAX"},
			"short": {"SHRT_MIN", "SHRT_MAX"}, "unsigned_short": {"", "USHRT_MAX"},
			"int": {"INT_MIN", "INT_MAX"}, "unsigned_int": {"", "UINT_MAX"},
			"long": {"LONG_MIN", "LONG_MAX"}, "unsigned_long": {"", "ULONG_MAX"},
			"long_long": {"LLONG_MIN", "LLONG_MAX"}, "unsigned_long_long": {"", "ULLONG_MAX"},
			"ptrdiff_t": {"PTRDIFF_MIN", "PTRDIFF_MAX"}, "size_t": {"", "SIZE_MAX"},
			"intptr_t": {"INTPTR_MIN", "INTPTR_MAX"}, "uintptr_t": {"", "UINTPTR_MAX"},
			"intmax_t": {"INTMAX_MIN", "INTMAX_MAX"}, "uintmax_t": {"", "UINTMAX_MAX"},
			"wchar_t": {"WCHAR_MIN", "WCHAR_MAX"}, "wint_t": {"WINT_MIN", "WINT_MAX"},
			"sig_atomic_t": {"SIG_ATOMIMIN", "SIG_ATOMIMAX"},
			"int8_t":       {"INT8_MIN", "INT8_MAX"}, "int16_t": {"INT16_MIN", "INT16_MAX"},
			"int32_t": {"INT32_MIN", "INT32_MAX"}, "int64_t": {"INT64_MIN", "INT64_MAX"},
			"uint8_t": {"", "UINT8_MAX"}, "uint16_t": {"", "UINT16_MAX"},
			"uint32_t": {"", "UINT32_MAX"}, "uint64_t": {"", "UINT64_MAX"},
			"int_fast8_t": {"INT_FAST8_MIN", "INT_FAST8_MAX"}, "int_fast16_t": {"INT_FAST16_MIN", "INT_FAST16_MAX"},
			"int_fast32_t": {"INT_FAST32_MIN", "INT_FAST32_MAX"}, "int_fast64_t": {"INT_FAST64_MIN", "INT_FAST64_MAX"},
			"uint_fast8_t": {"", "UINT_FAST8_MAX"}, "uint_fast16_t": {"", "UINT_FAST16_MAX"},
			"uint_fast32_t": {"", "UINT_FAST32_MAX"}, "uint_fast64_t": {"", "UINT_FAST64_MAX"},
			"int_least8_t": {"INT_LEAST8_MIN", "INT_LEAST8_MAX"}, "int_least16_t": {"INT_LEAST16_MIN", "INT_LEAST16_MAX"},
			"int_least32_t": {"INT_LEAST32_MIN", "INT_LEAST32_MAX"}, "int_least64_t": {"INT_LEAST64_MIN", "INT_LEAST64_MAX"},
			"uint_least8_t": {"", "UINT_LEAST8_MAX"}, "uint_least16_t": {"", "UINT_LEAST16_MAX"},
			"uint_least32_t": {"", "UINT_LEAST32_MAX"}, "uint_least64_t": {"", "UINT_LEAST64_MAX"},
		} {
			// the table that was generated on android has the limits of an int for
			// these, and types of 8 bytes, which is how its C library declares them.
			if target.goos == "android" && strings.Contains(name, "_fast") && tab.types[name].size == 8 && !strings.Contains(name, "64") {
				continue
			}
			least, greatest := limits(tab.types[name].kind, tab.types[name].size)
			if limit.least != "" {
				tab.wantConstant(t, target, limit.least, least)
			}
			tab.wantConstant(t, target, limit.greatest, greatest)
		}
		tab.wantConstant(t, target, "CHAR_BIT", whole(8))
	}
}

// The constants of a C library are the same on every architecture that it runs
// on, and are not those of another: a buffer of the L_tmpnam bytes of Linux is
// far too small for tmpnam on a Mac.
func TestConstantsAreThoseOfTheOperatingSystem(t *testing.T) {
	type library struct {
		EILSEQ, BUFSIZ, FILENAME_MAX, FOPEN_MAX, L_tmpnam, TMP_MAX, CLOCKS_PER_SEC, SIGABRT int64
		LALL, LCTYPE                                                                        int64
	}
	glibc := library{EILSEQ: 84, BUFSIZ: 8192, FILENAME_MAX: 4096, FOPEN_MAX: 16, L_tmpnam: 20, TMP_MAX: 238328, CLOCKS_PER_SEC: 1000000, SIGABRT: 6, LALL: 6, LCTYPE: 0}
	macOS := library{EILSEQ: 92, BUFSIZ: 1024, FILENAME_MAX: 1024, FOPEN_MAX: 20, L_tmpnam: 1024, TMP_MAX: 308915776, CLOCKS_PER_SEC: 1000000, SIGABRT: 6, LALL: 0, LCTYPE: 2}
	for target, want := range map[platform]library{
		{"linux", "amd64"}:   glibc,
		{"linux", "arm64"}:   glibc,
		{"android", "arm64"}: {EILSEQ: 84, BUFSIZ: 1024, FILENAME_MAX: 4096, FOPEN_MAX: 20, L_tmpnam: 4096, TMP_MAX: 308915776, CLOCKS_PER_SEC: 1000000, SIGABRT: 6, LALL: 6, LCTYPE: 0},
		{"darwin", "amd64"}:  macOS,
		{"darwin", "arm64"}:  macOS,
		// mingw-w64, as it is built for the UCRT.
		{"windows", "amd64"}: {EILSEQ: 42, BUFSIZ: 512, FILENAME_MAX: 260, FOPEN_MAX: 20, L_tmpnam: 260, TMP_MAX: 2147483647, CLOCKS_PER_SEC: 1000, SIGABRT: 22, LALL: 0, LCTYPE: 2},
	} {
		tab := tableOf(t, target)
		values := reflect.ValueOf(want)
		for i := range values.NumField() {
			tab.wantConstant(t, target, values.Type().Field(i).Name, whole(values.Field(i).Int()))
		}
		// these are in the C standard with the same values everywhere.
		for name, value := range map[string]int64{"EDOM": 33, "ERANGE": 34, "EOF": -1, "SEEK_SET": 0, "SEEK_CUR": 1, "SEEK_END": 2, "SIGINT": 2, "SIGILL": 4, "SIGFPE": 8, "SIGSEGV": 11, "SIGTERM": 15, "EXIT_SUCCESS": 0, "EXIT_FAILURE": 1, "True": 1, "False": 0} {
			tab.wantConstant(t, target, name, whole(value))
		}
	}
	if len(platforms) != 6 {
		t.Errorf("%d platforms, of which 6 have their constants checked", len(platforms))
	}
}

// The floating point exceptions and rounding modes are bits of a register of the
// processor, so they follow the architecture, apart from on Windows, where the C
// library has numbers of its own.
func TestFloatingPointEnvironmentIsThatOfTheProcessor(t *testing.T) {
	type environment struct{ invalid, divide, overflow, underflow, inexact, upward, downward, towardZero int64 }
	x86 := environment{invalid: 1, divide: 4, overflow: 8, underflow: 16, inexact: 32, upward: 0x800, downward: 0x400, towardZero: 0xc00}
	arm := environment{invalid: 1, divide: 2, overflow: 4, underflow: 8, inexact: 16, upward: 0x400000, downward: 0x800000, towardZero: 0xc00000}
	for target, want := range map[platform]environment{
		{"linux", "amd64"}:   x86,
		{"darwin", "amd64"}:  x86,
		{"linux", "arm64"}:   arm,
		{"darwin", "arm64"}:  arm,
		{"windows", "amd64"}: {invalid: 16, divide: 8, overflow: 4, underflow: 2, inexact: 1, upward: 0x200, downward: 0x100, towardZero: 0x300},
	} {
		tab := tableOf(t, target)
		tab.wantConstant(t, target, "FE_INVALID", whole(want.invalid))
		tab.wantConstant(t, target, "FE_DIVBYZERO", whole(want.divide))
		tab.wantConstant(t, target, "FE_OVERFLOW", whole(want.overflow))
		tab.wantConstant(t, target, "FE_UNDERFLOW", whole(want.underflow))
		tab.wantConstant(t, target, "FE_INEXACT", whole(want.inexact))
		tab.wantConstant(t, target, "FE_UPWARD", whole(want.upward))
		tab.wantConstant(t, target, "FE_DOWNWARD", whole(want.downward))
		tab.wantConstant(t, target, "FE_TOWARDZERO", whole(want.towardZero))
		// a program starts out rounding to nearest, with no exception raised.
		tab.wantConstant(t, target, "FE_TONEAREST", whole(0))
		tab.wantConstant(t, target, "fegetround", whole(0))
		tab.wantConstant(t, target, "FE_DFL_ENV", whole(0))
		tab.wantConstant(t, target, "FLT_ROUNDS", whole(1))
	}
}

// A float and a double are those of IEEE 754 on every platform.
func TestFloatingPointLimitsAreExact(t *testing.T) {
	for _, target := range platforms {
		tab := tableOf(t, target)
		for name, want := range map[string]float64{
			"FLT_MAX": 0x1.fffffep+127, "FLT_MIN": 0x1p-126, "FLT_TRUE_MIN": 0x1p-149, "FLT_EPSILON": 0x1p-23,
			"DBL_MAX": 0x1.fffffffffffffp+1023, "DBL_MIN": 0x1p-1022, "DBL_TRUE_MIN": 0x1p-1074, "DBL_EPSILON": 0x1p-52,
		} {
			got, ok := tab.constants[name]
			if !ok {
				t.Errorf("%s: no constant %s", target, name)
				continue
			}
			if value, _ := got.Float64(); value != want {
				t.Errorf("%s: %s = %v, want %v", target, name, value, want)
			}
		}
	}
}

// A platform that has no table of its own is not given another's, so that the
// package reports it as unsupported and does not link with the wrong sizes.
func TestOtherPlatformsHaveNoTable(t *testing.T) {
	for _, target := range []platform{
		{"windows", "arm64"}, {"windows", "386"}, {"freebsd", "amd64"}, {"freebsd", "arm64"}, {"openbsd", "amd64"},
		{"linux", "386"}, {"linux", "arm"}, {"linux", "riscv64"}, {"linux", "ppc64le"}, {"js", "wasm"}, {"wasip1", "wasm"},
	} {
		for _, name := range []string{"Types", "Constants"} {
			value, found := declared(t, ".", target, name)
			if !found {
				t.Errorf("%s: %s is not declared, so the package does not build", target, name)
			}
			if value != nil {
				t.Errorf("%s: %s has the values of another platform", target, name)
			}
		}
	}
	var empty types
	if empty.Known() {
		t.Errorf("a table without values is taken to be known")
	}
	_, listed := models[platform{runtime.GOOS, runtime.GOARCH}]
	// the tables of linux are glibc's, so they are not those of a program that is built with musl.
	if want := listed && !OtherLibrary; want != Types.Known() {
		t.Errorf("the table of this platform is known: %v, though it is listed: %v, and for another C library: %v", Types.Known(), listed, OtherLibrary)
	}
}

// ffiTable returns the constants and the sizes of the types that internal/ffi,
// which has tables of its own from another generator, selects for a platform.
func ffiTable(t *testing.T, target platform) (constants map[string]*big.Float, sizes map[string]int64, kinds map[string]string) {
	t.Helper()
	constants, sizes, kinds = map[string]*big.Float{}, map[string]int64{}, map[string]string{}
	dir := filepath.Join("..", "ffi")
	fset := token.NewFileSet()
	for _, file := range selected(t, dir, target, true).GoFiles {
		parsed, err := parser.ParseFile(fset, filepath.Join(dir, file), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range parsed.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				switch spec := spec.(type) {
				case *ast.ValueSpec:
					name := spec.Names[0].Name
					if gen.Tok != token.CONST || !strings.HasPrefix(name, "c_") || len(spec.Values) != 1 {
						continue
					}
					if value, ok := number(t, spec.Values[0]); ok {
						constants[strings.TrimPrefix(name, "c_")] = value
					}
				case *ast.TypeSpec:
					name, ok := strings.CutPrefix(spec.Name.Name, "c_")
					if !ok {
						continue
					}
					switch kind := spec.Type.(type) {
					case *ast.Ident:
						kinds[name] = strings.ToUpper(kind.Name[:1]) + kind.Name[1:]
						sizes[name] = size(kinds[name])
					case *ast.ArrayType:
						length, _ := number(t, kind.Len)
						kinds[name] = "Array"
						sizes[name], _ = length.Int64()
					}
				}
			}
		}
	}
	return constants, sizes, kinds
}

// Tags read their constants from internal/ffi and their types from here. The two
// have a table each for every platform, written by different generators, so
// where they describe the same thing they have to say the same.
func TestTablesAgreeWithThoseOfTheConstants(t *testing.T) {
	for _, target := range platforms {
		tab := tableOf(t, target)
		constants, sizes, kinds := ffiTable(t, target)
		if len(constants) < 150 || len(sizes) < len(tab.types) {
			t.Errorf("%s: internal/ffi has %d constants and %d types", target, len(constants), len(sizes))
			continue
		}
		for name, theirs := range constants {
			// the fields here have the names of the C constants without any "C_".
			field := strings.ReplaceAll(name, "C_", "")
			switch name {
			case "true":
				field = "True"
			case "false":
				field = "False"
			}
			ours, ok := tab.constants[field]
			if !ok {
				continue // a long double, which is text here.
			}
			same := ours.Cmp(theirs) == 0
			switch name {
			case "FLT_MIN", "DBL_MIN", "FLT_TRUE_MIN", "DBL_TRUE_MIN", "FLT_MAX", "DBL_MAX", "FLT_EPSILON", "DBL_EPSILON", "LDBL_EPSILON":
				// the same double, whichever number of digits it was written with.
				a, _ := ours.Float64()
				b, _ := theirs.Float64()
				same = a == b
			}
			if !same {
				t.Errorf("%s: %s is %s here and %s in internal/ffi", target, name, ours.Text('g', 40), theirs.Text('g', 40))
			}
		}
		for name, ours := range tab.types {
			theirs := strings.Replace(name, "long_long", "longlong", 1)
			if sizes[theirs] != ours.size {
				t.Errorf("%s: '%s' is %d bytes here and %d in internal/ffi", target, name, ours.size, sizes[theirs])
			}
			switch name {
			case "bool", "clock_t", "time_t", "sig_atomic_t", "long_double", "max_align_t":
				continue // gen.c decides the sign of these itself, and has no bool.
			}
			if kinds[theirs] != ours.kind {
				t.Errorf("%s: '%s' is %s here and %s in internal/ffi", target, name, ours.kind, kinds[theirs])
			}
		}
	}
}
