//go:build !darwin

/*

 Package: dyncall
 Library: dyncall
 File: dyncall/dyncall_callvm_arm64.c
 Description: ARM 64-bit AAPCS64 ABI implementation, everywhere but Apple's
 platforms, which have their own in dyncall_callvm_darwin_arm64.c. The two
 differ in the stack: here each argument takes a full 8 byte slot, Apple
 packs them by their natural size. Variadic arguments are passed no
 differently from any others.
 License:

   Copyright (c) 2015-2020 Daniel Adler <dadler@uni-goettingen.de>,
                           Tassilo Philipp <tphilipp@potion-studios.com>

   Permission to use, copy, modify, and distribute this software for any
   purpose with or without fee is hereby granted, provided that the above
   copyright notice and this permission notice appear in all copies.

   THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
   WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
   MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
   ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
   WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
   ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
   OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.

*/


#include "dyncall_callvm_arm64.h"
#include "dyncall_alloc.h"


void dcCall_arm64(DCpointer target, DCpointer data, DCsize size, DCpointer regdata);


static void reset(DCCallVM* in_p)
{
  DCCallVM_arm64* p = (DCCallVM_arm64*)in_p;
  p->i = 0;
  p->f = 0;
  dcVecReset(&p->mVecHead);
}


static void deinit(DCCallVM* in_p)
{
  dcFreeMem(in_p);
}


static void a_i64(DCCallVM* in_p, DClonglong x)
{
  DCCallVM_arm64* p = (DCCallVM_arm64*)in_p;
  if (p->i < 8) {
    p->I[p->i] = x;
    p->i++;
  } else {
    dcVecAppend(&p->mVecHead, &x, sizeof(DClonglong));
  }
}

static void a_bool   (DCCallVM* in_p, DCbool    x) { a_i64(in_p, (DClonglong) x); }
static void a_char   (DCCallVM* in_p, DCchar    x) { a_i64(in_p, (DClonglong) x); }
static void a_short  (DCCallVM* in_p, DCshort   x) { a_i64(in_p, (DClonglong) x); }
static void a_int    (DCCallVM* in_p, DCint     x) { a_i64(in_p, (DClonglong) x); }
static void a_long   (DCCallVM* in_p, DClong    x) { a_i64(in_p, (DClonglong) x); }
static void a_pointer(DCCallVM* in_p, DCpointer x) { a_i64(in_p, (DClonglong) x); }

static void a_float(DCCallVM* in_p, DCfloat x)
{
  DCCallVM_arm64* p = (DCCallVM_arm64*)in_p;

  if (p->f < 8) {
    p->u.S[ p->f << 1 ] = x;
    p->f++;
  } else {
    dcVecAppend(&p->mVecHead, &x, sizeof(DCfloat));
    dcVecSkip(&p->mVecHead, sizeof(DClonglong) - sizeof(DCfloat));
  }
}

static void a_double(DCCallVM* in_p, DCdouble x)
{
  DCCallVM_arm64* p = (DCCallVM_arm64*)in_p;
  if (p->f < 8) {
    p->u.D[ p->f ] = x;
    p->f++;
  } else {
    dcVecAppend(&p->mVecHead, &x, sizeof(DCdouble));
  }
}

static void call(DCCallVM* in_p, DCpointer target)
{
  DCCallVM_arm64* p = (DCCallVM_arm64*)in_p;

  /*
  ** copy 'size' argument is given in number of 16-byte 'pair' blocks.
  */

  dcCall_arm64(target, dcVecData(&p->mVecHead), ( dcVecSize(&p->mVecHead) + 15 ) & -16, &p->u.S[0]);
}

static void mode(DCCallVM* in_p,DCint mode);

DCCallVM_vt vt_arm64 =
{
  &deinit
, &reset
, &mode
, &a_bool
, &a_char
, &a_short
, &a_int
, &a_long
, &a_i64
, &a_float
, &a_double
, &a_pointer
, NULL /* argAggr */
, (DCvoidvmfunc*)       &call
, (DCboolvmfunc*)       &call
, (DCcharvmfunc*)       &call
, (DCshortvmfunc*)      &call
, (DCintvmfunc*)        &call
, (DClongvmfunc*)       &call
, (DClonglongvmfunc*)   &call
, (DCfloatvmfunc*)      &call
, (DCdoublevmfunc*)     &call
, (DCpointervmfunc*)    &call
, NULL /* callAggr */
, NULL /* beginAggr */
};

static void mode(DCCallVM* in_self, DCint mode)
{
  DCCallVM_arm64* self = (DCCallVM_arm64*)in_self;

  switch(mode) {
    case DC_CALL_C_DEFAULT:
    case DC_CALL_C_DEFAULT_THIS:
    case DC_CALL_C_ARM64:
    case DC_CALL_C_ELLIPSIS:
    case DC_CALL_C_ELLIPSIS_VARARGS:
      break;
    default:
      self->mInterface.mError = DC_ERROR_UNSUPPORTED_MODE;
      return;
  }
  dc_callvm_base_init(&self->mInterface, &vt_arm64);
}

/* Public API. */
DCCallVM* dcNewCallVM(DCsize size)
{
  DCCallVM_arm64* p = (DCCallVM_arm64*)dcAllocMem(sizeof(DCCallVM_arm64)+size);

  mode((DCCallVM*)p, DC_CALL_C_DEFAULT);

  dcVecInit(&p->mVecHead, size);
  reset((DCCallVM*)p);

  return (DCCallVM*)p;
}
