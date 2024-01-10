// Package lib provides a representation of the C standard library.
package lib

import (
	"os"
	"time"
	"unsafe"

	"runtime.link/api/call"
	"runtime.link/ref"
)

type location struct {
	linux   call.To `lib:"libc.so.6 libm.so.6"`
	darwin  call.To `lib:"libSystem.dylib"`
	windows call.To `lib:"msvcrt.dll"`
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

	IO struct { // IO provides stdin/stdout functions from <stdio.h>.
		Printf func(format string, args ...any) (int, error) `call:"printf func(&char,&void...?@1)int<0"`
		Scanf  func(format string, args ...any) (int, error) `call:"scanf func(&char,&void...?@1)int<0"`

		GetChar   func() rune          `call:"getchar func()int"`
		PutChar   func(c rune) error   `call:"putchar func(int)int<0"`
		PutString func(s string) error `call:"puts func(&char)int>0"`
		Error     func(s string)       `call:"perror func(&char)"`
	}
	Math struct { // Math provides numerical functions from <math.h> and <stdlib.h>.
		Abs      func(x int32) int32                 `call:"abs func(int)int"`
		Sin      func(x float64) float64             `call:"sin func(double)double"`
		Cos      func(x float64) float64             `call:"cos func(double)double"`
		Tan      func(x float64) float64             `call:"tan func(double)double"`
		Asin     func(x float64) float64             `call:"asin func(double)double"`
		Atan2    func(x, y float64) float64          `call:"atan2 func(double,double)double"`
		Sinh     func(x float64) float64             `call:"sinh func(double)double"`
		Cosh     func(x float64) float64             `call:"cosh func(double)double"`
		Tanh     func(x float64) float64             `call:"tanh func(double)double"`
		Exp      func(x float64) float64             `call:"exp func(double)double"`
		Log      func(x float64) float64             `call:"log func(double)double"`
		Log10    func(x float64) float64             `call:"log10 func(double)double"`
		Pow      func(x, y float64) float64          `call:"pow func(double,double)double"`
		Sqrt     func(x float64) float64             `call:"sqrt func(double)double"`
		Ceil     func(x float64) float64             `call:"ceil func(double)double"`
		Floor    func(x float64) float64             `call:"floor func(double)double"`
		Fabs     func(x float64) float64             `call:"fabs func(double)double"`
		Ldexp    func(x float64, n int32) float64    `call:"ldexp func(double,int)double"`
		Frexp    func(x float64, exp *int32) float64 `call:"frexp func(double,&int)double"`
		Modf     func(x float64, y *float64) float64 `call:"modf func(double,&double)double"`
		Rand     func() int32                        `call:"rand func()int"`
		SeedRand func(seed uint32) int32             `call:"srand func(unsigned_int)int"`
	}
	Time struct { // Time provides time-related functions from <time.h>.
		Sub    func(t1, t2 time.Time) time.Time `call:"difftime func(time_t,time_t)time_t"`
		String func(t time.Time) string         `call:"ctime func(time_t)$char"`
		UTC    func(t time.Time) time.Time      `call:"gmtime func(time_t)$tm"`
		Local  func(t time.Time) time.Time      `call:"localtime func(time_t)$tm"`
	}
	Date struct { // Date provides date-related functions from <time.h>.
		Time   func(t time.Time) time.Time `call:"mktime func(&tm)time_t"`
		String func(t time.Time) string    `call:"asctime func(&tm)$char"`

		Format func(s []byte, format string, tp time.Time) (int, error) `call:"strftime func(&char,-size_t[=@1],&char,&tm)size_t=0"`
	}
	File struct { // File provides file-related functions from <stdio.h>.
		Open   func(filename string, mode string) File              `call:"fopen func(&char,&char)$FILE"`
		Reopen func(filename string, mode string, stream File) File `call:"freopen func(&char,&char,$FILE)$FILE"`
		Flush  func(stream File) error                              `call:"fflush func(&FILE)int=0"`
		Close  func(stream File) error                              `call:"fclose func($FILE)int=0"`
		Remove func(filename string) error                          `call:"remove func(&char)int=0"`
		Rename func(oldname, newname string) error                  `call:"rename func(&char,&char)int=0"`

		Temp     func() File         `call:"tmpfile func()$FILE"`
		TempName func([]byte) string `call:"tmpnam func(&char[>=L_tmpnam_s])$char^@1"`

		SetBufferMode func(stream File, buf []byte, mode int) error `call:"setvbuf func(&FILE,&void,int,-size_t[=@2])int=0"`
		SetBuffer     func(stream File, buf []byte) error           `call:"setbuf func(&FILE,&void[>=BUFSIZE])int=0"`

		Printf func(stream File, format string, args ...any) (int, error) `call:"fprintf func(&FILE,&char,&void...?@2)int>=0 "`
		Scanf  func(stream File, format string, args ...any) (int, error) `call:"fscanf func(&FILE,&char,&void...?@2)int>=0"`

		GetChar    func(stream File) rune                          `call:"fgetc func(&FILE)int"`
		GetString  func(s []byte, stream File) string              `call:"fgets func(&char,-int[=@1],&FILE)$char^@1"`
		PutChar    func(c rune, stream File) error                 `call:"fputc func(int,&FILE)int=0"`
		Unget      func(c rune, stream File) error                 `call:"ungetc func(int,&FILE)int=0"`
		Read       func(ptr []byte, stream File) (int, error)      `call:"fread func(&void,-size_t=1,-size_t[=@1],&FILE)int>=0"`
		Write      func(ptr []byte, stream File) (int, error)      `call:"fwrite func(&void,-size_t=1,-size_t[=@1],&FILE)int>=0"`
		Seek       func(stream File, offset int, origin int) error `call:"fseek func(&FILE,long,int)int=0"`
		Tell       func(stream File) int                           `call:"ftell func(&FILE)long"`
		Rewind     func(stream File) error                         `call:"rewind func(&FILE)int"`
		GetPos     func(stream File, ptr *FilePosition) error      `call:"fgetpos func(&FILE,&fpos_t)int=0"`
		SetPos     func(stream File, ptr *FilePosition) error      `call:"fsetpos func(&FILE,&fpos_t)int=0"`
		ClearError func(stream File)                               `call:"clearerr func(&FILE)"`
		IsEOF      func(stream File) bool                          `call:"feof func(&FILE)int"`
		Error      func(stream File) bool                          `call:"ferror func(&FILE)int"`
	}
	Jump struct { // Jump provides the functions from <setjmp.h>.
		Set  func(env *JumpBuffer) error            `call:"setjmp func(&jmp_buf)int"`
		Long func(env *JumpBuffer, err error) error `call:"longjmp func(&jmp_buf,int)"`
	}
	ASCII struct { // ASCII provides the functions from <ctype.h>.
		IsAlphaNumeric func(c rune) bool `call:"isalnum func(int)int"` // IsAlpha || IsDigit
		IsAlpha        func(c rune) bool `call:"isalpha func(int)int"` // IsUpper || IsLower
		IsControl      func(c rune) bool `call:"iscntrl func(int)int"`
		IsDigit        func(c rune) bool `call:"isdigit func(int)int"`
		IsGraph        func(c rune) bool `call:"isgraph func(int)int"`
		IsLower        func(c rune) bool `call:"islower func(int)int"`
		IsPrintable    func(c rune) bool `call:"isprint func(int)int"`
		IsPuncuation   func(c rune) bool `call:"ispunct func(int)int"`
		IsSpace        func(c rune) bool `call:"isspace func(int)int"` // space, formfeed, newline, carriage return, tab, vertical tab
		IsUpper        func(c rune) bool `call:"isupper func(int)int"`
		IsHexDigit     func(c rune) bool `call:"isxdigit func(int)int"`

		ToLower func(c rune) rune `call:"tolower func(int)int"`
		ToUpper func(c rune) rune `call:"toupper func(int)int"`
	}
	Memory struct { // Memory provides memory-related functions from <stdlib.h>.
		AllocateZeros func(int) []byte         `call:"calloc func(-size_t=1,size_t)$void[=@2]"`
		Allocate      func(int) []byte         `call:"malloc func(size_t)$void[=@1] "`
		Reallocate    func([]byte, int) []byte `call:"realloc func($void,size_t)$void[=@2]"`
		Free          func([]byte)             `call:"free func($void)"`

		Sort   func(base any, cmp func(a, b any) int)                   `call:"qsort func(&void,-size_t[=@1],-size_t*@1,&func(&void:@1,&void:@1)int)"`
		Search func(key, base any, cmp func(keyval, datum any) int) any `call:"bsearch func(&void,&void,-size_t[=@1],size_t*@1,&func(&void:@1,&void:@2)int)$void:@2^@1"`

		Copy    func(dst, src []byte) []byte `call:"memcpy func(&void[>=@2]!|@2,&void,-size_t[=@2])$void[=@3]^@1"`
		Move    func(dst, src []byte) []byte `call:"memmove func(&void[>=@2],&void,-size_t[@2])$void[=@3]^@1"`
		Compare func(cs, ct []byte) int      `call:"memcmp func(&void[=@3],&void,-size_t[=@2])int"`
		Index   func([]byte, byte) int       `call:"memchr func(&void,int,-size_t[=@1]) $void^@1"`
		Set     func([]byte, byte) []byte    `call:"memset func(&void,int,-size_t[=@1]) $void^@1"`
	}
	System struct { // System provides system-related functions from <stdlib.h>.
		Command func(command string) int    `call:"system func(&char)int"`
		Clock   func() time.Duration        `call:"clock func()clock_t"`
		Time    func(t time.Time) time.Time `call:"time func(&time_t)time_t"`
	}
	Program struct { // Program provides program-related functions from <stdlib.h>.
		Abort  func()                   `call:"abort func()"`
		Exit   func(status int)         `call:"exit func(int)"`
		Getenv func(name string) string `call:"getenv func(&char)~char"`
	}
	Signals struct { // Signals provides the functions from <signal.h>.
		Handle func(sig os.Signal, handler func(os.Signal)) `call:"signal func(int,$func(int))-func(int)"`
		Raise  func(sig os.Signal) error                    `call:"raise func(int)int"`
	}
	Strings struct { // Strings provides string-related functions from <string.h>, <stdio.h> and <stdlib.h>.
		Printf            func(s unsafe.Pointer, fmt string, args ...any) (int, error) `call:"sprintf func(&char,&char,&void...?@2)int>=0"`
		Scanf             func(s, fmt string, args ...any) (int, error)                `call:"sscanf func(&char,&char,&void...?@2)int>=0"`
		ToFloat64         func(s string) float64                                       `call:"atof func(&char)double"`
		ToInt32           func(s string) int32                                         `call:"atoi func(&char)int"`
		ToInt64           func(s string) int64                                         `call:"atol func(&char)int"`
		ParseFloat64      func(s string) (float64, int)                                `call:"strtod func(&char,+&char^@1)double"`
		ParseInt64        func(s string, base int) (int64, int)                        `call:"strtol func(&char,+&char^@1,int)long"`
		ParseUint64       func(s string, base int) (uint64, int)                       `call:"strtoul func(&char,+&char^@1,int)unsigned_long"`
		Copy              func([]byte, string) string                                  `call:"strcpy func(&char[>@2],&char)$char^@1"`
		CopyLimited       func([]byte, string) string                                  `call:"strncpy func(&char[>@2],&char,-size_t[=@2])$char^@1"`
		Cat               func([]byte, string) string                                  `call:"strcat func(&char[>@2],&char)$char^@1"`
		CatLimited        func([]byte, string) string                                  `call:"strncat func(&char[>@2],&char,-size_t[=@2])$char^@1"`
		Compare           func(cs, ct string) int                                      `call:"strcmp func(&char,&char)int"`
		CompareLimited    func(cs, ct string) int                                      `call:"strncmp func(&char,&char,int[=@2])int"`
		Index             func(cs string, c rune) int                                  `call:"strchr func(&char,int)$char^@1"`
		IndexLast         func(cs string, c rune) int                                  `call:"strrchr func(&char,int)$char^@1"`
		Span              func(cs, ct string) int                                      `call:"strspn func(&char,&char)size_t"`
		ComplimentarySpan func(cs, ct string) int                                      `call:"strcspn func(&char,&char)size_t"`
		PointerBreak      func(cs, ct string) int                                      `call:"strpbrk func(&char,&char)$char^@1"`
		Search            func(cs, ct string) int                                      `call:"strstr func(&char,&char)$char^@1"`
		Length            func(cs string) int                                          `call:"strlen func(&char)size_t"`
		Error             func(n error) string                                         `call:"strerror func(int)$char"`
		Tokens            func(s []byte, delim string) string                          `call:"strtok func(&char,&char)$char^@1"`
	}
	Division struct { // Division provides division-related functions from <stdlib.h>.
		Int32 func(num, denom int32) (int32, int32) `call:"div func(int,int)div_t"`
		Int64 func(num, denom int64) (int64, int64) `call:"ldiv func(long,long)ldiv_t"`
	}
}

type JumpBuffer []byte

type File ref.For[C, File, unsafe.Pointer]

type FilePosition []byte
