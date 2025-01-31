//go:build linux

package cmdl

import (
	"os"
	"os/exec"
	"syscall"

	"runtime.link/api/xray"
)

func setupOperatingSystemSpecificsFor(cmd *exec.Cmd, stdoutWrite, stderrWrite *os.File) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if err := stdoutWrite.Close(); err != nil {
			return xray.New(err)
		}
		if err := stderrWrite.Close(); err != nil {
			return xray.New(err)
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}
