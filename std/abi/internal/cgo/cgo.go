// Package cgo provides information about the platform-native C ABI types.
package cgo

import (
	"fmt"
	"reflect"
	"unsafe"
)

const (
	CHAR_BIT                  = c_CHAR_BIT
	MB_LEN_MAX                = c_MB_LEN_MAX
	CHAR_MIN                  = c_CHAR_MIN
	CHAR_MAX                  = c_CHAR_MAX
	SCHAR_MIN                 = c_SCHAR_MIN
	SHRT_MIN                  = c_SHRT_MIN
	INT_MIN                   = c_INT_MIN
	LONG_MIN                  = c_LONG_MIN
	LLONG_MIN                 = c_LLONG_MIN
	SCHAR_MAX                 = c_SCHAR_MAX
	SHRT_MAX                  = c_SHRT_MAX
	INT_MAX                   = c_INT_MAX
	LONG_MAX                  = c_LONG_MAX
	LLONG_MAX                 = c_LLONG_MAX
	UCHAR_MAX                 = c_UCHAR_MAX
	USHRT_MAX                 = c_USHRT_MAX
	UINT_MAX                  = c_UINT_MAX
	ULONG_MAX                 = c_ULONG_MAX
	ULLONG_MAX                = c_ULLONG_MAX
	PTRDIFF_MIN               = c_PTRDIFF_MIN
	PTRDIFF_MAX               = c_PTRDIFF_MAX
	SIZE_MAX                  = c_SIZE_MAX
	WINT_MIN                  = c_WINT_MIN
	WINT_MAX                  = c_WINT_MAX
	WCHAR_MIN                 = c_WCHAR_MIN
	WCHAR_MAX                 = c_WCHAR_MAX
	INT8_MIN                  = c_INT8_MIN
	INT16_MIN                 = c_INT16_MIN
	INT32_MIN                 = c_INT32_MIN
	INT64_MIN                 = c_INT64_MIN
	INT8_MAX                  = c_INT8_MAX
	INT16_MAX                 = c_INT16_MAX
	INT32_MAX                 = c_INT32_MAX
	INT64_MAX                 = c_INT64_MAX
	UINT8_MAX                 = c_UINT8_MAX
	UINT16_MAX                = c_UINT16_MAX
	UINT32_MAX                = c_UINT32_MAX
	UINT64_MAX                = c_UINT64_MAX
	INT_FAST8_MIN             = c_INT_FAST8_MIN
	INT_FAST16_MIN            = c_INT_FAST16_MIN
	INT_FAST32_MIN            = c_INT_FAST32_MIN
	INT_FAST64_MIN            = c_INT_FAST64_MIN
	INT_FAST8_MAX             = c_INT_FAST8_MAX
	INT_FAST16_MAX            = c_INT_FAST16_MAX
	INT_FAST32_MAX            = c_INT_FAST32_MAX
	INT_FAST64_MAX            = c_INT_FAST64_MAX
	UINT_FAST8_MAX            = c_UINT_FAST8_MAX
	UINT_FAST16_MAX           = c_UINT_FAST16_MAX
	UINT_FAST32_MAX           = c_UINT_FAST32_MAX
	UINT_FAST64_MAX           = c_UINT_FAST64_MAX
	INT_LEAST8_MIN            = c_INT_LEAST8_MIN
	INT_LEAST16_MIN           = c_INT_LEAST16_MIN
	INT_LEAST32_MIN           = c_INT_LEAST32_MIN
	INT_LEAST64_MIN           = c_INT_LEAST64_MIN
	INT_LEAST8_MAX            = c_INT_LEAST8_MAX
	INT_LEAST16_MAX           = c_INT_LEAST16_MAX
	INT_LEAST32_MAX           = c_INT_LEAST32_MAX
	INT_LEAST64_MAX           = c_INT_LEAST64_MAX
	UINT_LEAST8_MAX           = c_UINT_LEAST8_MAX
	UINT_LEAST16_MAX          = c_UINT_LEAST16_MAX
	UINT_LEAST32_MAX          = c_UINT_LEAST32_MAX
	UINT_LEAST64_MAX          = c_UINT_LEAST64_MAX
	INTMAX_MIN                = c_INTMAX_MIN
	INTMAX_MAX                = c_INTMAX_MAX
	UINTMAX_MAX               = c_UINTMAX_MAX
	INTPTR_MIN                = c_INTPTR_MIN
	INTPTR_MAX                = c_INTPTR_MAX
	UINTPTR_MAX               = c_UINTPTR_MAX
	SIG_ATOMIC_MIN            = c_SIG_ATOMIC_MIN
	SIG_ATOMIC_MAX            = c_SIG_ATOMIC_MAX
	FLT_RADIX                 = c_FLT_RADIX
	DECIMAL_DIG               = c_DECIMAL_DIG
	FLT_DECIMAL_DIG           = c_FLT_DECIMAL_DIG
	DBL_DECIMAL_DIG           = c_DBL_DECIMAL_DIG
	LDBL_DECIMAL_DIG          = c_LDBL_DECIMAL_DIG
	FLT_MIN                   = c_FLT_MIN
	DBL_MIN                   = c_DBL_MIN
	LDBL_MIN                  = c_LDBL_MIN
	FLT_TRUE_MIN              = c_FLT_TRUE_MIN
	DBL_TRUE_MIN              = c_DBL_TRUE_MIN
	LDBL_TRUE_MIN             = c_LDBL_TRUE_MIN
	FLT_MAX                   = c_FLT_MAX
	DBL_MAX                   = c_DBL_MAX
	LDBL_MAX                  = c_LDBL_MAX
	FLT_EPSILON               = c_FLT_EPSILON
	DBL_EPSILON               = c_DBL_EPSILON
	LDBL_EPSILON              = c_LDBL_EPSILON
	FLT_DIG                   = c_FLT_DIG
	DBL_DIG                   = c_DBL_DIG
	LDBL_DIG                  = c_LDBL_DIG
	FLT_MANT_DIG              = c_FLT_MANT_DIG
	DBL_MANT_DIG              = c_DBL_MANT_DIG
	LDBL_MANT_DIG             = c_LDBL_MANT_DIG
	FLT_MIN_EXP               = c_FLT_MIN_EXP
	DBL_MIN_EXP               = c_DBL_MIN_EXP
	LDBL_MIN_EXP              = c_LDBL_MIN_EXP
	FLT_MIN_10_EXP            = c_FLT_MIN_10_EXP
	DBL_MIN_10_EXP            = c_DBL_MIN_10_EXP
	LDBL_MIN_10_EXP           = c_LDBL_MIN_10_EXP
	FLT_MAX_EXP               = c_FLT_MAX_EXP
	DBL_MAX_EXP               = c_DBL_MAX_EXP
	LDBL_MAX_EXP              = c_LDBL_MAX_EXP
	FLT_MAX_10_EXP            = c_FLT_MAX_10_EXP
	DBL_MAX_10_EXP            = c_DBL_MAX_10_EXP
	LDBL_MAX_10_EXP           = c_LDBL_MAX_10_EXP
	FLT_ROUNDS                = c_FLT_ROUNDS
	FLT_EVAL_METHOD           = c_FLT_EVAL_METHOD
	FLT_HAS_SUBNORM           = c_FLT_HAS_SUBNORM
	DBL_HAS_SUBNORM           = c_DBL_HAS_SUBNORM
	LDBL_HAS_SUBNORM          = c_LDBL_HAS_SUBNORM
	EDOM                      = c_EDOM
	ERANGE                    = c_ERANGE
	EILSEQ                    = c_EILSEQ
	FE_DFL_ENV                = c_FE_DFL_ENV
	FE_DIVBYZERO              = c_FE_DIVBYZERO
	FE_INEXACT                = c_FE_INEXACT
	FE_INVALID                = c_FE_INVALID
	FE_OVERFLOW               = c_FE_OVERFLOW
	FE_UNDERFLOW              = c_FE_UNDERFLOW
	FE_ALL_EXCEPT             = c_FE_ALL_EXCEPT
	Fegetround                = c_fegetround
	FE_DOWNWARD               = c_FE_DOWNWARD
	FE_TONEAREST              = c_FE_TONEAREST
	FE_TOWARDZERO             = c_FE_TOWARDZERO
	FE_UPWARD                 = c_FE_UPWARD
	FP_NORMAL                 = c_FP_NORMAL
	FP_SUBNORMAL              = c_FP_SUBNORMAL
	FP_ZERO                   = c_FP_ZERO
	FP_INFINITE               = c_FP_INFINITE
	FP_NAN                    = c_FP_NAN
	SIGTERM                   = c_SIGTERM
	SIGSEGV                   = c_SIGSEGV
	SIGINT                    = c_SIGINT
	SIGILL                    = c_SIGILL
	SIGABRT                   = c_SIGABRT
	SIGFPE                    = c_SIGFPE
	LC_ALL                    = c_LC_ALL
	LC_COLLATE                = c_LC_COLLATE
	LC_CTYPE                  = c_LC_CTYPE
	LC_MONETARY               = c_LC_MONETARY
	LC_NUMERIC                = c_LC_NUMERIC
	LC_TIME                   = c_LC_TIME
	MATH_ERRNO                = c_MATH_ERRNO
	MATH_ERREXCEPT            = c_MATH_ERREXCEPT
	Math_errhandling          = c_math_errhandling
	EXIT_SUCCESS              = c_EXIT_SUCCESS
	EXIT_FAILURE              = c_EXIT_FAILURE
	True                      = c_true
	False                     = c_false
	ATOMIC_BOOL_LOCK_FREE     = c_ATOMIC_BOOL_LOCK_FREE
	ATOMIC_CHAR_LOCK_FREE     = c_ATOMIC_CHAR_LOCK_FREE
	ATOMIC_CHAR16_T_LOCK_FREE = c_ATOMIC_CHAR16_T_LOCK_FREE
	ATOMIC_CHAR32_T_LOCK_FREE = c_ATOMIC_CHAR32_T_LOCK_FREE
	ATOMIC_WCHAR_T_LOCK_FREE  = c_ATOMIC_WCHAR_T_LOCK_FREE
	ATOMIC_SHORT_LOCK_FREE    = c_ATOMIC_SHORT_LOCK_FREE
	ATOMIC_INT_LOCK_FREE      = c_ATOMIC_INT_LOCK_FREE
	ATOMIC_LONG_LOCK_FREE     = c_ATOMIC_LONG_LOCK_FREE
	ATOMIC_LLONG_LOCK_FREE    = c_ATOMIC_LLONG_LOCK_FREE
	ATOMIC_POINTER_LOCK_FREE  = c_ATOMIC_POINTER_LOCK_FREE
	EOF                       = c_EOF
	FOPEN_MAX                 = c_FOPEN_MAX
	FILENAME_MAX              = c_FILENAME_MAX
	L_tmpnam                  = c_L_tmpnam
	TMP_MAX                   = c_TMP_MAX
	IOFBF                     = c__IOFBF
	IOLBF                     = c__IOLBF
	IONBF                     = c__IONBF
	BUFSIZ                    = c_BUFSIZ
	SEEK_SET                  = c_SEEK_SET
	SEEK_CUR                  = c_SEEK_CUR
	SEEK_END                  = c_SEEK_END
	CLOCKS_PER_SEC            = c_CLOCKS_PER_SEC
)

type (
	Char             c_char
	SignedChar       c_signed_char
	UnsignedChar     c_unsigned_char
	Short            c_short
	UnsignedShort    c_unsigned_short
	Int              c_int
	UnsignedInt      c_unsigned_int
	Long             c_long
	UnsignedLong     c_unsigned_long
	LongLong         c_longlong
	UnsignedLongLong c_unsigned_longlong
	Float            c_float
	Double           c_double
	LongDouble       c_long_double
	FastFloat        c_float_t
	FastDouble       c_double_t
	Int8             c_int8_t
	Int16            c_int16_t
	Int32            c_int32_t
	Int64            c_int64_t
	Uint8            c_uint8_t
	Uint16           c_uint16_t
	Uint32           c_uint32_t
	Uint64           c_uint64_t
	Char16           c_char16_t
	Char32           c_char32_t
	WideChar         c_wchar_t
	WideInt          c_wint_t
	Size             c_size_t
	Time             c_time_t
	Clock            c_clock_t
	Bool             c_bool
	Uintptr          c_uintptr_t
	Ptrdiff          c_ptrdiff_t
	Intptr           c_intptr_t
	MaxAlign         c_max_align_t
	SignedAtomic     c_sig_atomic_t
	IntMax           c_intmax_t
	UintMax          c_uintmax_t
	FastInt8         c_int_fast8_t
	FastInt16        c_int_fast16_t
	FastInt32        c_int_fast32_t
	FastInt64        c_int_fast64_t
	FastUint8        c_uint_fast8_t
	FastUint16       c_uint_fast16_t
	FastUint32       c_uint_fast32_t
	FastUint64       c_uint_fast64_t
	AtLeastInt8      c_int_least8_t
	AtLeastInt16     c_int_least16_t
	AtLeastInt32     c_int_least32_t
	AtLeastInt64     c_int_least64_t
	AtLeastUint8     c_uint_least8_t
	AtLeastUint16    c_uint_least16_t
	AtLeastUint32    c_uint_least32_t
	AtLeastUint64    c_uint_least64_t
)

// Sizeof returns 0 if the type is not supported.
func Sizeof(name string) uintptr {
	switch name {
	case "c_char":
		return unsafe.Sizeof(c_char(0))
	case "c_signed_char":
		return unsafe.Sizeof(c_signed_char(0))
	case "c_unsigned_char":
		return unsafe.Sizeof(c_unsigned_char(0))
	case "c_short":
		return unsafe.Sizeof(c_short(0))
	case "c_unsigned_short":
		return unsafe.Sizeof(c_unsigned_short(0))
	case "c_int":
		return unsafe.Sizeof(c_int(0))
	case "c_unsigned_int":
		return unsafe.Sizeof(c_unsigned_int(0))
	case "c_long":
		return unsafe.Sizeof(c_long(0))
	case "c_unsigned_long":
		return unsafe.Sizeof(c_unsigned_long(0))
	case "c_longlong":
		return unsafe.Sizeof(c_longlong(0))
	case "c_unsigned_longlong":
		return unsafe.Sizeof(c_unsigned_longlong(0))
	case "c_float":
		return unsafe.Sizeof(c_float(0))
	case "c_double":
		return unsafe.Sizeof(c_double(0))
	case "c_long_double":
		return unsafe.Sizeof(c_long_double(0))
	case "c_float_t":
		return unsafe.Sizeof(c_float_t(0))
	case "c_double_t":
		return unsafe.Sizeof(c_double_t(0))
	case "c_int8_t":
		return unsafe.Sizeof(c_int8_t(0))
	case "c_int16_t":
		return unsafe.Sizeof(c_int16_t(0))
	case "c_int32_t":
		return unsafe.Sizeof(c_int32_t(0))
	case "c_int64_t":
		return unsafe.Sizeof(c_int64_t(0))
	case "c_uint8_t":
		return unsafe.Sizeof(c_uint8_t(0))
	case "c_uint16_t":
		return unsafe.Sizeof(c_uint16_t(0))
	case "c_uint32_t":
		return unsafe.Sizeof(c_uint32_t(0))
	case "c_uint64_t":
		return unsafe.Sizeof(c_uint64_t(0))
	case "c_char16_t":
		return unsafe.Sizeof(c_char16_t(0))
	case "c_char32_t":
		return unsafe.Sizeof(c_char32_t(0))
	case "c_wchar_t":
		return unsafe.Sizeof(c_wchar_t(0))
	case "c_wint_t":
		return unsafe.Sizeof(c_wint_t(0))
	case "c_size_t":
		return unsafe.Sizeof(c_size_t(0))
	case "c_time_t":
		return unsafe.Sizeof(c_time_t(0))
	case "c_clock_t":
		return unsafe.Sizeof(c_clock_t(0))
	case "c_bool":
		return unsafe.Sizeof(c_bool(0))
	case "c_uintptr_t":
		return unsafe.Sizeof(c_uintptr_t(0))
	case "c_ptrdiff_t":
		return unsafe.Sizeof(c_ptrdiff_t(0))
	case "c_intptr_t":
		return unsafe.Sizeof(c_intptr_t(0))
	case "c_max_align_t":
		var maxalign c_max_align_t
		return unsafe.Sizeof(maxalign)
	case "c_sig_atomic_t":
		return unsafe.Sizeof(c_sig_atomic_t(0))
	case "c_intmax_t":
		return unsafe.Sizeof(c_intmax_t(0))
	case "c_uintmax_t":
		return unsafe.Sizeof(c_uintmax_t(0))
	case "c_int_fast8_t":
		return unsafe.Sizeof(c_int_fast8_t(0))
	case "c_int_fast16_t":
		return unsafe.Sizeof(c_int_fast16_t(0))
	case "c_int_fast32_t":
		return unsafe.Sizeof(c_int_fast32_t(0))
	case "c_int_fast64_t":
		return unsafe.Sizeof(c_int_fast64_t(0))
	case "c_uint_fast8_t":
		return unsafe.Sizeof(c_uint_fast8_t(0))
	case "c_uint_fast16_t":
		return unsafe.Sizeof(c_uint_fast16_t(0))
	case "c_uint_fast32_t":
		return unsafe.Sizeof(c_uint_fast32_t(0))
	case "c_uint_fast64_t":
		return unsafe.Sizeof(c_uint_fast64_t(0))
	case "c_int_least8_t":
		return unsafe.Sizeof(c_int_least8_t(0))
	case "c_int_least16_t":
		return unsafe.Sizeof(c_int_least16_t(0))
	case "c_int_least32_t":
		return unsafe.Sizeof(c_int_least32_t(0))
	case "c_int_least64_t":
		return unsafe.Sizeof(c_int_least64_t(0))
	case "c_uint_least8_t":
		return unsafe.Sizeof(c_uint_least8_t(0))
	case "c_uint_least16_t":
		return unsafe.Sizeof(c_uint_least16_t(0))
	case "c_uint_least32_t":
		return unsafe.Sizeof(c_uint_least32_t(0))
	case "c_uint_least64_t":
		return unsafe.Sizeof(c_uint_least64_t(0))
	default:
		return 0
	}
}

// Kind returns [reflect.Invalid] if the type is not supported.
func Kind(name string) reflect.Kind {
	switch name {
	case "char":
		return reflect.TypeOf(c_char(0)).Kind()
	case "signed_char":
		return reflect.TypeOf(c_signed_char(0)).Kind()
	case "unsigned_char":
		return reflect.TypeOf(c_unsigned_char(0)).Kind()
	case "short":
		return reflect.TypeOf(c_short(0)).Kind()
	case "unsigned_short":
		return reflect.TypeOf(c_unsigned_short(0)).Kind()
	case "int":
		return reflect.TypeOf(c_int(0)).Kind()
	case "unsigned_int":
		return reflect.TypeOf(c_unsigned_int(0)).Kind()
	case "long":
		return reflect.TypeOf(c_long(0)).Kind()
	case "unsigned_long":
		return reflect.TypeOf(c_unsigned_long(0)).Kind()
	case "longlong":
		return reflect.TypeOf(c_longlong(0)).Kind()
	case "unsigned_longlong":
		return reflect.TypeOf(c_unsigned_longlong(0)).Kind()
	case "float":
		return reflect.TypeOf(c_float(0)).Kind()
	case "double":
		return reflect.TypeOf(c_double(0)).Kind()
	case "long_double":
		return reflect.TypeOf(c_long_double(0)).Kind()
	case "float_t":
		return reflect.TypeOf(c_float_t(0)).Kind()
	case "double_t":
		return reflect.TypeOf(c_double_t(0)).Kind()
	case "int8_t":
		return reflect.TypeOf(c_int8_t(0)).Kind()
	case "int16_t":
		return reflect.TypeOf(c_int16_t(0)).Kind()
	case "int32_t":
		return reflect.TypeOf(c_int32_t(0)).Kind()
	case "int64_t":
		return reflect.TypeOf(c_int64_t(0)).Kind()
	case "uint8_t":
		return reflect.TypeOf(c_uint8_t(0)).Kind()
	case "uint16_t":
		return reflect.TypeOf(c_uint16_t(0)).Kind()
	case "uint32_t":
		return reflect.TypeOf(c_uint32_t(0)).Kind()
	case "uint64_t":
		return reflect.TypeOf(c_uint64_t(0)).Kind()
	case "char16_t":
		return reflect.TypeOf(c_char16_t(0)).Kind()
	case "char32_t":
		return reflect.TypeOf(c_char32_t(0)).Kind()
	case "wchar_t":
		return reflect.TypeOf(c_wchar_t(0)).Kind()
	case "wint_t":
		return reflect.TypeOf(c_wint_t(0)).Kind()
	case "size_t":
		return reflect.TypeOf(c_size_t(0)).Kind()
	case "time_t":
		return reflect.TypeOf(c_time_t(0)).Kind()
	case "clock_t":
		return reflect.TypeOf(c_clock_t(0)).Kind()
	case "bool":
		return reflect.TypeOf(c_bool(0)).Kind()
	case "uintptr_t":
		return reflect.TypeOf(c_uintptr_t(0)).Kind()
	case "ptrdiff_t":
		return reflect.TypeOf(c_ptrdiff_t(0)).Kind()
	case "intptr_t":
		return reflect.TypeOf(c_intptr_t(0)).Kind()
	case "max_align_t":
		var maxalign c_max_align_t
		return reflect.TypeOf(maxalign).Kind()
	case "sig_atomic_t":
		return reflect.TypeOf(c_sig_atomic_t(0)).Kind()
	case "intmax_t":
		return reflect.TypeOf(c_intmax_t(0)).Kind()
	case "uintmax_t":
		return reflect.TypeOf(c_uintmax_t(0)).Kind()
	case "int_fast8_t":
		return reflect.TypeOf(c_int_fast8_t(0)).Kind()
	case "int_fast16_t":
		return reflect.TypeOf(c_int_fast16_t(0)).Kind()
	case "int_fast32_t":
		return reflect.TypeOf(c_int_fast32_t(0)).Kind()
	case "int_fast64_t":
		return reflect.TypeOf(c_int_fast64_t(0)).Kind()
	case "uint_fast8_t":
		return reflect.TypeOf(c_uint_fast8_t(0)).Kind()
	case "uint_fast16_t":
		return reflect.TypeOf(c_uint_fast16_t(0)).Kind()
	case "uint_fast32_t":
		return reflect.TypeOf(c_uint_fast32_t(0)).Kind()
	case "uint_fast64_t":
		return reflect.TypeOf(c_uint_fast64_t(0)).Kind()
	case "int_least8_t":
		return reflect.TypeOf(c_int_least8_t(0)).Kind()
	case "int_least16_t":
		return reflect.TypeOf(c_int_least16_t(0)).Kind()
	case "int_least32_t":
		return reflect.TypeOf(c_int_least32_t(0)).Kind()
	case "int_least64_t":
		return reflect.TypeOf(c_int_least64_t(0)).Kind()
	case "uint_least8_t":
		return reflect.TypeOf(c_uint_least8_t(0)).Kind()
	case "uint_least16_t":
		return reflect.TypeOf(c_uint_least16_t(0)).Kind()
	case "uint_least32_t":
		return reflect.TypeOf(c_uint_least32_t(0)).Kind()
	case "uint_least64_t":
		return reflect.TypeOf(c_uint_least64_t(0)).Kind()
	default:
		return reflect.Invalid
	}
}

// Const returns an empty string if the constant is not supported.
func Const(name string) string {
	switch name {
	case "CHAR_BIT":
		return fmt.Sprint(c_CHAR_BIT)
	case "MB_LEN_MAX":
		return fmt.Sprint(c_MB_LEN_MAX)
	case "CHAR_MIN":
		return fmt.Sprint(c_CHAR_MIN)
	case "CHAR_MAX":
		return fmt.Sprint(c_CHAR_MAX)
	case "SCHAR_MIN":
		return fmt.Sprint(c_SCHAR_MIN)
	case "SHRT_MIN":
		return fmt.Sprint(c_SHRT_MIN)
	case "INT_MIN":
		return fmt.Sprint(c_INT_MIN)
	case "LONG_MIN":
		return fmt.Sprint(c_LONG_MIN)
	case "LLONG_MIN":
		return fmt.Sprint(c_LLONG_MIN)
	case "SCHAR_MAX":
		return fmt.Sprint(c_SCHAR_MAX)
	case "SHRT_MAX":
		return fmt.Sprint(c_SHRT_MAX)
	case "INT_MAX":
		return fmt.Sprint(c_INT_MAX)
	case "LONG_MAX":
		return fmt.Sprint(c_LONG_MAX)
	case "LLONG_MAX":
		return fmt.Sprint(c_LLONG_MAX)
	case "UCHAR_MAX":
		return fmt.Sprint(c_UCHAR_MAX)
	case "USHRT_MAX":
		return fmt.Sprint(c_USHRT_MAX)
	case "UINT_MAX":
		return fmt.Sprint(c_UINT_MAX)
	case "ULONG_MAX":
		return fmt.Sprint(uint64(c_ULONG_MAX))
	case "ULLONG_MAX":
		return fmt.Sprint(uint64(c_ULLONG_MAX))
	case "PTRDIFF_MIN":
		return fmt.Sprint(c_PTRDIFF_MIN)
	case "PTRDIFF_MAX":
		return fmt.Sprint(c_PTRDIFF_MAX)
	case "SIZE_MAX":
		return fmt.Sprint(uint64(c_SIZE_MAX))
	case "WINT_MIN":
		return fmt.Sprint(c_WINT_MIN)
	case "WINT_MAX":
		return fmt.Sprint(c_WINT_MAX)
	case "WCHAR_MIN":
		return fmt.Sprint(c_WCHAR_MIN)
	case "WCHAR_MAX":
		return fmt.Sprint(c_WCHAR_MAX)
	case "INT8_MIN":
		return fmt.Sprint(c_INT8_MIN)
	case "INT16_MIN":
		return fmt.Sprint(c_INT16_MIN)
	case "INT32_MIN":
		return fmt.Sprint(c_INT32_MIN)
	case "INT64_MIN":
		return fmt.Sprint(c_INT64_MIN)
	case "INT8_MAX":
		return fmt.Sprint(c_INT8_MAX)
	case "INT16_MAX":
		return fmt.Sprint(c_INT16_MAX)
	case "INT32_MAX":
		return fmt.Sprint(c_INT32_MAX)
	case "INT64_MAX":
		return fmt.Sprint(c_INT64_MAX)
	case "UINT8_MAX":
		return fmt.Sprint(c_UINT8_MAX)
	case "UINT16_MAX":
		return fmt.Sprint(c_UINT16_MAX)
	case "UINT32_MAX":
		return fmt.Sprint(c_UINT32_MAX)
	case "UINT64_MAX":
		return fmt.Sprint(uint64(c_UINT64_MAX))
	case "INT_FAST8_MIN":
		return fmt.Sprint(c_INT_FAST8_MIN)
	case "INT_FAST16_MIN":
		return fmt.Sprint(c_INT_FAST16_MIN)
	case "INT_FAST32_MIN":
		return fmt.Sprint(c_INT_FAST32_MIN)
	case "INT_FAST64_MIN":
		return fmt.Sprint(c_INT_FAST64_MIN)
	case "INT_FAST8_MAX":
		return fmt.Sprint(c_INT_FAST8_MAX)
	case "INT_FAST16_MAX":
		return fmt.Sprint(c_INT_FAST16_MAX)
	case "INT_FAST32_MAX":
		return fmt.Sprint(c_INT_FAST32_MAX)
	case "INT_FAST64_MAX":
		return fmt.Sprint(c_INT_FAST64_MAX)
	case "UINT_FAST8_MAX":
		return fmt.Sprint(c_UINT_FAST8_MAX)
	case "UINT_FAST16_MAX":
		return fmt.Sprint(uint64(c_UINT_FAST16_MAX))
	case "UINT_FAST32_MAX":
		return fmt.Sprint(uint64(c_UINT_FAST32_MAX))
	case "UINT_FAST64_MAX":
		return fmt.Sprint(uint64(c_UINT_FAST64_MAX))
	case "INT_LEAST8_MIN":
		return fmt.Sprint(c_INT_LEAST8_MIN)
	case "INT_LEAST16_MIN":
		return fmt.Sprint(c_INT_LEAST16_MIN)
	case "INT_LEAST32_MIN":
		return fmt.Sprint(c_INT_LEAST32_MIN)
	case "INT_LEAST64_MIN":
		return fmt.Sprint(c_INT_LEAST64_MIN)
	case "INT_LEAST8_MAX":
		return fmt.Sprint(c_INT_LEAST8_MAX)
	case "INT_LEAST16_MAX":
		return fmt.Sprint(c_INT_LEAST16_MAX)
	case "INT_LEAST32_MAX":
		return fmt.Sprint(c_INT_LEAST32_MAX)
	case "INT_LEAST64_MAX":
		return fmt.Sprint(c_INT_LEAST64_MAX)
	case "UINT_LEAST8_MAX":
		return fmt.Sprint(c_UINT_LEAST8_MAX)
	case "UINT_LEAST16_MAX":
		return fmt.Sprint(c_UINT_LEAST16_MAX)
	case "UINT_LEAST32_MAX":
		return fmt.Sprint(c_UINT_LEAST32_MAX)
	case "UINT_LEAST64_MAX":
		return fmt.Sprint(uint64(c_UINT_LEAST64_MAX))
	case "INTMAX_MIN":
		return fmt.Sprint(c_INTMAX_MIN)
	case "INTMAX_MAX":
		return fmt.Sprint(c_INTMAX_MAX)
	case "UINTMAX_MAX":
		return fmt.Sprint(uint64(c_UINTMAX_MAX))
	case "INTPTR_MIN":
		return fmt.Sprint(c_INTPTR_MIN)
	case "INTPTR_MAX":
		return fmt.Sprint(c_INTPTR_MAX)
	case "UINTPTR_MAX":
		return fmt.Sprint(uint64(c_UINTPTR_MAX))
	case "SIG_ATOMIC_MIN":
		return fmt.Sprint(c_SIG_ATOMIC_MIN)
	case "SIG_ATOMIC_MAX":
		return fmt.Sprint(c_SIG_ATOMIC_MAX)
	case "FLT_RADIX":
		return fmt.Sprint(c_FLT_RADIX)
	case "DECIMAL_DIG":
		return fmt.Sprint(c_DECIMAL_DIG)
	case "FLT_DECIMAL_DIG":
		return fmt.Sprint(c_FLT_DECIMAL_DIG)
	case "DBL_DECIMAL_DIG":
		return fmt.Sprint(c_DBL_DECIMAL_DIG)
	case "LDBL_DECIMAL_DIG":
		return fmt.Sprint(c_LDBL_DECIMAL_DIG)
	case "FLT_MIN":
		return fmt.Sprint(c_FLT_MIN)
	case "DBL_MIN":
		return fmt.Sprint(c_DBL_MIN)
	case "LDBL_MIN":
		return fmt.Sprint(c_LDBL_MIN)
	case "FLT_TRUE_MIN":
		return fmt.Sprint(c_FLT_TRUE_MIN)
	case "DBL_TRUE_MIN":
		return fmt.Sprint(c_DBL_TRUE_MIN)
	case "LDBL_TRUE_MIN":
		return fmt.Sprint(c_LDBL_TRUE_MIN)
	case "FLT_MAX":
		return fmt.Sprint(c_FLT_MAX)
	case "DBL_MAX":
		return fmt.Sprint(c_DBL_MAX)
	case "LDBL_MAX":
		return "" // too big
	case "FLT_EPSILON":
		return fmt.Sprint(c_FLT_EPSILON)
	case "DBL_EPSILON":
		return fmt.Sprint(c_DBL_EPSILON)
	case "LDBL_EPSILON":
		return fmt.Sprint(c_LDBL_EPSILON)
	case "FLT_DIG":
		return fmt.Sprint(c_FLT_DIG)
	case "DBL_DIG":
		return fmt.Sprint(c_DBL_DIG)
	case "LDBL_DIG":
		return fmt.Sprint(c_LDBL_DIG)
	case "FLT_MANT_DIG":
		return fmt.Sprint(c_FLT_MANT_DIG)
	case "DBL_MANT_DIG":
		return fmt.Sprint(c_DBL_MANT_DIG)
	case "LDBL_MANT_DIG":
		return fmt.Sprint(c_LDBL_MANT_DIG)
	case "FLT_MIN_EXP":
		return fmt.Sprint(c_FLT_MIN_EXP)
	case "DBL_MIN_EXP":
		return fmt.Sprint(c_DBL_MIN_EXP)
	case "LDBL_MIN_EXP":
		return fmt.Sprint(c_LDBL_MIN_EXP)
	case "FLT_MIN_10_EXP":
		return fmt.Sprint(c_FLT_MIN_10_EXP)
	case "DBL_MIN_10_EXP":
		return fmt.Sprint(c_DBL_MIN_10_EXP)
	case "LDBL_MIN_10_EXP":
		return fmt.Sprint(c_LDBL_MIN_10_EXP)
	case "FLT_MAX_EXP":
		return fmt.Sprint(c_FLT_MAX_EXP)
	case "DBL_MAX_EXP":
		return fmt.Sprint(c_DBL_MAX_EXP)
	case "LDBL_MAX_EXP":
		return fmt.Sprint(c_LDBL_MAX_EXP)
	case "FLT_MAX_10_EXP":
		return fmt.Sprint(c_FLT_MAX_10_EXP)
	case "DBL_MAX_10_EXP":
		return fmt.Sprint(c_DBL_MAX_10_EXP)
	case "LDBL_MAX_10_EXP":
		return fmt.Sprint(c_LDBL_MAX_10_EXP)
	case "FLT_ROUNDS":
		return fmt.Sprint(c_FLT_ROUNDS)
	case "FLT_EVAL_METHOD":
		return fmt.Sprint(c_FLT_EVAL_METHOD)
	case "FLT_HAS_SUBNORM":
		return fmt.Sprint(c_FLT_HAS_SUBNORM)
	case "DBL_HAS_SUBNORM":
		return fmt.Sprint(c_DBL_HAS_SUBNORM)
	case "LDBL_HAS_SUBNORM":
		return fmt.Sprint(c_LDBL_HAS_SUBNORM)
	case "EDOM":
		return fmt.Sprint(c_EDOM)
	case "ERANGE":
		return fmt.Sprint(c_ERANGE)
	case "EILSEQ":
		return fmt.Sprint(c_EILSEQ)
	case "FE_DFL_ENV":
		return fmt.Sprint(c_FE_DFL_ENV)
	case "FE_DIVBYZERO":
		return fmt.Sprint(c_FE_DIVBYZERO)
	case "FE_INEXACT":
		return fmt.Sprint(c_FE_INEXACT)
	case "FE_INVALID":
		return fmt.Sprint(c_FE_INVALID)
	case "FE_OVERFLOW":
		return fmt.Sprint(c_FE_OVERFLOW)
	case "FE_UNDERFLOW":
		return fmt.Sprint(c_FE_UNDERFLOW)
	case "FE_ALL_EXCEPT":
		return fmt.Sprint(c_FE_ALL_EXCEPT)
	case "fegetround":
		return fmt.Sprint(c_fegetround)
	case "FE_DOWNWARD":
		return fmt.Sprint(c_FE_DOWNWARD)
	case "FE_TONEAREST":
		return fmt.Sprint(c_FE_TONEAREST)
	case "FE_TOWARDZERO":
		return fmt.Sprint(c_FE_TOWARDZERO)
	case "FE_UPWARD":
		return fmt.Sprint(c_FE_UPWARD)
	case "FP_NORMAL":
		return fmt.Sprint(c_FP_NORMAL)
	case "FP_SUBNORMAL":
		return fmt.Sprint(c_FP_SUBNORMAL)
	case "FP_ZERO":
		return fmt.Sprint(c_FP_ZERO)
	case "FP_INFINITE":
		return fmt.Sprint(c_FP_INFINITE)
	case "FP_NAN":
		return fmt.Sprint(c_FP_NAN)
	case "SIGTERM":
		return fmt.Sprint(c_SIGTERM)
	case "SIGSEGV":
		return fmt.Sprint(c_SIGSEGV)
	case "SIGINT":
		return fmt.Sprint(c_SIGINT)
	case "SIGILL":
		return fmt.Sprint(c_SIGILL)
	case "SIGABRT":
		return fmt.Sprint(c_SIGABRT)
	case "SIGFPE":
		return fmt.Sprint(c_SIGFPE)
	case "LC_ALL":
		return fmt.Sprint(c_LC_ALL)
	case "LC_COLLATE":
		return fmt.Sprint(c_LC_COLLATE)
	case "LC_CTYPE":
		return fmt.Sprint(c_LC_CTYPE)
	case "LC_MONETARY":
		return fmt.Sprint(c_LC_MONETARY)
	case "LC_NUMERIC":
		return fmt.Sprint(c_LC_NUMERIC)
	case "LC_TIME":
		return fmt.Sprint(c_LC_TIME)
	case "MATH_ERRNO":
		return fmt.Sprint(c_MATH_ERRNO)
	case "MATH_ERREXCEPT":
		return fmt.Sprint(c_MATH_ERREXCEPT)
	case "math_errhandling":
		return fmt.Sprint(c_math_errhandling)
	case "EXIT_SUCCESS":
		return fmt.Sprint(c_EXIT_SUCCESS)
	case "EXIT_FAILURE":
		return fmt.Sprint(c_EXIT_FAILURE)
	case "true":
		return fmt.Sprint(c_true)
	case "false":
		return fmt.Sprint(c_false)
	case "ATOMIC_BOOL_LOCK_FREE":
		return fmt.Sprint(c_ATOMIC_BOOL_LOCK_FREE)
	case "ATOMIC_CHAR_LOCK_FREE":
		return fmt.Sprint(c_ATOMIC_CHAR_LOCK_FREE)
	case "ATOMIC_CHAR16_T_LOCK_FREE":
		return fmt.Sprint(c_ATOMIC_CHAR16_T_LOCK_FREE)
	case "ATOMIC_CHAR32_T_LOCK_FREE":
		return fmt.Sprint(c_ATOMIC_CHAR32_T_LOCK_FREE)
	case "ATOMIC_WCHAR_T_LOCK_FREE":
		return fmt.Sprint(c_ATOMIC_WCHAR_T_LOCK_FREE)
	case "ATOMIC_SHORT_LOCK_FREE":
		return fmt.Sprint(c_ATOMIC_SHORT_LOCK_FREE)
	case "ATOMIC_INT_LOCK_FREE":
		return fmt.Sprint(c_ATOMIC_INT_LOCK_FREE)
	case "ATOMIC_LONG_LOCK_FREE":
		return fmt.Sprint(c_ATOMIC_LONG_LOCK_FREE)
	case "ATOMIC_LLONG_LOCK_FREE":
		return fmt.Sprint(c_ATOMIC_LLONG_LOCK_FREE)
	case "ATOMIC_POINTER_LOCK_FREE":
		return fmt.Sprint(c_ATOMIC_POINTER_LOCK_FREE)
	case "EOF":
		return fmt.Sprint(c_EOF)
	case "FOPEN_MAX":
		return fmt.Sprint(c_FOPEN_MAX)
	case "FILENAME_MAX":
		return fmt.Sprint(c_FILENAME_MAX)
	case "L_tmpnam":
		return fmt.Sprint(c_L_tmpnam)
	case "TMP_MAX":
		return fmt.Sprint(c_TMP_MAX)
	case "_IOFBF":
		return fmt.Sprint(c__IOFBF)
	case "_IOLBF":
		return fmt.Sprint(c__IOLBF)
	case "_IONBF":
		return fmt.Sprint(c__IONBF)
	case "BUFSIZ":
		return fmt.Sprint(c_BUFSIZ)
	case "SEEK_SET":
		return fmt.Sprint(c_SEEK_SET)
	case "SEEK_CUR":
		return fmt.Sprint(c_SEEK_CUR)
	case "SEEK_END":
		return fmt.Sprint(c_SEEK_END)
	case "CLOCKS_PER_SEC":
		return fmt.Sprint(c_CLOCKS_PER_SEC)
	default:
		return ""
	}
}

type Error int

func (err *Error) Error() string {
	return fmt.Sprintf("error %d", *err)
}
