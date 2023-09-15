#include "go_asm.h"
#include "textflag.h"


TEXT ·PushFunc(SB),NOSPLIT|NOPTR,$0-0
 MOVQ AX, R13
 RET

TEXT ·CallFunc(SB),NOSPLIT|NOPTR,$0-0
    MOVQ	SP, DX
	ANDQ	$~15, SP	// alignment
    // load the difference between the alignment and the current stack pointer into R14
    MOVQ    SP, R14
    SUBQ    DX, R14
    CALL R13
    ADDQ    R14, SP  // restore the stack pointer
    RET
