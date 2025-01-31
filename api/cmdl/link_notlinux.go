//go:build !linux

package cmdl

import (
	"os"
	"os/exec"

	"runtime.link/api/xray"
)

func setupOperatingSystemSpecificsFor(cmd *exec.Cmd, stdoutWrite, stderrWrite *os.File) {
	cancel := cmd.Cancel
	cmd.Cancel = func() error {
		if err := stdoutWrite.Close(); err != nil {
			return xray.New(err)
		}
		if err := stderrWrite.Close(); err != nil {
			return xray.New(err)
		}
		if cancel != nil {
			cancel()
		}
		return nil
	}
}
