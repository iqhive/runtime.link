package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// List of ARM64 instructions from ARM Architecture Reference Manual ARMv8
// Source: ARM Architecture Reference Manual ARMv8
var arm64Instructions = []string{
	// Data Processing - Immediate
	"ADD", "ADDS", "SUB", "SUBS", "CMP", "CMN", "AND", "ORR", "EOR", "ANDS",
	"MOVZ", "MOVN", "MOVK", "ADR", "ADRP", "BFM", "SBFM", "UBFM", "EXTR",

	// Branches
	"B", "BR", "BL", "BLR", "RET", "CBNZ", "CBZ", "TBNZ", "TBZ",

	// Data Processing - Register
	"ADD", "ADDS", "SUB", "SUBS", "CMP", "CMN", "AND", "ORR", "EOR", "ANDS",
	"BIC", "BICS", "EON", "ORN", "ASR", "LSL", "LSR", "ROR", "MADD", "MSUB",
	"SMADDL", "SMSUBL", "UMADDL", "UMSUBL", "SMULH", "UMULH", "MUL", "MNEG",
	"SDIV", "UDIV", "CLS", "CLZ", "RBIT", "REV", "REV16", "REV32",

	// Loads and Stores
	"LDR", "LDRB", "LDRH", "LDRSB", "LDRSH", "LDRSW", "STR", "STRB", "STRH",
	"LDUR", "LDURB", "LDURH", "LDURSB", "LDURSH", "LDURSW", "STUR", "STURB", "STURH",
	"LDP", "LDPSW", "STP", "LDNP", "STNP", "LDAR", "LDARB", "LDARH", "STLR", "STLRB", "STLRH",

	// SIMD and Floating Point
	"FABS", "FADD", "FCMP", "FCSEL", "FDIV", "FMADD", "FMSUB", "FMUL", "FNEG",
	"FNMADD", "FNMSUB", "FMAX", "FMIN", "FSQRT", "FSUB", "SCVTF", "UCVTF", "FCVTZS", "FCVTZU",

	// System Instructions
	"SVC", "HVC", "SMC", "BRK", "HLT", "DMB", "DSB", "ISB", "MSR", "MRS",
	"HINT", "NOP", "YIELD", "WFE", "WFI", "SEV", "SEVL",

	// Atomic Operations
	"LDXR", "LDXRB", "LDXRH", "STXR", "STXRB", "STXRH",
	"LDAXR", "LDAXRB", "LDAXRH", "STLXR", "STLXRB", "STLXRH",
	"CAS", "CASA", "CASL", "CASAL", "SWP", "SWPA", "SWPL", "SWPAL",

	// Advanced SIMD
	"ABS", "ADD", "ADDHN", "ADDP", "AND", "BIC", "BIF", "BIT", "BSL",
	"CLS", "CLZ", "CNT", "EOR", "EXT", "FABD", "FABS", "FADD", "FADDP",
	"FMAX", "FMAXNM", "FMAXNMP", "FMAXP", "FMIN", "FMINNM", "FMINNMP", "FMINP",
	"FMLA", "FMLS", "FMUL", "FMULX", "FNEG", "FRECPE", "FRECPS", "FRECPX",
	"FRSQRTE", "FRSQRTS", "FSQRT", "FSUB", "MLA", "MLS", "MOV", "MOVI", "MUL",
	"MVN", "NEG", "NOT", "ORN", "ORR", "PMUL", "PMULL", "RBIT", "REV16", "REV32", "REV64",
	"RSHRN", "RSUBHN", "SABA", "SABAL", "SABD", "SABDL", "SADALP", "SADDL", "SADDLP",
	"SADDW", "SHADD", "SHL", "SHLL", "SHRN", "SHSUB", "SLI", "SMAX", "SMAXP", "SMIN",
	"SMINP", "SMLAL", "SMLSL", "SMOV", "SMULL", "SQABS", "SQADD", "SQDMLAL", "SQDMLSL",
	"SQDMULH", "SQDMULL", "SQNEG", "SQRDMULH", "SQRSHL", "SQRSHRN", "SQRSHRUN",
	"SQSHL", "SQSHLU", "SQSHRN", "SQSHRUN", "SQSUB", "SQXTN", "SQXTUN", "SRHADD",
	"SRI", "SRSHL", "SRSHR", "SRSRA", "SSHL", "SSHLL", "SSHR", "SSRA", "SSUBL",
	"SSUBW", "SUB", "SUBHN", "SUQADD", "TBL", "TBX", "TRN", "UABA", "UABAL",
	"UABD", "UABDL", "UADALP", "UADDL", "UADDLP", "UADDW", "UCVTF", "UHADD",
	"UHSUB", "UMAX", "UMAXP", "UMIN", "UMINP", "UMLAL", "UMLSL", "UMOV",
	"UMULL", "UQADD", "UQRSHL", "UQRSHRN", "UQSHL", "UQSHRN", "UQSUB",
	"UQXTN", "URECPE", "URHADD", "URSHL", "URSHR", "URSQRTE", "URSRA",
	"USHL", "USHLL", "USHR", "USQADD", "USRA", "USUBL", "USUBW", "UZP", "XTN", "ZIP",
}

func main() {
	// Parse the cpu/arm64 package
	fset := token.NewFileSet()
	pkgPath := filepath.Join("cpu", "arm64")
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}
	if strings.Contains(wd, "cpu/arm64") {
		pkgPath = wd
	}
	// Files to scan for instruction implementations
	files := []string{
		filepath.Join(pkgPath, "api.go"),
		filepath.Join(pkgPath, "asm.go"),
	}

	// Find all implemented instructions
	implemented := make(map[string]bool)
	for _, file := range files {
		f, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", file, err)
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			// function name followed by ASM name in brackets
			if fn, ok := n.(*ast.FuncDecl); ok {
				if fn.Doc == nil || len(fn.Doc.List) == 0 {
					return true
				}
				comment := fn.Doc.List[0].Text
				_, asmName, ok := strings.Cut(comment, "(")
				if !ok {
					return true
				}
				asmName = strings.TrimSuffix(asmName, ")")
				implemented[asmName] = true
			}
			return true
		})
	}

	// Sort instructions for stable output
	sort.Strings(arm64Instructions)

	// Print missing instructions
	fmt.Println("Missing ARM64 Instructions:")
	fmt.Println("==========================")
	for _, instr := range arm64Instructions {
		if !implemented[instr] {
			fmt.Printf("- %s\n", instr)
		}
	}

	// Print implementation progress
	total := len(arm64Instructions)
	done := len(implemented)
	fmt.Printf("\nProgress: %d/%d instructions (%.1f%%)\n",
		done, total, float64(done)/float64(total)*100)
}
