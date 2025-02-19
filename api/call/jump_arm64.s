#include "textflag.h"

TEXT ·jump_call(SB), NOSPLIT, $0-32
    MOVD fn+0(FP), R9
    MOVD arg1+8(FP), R0
    MOVD arg2+16(FP), R1
    MOVD arg3+24(FP), R2
    CALL R9
    RET
