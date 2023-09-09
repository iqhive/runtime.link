package lib

import (
	"os"
	"time"
	"unsafe"

	"runtime.link/ffi"
)

type location struct {
	linux   Location `lib:"libc.so.6 libm.so.6"`
	darwin  Location `lib:"libSystem.dylib"`
	windows Location `lib:"msvcrt.dll"`
}

// Library provides provides the ANSI C standard library.
// https://www.csse.uwa.edu.au/programming/ansic-library.html
// Function names have been expanded to prefer full words over
// abbreviations. The functions have been organised into sensible
// categories. All functions except for those in the "Jump" field
// are safe to use from Go.
type C struct {
	location

	IO struct { // IO provides stdin/stdout functions from <stdio.h>.
		Printf func(format string, args ...any) (int, error) `lib:"fn(&s,&v%v...%@1)i<0; printf"`
		Scanf  func(format string, args ...any) (int, error) `lib:"fn(&s,&v%v...%@1)i<0; scanf"`

		GetChar   func() rune                   `lib:"fn()i;       getchar"`
		GetString func(s unsafe.Pointer) string `lib:"fn(&v)$s^@1; gets"`
		PutChar   func(c rune) error            `lib:"fn(i)i<0;    putchar"`
		PutString func(s string) error          `lib:"fn(&s)i<0;   puts"`
		Error     func(s string)                `lib:"fn(&s);      perror"`
	}
	Math struct { // Math provides numerical functions from <math.h> and <stdlib.h>.
		Abs      func(x int32) int32                 `lib:"abs func(int)int"`
		Sin      func(x float64) float64             `lib:"sin func(double)double"`
		Cos      func(x float64) float64             `lib:"cos func(double)double"`
		Tan      func(x float64) float64             `lib:"tan func(double)double"`
		Asin     func(x float64) float64             `lib:"asin func(double)double"`
		Atan2    func(x, y float64) float64          `lib:"atan2 func(double,double)double"`
		Sinh     func(x float64) float64             `lib:"sinh func(double)double"`
		Cosh     func(x float64) float64             `lib:"cosh func(double)double"`
		Tanh     func(x float64) float64             `lib:"tanh func(double)double"`
		Exp      func(x float64) float64             `lib:"exp func(double)double"`
		Log      func(x float64) float64             `lib:"log func(double)double"`
		Log10    func(x float64) float64             `lib:"log10 func(double)double"`
		Pow      func(x, y float64) float64          `lib:"pow func(double,double)double"`
		Sqrt     func(x float64) float64             `lib:"sqrt func(double)double"`
		Ceil     func(x float64) float64             `lib:"ceil func(double)double"`
		Floor    func(x float64) float64             `lib:"floor func(double)double"`
		Fabs     func(x float64) float64             `lib:"fabs func(double)double"`
		Ldexp    func(x float64, n int32) float64    `lib:"ldexp func(double,int)double"`
		Frexp    func(x float64, exp *int32) float64 `lib:"frexp func(double,&int)double"`
		Modf     func(x float64, y *float64) float64 `lib:"modf func(double,&double)double"`
		Rand     func() int32                        `lib:"rand func()int"`
		SeedRand func(seed uint32) int32             `lib:"srand func(unsigned_int)int"`
	}
	Time struct { // Time provides time-related functions from <time.h>.
		Sub    func(t1, t2 time.Time) time.Time `lib:"difftime func(time_t,time_t)time_t"`
		String func(t time.Time) string         `lib:"ctime func(time_t)$char"`
		UTC    func(t time.Time) time.Time      `lib:"gmtime func(time_t)$tm"`
		Local  func(t time.Time) time.Time      `lib:"localtime func(time_t)$tm"`
	}
	Date struct { // Date provides date-related functions from <time.h>.
		Time   func(t time.Time) time.Time `lib:"mktime func(&tm)time_t"`
		String func(t time.Time) string    `lib:"asctime func(&tm)$char"`

		Format func(s []byte, format string, tp time.Time) (int, error) `lib:"strftime func(&char,-size_t[=@1],&char,&tm)size_t=0"`
	}
	File struct { // File provides file-related functions from <stdio.h>.
		Open   func(filename string, mode string) ffi.File                  `lib:"fopen func(&char,&char)$FILE"`
		Reopen func(filename string, mode string, stream ffi.File) ffi.File `lib:"freopen func(&char,&char,#FILE)$FILE"`
		Flush  func(stream ffi.File) error                                  `lib:"fflush func(&FILE)int=0"`
		Close  func(stream ffi.File) error                                  `lib:"fclose func(#FILE)int=0"`
		Remove func(filename string) error                                  `lib:"remove func(&char)int=0"`
		Rename func(oldname, newname string) error                          `lib:"rename func(&char,&char)int=0"`

		Temp     func() ffi.File     `lib:"tmpfile func()$FILE"`
		TempName func([]byte) string `lib:"tmpnam func(&char[>=L_tmpnam_s])$char^@1"`

		SetBufferMode func(stream ffi.File, buf []byte, mode int) error `lib:"setvbuf func(&FILE,&void,int,-size_t[=@2])int=0"`
		SetBuffer     func(stream ffi.File, buf []byte) error           `lib:"setbuf func(&FILE,&void[>=BUFSIZE])int=0"`

		Printf func(stream ffi.File, format string, args ...any) (int, error) `lib:"fprintf func(&FILE,&char,&void...?@2)int>=0 "`
		Scanf  func(stream ffi.File, format string, args ...any) (int, error) `lib:"fscanf func(&FILE,&char,&void...?@2)int>=0"`

		GetChar    func(stream ffi.File) rune                          `lib:"fgetc func(&FILE)int"`
		GetString  func(s []byte, stream ffi.File) string              `lib:"fgets func(&char,-int[=@1],&FILE)$char^@1"`
		PutChar    func(c rune, stream ffi.File) rune                  `lib:"fputc func(int,&FILE)int=0"`
		Unget      func(c rune, stream ffi.File) rune                  `lib:"ungetc func(int,&FILE)int=0"`
		Read       func(ptr []byte, stream ffi.File) int               `lib:"fread func(&void,-size_t=1,-size_t[=@1],&FILE)int>=0"`
		Write      func(ptr []byte, stream ffi.File) int               `lib:"fwrite func(&void,-size_t=1,-size_t[=@1],&FILE)int>=0"`
		Seek       func(stream ffi.File, offset int, origin int) error `lib:"fseek func(&FILE,long,int)int=0"`
		Tell       func(stream ffi.File) int                           `lib:"ftell func(&FILE)long"`
		Rewind     func(stream ffi.File) error                         `lib:"rewind func(&FILE)int"`
		GetPos     func(stream ffi.File, ptr *ffi.FilePosition) error  `lib:"fgetpos func(&FILE,&fpos_t)int=0"`
		SetPos     func(stream ffi.File, ptr *ffi.FilePosition) error  `lib:"fsetpos func(&FILE,&fpos_t)int=0"`
		ClearError func(stream ffi.File)                               `lib:"clearerr func(&FILE)"`
		IsEOF      func(stream ffi.File) bool                          `lib:"feof func(&FILE)int"`
		Error      func(stream ffi.File) bool                          `lib:"ferror func(&FILE)int"`
	}
	Jump struct { // Jump provides the functions from <setjmp.h>.
		Set  func(env ffi.JumpBuffer) error            `std:"setjmp func(&jmp_buf)int"`
		Long func(env ffi.JumpBuffer, err error) error `std:"longjmp func(&jmp_buf,int)"`
	}
	ASCII struct { // ASCII provides the functions from <ctype.h>.
		IsAlphaNumeric func(c rune) rune `std:"isalnum func(int)int"` // IsAlpha || IsDigit
		IsAlpha        func(c rune) rune `std:"isalpha func(int)int"` // IsUpper || IsLower
		IsControl      func(c rune) rune `std:"iscntrl func(int)int"`
		IsDigit        func(c rune) rune `std:"isdigit func(int)int"`
		IsGraph        func(c rune) rune `std:"isgraph func(int)int"`
		IsLower        func(c rune) rune `std:"islower func(int)int"`
		IsPrintable    func(c rune) rune `std:"isprint func(int)int"`
		IsPuncuation   func(c rune) rune `std:"ispunct func(int)int"`
		IsSpace        func(c rune) rune `std:"isspace func(int)int"` // space, formfeed, newline, carriage return, tab, vertical tab
		IsUpper        func(c rune) rune `std:"isupper func(int)int"`
		IsHexDigit     func(c rune) rune `std:"isxdigit func(int)int"`

		ToLower func(c rune) rune `std:"tolower func(int)int"`
		ToUpper func(c rune) rune `std:"toupper func(int)int"`
	}
	Memory struct { // Memory provides memory-related functions from <stdlib.h>.
		AllocateZeros func(int) ffi.Buffer             `std:"calloc func(-size_t=1,size_t)$void[=@2]"`
		Allocate      func(int) ffi.Buffer             `std:"malloc func(size_t)$void[=@1] "`
		Reallocate    func(ffi.Buffer, int) ffi.Buffer `std:"realloc func(#void,size_t)$void[=@2]"`
		Free          func(ffi.Buffer)                 `std:"free func(#void)"`

		Sort   func(base any, cmp func(a, b any) int)                   `std:"qsort func(&void,-size_t[=@1],-size_t*@1,&func(&void:@1,&void:@1)int)"`
		Search func(key, base any, cmp func(keyval, datum any) int) any `std:"bsearch func(&void,&void,-size_t[=@1],size_t*@1,&func(&void:@1,&void:@2)int)$void:@2^@1"`

		Copy    func(dst, src []byte) []byte `std:"memcpy func(&void[>=@2]!|@2,&void,-size_t[=@2])$void[=@3]^@1"`
		Move    func(dst, src []byte) []byte `std:"memmove func(&void[>=@2],&void,-size_t[@2])$void[=@3]^@1"`
		Compare func(cs, ct []byte) int      `std:"memcmp func(&void[=@3],&void,-size_t[=@2])int"`
		Index   func([]byte, byte) int       `std:"memchr func(&void,int,-size_t[=@1]) $void^@1"`
		Set     func([]byte, byte) []byte    `std:"memset func(&void,int,-size_t[=@1]) $void^@1"`
	}
	System struct { // System provides system-related functions from <stdlib.h>.
		Command func(command string) int    `std:"system func(&char)int"`
		Clock   func() time.Duration        `std:"clock func()clock_t"`
		Time    func(t time.Time) time.Time `std:"time func(&time_t)time_t"`
	}
	Program struct { // Program provides program-related functions from <stdlib.h>.
		Abort  func()                   `std:"abort func()"`
		Exit   func(status int)         `std:"exit func(int)"`
		OnExit func(func())             `std:"atexit,__cxa_atexit func($func())"`
		Getenv func(name string) string `std:"getenv func(&char)~char"`
	}
	Signals struct { // Signals provides the functions from <signal.h>.
		Handle func(sig os.Signal, handler func(os.Signal)) `std:"signal func(int,$func(int))-func(int)"`
		Raise  func(sig os.Signal) error                    `std:"raise func(int)int"`
	}
	Strings struct { // Strings provides string-related functions from <string.h>, <stdio.h> and <stdlib.h>.
		Printf            func(s unsafe.Pointer, fmt string, args ...any) (int, error) `std:"sprintf func(&char,&char,&void...?@2)int>=0"`
		Scanf             func(s, fmt string, args ...any) (int, error)                `std:"sscanf func(&char,&char,&void...?@2)int>=0"`
		ToFloat64         func(s string) float64                                       `std:"atof func(&char)double"`
		ToInt32           func(s string) int32                                         `std:"atoi func(&char)int"`
		ToInt64           func(s string) int64                                         `std:"atol func(&char)int"`
		ParseFloat64      func(s string) (float64, int)                                `std:"strtod func(&char,+&char^@1)double"`
		ParseInt64        func(s string, base int) (int64, int)                        `std:"strtol func(&char,+&char^@1,int)long"`
		ParseUint64       func(s string, base int) (uint64, int)                       `std:"strtoul func(&char,+&char^@1,int)unsigned_long"`
		Copy              func([]byte, string) string                                  `std:"strcpy func(&char[>@2],&char)$char^@1"`
		CopyLimited       func([]byte, string) string                                  `std:"strncpy func(&char[>@2],&char,-size_t[=@2])$char^@1"`
		Cat               func([]byte, string) string                                  `std:"strcat func(&char[>@2],&char)$char^@1"`
		CatLimited        func([]byte, string) string                                  `std:"strncat func(&char[>@2],&char,-size_t[=@2])$char^@1"`
		Compare           func(cs, ct string) int                                      `std:"strcmp func(&char,&char)int"`
		CompareLimited    func(cs, ct string) int                                      `std:"strncmp func(&char,&char,int[=@2])int"`
		Index             func(cs string, c rune) int                                  `std:"strchr func(&char,int)$char^@1"`
		IndexLast         func(cs string, c rune) int                                  `std:"strrchr func(&char,int)$char^@1"`
		Span              func(cs, ct string) int                                      `std:"strspn func(&char,&char)size_t"`
		ComplimentarySpan func(cs, ct string) int                                      `std:"strcspn func(&char,&char)size_t"`
		PointerBreak      func(cs, ct string) int                                      `std:"strpbrk func(&char,&char)$char^@1"`
		Search            func(cs, ct string) int                                      `std:"strstr func(&char,&char)$char^@1"`
		Length            func(cs string) int                                          `std:"strlen func(&char)size_t"`
		Error             func(n error) string                                         `std:"strerror func(int)$char"`
		Tokens            func(s []byte, delim string) string                          `std:"strtok func(&char,&char)$char^@1"`
	}
	Division struct { // Division provides division-related functions from <stdlib.h>.
		Int32 func(num, denom int32) (int32, int32) `std:"div func(int,int)div_t"`
		Int64 func(num, denom int64) (int64, int64) `std:"ldiv func(long,long)ldiv_t"`
	}
	SeekModes struct {
	}
	BufferModes struct {
	}
}
