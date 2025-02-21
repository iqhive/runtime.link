#include "textflag.h"

TEXT ·fast_call(SB), NOSPLIT, $8-0
    MOVQ 8(AX), R13
    MOVQ AX, R12
    SUBQ SP, R12
    MOVQ R12, offset+0(SP)
    MOVQ 0(AX), AX
    CALL R13
    MOVQ offset+0(SP), R12
    ADDQ SP, R12
    MOVQ AX, 0(R12)
    RET
