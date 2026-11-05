//go:build !linux

package cmdl

import "os/exec"

// The default Cancel of [exec.CommandContext] already kills the process.
func setupOperatingSystemSpecificsFor(cmd *exec.Cmd) {}
