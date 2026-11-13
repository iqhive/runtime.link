/*

 Package: dyncall
 File: dyncall/dyncall_layout_aapcsvm.c
 Description: The CallVM for arm64 as it is built everywhere but for Apple's
              platforms, apart from the names that their own copy of it has
              taken, and its call kernel, which records instead of calling:
              see dyncall_layout.c.

*/

/* its headers come first, so that the new names are only given to what it defines. */
#include "dyncall_callvm_arm64.h"
#include "dyncall_alloc.h"

#define dcNewCallVM goAAPCSNewCallVM
#define vt_arm64 goAAPCSVT
#define dcCall_arm64 goLayoutCapture

#include "dyncall_callvm_arm64.c"
