#include "textflag.h"

TEXT ·jump_call(SB), NOSPLIT, $0-32
    MOVQ fn+0(FP), AX
    MOVQ arg1+8(FP), DI
    MOVQ arg2+16(FP), SI
    MOVQ arg3+24(FP), DX
    CALL AX
    RET
