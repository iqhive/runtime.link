package lib

import (
	"os"
	"time"
	"unsafe"

	link "runtime.link/api/link"
)

type location struct {
	linux   link.Library `link:"libc.so.6 libm.so.6"`
	darwin  link.Library `link:"libSystem.dylib"`
	windows link.Library `link:"msvcrt.dll"`
}

// Library provides provides the ANSI C standard library.
// https://www.csse.uwa.edu.au/programming/ansic-library.html
// Function names have been expanded to prefer full words over
// abbreviations. The functions have been organised into sensible
// categories. All functions except for those in the "Jump" field
// are safe to use from Go. 'gets' has been removed as it is always
// unsafe.
type C struct {
	location

	IO struct { // IO provides qnqin/qnqout functions from <stdio.h>.
		Printf func(format string, args ...any) (int, error) `ffi:"printf func(&char,&void...?@1)int<0"`
		Scanf  func(format string, args ...any) (int, error) `ffi:"scanf func(&char,&void...?@1)int<0"`

		GetChar   func() rune          `ffi:"getchar func()int"`
		PutChar   func(c rune) error   `ffi:"putchar func(int)int<0"`
		PutString func(s string) error `ffi:"puts func(&char)int>0"`
		Error     func(s string)       `ffi:"perror func(&char)"`
	}
	Math struct { // Math provides numerical functions from <math.h> and <stdlib.h>.
		Abs      func(x int32) int32                 `ffi:"abs func(int)int"`
		Sin      func(x float64) float64             `ffi:"sin func(double)double"`
		Cos      func(x float64) float64             `ffi:"cos func(double)double"`
		Tan      func(x float64) float64             `ffi:"tan func(double)double"`
		Asin     func(x float64) float64             `ffi:"asin func(double)double"`
		Atan2    func(x, y float64) float64          `ffi:"atan2 func(double,double)double"`
		Sinh     func(x float64) float64             `ffi:"sinh func(double)double"`
		Cosh     func(x float64) float64             `ffi:"cosh func(double)double"`
		Tanh     func(x float64) float64             `ffi:"tanh func(double)double"`
		Exp      func(x float64) float64             `ffi:"exp func(double)double"`
		Log      func(x float64) float64             `ffi:"log func(double)double"`
		Log10    func(x float64) float64             `ffi:"log10 func(double)double"`
		Pow      func(x, y float64) float64          `ffi:"pow func(double,double)double"`
		Sqrt     func(x float64) float64             `ffi:"sqrt func(double)double"`
		Ceil     func(x float64) float64             `ffi:"ceil func(double)double"`
		Floor    func(x float64) float64             `ffi:"floor func(double)double"`
		Fabs     func(x float64) float64             `ffi:"fabs func(double)double"`
		Ldexp    func(x float64, n int32) float64    `ffi:"ldexp func(double,int)double"`
		Frexp    func(x float64, exp *int32) float64 `ffi:"frexp func(double,&int)double"`
		Modf     func(x float64, y *float64) float64 `ffi:"modf func(double,&double)double"`
		Rand     func() int32                        `ffi:"rand func()int"`
		SeedRand func(seed uint32) int32             `ffi:"srand func(unsigned_int)int"`
	}
	Time struct { // Time provides time-related functions from <time.h>.
		Sub    func(t1, t2 time.Time) time.Time `ffi:"difftime func(time_t,time_t)time_t"`
		String func(t time.Time) string         `ffi:"ctime func(time_t)$char"`
		UTC    func(t time.Time) time.Time      `ffi:"gmtime func(time_t)$tm"`
		Local  func(t time.Time) time.Time      `ffi:"localtime func(time_t)$tm"`
	}
	Date struct { // Date provides date-related functions from <time.h>.
		Time   func(t time.Time) time.Time `ffi:"mktime func(&tm)time_t"`
		String func(t time.Time) string    `ffi:"asctime func(&tm)$char"`

		Format func(s []byte, format string, tp time.Time) (int, error) `ffi:"strftime func(&char,-size_t[=@1],&char,&tm)size_t=0"`
	}
	File struct { // File provides file-related functions from <stdio.h>.
		Open   func(filename string, mode string) File              `ffi:"fopen func(&char,&char)$FILE"`
		Reopen func(filename string, mode string, stream File) File `ffi:"freopen func(&char,&char,$FILE)$FILE"`
		Flush  func(stream File) error                              `ffi:"fflush func(&FILE)int=0"`
		Close  func(stream File) error                              `ffi:"fclose func($FILE)int=0"`
		Remove func(filename string) error                          `ffi:"remove func(&char)int=0"`
		Rename func(oldname, newname string) error                  `ffi:"rename func(&char,&char)int=0"`

		Temp     func() File         `ffi:"tmpfile func()$FILE"`
		TempName func([]byte) string `ffi:"tmpnam func(&char[>=L_tmpnam_s])$char^@1"`

		SetBufferMode func(stream File, buf []byte, mode int) error `ffi:"setvbuf func(&FILE,&void,int,-size_t[=@2])int=0"`
		SetBuffer     func(stream File, buf []byte) error           `ffi:"setbuf func(&FILE,&void[>=BUFSIZE])int=0"`

		Printf func(stream File, format string, args ...any) (int, error) `ffi:"fprintf func(&FILE,&char,&void...?@2)int>=0 "`
		Scanf  func(stream File, format string, args ...any) (int, error) `ffi:"fscanf func(&FILE,&char,&void...?@2)int>=0"`

		GetChar    func(stream File) rune                          `ffi:"fgetc func(&FILE)int"`
		GetString  func(s []byte, stream File) string              `ffi:"fgets func(&char,-int[=@1],&FILE)$char^@1"`
		PutChar    func(c rune, stream File) rune                  `ffi:"fputc func(int,&FILE)int=0"`
		Unget      func(c rune, stream File) rune                  `ffi:"ungetc func(int,&FILE)int=0"`
		Read       func(ptr []byte, stream File) int               `ffi:"fread func(&void,-size_t=1,-size_t[=@1],&FILE)int>=0"`
		Write      func(ptr []byte, stream File) int               `ffi:"fwrite func(&void,-size_t=1,-size_t[=@1],&FILE)int>=0"`
		Seek       func(stream File, offset int, origin int) error `ffi:"fseek func(&FILE,long,int)int=0"`
		Tell       func(stream File) int                           `ffi:"ftell func(&FILE)long"`
		Rewind     func(stream File) error                         `ffi:"rewind func(&FILE)int"`
		GetPos     func(stream File, ptr FilePosition) error       `ffi:"fgetpos func(&FILE,&fpos_t)int=0"`
		SetPos     func(stream File, ptr FilePosition) error       `ffi:"fsetpos func(&FILE,&fpos_t)int=0"`
		ClearError func(stream File)                               `ffi:"clearerr func(&FILE)"`
		IsEOF      func(stream File) bool                          `ffi:"feof func(&FILE)int"`
		Error      func(stream File) bool                          `ffi:"ferror func(&FILE)int"`
	}
	Jump struct { // Jump provides the functions from <setjmp.h>.
		Set  func(env JumpBuffer) error            `ffi:"setjmp func(&jmp_buf)int"`
		Long func(env JumpBuffer, err error) error `ffi:"longjmp func(&jmp_buf,int)"`
	}
	ASCII struct { // ASCII provides the functions from <ctype.h>.
		IsAlphaNumeric func(c rune) bool `ffi:"isalnum func(int)int"` // IsAlpha || IsDigit
		IsAlpha        func(c rune) bool `ffi:"isalpha func(int)int"` // IsUpper || IsLower
		IsControl      func(c rune) bool `ffi:"iscntrl func(int)int"`
		IsDigit        func(c rune) bool `ffi:"isdigit func(int)int"`
		IsGraph        func(c rune) bool `ffi:"isgraph func(int)int"`
		IsLower        func(c rune) bool `ffi:"islower func(int)int"`
		IsPrintable    func(c rune) bool `ffi:"isprint func(int)int"`
		IsPuncuation   func(c rune) bool `ffi:"ispunct func(int)int"`
		IsSpace        func(c rune) bool `ffi:"isspace func(int)int"` // space, formfeed, newline, carriage return, tab, vertical tab
		IsUpper        func(c rune) bool `ffi:"isupper func(int)int"`
		IsHexDigit     func(c rune) bool `ffi:"isxdigit func(int)int"`

		ToLower func(c rune) rune `ffi:"tolower func(int)int"`
		ToUpper func(c rune) rune `ffi:"toupper func(int)int"`
	}
	Memory struct { // Memory provides memory-related functions from <stdlib.h>.
		AllocateZeros func(int) []byte         `ffi:"calloc func(-size_t=1,size_t)$void[=@2]"`
		Allocate      func(int) []byte         `ffi:"malloc func(size_t)$void[=@1] "`
		Reallocate    func([]byte, int) []byte `ffi:"realloc func($void,size_t)$void[=@2]"`
		Free          func([]byte)             `ffi:"free func($void)"`

		Sort   func(base any, cmp func(a, b any) int)                   `ffi:"qsort func(&void,-size_t[=@1],-size_t*@1,&func(&void:@1,&void:@1)int)"`
		Search func(key, base any, cmp func(keyval, datum any) int) any `ffi:"bsearch func(&void,&void,-size_t[=@1],size_t*@1,&func(&void:@1,&void:@2)int)$void:@2^@1"`

		Copy    func(dst, src []byte) []byte `ffi:"memcpy func(&void[>=@2]!|@2,&void,-size_t[=@2])$void[=@3]^@1"`
		Move    func(dst, src []byte) []byte `ffi:"memmove func(&void[>=@2],&void,-size_t[@2])$void[=@3]^@1"`
		Compare func(cs, ct []byte) int      `ffi:"memcmp func(&void[=@3],&void,-size_t[=@2])int"`
		Index   func([]byte, byte) int       `ffi:"memchr func(&void,int,-size_t[=@1]) $void^@1"`
		Set     func([]byte, byte) []byte    `ffi:"memset func(&void,int,-size_t[=@1]) $void^@1"`
	}
	System struct { // System provides system-related functions from <stdlib.h>.
		Command func(command string) int    `ffi:"system func(&char)int"`
		Clock   func() time.Duration        `ffi:"clock func()clock_t"`
		Time    func(t time.Time) time.Time `ffi:"time func(&time_t)time_t"`
	}
	Program struct { // Program provides program-related functions from <stdlib.h>.
		Abort  func()                   `ffi:"abort func()"`
		Exit   func(status int)         `ffi:"exit func(int)"`
		Getenv func(name string) string `ffi:"getenv func(&char)~char"`
	}
	Signals struct { // Signals provides the functions from <signal.h>.
		Handle func(sig os.Signal, handler func(os.Signal)) `ffi:"signal func(int,$func(int))-func(int)"`
		Raise  func(sig os.Signal) error                    `ffi:"raise func(int)int"`
	}
	Strings struct { // Strings provides string-related functions from <string.h>, <stdio.h> and <stdlib.h>.
		Printf            func(s unsafe.Pointer, fmt string, args ...any) (int, error) `ffi:"sprintf func(&char,&char,&void...?@2)int>=0"`
		Scanf             func(s, fmt string, args ...any) (int, error)                `ffi:"sscanf func(&char,&char,&void...?@2)int>=0"`
		ToFloat64         func(s string) float64                                       `ffi:"atof func(&char)double"`
		ToInt32           func(s string) int32                                         `ffi:"atoi func(&char)int"`
		ToInt64           func(s string) int64                                         `ffi:"atol func(&char)int"`
		ParseFloat64      func(s string) (float64, int)                                `ffi:"strtod func(&char,+&char^@1)double"`
		ParseInt64        func(s string, base int) (int64, int)                        `ffi:"strtol func(&char,+&char^@1,int)long"`
		ParseUint64       func(s string, base int) (uint64, int)                       `ffi:"strtoul func(&char,+&char^@1,int)unsigned_long"`
		Copy              func([]byte, string) string                                  `ffi:"strcpy func(&char[>@2],&char)$char^@1"`
		CopyLimited       func([]byte, string) string                                  `ffi:"strncpy func(&char[>@2],&char,-size_t[=@2])$char^@1"`
		Cat               func([]byte, string) string                                  `ffi:"strcat func(&char[>@2],&char)$char^@1"`
		CatLimited        func([]byte, string) string                                  `ffi:"strncat func(&char[>@2],&char,-size_t[=@2])$char^@1"`
		Compare           func(cs, ct string) int                                      `ffi:"strcmp func(&char,&char)int"`
		CompareLimited    func(cs, ct string) int                                      `ffi:"strncmp func(&char,&char,int[=@2])int"`
		Index             func(cs string, c rune) int                                  `ffi:"strchr func(&char,int)$char^@1"`
		IndexLast         func(cs string, c rune) int                                  `ffi:"strrchr func(&char,int)$char^@1"`
		Span              func(cs, ct string) int                                      `ffi:"strspn func(&char,&char)size_t"`
		ComplimentarySpan func(cs, ct string) int                                      `ffi:"strcspn func(&char,&char)size_t"`
		PointerBreak      func(cs, ct string) int                                      `ffi:"strpbrk func(&char,&char)$char^@1"`
		Search            func(cs, ct string) int                                      `ffi:"strstr func(&char,&char)$char^@1"`
		Length            func(cs string) int                                          `ffi:"strlen func(&char)size_t"`
		Error             func(n error) string                                         `ffi:"strerror func(int)$char"`
		Tokens            func(s []byte, delim string) string                          `ffi:"strtok func(&char,&char)$char^@1"`
	}
	Division struct { // Division provides division-related functions from <stdlib.h>.
		Int32 func(num, denom int32) (int32, int32) `ffi:"div func(int,int)div_t"`
		Int64 func(num, denom int64) (int64, int64) `ffi:"ldiv func(long,long)ldiv_t"`
	}
}

type JumpBuffer struct{}

type File struct{}

type FilePosition struct{}
