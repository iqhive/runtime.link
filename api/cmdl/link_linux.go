//go:build linux

package cmdl

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

// setupOperatingSystemSpecificsFor gives cmd a process group of its own, and
// has a cancelled context kill the whole group.
//
// A command that is given the terminal of the caller stays in the group of
// the caller instead, with the default Cancel of [exec.CommandContext], which
// kills the command and nothing that it started. A terminal only lets its
// foreground group read it, and only interrupts that group, so in a group of
// its own the command would be stopped at its first read and would outlive a
// caller that was interrupted.
func setupOperatingSystemSpecificsFor(cmd *exec.Cmd) {
	if hasTerminalOfCaller(cmd) {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}

// hasTerminalOfCaller reports whether cmd inherits the controlling terminal
// of this process as its stdin, stdout or stderr. Only a file is inherited,
// anything else reaches the command through a pipe.
func hasTerminalOfCaller(cmd *exec.Cmd) bool {
	for _, stream := range []any{cmd.Stdin, cmd.Stdout, cmd.Stderr} {
		if file, ok := stream.(*os.File); ok && isControllingTerminal(file) {
			return true
		}
	}
	return false
}

// isControllingTerminal reports whether file is the controlling terminal of
// this process. A terminal only tells its foreground group to the processes
// it controls, so being told is the answer, whichever group it is.
func isControllingTerminal(file *os.File) bool {
	conn, err := file.SyscallConn()
	if err != nil {
		return false
	}
	var isTerminal bool
	err = conn.Control(func(fd uintptr) {
		var foreground int32
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCGPGRP), uintptr(unsafe.Pointer(&foreground)))
		isTerminal = errno == 0
	})
	return err == nil && isTerminal
}
