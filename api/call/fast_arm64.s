#include "textflag.h"

TEXT ·fast_call(SB), NOSPLIT, $8-0
    MOVD 8(R0), R17
    MOVD RSP, R16
    SUB R16, R0, R16
    MOVD R16, offset+0(SP)
    MOVD 0(R0), R0
    CALL R17
    MOVD offset+0(SP), R16
    ADD R16, RSP, R16
    MOVD R0, 0(R16)
    RET
