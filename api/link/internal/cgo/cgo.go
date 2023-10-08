//go:generate go run gen/gen.go
//go:build cgo

package cgo

import (
	"fmt"
	"io"
	"strings"
)

/*
#cgo linux LDFLAGS: -lm

#include <complex.h>
#include <errno.h>
#include <fenv.h>
#include <float.h>
#include <inttypes.h>
#include <limits.h>
#include <locale.h>
#include <math.h>
#include <setjmp.h>
#include <signal.h>
#include <stdatomic.h>
#include <stddef.h>
#include <stdbool.h>
#include <stdlib.h>
#include <stdint.h>
#include <stdio.h>
#include <time.h>
#include <wchar.h>

#ifdef __APPLE__
typedef uint16_t char16_t;
typedef uint32_t char32_t;
#else
#include <uchar.h>
#endif

const double LDBL_EPSILON_DOUBLE = LDBL_EPSILON;

char* LDBL_MAX_STRING() {
	char* str = malloc(64);
	sprintf(str, "%Le", LDBL_MAX);
	return str;
}
char* LDBL_MIN_STRING() {
	char* str = malloc(64);
	sprintf(str, "%Le", LDBL_MIN);
	return str;
}
char* LDBL_TRUE_MIN_STRING() {
	char* str = malloc(64);
	sprintf(str, "%Le", LDBL_TRUE_MIN);
	return str;
}

typedef char long_double_bytes[sizeof(long double)];
typedef char max_align_t_bytes[sizeof(max_align_t)];
*/
import "C"

func Dump(w io.Writer) {
	var Constants constants
	Constants.CHAR_BIT = C.CHAR_BIT
	Constants.MB_LEN_MAX = C.MB_LEN_MAX
	Constants.CHAR_MIN = C.CHAR_MIN
	Constants.CHAR_MAX = C.CHAR_MAX
	Constants.SCHAR_MIN = C.SCHAR_MIN
	Constants.SHRT_MIN = C.SHRT_MIN
	Constants.INT_MIN = C.INT_MIN
	Constants.LONG_MIN = C.LONG_MIN
	Constants.LLONG_MIN = C.LLONG_MIN
	Constants.SCHAR_MAX = C.SCHAR_MAX
	Constants.SHRT_MAX = C.SHRT_MAX
	Constants.INT_MAX = C.INT_MAX
	Constants.LONG_MAX = C.LONG_MAX
	Constants.LLONG_MAX = C.LLONG_MAX
	Constants.UCHAR_MAX = C.UCHAR_MAX
	Constants.USHRT_MAX = C.USHRT_MAX
	Constants.UINT_MAX = C.UINT_MAX
	Constants.ULONG_MAX = C.ULONG_MAX
	Constants.ULLONG_MAX = C.ULLONG_MAX
	Constants.PTRDIFF_MIN = C.PTRDIFF_MIN
	Constants.PTRDIFF_MAX = C.PTRDIFF_MAX
	Constants.SIZE_MAX = C.SIZE_MAX
	Constants.WINT_MIN = C.WINT_MIN
	Constants.WINT_MAX = C.WINT_MAX
	Constants.WCHAR_MIN = C.WCHAR_MIN
	Constants.WCHAR_MAX = C.WCHAR_MAX
	Constants.INT8_MIN = C.INT8_MIN
	Constants.INT16_MIN = C.INT16_MIN
	Constants.INT32_MIN = C.INT32_MIN
	Constants.INT64_MIN = C.INT64_MIN
	Constants.INT8_MAX = C.INT8_MAX
	Constants.INT16_MAX = C.INT16_MAX
	Constants.INT32_MAX = C.INT32_MAX
	Constants.INT64_MAX = C.INT64_MAX
	Constants.UINT8_MAX = C.UINT8_MAX
	Constants.UINT16_MAX = C.UINT16_MAX
	Constants.UINT32_MAX = C.UINT32_MAX
	Constants.UINT64_MAX = C.UINT64_MAX
	Constants.INT_FAST8_MIN = C.INT_FAST8_MIN
	Constants.INT_FAST16_MIN = C.INT_FAST16_MIN
	Constants.INT_FAST32_MIN = C.INT_FAST32_MIN
	Constants.INT_FAST64_MIN = C.INT_FAST64_MIN
	Constants.INT_FAST8_MAX = C.INT_FAST8_MAX
	Constants.INT_FAST16_MAX = C.INT_FAST16_MAX
	Constants.INT_FAST32_MAX = C.INT_FAST32_MAX
	Constants.INT_FAST64_MAX = C.INT_FAST64_MAX
	Constants.UINT_FAST8_MAX = C.UINT_FAST8_MAX
	Constants.UINT_FAST16_MAX = C.UINT_FAST16_MAX
	Constants.UINT_FAST32_MAX = C.UINT_FAST32_MAX
	Constants.UINT_FAST64_MAX = C.UINT_FAST64_MAX
	Constants.INT_LEAST8_MIN = C.INT_LEAST8_MIN
	Constants.INT_LEAST16_MIN = C.INT_LEAST16_MIN
	Constants.INT_LEAST32_MIN = C.INT_LEAST32_MIN
	Constants.INT_LEAST64_MIN = C.INT_LEAST64_MIN
	Constants.INT_LEAST8_MAX = C.INT_LEAST8_MAX
	Constants.INT_LEAST16_MAX = C.INT_LEAST16_MAX
	Constants.INT_LEAST32_MAX = C.INT_LEAST32_MAX
	Constants.INT_LEAST64_MAX = C.INT_LEAST64_MAX
	Constants.UINT_LEAST8_MAX = C.UINT_LEAST8_MAX
	Constants.UINT_LEAST16_MAX = C.UINT_LEAST16_MAX
	Constants.UINT_LEAST32_MAX = C.UINT_LEAST32_MAX
	Constants.UINT_LEAST64_MAX = C.UINT_LEAST64_MAX
	Constants.INTMAX_MIN = C.INTMAX_MIN
	Constants.INTMAX_MAX = C.INTMAX_MAX
	Constants.UINTMAX_MAX = C.UINTMAX_MAX
	Constants.INTPTR_MIN = C.INTPTR_MIN
	Constants.INTPTR_MAX = C.INTPTR_MAX
	Constants.UINTPTR_MAX = C.UINTPTR_MAX
	Constants.SIG_ATOMIMIN = C.SIG_ATOMIC_MIN
	Constants.SIG_ATOMIMAX = C.SIG_ATOMIC_MAX
	Constants.FLT_RADIX = C.FLT_RADIX
	Constants.DECIMAL_DIG = C.DECIMAL_DIG
	Constants.FLT_DECIMAL_DIG = C.FLT_DECIMAL_DIG
	Constants.DBL_DECIMAL_DIG = C.DBL_DECIMAL_DIG
	Constants.LDBL_DECIMAL_DIG = C.LDBL_DECIMAL_DIG
	Constants.FLT_MIN = C.FLT_MIN
	Constants.DBL_MIN = C.DBL_MIN
	Constants.LDBL_MIN = C.GoString(C.LDBL_MIN_STRING())
	Constants.FLT_TRUE_MIN = C.FLT_TRUE_MIN
	Constants.DBL_TRUE_MIN = C.DBL_TRUE_MIN
	Constants.LDBL_TRUE_MIN = C.GoString(C.LDBL_TRUE_MIN_STRING())
	Constants.FLT_MAX = C.FLT_MAX
	Constants.DBL_MAX = C.DBL_MAX
	Constants.LDBL_MAX = C.GoString(C.LDBL_MAX_STRING())
	Constants.FLT_EPSILON = C.FLT_EPSILON
	Constants.DBL_EPSILON = C.DBL_EPSILON
	Constants.LDBL_EPSILON = C.LDBL_EPSILON_DOUBLE
	Constants.FLT_DIG = C.FLT_DIG
	Constants.DBL_DIG = C.DBL_DIG
	Constants.LDBL_DIG = C.LDBL_DIG
	Constants.FLT_MANT_DIG = C.FLT_MANT_DIG
	Constants.DBL_MANT_DIG = C.DBL_MANT_DIG
	Constants.LDBL_MANT_DIG = C.LDBL_MANT_DIG
	Constants.FLT_MIN_EXP = C.FLT_MIN_EXP
	Constants.DBL_MIN_EXP = C.DBL_MIN_EXP
	Constants.LDBL_MIN_EXP = C.LDBL_MIN_EXP
	Constants.FLT_MIN_10_EXP = C.FLT_MIN_10_EXP
	Constants.DBL_MIN_10_EXP = C.DBL_MIN_10_EXP
	Constants.LDBL_MIN_10_EXP = C.LDBL_MIN_10_EXP
	Constants.FLT_MAX_EXP = C.FLT_MAX_EXP
	Constants.DBL_MAX_EXP = C.DBL_MAX_EXP
	Constants.LDBL_MAX_EXP = C.LDBL_MAX_EXP
	Constants.FLT_MAX_10_EXP = C.FLT_MAX_10_EXP
	Constants.DBL_MAX_10_EXP = C.DBL_MAX_10_EXP
	Constants.LDBL_MAX_10_EXP = C.LDBL_MAX_10_EXP
	Constants.FLT_ROUNDS = C.FLT_ROUNDS
	Constants.FLT_EVAL_METHOD = C.FLT_EVAL_METHOD
	Constants.FLT_HAS_SUBNORM = C.FLT_HAS_SUBNORM
	Constants.DBL_HAS_SUBNORM = C.DBL_HAS_SUBNORM
	Constants.LDBL_HAS_SUBNORM = C.LDBL_HAS_SUBNORM
	Constants.EDOM = C.EDOM
	Constants.ERANGE = C.ERANGE
	Constants.EILSEQ = C.EILSEQ
	Constants.FE_DFL_ENV = int64(C.fetestexcept(C.FE_ALL_EXCEPT))
	Constants.FE_DIVBYZERO = C.FE_DIVBYZERO
	Constants.FE_INEXACT = C.FE_INEXACT
	Constants.FE_INVALID = C.FE_INVALID
	Constants.FE_OVERFLOW = C.FE_OVERFLOW
	Constants.FE_UNDERFLOW = C.FE_UNDERFLOW
	Constants.FE_ALL_EXCEPT = C.FE_ALL_EXCEPT
	Constants.fegetround = int64(C.fegetround())
	Constants.FE_DOWNWARD = C.FE_DOWNWARD
	Constants.FE_TONEAREST = C.FE_TONEAREST
	Constants.FE_TOWARDZERO = C.FE_TOWARDZERO
	Constants.FE_UPWARD = C.FE_UPWARD
	Constants.FP_NORMAL = C.FP_NORMAL
	Constants.FP_SUBNORMAL = C.FP_SUBNORMAL
	Constants.FP_ZERO = C.FP_ZERO
	Constants.FP_INFINITE = C.FP_INFINITE
	Constants.FP_NAN = C.FP_NAN
	Constants.SIGTERM = C.SIGTERM
	Constants.SIGSEGV = C.SIGSEGV
	Constants.SIGINT = C.SIGINT
	Constants.SIGILL = C.SIGILL
	Constants.SIGABRT = C.SIGABRT
	Constants.SIGFPE = C.SIGFPE
	Constants.LALL = C.LC_ALL
	Constants.LCOLLATE = C.LC_COLLATE
	Constants.LCTYPE = C.LC_CTYPE
	Constants.LMONETARY = C.LC_MONETARY
	Constants.LNUMERIC = C.LC_NUMERIC
	Constants.LTIME = C.LC_TIME
	Constants.MATH_ERRNO = C.MATH_ERRNO
	Constants.MATH_ERREXCEPT = C.MATH_ERREXCEPT
	Constants.math_errhandling = C.math_errhandling
	Constants.EXIT_SUCCESS = C.EXIT_SUCCESS
	Constants.EXIT_FAILURE = C.EXIT_FAILURE
	Constants.True = C.true
	Constants.False = C.false
	Constants.ATOMIBOOL_LOCK_FREE = C.ATOMIC_BOOL_LOCK_FREE
	Constants.ATOMICHAR_LOCK_FREE = C.ATOMIC_CHAR_LOCK_FREE
	Constants.ATOMICHAR16_T_LOCK_FREE = C.ATOMIC_CHAR16_T_LOCK_FREE
	Constants.ATOMICHAR32_T_LOCK_FREE = C.ATOMIC_CHAR32_T_LOCK_FREE
	Constants.ATOMIWCHAR_T_LOCK_FREE = C.ATOMIC_WCHAR_T_LOCK_FREE
	Constants.ATOMISHORT_LOCK_FREE = C.ATOMIC_SHORT_LOCK_FREE
	Constants.ATOMIINT_LOCK_FREE = C.ATOMIC_INT_LOCK_FREE
	Constants.ATOMILONG_LOCK_FREE = C.ATOMIC_LONG_LOCK_FREE
	Constants.ATOMILLONG_LOCK_FREE = C.ATOMIC_LLONG_LOCK_FREE
	Constants.ATOMIPOINTER_LOCK_FREE = C.ATOMIC_POINTER_LOCK_FREE
	Constants.EOF = C.EOF
	Constants.FOPEN_MAX = C.FOPEN_MAX
	Constants.FILENAME_MAX = C.FILENAME_MAX
	Constants.L_tmpnam = C.L_tmpnam
	Constants.TMP_MAX = C.TMP_MAX
	Constants._IOFBF = C._IOFBF
	Constants._IOLBF = C._IOLBF
	Constants._IONBF = C._IONBF
	Constants.BUFSIZ = C.BUFSIZ
	Constants.SEEK_SET = C.SEEK_SET
	Constants.SEEK_CUR = C.SEEK_CUR
	Constants.SEEK_END = C.SEEK_END
	Constants.CLOCKS_PER_SEC = C.CLOCKS_PER_SEC

	var Types types
	Types.char = newValue[C.char]()
	Types.signed_char = newValue[C.schar]()
	Types.unsigned_char = newValue[C.uchar]()
	Types.short = newValue[C.short]()
	Types.unsigned_short = newValue[C.ushort]()
	Types.int = newValue[C.int]()
	Types.unsigned_int = newValue[C.uint]()
	Types.long = newValue[C.long]()
	Types.unsigned_long = newValue[C.ulong]()
	Types.long_long = newValue[C.longlong]()
	Types.unsigned_long_long = newValue[C.ulonglong]()
	Types.float = newValue[C.float]()
	Types.double = newValue[C.double]()
	Types.long_double = newValue[C.long_double_bytes]()
	Types.float_t = newValue[C.float_t]()
	Types.double_t = newValue[C.double_t]()
	Types.int8_t = newValue[C.int8_t]()
	Types.int16_t = newValue[C.int16_t]()
	Types.int32_t = newValue[C.int32_t]()
	Types.int64_t = newValue[C.int64_t]()
	Types.uint8_t = newValue[C.uint8_t]()
	Types.uint16_t = newValue[C.uint16_t]()
	Types.uint32_t = newValue[C.uint32_t]()
	Types.uint64_t = newValue[C.uint64_t]()
	Types.char16_t = newValue[C.char16_t]()
	Types.char32_t = newValue[C.char32_t]()
	Types.wchar_t = newValue[C.wchar_t]()
	Types.wint_t = newValue[C.wint_t]()
	Types.size_t = newValue[C.size_t]()
	Types.time_t = newValue[C.time_t]()
	Types.clock_t = newValue[C.clock_t]()
	Types.bool = newValue[C._Bool]()
	Types.uintptr_t = newValue[C.uintptr_t]()
	Types.ptrdiff_t = newValue[C.ptrdiff_t]()
	Types.intptr_t = newValue[C.intptr_t]()
	Types.max_align_t = newValue[C.max_align_t_bytes]()
	Types.sig_atomic_t = newValue[C.sig_atomic_t]()
	Types.intmax_t = newValue[C.intmax_t]()
	Types.uintmax_t = newValue[C.uintmax_t]()
	Types.int_fast8_t = newValue[C.int_fast8_t]()
	Types.int_fast16_t = newValue[C.int_fast16_t]()
	Types.int_fast32_t = newValue[C.int_fast32_t]()
	Types.int_fast64_t = newValue[C.int_fast64_t]()
	Types.uint_fast8_t = newValue[C.uint_fast8_t]()
	Types.uint_fast16_t = newValue[C.uint_fast16_t]()
	Types.uint_fast32_t = newValue[C.uint_fast32_t]()
	Types.uint_fast64_t = newValue[C.uint_fast64_t]()
	Types.int_least8_t = newValue[C.int_least8_t]()
	Types.int_least16_t = newValue[C.int_least16_t]()
	Types.int_least32_t = newValue[C.int_least32_t]()
	Types.int_least64_t = newValue[C.int_least64_t]()
	Types.uint_least8_t = newValue[C.uint_least8_t]()
	Types.uint_least16_t = newValue[C.uint_least16_t]()
	Types.uint_least32_t = newValue[C.uint_least32_t]()
	Types.uint_least64_t = newValue[C.uint_least64_t]()

	src := fmt.Sprintf("%#v\n", Constants)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "var Constants = %s\n", src[4:])
	fmt.Fprintln(w)
	src = fmt.Sprintf("%#v\n", Types)
	fmt.Fprintf(w, "var Types = %s\n", strings.Replace(src[4:], "cgo.", "", -1))
}
