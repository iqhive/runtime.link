package cgo

import (
	"fmt"
	"reflect"
	"unsafe"
)

type value struct {
	kind reflect.Kind
	size uintptr

	align_memory int
	align_struct int
}

func newValue[T any]() value {
	rtype := reflect.TypeOf([0]T{}).Elem()
	return value{
		kind:         rtype.Kind(),
		size:         rtype.Size(),
		align_memory: rtype.Align(),
		align_struct: rtype.FieldAlign(),
	}
}

type types struct {
	char,
	signed_char,
	unsigned_char,
	short,
	unsigned_short,
	int,
	unsigned_int,
	long,
	unsigned_long,
	long_long,
	unsigned_long_long,
	float,
	double,
	long_double,
	float_t,
	double_t,
	int8_t,
	int16_t,
	int32_t,
	int64_t,
	uint8_t,
	uint16_t,
	uint32_t,
	uint64_t,
	char16_t,
	char32_t,
	wchar_t,
	wint_t,
	size_t,
	time_t,
	clock_t,
	bool,
	uintptr_t,
	ptrdiff_t,
	intptr_t,
	max_align_t,
	sig_atomic_t,
	intmax_t,
	uintmax_t,
	int_fast8_t,
	int_fast16_t,
	int_fast32_t,
	int_fast64_t,
	uint_fast8_t,
	uint_fast16_t,
	uint_fast32_t,
	uint_fast64_t,
	int_least8_t,
	int_least16_t,
	int_least32_t,
	int_least64_t,
	uint_least8_t,
	uint_least16_t,
	uint_least32_t,
	uint_least64_t value
}

func (c *types) LookupKind(name string) reflect.Kind {
	if name == "void" {
		return reflect.UnsafePointer
	}
	if name == "func" {
		return reflect.Func
	}
	field, ok := reflect.TypeOf(c).Elem().FieldByName(name)
	if !ok {
		return reflect.Invalid
	}
	return (*value)(unsafe.Add(unsafe.Pointer(c), field.Offset)).kind
}

func (c *types) LookupSize(name string) uintptr {
	field, ok := reflect.TypeOf(c).Elem().FieldByName(name)
	if !ok {
		return 0
	}
	return (*value)(unsafe.Add(unsafe.Pointer(c), field.Offset)).size
}

func (c *types) LookupMemoryAlignment(name string) int {
	field, ok := reflect.TypeOf(c).Elem().FieldByName(name)
	if !ok {
		return -1
	}
	return (*value)(unsafe.Add(unsafe.Pointer(c), field.Offset)).align_memory
}

func (c *types) LookupStructAlignment(name string) int {
	field, ok := reflect.TypeOf(c).Elem().FieldByName(name)
	if !ok {
		return -1
	}
	return (*value)(unsafe.Add(unsafe.Pointer(c), field.Offset)).align_struct
}

// Constants defined by the C standard.
type constants struct {
	CHAR_BIT                uint64
	MB_LEN_MAX              uint64
	CHAR_MIN                int64
	CHAR_MAX                uint64
	SCHAR_MIN               int64
	SHRT_MIN                int64
	INT_MIN                 int64
	LONG_MIN                int64
	LLONG_MIN               int64
	SCHAR_MAX               uint64
	SHRT_MAX                uint64
	INT_MAX                 uint64
	LONG_MAX                uint64
	LLONG_MAX               uint64
	UCHAR_MAX               uint64
	USHRT_MAX               uint64
	UINT_MAX                uint64
	ULONG_MAX               uint64
	ULLONG_MAX              uint64
	PTRDIFF_MIN             int64
	PTRDIFF_MAX             uint64
	SIZE_MAX                uint64
	WINT_MIN                int64
	WINT_MAX                uint64
	WCHAR_MIN               int64
	WCHAR_MAX               uint64
	INT8_MIN                int64
	INT16_MIN               int64
	INT32_MIN               int64
	INT64_MIN               int64
	INT8_MAX                uint64
	INT16_MAX               uint64
	INT32_MAX               uint64
	INT64_MAX               uint64
	UINT8_MAX               uint64
	UINT16_MAX              uint64
	UINT32_MAX              uint64
	UINT64_MAX              uint64
	INT_FAST8_MIN           int64
	INT_FAST16_MIN          int64
	INT_FAST32_MIN          int64
	INT_FAST64_MIN          int64
	INT_FAST8_MAX           uint64
	INT_FAST16_MAX          uint64
	INT_FAST32_MAX          uint64
	INT_FAST64_MAX          uint64
	UINT_FAST8_MAX          uint64
	UINT_FAST16_MAX         uint64
	UINT_FAST32_MAX         uint64
	UINT_FAST64_MAX         uint64
	INT_LEAST8_MIN          int64
	INT_LEAST16_MIN         int64
	INT_LEAST32_MIN         int64
	INT_LEAST64_MIN         int64
	INT_LEAST8_MAX          uint64
	INT_LEAST16_MAX         uint64
	INT_LEAST32_MAX         uint64
	INT_LEAST64_MAX         uint64
	UINT_LEAST8_MAX         uint64
	UINT_LEAST16_MAX        uint64
	UINT_LEAST32_MAX        uint64
	UINT_LEAST64_MAX        uint64
	INTMAX_MIN              int64
	INTMAX_MAX              uint64
	UINTMAX_MAX             uint64
	INTPTR_MIN              int64
	INTPTR_MAX              uint64
	UINTPTR_MAX             uint64
	SIG_ATOMIMIN            int64
	SIG_ATOMIMAX            uint64
	FLT_RADIX               uint8
	DECIMAL_DIG             uint8
	FLT_DECIMAL_DIG         uint8
	DBL_DECIMAL_DIG         uint8
	LDBL_DECIMAL_DIG        uint8
	FLT_MIN                 float64
	DBL_MIN                 float64
	LDBL_MIN                string
	FLT_TRUE_MIN            float64
	DBL_TRUE_MIN            float64
	LDBL_TRUE_MIN           string
	FLT_MAX                 float64
	DBL_MAX                 float64
	LDBL_MAX                string
	FLT_EPSILON             float64
	DBL_EPSILON             float64
	LDBL_EPSILON            float64
	FLT_DIG                 uint8
	DBL_DIG                 uint8
	LDBL_DIG                uint8
	FLT_MANT_DIG            uint8
	DBL_MANT_DIG            uint8
	LDBL_MANT_DIG           uint8
	FLT_MIN_EXP             int64
	DBL_MIN_EXP             int64
	LDBL_MIN_EXP            int64
	FLT_MIN_10_EXP          int64
	DBL_MIN_10_EXP          int64
	LDBL_MIN_10_EXP         int64
	FLT_MAX_EXP             uint64
	DBL_MAX_EXP             uint64
	LDBL_MAX_EXP            uint64
	FLT_MAX_10_EXP          uint64
	DBL_MAX_10_EXP          uint64
	LDBL_MAX_10_EXP         uint64
	FLT_ROUNDS              int64
	FLT_EVAL_METHOD         int64
	FLT_HAS_SUBNORM         int64
	DBL_HAS_SUBNORM         int64
	LDBL_HAS_SUBNORM        int64
	EDOM                    int64
	ERANGE                  int64
	EILSEQ                  int64
	FE_DFL_ENV              int64
	FE_DIVBYZERO            int64
	FE_INEXACT              int64
	FE_INVALID              int64
	FE_OVERFLOW             int64
	FE_UNDERFLOW            int64
	FE_ALL_EXCEPT           int64
	fegetround              int64
	FE_DOWNWARD             int64
	FE_TONEAREST            int64
	FE_TOWARDZERO           int64
	FE_UPWARD               int64
	FP_NORMAL               int64
	FP_SUBNORMAL            int64
	FP_ZERO                 int64
	FP_INFINITE             int64
	FP_NAN                  int64
	SIGTERM                 int64
	SIGSEGV                 int64
	SIGINT                  int64
	SIGILL                  int64
	SIGABRT                 int64
	SIGFPE                  int64
	LALL                    int64
	LCOLLATE                int64
	LCTYPE                  int64
	LMONETARY               int64
	LNUMERIC                int64
	LTIME                   int64
	MATH_ERRNO              int64
	MATH_ERREXCEPT          int64
	math_errhandling        int64
	EXIT_SUCCESS            int64
	EXIT_FAILURE            int64
	True                    int64
	False                   int64
	ATOMIBOOL_LOCK_FREE     int64
	ATOMICHAR_LOCK_FREE     int64
	ATOMICHAR16_T_LOCK_FREE int64
	ATOMICHAR32_T_LOCK_FREE int64
	ATOMIWCHAR_T_LOCK_FREE  int64
	ATOMISHORT_LOCK_FREE    int64
	ATOMIINT_LOCK_FREE      int64
	ATOMILONG_LOCK_FREE     int64
	ATOMILLONG_LOCK_FREE    int64
	ATOMIPOINTER_LOCK_FREE  int64
	EOF                     int64
	FOPEN_MAX               uint64
	FILENAME_MAX            uint64
	L_tmpnam                uint64
	TMP_MAX                 uint64
	_IOFBF                  int64
	_IOLBF                  int64
	_IONBF                  int64
	BUFSIZ                  uint64
	SEEK_SET                int64
	SEEK_CUR                int64
	SEEK_END                int64
	CLOCKS_PER_SEC          int64
}

func (c *constants) Lookup(name string) string {
	return fmt.Sprint(reflect.ValueOf(c).Elem().FieldByName(name))
}
