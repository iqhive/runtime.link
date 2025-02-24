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

// List of AMD64 instructions from x86 reference
// Source: https://www.felixcloutier.com/x86/
var amd64Instructions = []string{
	// Basic x86 Instructions
	"AAA", "AAD", "AAM", "AAS", "ADC", "ADD", "AND", "CALL", "CBW",
	"CLC", "CLD", "CLI", "CMC", "CMP", "CMPSB", "CMPSW", "CWD",
	"DAA", "DAS", "DEC", "DIV", "HLT", "IDIV", "IMUL", "IN", "INC",
	"INT", "INTO", "IRET", "JMP", "LAHF", "LDS", "LEA", "LES",
	"LODSB", "LODSW", "LOOP", "MOV", "MOVSB", "MOVSW", "MUL",
	"NEG", "NOP", "NOT", "OR", "OUT", "POP", "POPF", "PUSH",
	"PUSHF", "RCL", "RCR", "RET", "ROL", "ROR", "SAHF", "SAL",
	"SAR", "SBB", "SCASB", "SCASW", "SHL", "SHR", "STC", "STD",
	"STI", "STOSB", "STOSW", "SUB", "TEST", "XCHG", "XLAT", "XOR",

	// Conditional Jumps
	"JA", "JAE", "JB", "JBE", "JC", "JCXZ", "JE", "JG", "JGE",
	"JL", "JLE", "JNA", "JNAE", "JNB", "JNBE", "JNC", "JNE",
	"JNG", "JNGE", "JNL", "JNLE", "JNO", "JNP", "JNS", "JNZ",
	"JO", "JP", "JPE", "JPO", "JS", "JZ",

	// System & FPU Instructions
	"CPUID", "RDTSC", "RDMSR", "WRMSR", "INVD", "WBINVD",
	"CLFLUSH", "SFENCE", "LFENCE", "MFENCE", "PAUSE",
	"FABS", "FADD", "FADDP", "FCHS", "FDIV", "FDIVP", "FDIVR",
	"FDIVRP", "FIADD", "FIDIV", "FIDIVR", "FIMUL", "FISUB",
	"FISUBR", "FMUL", "FMULP", "FSUB", "FSUBP", "FSUBR", "FSUBRP",

	// SIMD Instructions
	"ADDPS", "ADDSS", "ANDPS", "ANDNPS", "CMPPS", "CMPSS",
	"COMISS", "CVTPI2PS", "CVTPS2PI", "CVTSI2SS", "CVTSS2SI",
	"CVTTPS2PI", "CVTTSS2SI", "DIVPS", "DIVSS", "LDMXCSR",
	"MAXPS", "MAXSS", "MINPS", "MINSS", "MOVAPS", "MOVHLPS",
	"MOVHPS", "MOVLHPS", "MOVLPS", "MOVMSKPS", "MOVSS",
	"MOVUPS", "MULPS", "MULSS", "RCPPS", "RCPSS", "RSQRTPS",
	"RSQRTSS", "SHUFPS", "SQRTPS", "SQRTSS", "STMXCSR",
	"SUBPS", "SUBSS", "UCOMISS", "UNPCKHPS", "UNPCKLPS",
	"XORPS",
}

func main() {
	// Parse the cpu/amd64 package
	fset := token.NewFileSet()
	pkgPath := filepath.Join("cpu", "amd64")
	pkgs, err := parser.ParseDir(fset, pkgPath, nil, parser.ParseComments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing package: %v\n", err)
		os.Exit(1)
	}

	// Find all //asm: tags
	implemented := make(map[string]bool)
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				if fn, ok := n.(*ast.FuncDecl); ok {
					if fn.Doc != nil {
						for _, comment := range fn.Doc.List {
							if strings.HasPrefix(comment.Text, "//asm:") {
								instr := strings.TrimPrefix(comment.Text, "//asm:")
								implemented[strings.TrimSpace(instr)] = true
							}
						}
					}
				}
				return true
			})
		}
	}

	// Sort instructions for stable output
	sort.Strings(amd64Instructions)

	// Print missing instructions
	fmt.Println("Missing AMD64 Instructions:")
	fmt.Println("==========================")
	for _, instr := range amd64Instructions {
		if !implemented[instr] {
			fmt.Printf("- %s\n", instr)
		}
	}

	// Print implementation progress
	total := len(amd64Instructions)
	done := len(implemented)
	fmt.Printf("\nProgress: %d/%d instructions (%.1f%%)\n", 
		done, total, float64(done)/float64(total)*100)
}
