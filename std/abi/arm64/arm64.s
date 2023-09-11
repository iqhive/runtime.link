#include "go_asm.h"
#include "textflag.h"

TEXT ·Prepare(SB),NOPTR,$1000000-0
    MOVD g, R0
    RET
TEXT ·Restore(SB),NOPTR,$0-0
    MOVD R0, g
    RET

