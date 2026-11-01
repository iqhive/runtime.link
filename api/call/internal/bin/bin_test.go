package bin_test

import (
	"encoding/hex"
	"testing"

	"github.com/iqhive/runtime.link/api/call/internal/bin"
	"github.com/iqhive/runtime.link/api/call/internal/bin/std/cpu/amd64"
	"github.com/iqhive/runtime.link/api/call/internal/bin/std/cpu/arm64"
)

type testFormat struct {
	bin.Format `bin:"test"`

	Greater func(a int) `bin:"0000000[a>10]"`
	Literal func(a int) `bin:"00000[a000]"`
}

func TestAmd64EncodersMatchAnAssembler(t *testing.T) {
	for want, encode := range map[string]func(*amd64.InstructionSet){
		"c3":     func(asm *amd64.InstructionSet) { asm.Return() },
		"4801d1": func(asm *amd64.InstructionSet) { asm.Math.Add(amd64.RCX, amd64.RDX) },
		"4d01c8": func(asm *amd64.InstructionSet) { asm.Math.Add(amd64.R8, amd64.R9) },
		"4829d1": func(asm *amd64.InstructionSet) { asm.Math.Sub(amd64.RCX, amd64.RDX) },
		"4d29c8": func(asm *amd64.InstructionSet) { asm.Math.Sub(amd64.R8, amd64.R9) },
		"48f7e1": func(asm *amd64.InstructionSet) { asm.Math.Mul(amd64.RCX) },
		"49f7e0": func(asm *amd64.InstructionSet) { asm.Math.Mul(amd64.R8) },
		"48f7f1": func(asm *amd64.InstructionSet) { asm.Math.Div(amd64.RCX) },
		"49f7f0": func(asm *amd64.InstructionSet) { asm.Math.Div(amd64.R8) },
	} {
		w := bin.NewWriter[amd64.InstructionSet]()
		encode(w.Encoder)
		if got := hex.EncodeToString(w.Bytes()); got != want {
			t.Errorf("encoded %s, want %s", got, want)
		}
	}
}

func TestArm64EncodersAreLittleEndian(t *testing.T) {
	for want, encode := range map[string]func(*arm64.InstructionSet){
		"c0035fd6": func(asm *arm64.InstructionSet) { asm.Return() },
		"2000028b": func(asm *arm64.InstructionSet) { asm.Math.Add(arm64.X0, arm64.X1, arm64.X2) },
		"be03118b": func(asm *arm64.InstructionSet) { asm.Math.Add(arm64.X30, arm64.X29, arm64.X17) },
	} {
		w := bin.NewWriter[arm64.InstructionSet]()
		encode(w.Encoder)
		if got := hex.EncodeToString(w.Bytes()); got != want {
			t.Errorf("encoded %s, want %s", got, want)
		}
	}
}

func TestComparisonLiteralsMayContainZero(t *testing.T) {
	w := bin.NewWriter[testFormat]()
	w.Encoder.Greater(11)
	w.Encoder.Greater(10)
	if got := hex.EncodeToString(w.Bytes()); got != "0100" {
		t.Errorf("encoded %s", got)
	}
}

func TestSubstitutionWrapsTheArgument(t *testing.T) {
	w := bin.NewWriter[testFormat]()
	w.Encoder.Literal(13)
	if got := hex.EncodeToString(w.Bytes()); got != "05" {
		t.Errorf("encoded %s", got)
	}
}
