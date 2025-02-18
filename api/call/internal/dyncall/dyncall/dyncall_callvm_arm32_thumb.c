/*

 Package: dyncall
 Library: dyncall
 File: dyncall/dyncall_callvm_arm32_thumb.c
 Description: ARM 32-bit "thumb" ABI callvm implementation
 License:

   Copyright (c) 2007-2018 Daniel Adler <dadler@uni-goettingen.de>,
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



/*

  dyncall callvm for 32bit ARM32 family of processors

  SUPPORTED CALLING CONVENTIONS
  armthumbcall

  REVISION
  2007/12/11 initial

*/


#include "dyncall_callvm_arm32_thumb.h"
#include "dyncall_alloc.h"

/*
** arm32 thumb mode calling convention calls
**
** - hybrid return-type call (bool ... pointer)
**
** Note: return types of this decl and func below are of double-word size, intentionally
** to keep some platforms' compilers from generating code in the callers that reuse, and
** thus overwrite, r0 and r1 directly after the call.
** With this "hint", we preserve those registers by letting the compiler assume both
** registers are used for the return type.
*/
DClonglong dcCall_arm32_thumb(DCpointer target, DCpointer stackdata, DCsize size);


/* Call. */
DClonglong dc_callvm_call_arm32_thumb(DCCallVM* in_self, DCpointer target)
{
  DCCallVM_arm32_thumb* self = (DCCallVM_arm32_thumb*)in_self;
  return dcCall_arm32_thumb(target, dcVecData(&self->mVecHead), dcVecSize(&self->mVecHead));
}


static void dc_callvm_mode_arm32_thumb(DCCallVM* in_self,DCint mode);

static void dc_callvm_free_arm32_thumb(DCCallVM* in_self)
{
  dcFreeMem(in_self);
}


static void dc_callvm_reset_arm32_thumb(DCCallVM* in_self)
{
  DCCallVM_arm32_thumb* self = (DCCallVM_arm32_thumb*)in_self;
  dcVecReset(&self->mVecHead);
}


static void dc_callvm_argInt_arm32_thumb(DCCallVM* in_self, DCint x)
{
  DCCallVM_arm32_thumb* self = (DCCallVM_arm32_thumb*)in_self;
  dcVecAppend(&self->mVecHead, &x, sizeof(DCint));
}


static void dc_callvm_argBool_arm32_thumb(DCCallVM* in_self, DCbool x)
{
  dc_callvm_argInt_arm32_thumb(in_self, (DCint)x);
}


static void dc_callvm_argChar_arm32_thumb(DCCallVM* in_self, DCchar x)
{
  dc_callvm_argInt_arm32_thumb(in_self, x);
}


static void dc_callvm_argShort_arm32_thumb(DCCallVM* in_self, DCshort x)
{
  dc_callvm_argInt_arm32_thumb(in_self, x);
}


static void dc_callvm_argLong_arm32_thumb(DCCallVM* in_self, DClong x)
{
  dc_callvm_argInt_arm32_thumb(in_self, x);
}


static void dc_callvm_argLongLong_arm32_thumb(DCCallVM* in_self, DClonglong x)
{
  DCCallVM_arm32_thumb* self = (DCCallVM_arm32_thumb*)in_self;
  dcVecAppend(&self->mVecHead, &x, sizeof(DClonglong));
}


static void dc_callvm_argLongLong_arm32_thumb_eabi(DCCallVM* in_self, DClonglong x)
{
  DCCallVM_arm32_thumb* self = (DCCallVM_arm32_thumb*)in_self;

  /* 64 bit values need to be aligned on 8 byte boundaries */
  dcVecSkip(&self->mVecHead, dcVecSize(&self->mVecHead) & 4);
  dcVecAppend(&self->mVecHead, &x, sizeof(DClonglong));
}


static void dc_callvm_argFloat_arm32_thumb(DCCallVM* in_self, DCfloat x)
{
  DCCallVM_arm32_thumb* self = (DCCallVM_arm32_thumb*)in_self;
  dcVecAppend(&self->mVecHead, &x, sizeof(DCfloat));
}


static void dc_callvm_argDouble_arm32_thumb(DCCallVM* in_self, DCdouble x)
{
  DCCallVM_arm32_thumb* self = (DCCallVM_arm32_thumb*)in_self;
  dcVecAppend(&self->mVecHead, &x, sizeof(DCdouble));
}


static void dc_callvm_argDouble_arm32_thumb_eabi(DCCallVM* in_self, DCdouble x)
{
  DCCallVM_arm32_thumb* self = (DCCallVM_arm32_thumb*)in_self;

  /* 64 bit values need to be aligned on 8 byte boundaries */
  dcVecSkip(&self->mVecHead, dcVecSize(&self->mVecHead) & 4);
  dcVecAppend(&self->mVecHead, &x, sizeof(DCdouble));
}


static void dc_callvm_argPointer_arm32_thumb(DCCallVM* in_self, DCpointer x)
{
  DCCallVM_arm32_thumb* self = (DCCallVM_arm32_thumb*)in_self;
  dcVecAppend(&self->mVecHead, &x, sizeof(DCpointer));
}


DCCallVM_vt gVT_arm32_thumb =
{
  &dc_callvm_free_arm32_thumb
, &dc_callvm_reset_arm32_thumb
, &dc_callvm_mode_arm32_thumb
, &dc_callvm_argBool_arm32_thumb
, &dc_callvm_argChar_arm32_thumb
, &dc_callvm_argShort_arm32_thumb
, &dc_callvm_argInt_arm32_thumb
, &dc_callvm_argLong_arm32_thumb
, &dc_callvm_argLongLong_arm32_thumb
, &dc_callvm_argFloat_arm32_thumb
, &dc_callvm_argDouble_arm32_thumb
, &dc_callvm_argPointer_arm32_thumb
, NULL /* argAggr */
, (DCvoidvmfunc*)       &dc_callvm_call_arm32_thumb
, (DCboolvmfunc*)       &dc_callvm_call_arm32_thumb
, (DCcharvmfunc*)       &dc_callvm_call_arm32_thumb
, (DCshortvmfunc*)      &dc_callvm_call_arm32_thumb
, (DCintvmfunc*)        &dc_callvm_call_arm32_thumb
, (DClongvmfunc*)       &dc_callvm_call_arm32_thumb
, (DClonglongvmfunc*)   &dc_callvm_call_arm32_thumb
, (DCfloatvmfunc*)      &dc_callvm_call_arm32_thumb
, (DCdoublevmfunc*)     &dc_callvm_call_arm32_thumb
, (DCpointervmfunc*)    &dc_callvm_call_arm32_thumb
, NULL /* callAggr */
, NULL /* beginAggr */
};

DCCallVM_vt gVT_arm32_thumb_eabi =
{
  &dc_callvm_free_arm32_thumb
, &dc_callvm_reset_arm32_thumb
, &dc_callvm_mode_arm32_thumb
, &dc_callvm_argBool_arm32_thumb
, &dc_callvm_argChar_arm32_thumb
, &dc_callvm_argShort_arm32_thumb
, &dc_callvm_argInt_arm32_thumb
, &dc_callvm_argLong_arm32_thumb
, &dc_callvm_argLongLong_arm32_thumb_eabi
, &dc_callvm_argFloat_arm32_thumb
, &dc_callvm_argDouble_arm32_thumb_eabi
, &dc_callvm_argPointer_arm32_thumb
, NULL /* argAggr */
, (DCvoidvmfunc*)       &dc_callvm_call_arm32_thumb
, (DCboolvmfunc*)       &dc_callvm_call_arm32_thumb
, (DCcharvmfunc*)       &dc_callvm_call_arm32_thumb
, (DCshortvmfunc*)      &dc_callvm_call_arm32_thumb
, (DCintvmfunc*)        &dc_callvm_call_arm32_thumb
, (DClongvmfunc*)       &dc_callvm_call_arm32_thumb
, (DClonglongvmfunc*)   &dc_callvm_call_arm32_thumb
, (DCfloatvmfunc*)      &dc_callvm_call_arm32_thumb
, (DCdoublevmfunc*)     &dc_callvm_call_arm32_thumb
, (DCpointervmfunc*)    &dc_callvm_call_arm32_thumb
, NULL /* callAggr */
, NULL /* beginAggr */
};

static void dc_callvm_mode_arm32_thumb(DCCallVM* in_self, DCint mode)
{
  DCCallVM_arm32_thumb* self = (DCCallVM_arm32_thumb*)in_self;
  DCCallVM_vt* vt;

  switch(mode) {
    case DC_CALL_C_ELLIPSIS:
    case DC_CALL_C_ELLIPSIS_VARARGS:
    case DC_CALL_C_DEFAULT_THIS:
/* Check OS if we need EABI as default. */
#if defined(DC__ABI_ARM_EABI)
    case DC_CALL_C_DEFAULT:        vt = &gVT_arm32_thumb_eabi; break;
#else
    case DC_CALL_C_DEFAULT:        vt = &gVT_arm32_thumb;      break;
#endif
    case DC_CALL_C_ARM_THUMB:      vt = &gVT_arm32_thumb;      break;
    case DC_CALL_C_ARM_THUMB_EABI: vt = &gVT_arm32_thumb_eabi; break;
    default:
      self->mInterface.mError = DC_ERROR_UNSUPPORTED_MODE;
      return;
  }
  dc_callvm_base_init(&self->mInterface, vt);
}

/* Public API. */
DCCallVM* dcNewCallVM(DCsize size)
{
  /* Store at least 16 bytes (4 words) for internal spill area. Assembly code depends on it. */
  DCCallVM_arm32_thumb* p = (DCCallVM_arm32_thumb*)dcAllocMem(sizeof(DCCallVM_arm32_thumb)+size+16);

  dc_callvm_mode_arm32_thumb((DCCallVM*)p, DC_CALL_C_DEFAULT);

  dcVecInit(&p->mVecHead, size);

  return (DCCallVM*)p;
}

