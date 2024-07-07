package main

import (
	"math"
	"testing"
)

func TestImports(t *testing.T) {
	result := math.Sqrt(16)
	if result != 4 {
		t.FailNow()
	}
}

var packageLevelVar = "initialized"

func TestPackageLevelVar(t *testing.T) {
	if packageLevelVar != "initialized" {
		t.FailNow()
	}
}
