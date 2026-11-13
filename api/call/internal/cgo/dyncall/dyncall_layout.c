/*

 Package: dyncall
 File: dyncall/dyncall_layout.c
 Description: Where the arguments of a call are put on arm64, for the tests.

 Apple's convention for arm64 differs from everyone else's, most of all for the
 variable arguments of a C function, and the CallVM for it is otherwise only
 built for, and run on, a Mac. Both CallVMs for arm64 are plain C up to the
 point where the call kernel loads the registers and the stack, so they are
 built here for every platform, under other names, with a kernel that records
 what it was handed and calls nothing. See dyncall_layout_applevm.c and
 dyncall_layout_aapcsvm.c.

*/

#include <string.h>
#include "dyncall_layout.h"

DCCallVM *goAppleNewCallVM(DCsize size);
DCCallVM *goAAPCSNewCallVM(DCsize size);

/*
 Takes the place of dcCall_arm64, and has its parameters. That loads d0 to d7
 and then x0 to x7 from regdata, and copies size bytes of data to the stack.
 The target is the GoFrame to record it all in, not a function.
*/
void goLayoutCapture(DCpointer target, DCpointer data, DCsize size, DCpointer regdata) {
	GoFrame *frame = target;
	memcpy(frame->d, regdata, sizeof frame->d);
	memcpy(frame->x, (char *)regdata + sizeof frame->d, sizeof frame->x);
	frame->size = size;
	memcpy(frame->stack, data, size < sizeof frame->stack ? size : sizeof frame->stack);
}

void goLayoutARM64(int apple, GoArg *args, int argc, int fixed, GoFrame *frame) {
	/* twice the room that there is to record, so that too many arguments are seen to be. */
	DCsize room = 2 * sizeof frame->stack;
	DCCallVM *vm = apple ? goAppleNewCallVM(room) : goAAPCSNewCallVM(room);
	goArgs(vm, args, argc, fixed);
	dcCallVoid(vm, frame);
	dcFree(vm);
}
