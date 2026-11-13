/*

 Package: dyncall
 File: dyncall/dyncall_go.h
 Description: How Go hands the arguments of a call to a CallVM. It is a
              header so that dyncall_layout.c, which shows the tests where
              arguments are put, runs the very same code as a call does.

*/

#ifndef DYNCALL_GO_H
#define DYNCALL_GO_H

#include <assert.h>
#include "dyncall.h"
#include "dyncall_signature.h"
#include "dyncall_value.h"

typedef struct {
	DCsigchar vtype;
	DCValue value;
} GoArg;

/*
 fixed is the number of named arguments of a function with variable arguments,
 and negative for any other function. The CallVM has to be told where the
 variable ones begin, as some platforms pass them differently: Apple's arm64
 puts every one of them on the stack, even while registers are free.
*/
static void goArgs(DCCallVM *vm, GoArg *arg, int argc, int fixed) {
	/* a CallVM keeps the mode of its last call, which may not be this one's. */
	dcMode(vm, fixed < 0 ? DC_CALL_C_DEFAULT : DC_CALL_C_ELLIPSIS);
	dcReset(vm);
	DCValue value;
	for (int i = 0; i < argc; i++) {
		if (i == fixed) {
			dcMode(vm, DC_CALL_C_ELLIPSIS_VARARGS);
		}
		value = arg[i].value;
		switch (arg[i].vtype) {
		case DC_SIGCHAR_BOOL:
			dcArgBool(vm, value.B);
			break;
		case DC_SIGCHAR_CHAR:
			dcArgChar(vm, value.c);
			break;
		case DC_SIGCHAR_UCHAR:
			// dcArgChar would sign-extend, see dcArgUChar.
			dcArgUChar(vm, value.C);
			break;
		case DC_SIGCHAR_SHORT:
			dcArgShort(vm, value.s);
			break;
		case DC_SIGCHAR_USHORT:
			dcArgUShort(vm, value.S);
			break;
		case DC_SIGCHAR_INT:
			dcArgInt(vm, value.i);
			break;
		case DC_SIGCHAR_UINT:
			dcArgInt(vm, value.i);
			break;
		case DC_SIGCHAR_LONG:
			dcArgLong(vm, value.l);
			break;
		case DC_SIGCHAR_ULONG:
			dcArgLong(vm, value.J);
			break;
		case DC_SIGCHAR_LONGLONG:
			dcArgLongLong(vm, value.l);
			break;
		case DC_SIGCHAR_ULONGLONG:
			dcArgLongLong(vm, value.l);
			break;
		case DC_SIGCHAR_FLOAT:
			dcArgFloat(vm, value.f);
			break;
		case DC_SIGCHAR_DOUBLE:
			dcArgDouble(vm, value.d);
			break;
		case DC_SIGCHAR_POINTER:
			dcArgPointer(vm, value.p);
			break;
		case DC_SIGCHAR_STRING:
			dcArgPointer(vm, value.p);
			break;
		case DC_SIGCHAR_AGGREGATE:
			assert(0); // FIXME
			break;
		}
	}
}

#endif /* DYNCALL_GO_H */
