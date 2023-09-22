#include "go_asm.h"
#include "textflag.h"


TEXT ·PushFunc(SB),NOSPLIT|NOPTR,$0-0
 MOVQ AX, R13
 RET

TEXT ·CallFunc(SB),NOSPLIT|NOPTR,$0-0
    MOVQ	SP, R13
	ANDQ	$~15, SP	// alignment
    CALL    AX
    MOVQ    R13, SP  // restore the stack pointer
    RET
