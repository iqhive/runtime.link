/*

 Package: dyncall
 Library: dyncallback
 File: dyncallback/dyncall_callback_sparc32.c
 Description: Callback - Implementation for sparc32
 License:

   Copyright (c) 2007-2022 Daniel Adler <dadler@uni-goettingen.de>,
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


#include "dyncall_callback.h"
#include "dyncall_alloc_wx.h"
#include "dyncall_thunk.h"


/* Callback symbol. */
extern void dcCallbackThunkEntry();

struct DCCallback
{
  DCThunk            thunk;         /* offset  0 size 24 */
  DCCallbackHandler* handler;       /* offset 24 size  4 */
  void*              userdata;      /* offset 28 size  4 */
};


void dcbInitCallback2(DCCallback* pcb, const DCsigchar* signature, DCCallbackHandler* handler, void* userdata, DCaggr *const * aggrs)
{
  pcb->handler  = handler;
  pcb->userdata = userdata;
}


DCCallback* dcbNewCallback2(const DCsigchar* signature, DCCallbackHandler* handler, void* userdata, DCaggr *const * aggrs)
{
  DCCallback* pcb;
  int err = dcAllocWX(sizeof(DCCallback), (void**)&pcb);
  if(err)
    return NULL;

  dcbInitCallback2(pcb, signature, handler, userdata, aggrs);
  dcbInitThunk(&pcb->thunk, dcCallbackThunkEntry);

  err = dcInitExecWX(pcb, sizeof(DCCallback));
  if(err) {
    dcFreeWX(pcb, sizeof(DCCallback));
    return NULL;
  }

  return pcb;
}

