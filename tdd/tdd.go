// Package tdd provides a structure for creating API testing frameworks.
package tdd

import (
	"time"

	"runtime.link/nix"
)

type literal string

// Framework can be embedded inside of a struct to signal that the struct is an API testing framework.
type Framework struct {
	OS nix.Standard

	time time.Time
}

func (fw Framework) framework() {}

type embedsFramework interface {
	framework()
}

func (fw Framework) Story(description literal) {}
func (fw Framework) Guide(description literal) {}
