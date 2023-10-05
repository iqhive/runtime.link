// Package lib provides a representation of the C standard library.
package lib

import (
	"os"
	"time"
	"unsafe"

	link "runtime.link/api/link"
	"runtime.link/ref"
)

type location struct {
	linux   link.To `lib:"libc.so.6 libm.so.6"`
	darwin  link.To `lib:"libSystem.dylib"`
	windows link.To `lib:"msvcrt.dll"`
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
		Printf func(format string, args ...any) (int, error) `link:"printf func(&char,&void...?@1)int<0"`
		Scanf  func(format string, args ...any) (int, error) `link:"scanf func(&char,&void...?@1)int<0"`

		GetChar   func() rune          `link:"getchar func()int"`
		PutChar   func(c rune) error   `link:"putchar func(int)int<0"`
		PutString func(s string) error `link:"puts func(&char)int>0"`
		Error     func(s string)       `link:"perror func(&char)"`
	}
	Math struct { // Math provides numerical functions from <math.h> and <stdlib.h>.
		Abs      func(x int32) int32                 `link:"abs func(int)int"`
		Sin      func(x float64) float64             `link:"sin func(double)double"`
		Cos      func(x float64) float64             `link:"cos func(double)double"`
		Tan      func(x float64) float64             `link:"tan func(double)double"`
		Asin     func(x float64) float64             `link:"asin func(double)double"`
		Atan2    func(x, y float64) float64          `link:"atan2 func(double,double)double"`
		Sinh     func(x float64) float64             `link:"sinh func(double)double"`
		Cosh     func(x float64) float64             `link:"cosh func(double)double"`
		Tanh     func(x float64) float64             `link:"tanh func(double)double"`
		Exp      func(x float64) float64             `link:"exp func(double)double"`
		Log      func(x float64) float64             `link:"log func(double)double"`
		Log10    func(x float64) float64             `link:"log10 func(double)double"`
		Pow      func(x, y float64) float64          `link:"pow func(double,double)double"`
		Sqrt     func(x float64) float64             `link:"sqrt func(double)double"`
		Ceil     func(x float64) float64             `link:"ceil func(double)double"`
		Floor    func(x float64) float64             `link:"floor func(double)double"`
		Fabs     func(x float64) float64             `link:"fabs func(double)double"`
		Ldexp    func(x float64, n int32) float64    `link:"ldexp func(double,int)double"`
		Frexp    func(x float64, exp *int32) float64 `link:"frexp func(double,&int)double"`
		Modf     func(x float64, y *float64) float64 `link:"modf func(double,&double)double"`
		Rand     func() int32                        `link:"rand func()int"`
		SeedRand func(seed uint32) int32             `link:"srand func(unsigned_int)int"`
	}
	Time struct { // Time provides time-related functions from <time.h>.
		Sub    func(t1, t2 time.Time) time.Time `link:"difftime func(time_t,time_t)time_t"`
		String func(t time.Time) string         `link:"ctime func(time_t)$char"`
		UTC    func(t time.Time) time.Time      `link:"gmtime func(time_t)$tm"`
		Local  func(t time.Time) time.Time      `link:"localtime func(time_t)$tm"`
	}
	Date struct { // Date provides date-related functions from <time.h>.
		Time   func(t time.Time) time.Time `link:"mktime func(&tm)time_t"`
		String func(t time.Time) string    `link:"asctime func(&tm)$char"`

		Format func(s []byte, format string, tp time.Time) (int, error) `link:"strftime func(&char,-size_t[=@1],&char,&tm)size_t=0"`
	}
	File struct { // File provides file-related functions from <stdio.h>.
		Open   func(filename string, mode string) File              `link:"fopen func(&char,&char)$FILE"`
		Reopen func(filename string, mode string, stream File) File `link:"freopen func(&char,&char,$FILE)$FILE"`
		Flush  func(stream File) error                              `link:"fflush func(&FILE)int=0"`
		Close  func(stream File) error                              `link:"fclose func($FILE)int=0"`
		Remove func(filename string) error                          `link:"remove func(&char)int=0"`
		Rename func(oldname, newname string) error                  `link:"rename func(&char,&char)int=0"`

		Temp     func() File         `link:"tmpfile func()$FILE"`
		TempName func([]byte) string `link:"tmpnam func(&char[>=L_tmpnam_s])$char^@1"`

		SetBufferMode func(stream File, buf []byte, mode int) error `link:"setvbuf func(&FILE,&void,int,-size_t[=@2])int=0"`
		SetBuffer     func(stream File, buf []byte) error           `link:"setbuf func(&FILE,&void[>=BUFSIZE])int=0"`

		Printf func(stream File, format string, args ...any) (int, error) `link:"fprintf func(&FILE,&char,&void...?@2)int>=0 "`
		Scanf  func(stream File, format string, args ...any) (int, error) `link:"fscanf func(&FILE,&char,&void...?@2)int>=0"`

		GetChar    func(stream File) rune                          `link:"fgetc func(&FILE)int"`
		GetString  func(s []byte, stream File) string              `link:"fgets func(&char,-int[=@1],&FILE)$char^@1"`
		PutChar    func(c rune, stream File) error                 `link:"fputc func(int,&FILE)int=0"`
		Unget      func(c rune, stream File) error                 `link:"ungetc func(int,&FILE)int=0"`
		Read       func(ptr []byte, stream File) (int, error)      `link:"fread func(&void,-size_t=1,-size_t[=@1],&FILE)int>=0"`
		Write      func(ptr []byte, stream File) (int, error)      `link:"fwrite func(&void,-size_t=1,-size_t[=@1],&FILE)int>=0"`
		Seek       func(stream File, offset int, origin int) error `link:"fseek func(&FILE,long,int)int=0"`
		Tell       func(stream File) int                           `link:"ftell func(&FILE)long"`
		Rewind     func(stream File) error                         `link:"rewind func(&FILE)int"`
		GetPos     func(stream File, ptr *FilePosition) error      `link:"fgetpos func(&FILE,&fpos_t)int=0"`
		SetPos     func(stream File, ptr *FilePosition) error      `link:"fsetpos func(&FILE,&fpos_t)int=0"`
		ClearError func(stream File)                               `link:"clearerr func(&FILE)"`
		IsEOF      func(stream File) bool                          `link:"feof func(&FILE)int"`
		Error      func(stream File) bool                          `link:"ferror func(&FILE)int"`
	}
	Jump struct { // Jump provides the functions from <setjmp.h>.
		Set  func(env *JumpBuffer) error            `link:"setjmp func(&jmp_buf)int"`
		Long func(env *JumpBuffer, err error) error `link:"longjmp func(&jmp_buf,int)"`
	}
	ASCII struct { // ASCII provides the functions from <ctype.h>.
		IsAlphaNumeric func(c rune) bool `link:"isalnum func(int)int"` // IsAlpha || IsDigit
		IsAlpha        func(c rune) bool `link:"isalpha func(int)int"` // IsUpper || IsLower
		IsControl      func(c rune) bool `link:"iscntrl func(int)int"`
		IsDigit        func(c rune) bool `link:"isdigit func(int)int"`
		IsGraph        func(c rune) bool `link:"isgraph func(int)int"`
		IsLower        func(c rune) bool `link:"islower func(int)int"`
		IsPrintable    func(c rune) bool `link:"isprint func(int)int"`
		IsPuncuation   func(c rune) bool `link:"ispunct func(int)int"`
		IsSpace        func(c rune) bool `link:"isspace func(int)int"` // space, formfeed, newline, carriage return, tab, vertical tab
		IsUpper        func(c rune) bool `link:"isupper func(int)int"`
		IsHexDigit     func(c rune) bool `link:"isxdigit func(int)int"`

		ToLower func(c rune) rune `link:"tolower func(int)int"`
		ToUpper func(c rune) rune `link:"toupper func(int)int"`
	}
	Memory struct { // Memory provides memory-related functions from <stdlib.h>.
		AllocateZeros func(int) []byte         `link:"calloc func(-size_t=1,size_t)$void[=@2]"`
		Allocate      func(int) []byte         `link:"malloc func(size_t)$void[=@1] "`
		Reallocate    func([]byte, int) []byte `link:"realloc func($void,size_t)$void[=@2]"`
		Free          func([]byte)             `link:"free func($void)"`

		Sort   func(base any, cmp func(a, b any) int)                   `link:"qsort func(&void,-size_t[=@1],-size_t*@1,&func(&void:@1,&void:@1)int)"`
		Search func(key, base any, cmp func(keyval, datum any) int) any `link:"bsearch func(&void,&void,-size_t[=@1],size_t*@1,&func(&void:@1,&void:@2)int)$void:@2^@1"`

		Copy    func(dst, src []byte) []byte `link:"memcpy func(&void[>=@2]!|@2,&void,-size_t[=@2])$void[=@3]^@1"`
		Move    func(dst, src []byte) []byte `link:"memmove func(&void[>=@2],&void,-size_t[@2])$void[=@3]^@1"`
		Compare func(cs, ct []byte) int      `link:"memcmp func(&void[=@3],&void,-size_t[=@2])int"`
		Index   func([]byte, byte) int       `link:"memchr func(&void,int,-size_t[=@1]) $void^@1"`
		Set     func([]byte, byte) []byte    `link:"memset func(&void,int,-size_t[=@1]) $void^@1"`
	}
	System struct { // System provides system-related functions from <stdlib.h>.
		Command func(command string) int    `link:"system func(&char)int"`
		Clock   func() time.Duration        `link:"clock func()clock_t"`
		Time    func(t time.Time) time.Time `link:"time func(&time_t)time_t"`
	}
	Program struct { // Program provides program-related functions from <stdlib.h>.
		Abort  func()                   `link:"abort func()"`
		Exit   func(status int)         `link:"exit func(int)"`
		Getenv func(name string) string `link:"getenv func(&char)~char"`
	}
	Signals struct { // Signals provides the functions from <signal.h>.
		Handle func(sig os.Signal, handler func(os.Signal)) `link:"signal func(int,$func(int))-func(int)"`
		Raise  func(sig os.Signal) error                    `link:"raise func(int)int"`
	}
	Strings struct { // Strings provides string-related functions from <string.h>, <stdio.h> and <stdlib.h>.
		Printf            func(s unsafe.Pointer, fmt string, args ...any) (int, error) `link:"sprintf func(&char,&char,&void...?@2)int>=0"`
		Scanf             func(s, fmt string, args ...any) (int, error)                `link:"sscanf func(&char,&char,&void...?@2)int>=0"`
		ToFloat64         func(s string) float64                                       `link:"atof func(&char)double"`
		ToInt32           func(s string) int32                                         `link:"atoi func(&char)int"`
		ToInt64           func(s string) int64                                         `link:"atol func(&char)int"`
		ParseFloat64      func(s string) (float64, int)                                `link:"strtod func(&char,+&char^@1)double"`
		ParseInt64        func(s string, base int) (int64, int)                        `link:"strtol func(&char,+&char^@1,int)long"`
		ParseUint64       func(s string, base int) (uint64, int)                       `link:"strtoul func(&char,+&char^@1,int)unsigned_long"`
		Copy              func([]byte, string) string                                  `link:"strcpy func(&char[>@2],&char)$char^@1"`
		CopyLimited       func([]byte, string) string                                  `link:"strncpy func(&char[>@2],&char,-size_t[=@2])$char^@1"`
		Cat               func([]byte, string) string                                  `link:"strcat func(&char[>@2],&char)$char^@1"`
		CatLimited        func([]byte, string) string                                  `link:"strncat func(&char[>@2],&char,-size_t[=@2])$char^@1"`
		Compare           func(cs, ct string) int                                      `link:"strcmp func(&char,&char)int"`
		CompareLimited    func(cs, ct string) int                                      `link:"strncmp func(&char,&char,int[=@2])int"`
		Index             func(cs string, c rune) int                                  `link:"strchr func(&char,int)$char^@1"`
		IndexLast         func(cs string, c rune) int                                  `link:"strrchr func(&char,int)$char^@1"`
		Span              func(cs, ct string) int                                      `link:"strspn func(&char,&char)size_t"`
		ComplimentarySpan func(cs, ct string) int                                      `link:"strcspn func(&char,&char)size_t"`
		PointerBreak      func(cs, ct string) int                                      `link:"strpbrk func(&char,&char)$char^@1"`
		Search            func(cs, ct string) int                                      `link:"strstr func(&char,&char)$char^@1"`
		Length            func(cs string) int                                          `link:"strlen func(&char)size_t"`
		Error             func(n error) string                                         `link:"strerror func(int)$char"`
		Tokens            func(s []byte, delim string) string                          `link:"strtok func(&char,&char)$char^@1"`
	}
	Division struct { // Division provides division-related functions from <stdlib.h>.
		Int32 func(num, denom int32) (int32, int32) `link:"div func(int,int)div_t"`
		Int64 func(num, denom int64) (int64, int64) `link:"ldiv func(long,long)ldiv_t"`
	}
}

type JumpBuffer []byte

type File ref.For[C, File, unsafe.Pointer]

type FilePosition []byte
