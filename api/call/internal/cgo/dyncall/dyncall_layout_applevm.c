/*

 Package: dyncall
 File: dyncall/dyncall_layout_applevm.c
 Description: The CallVM for Apple's arm64 as it is built for a Mac, apart from
              the names that a Mac's own copy of it has taken, and its call
              kernel, which records instead of calling: see dyncall_layout.c.

*/

/* its headers come first, so that the new names are only given to what it defines. */
#include "dyncall_callvm_arm64.h"
#include "dyncall_alloc.h"

#define dcNewCallVM goAppleNewCallVM
#define vt_arm64 goAppleVT
#define vt_arm64_variadic goAppleVariadicVT
#define call goAppleCall
#define dcCall_arm64 goLayoutCapture

#include "dyncall_callvm_darwin_arm64.c"
