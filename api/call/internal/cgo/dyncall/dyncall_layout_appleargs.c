/*

 Package: dyncall
 File: dyncall/dyncall_layout_appleargs.c
 Description: How a callback on Apple's arm64 reads its arguments, as it is
              built for a Mac, apart from the names that the platform's own
              copy of it has taken: see dyncall_layout.c.

 The callback thunk for arm64 saves x0 to x7 and d0 to d7 and a pointer to the
 arguments on the stack, and the functions here take the arguments out of that
 one at a time. The thunk is assembly that only arm64 runs, so it is left out:
 the registers and the stack are taken from a GoFrame instead.

*/

#include <string.h>
#include "dyncall_layout.h"

/* its headers come first, so that the new names are only given to what it defines. */
#include "dyncall_args.h"

#ifndef DC__OS_Darwin
#define DC__OS_Darwin
#endif

#define dcbArgBool goAppleArgBool
#define dcbArgChar goAppleArgChar
#define dcbArgShort goAppleArgShort
#define dcbArgInt goAppleArgInt
#define dcbArgLong goAppleArgLong
#define dcbArgLongLong goAppleArgLongLong
#define dcbArgUChar goAppleArgUChar
#define dcbArgUShort goAppleArgUShort
#define dcbArgUInt goAppleArgUInt
#define dcbArgULong goAppleArgULong
#define dcbArgULongLong goAppleArgULongLong
#define dcbArgFloat goAppleArgFloat
#define dcbArgDouble goAppleArgDouble
#define dcbArgPointer goAppleArgPointer
#define dcbArgAggr goAppleArgAggr
#define dcbReturnAggr goAppleReturnAggr

#include "dyncall_args_arm64.c"

int goAppleReadARM64(const GoFrame *frame, const DCsigchar *sig, int argc, unsigned long long *out) {
	DCArgs args;
	memset(&args, 0, sizeof args);
	memcpy(args.I, frame->x, sizeof args.I);
	memcpy(args.F, frame->d, sizeof args.F);
	args.sp = (uint8_t *)frame->stack;
	for (int i = 0; i < argc; i++) {
		switch (sig[i]) {
		case DC_SIGCHAR_BOOL:
			out[i] = (unsigned long long)(long long)dcbArgBool(&args);
			break;
		case DC_SIGCHAR_CHAR:
			out[i] = (unsigned long long)(long long)dcbArgChar(&args);
			break;
		case DC_SIGCHAR_UCHAR:
			out[i] = dcbArgUChar(&args);
			break;
		case DC_SIGCHAR_SHORT:
			out[i] = (unsigned long long)(long long)dcbArgShort(&args);
			break;
		case DC_SIGCHAR_USHORT:
			out[i] = dcbArgUShort(&args);
			break;
		case DC_SIGCHAR_INT:
			out[i] = (unsigned long long)(long long)dcbArgInt(&args);
			break;
		case DC_SIGCHAR_UINT:
			out[i] = dcbArgUInt(&args);
			break;
		case DC_SIGCHAR_LONGLONG:
			out[i] = (unsigned long long)dcbArgLongLong(&args);
			break;
		case DC_SIGCHAR_ULONGLONG:
			out[i] = dcbArgULongLong(&args);
			break;
		case DC_SIGCHAR_FLOAT: {
			DCfloat value = dcbArgFloat(&args);
			unsigned int bits;
			memcpy(&bits, &value, sizeof bits);
			out[i] = bits;
			break;
		}
		case DC_SIGCHAR_DOUBLE: {
			DCdouble value = dcbArgDouble(&args);
			memcpy(&out[i], &value, sizeof value);
			break;
		}
		default:
			return 0;
		}
	}
	/* an argument that was read from past the end of the stack was never passed. */
	return args.sp <= frame->stack + frame->size;
}
