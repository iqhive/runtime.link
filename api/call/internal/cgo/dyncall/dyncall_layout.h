/*

 Package: dyncall
 File: dyncall/dyncall_layout.h
 Description: Where the arguments of a call are put on arm64, for the tests.

*/

#ifndef DYNCALL_LAYOUT_H
#define DYNCALL_LAYOUT_H

#include <stddef.h>
#include "dyncall_go.h"

/* What the call kernel for arm64 is handed to load before it calls a function. */
typedef struct {
	unsigned long long x[8]; /* the general registers. */
	double d[8];             /* the floating point registers. */
	unsigned char stack[512];
	size_t size; /* of the stack, which a CallVM rounds up to a multiple of 16. */
} GoFrame;

/*
 Lays the arguments out as Apple's platforms do, or else as every other platform
 does, and records the result in frame. Nothing is called.
*/
void goLayoutARM64(int apple, GoArg *args, int argc, int fixed, GoFrame *frame);

/*
 Reads argc arguments of the types in sig out of frame, as a function that C
 calls back on Apple's platforms reads them, into out: an integer widened as
 its type is, and the bits of a float or a double. It returns 0 for a type that
 it doesn't read, or if the arguments end after the stack does.
*/
int goAppleReadARM64(const GoFrame *frame, const DCsigchar *sig, int argc, unsigned long long *out);

#endif /* DYNCALL_LAYOUT_H */
