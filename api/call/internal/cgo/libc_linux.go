//go:build cgo && linux && !android

package cgo

/*
#include <limits.h>

// glibc names itself in every one of its headers. musl has no macro of its
// own, by design, and uClibc claims to be glibc as well as itself.
#if defined(__GLIBC__) && !defined(__UCLIBC__)
enum { tables_are_of_this_library = 1 };
#else
enum { tables_are_of_this_library = 0 };
#endif
*/
import "C"

// OtherLibrary reports whether the program is built with a C library that the
// tables of the platform were not written from. Those of linux are glibc's,
// and another library has types and constants of its own: an int_fast16_t is
// 8 bytes wide in glibc and 4 in musl, and BUFSIZ is 8192 in one and 1024 in
// the other. Its tables are not known, and glibc's would be the wrong ones.
const OtherLibrary = C.tables_are_of_this_library == 0
