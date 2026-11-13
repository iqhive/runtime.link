//go:build linux

package cmdl_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/cmdl"
)

// TestProgramOnTerminal is not a test. It is the program that the terminal
// tests run on a terminal of its own: it runs CMDL_TERMINAL_SCRIPT with sh,
// in the way that CMDL_TERMINAL names, and dies of an interrupt as a program
// does that has no handler for one.
func TestProgramOnTerminal(t *testing.T) {
	mode := os.Getenv("CMDL_TERMINAL")
	if mode == "" {
		t.Skip("only runs as the program of the terminal tests")
	}
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, syscall.SIGINT)
	go func() {
		<-interrupted
		os.Exit(130)
	}()
	type Opts struct {
		In io.Reader
	}
	A := api.Import[struct {
		api.Specification
		Run      func(context.Context, string, Opts) error     `cmdl:"-c %v %v"`
		Captured func(context.Context, string) (string, error) `cmdl:"-c %v"`
	}](cmdl.API, "sh", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	script := os.Getenv("CMDL_TERMINAL_SCRIPT")
	switch mode {
	case "link":
		A.Run(ctx, script, Opts{In: os.Stdin})
	case "cancel":
		soon, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		A.Run(soon, script, Opts{In: os.Stdin})
	case "captured":
		// Nothing of this command is the terminal: no stdin, and its output
		// is collected for the results.
		soon, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		A.Captured(soon, script)
	case "exec":
		cmd := exec.CommandContext(ctx, "sh", "-c", script)
		cmd.Stdin = os.Stdin
		cmd.Run()
	}
	os.Exit(0)
}

// openTerminal returns both ends of a new pseudo terminal.
func openTerminal(t *testing.T) (master, slave *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("no pseudo terminal: %v", err)
	}
	t.Cleanup(func() { master.Close() })
	var (
		unlock int32
		number uint32
	)
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&unlock))); errno != 0 {
		t.Skipf("no pseudo terminal: %v", errno)
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), syscall.TIOCGPTN, uintptr(unsafe.Pointer(&number))); errno != 0 {
		t.Skipf("no pseudo terminal: %v", errno)
	}
	slave, err = os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("no pseudo terminal: %v", err)
	}
	return master, slave
}

// runOnTerminal runs [TestProgramOnTerminal] as the foreground job of a new
// terminal, types input once the script has written its pid, and returns
// that pid and how long the program took from then on to exit.
func runOnTerminal(t *testing.T, mode, script, input string) (pid int, took time.Duration) {
	t.Helper()
	master, slave := openTerminal(t)
	pidfile := filepath.Join(t.TempDir(), "pid")
	ctx, cancel := context.WithTimeout(context.Background(), streamTimeout)
	defer cancel()
	program := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProgramOnTerminal$")
	program.Env = append(os.Environ(), "CMDL_TERMINAL="+mode, "CMDL_TERMINAL_SCRIPT=echo $$ > "+pidfile+"; "+script)
	program.Stdin, program.Stdout, program.Stderr = slave, slave, slave
	program.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := program.Start(); err != nil {
		t.Fatal(err)
	}
	slave.Close()
	// What the terminal echoes has to go somewhere, or the program blocks
	// writing it. The copy ends with the terminal.
	go io.Copy(io.Discard, master)

	for deadline := time.Now().Add(streamTimeout); pid == 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			program.Process.Kill()
			program.Wait()
			t.Fatal("the script did not start")
		}
		text, _ := os.ReadFile(pidfile)
		if strings.HasSuffix(string(text), "\n") {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(text)))
		}
	}
	t.Cleanup(func() { syscall.Kill(pid, syscall.SIGKILL) })
	// The script has to reach the command after the one that wrote its pid.
	time.Sleep(200 * time.Millisecond)
	started := time.Now()
	if _, err := master.WriteString(input); err != nil {
		t.Fatal(err)
	}
	program.Wait()
	took = time.Since(started)
	// What was signalled needs a moment to die.
	time.Sleep(300 * time.Millisecond)
	return pid, took
}

func isRunning(pid int) bool {
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false
	}
	// The state follows the command name, which is in brackets.
	_, state, _ := strings.Cut(string(stat), ") ")
	return !strings.HasPrefix(state, "Z") && !strings.HasPrefix(state, "X")
}

// Ctrl-C interrupts the foreground job of a terminal: the program and what
// it runs. A linked command was once always put in a process group of its
// own, which is not that job, so the program died and the command it had
// been waiting for carried on with nobody left to cancel it.
func TestInterruptReachesACommandOnTheTerminal(t *testing.T) {
	const interrupt = "\x03"
	if pid, _ := runOnTerminal(t, "exec", "sleep 30", interrupt); isRunning(pid) {
		t.Fatal("the test is wrong: a command run with os/exec survived the interrupt")
	}
	if pid, _ := runOnTerminal(t, "link", "sleep 30", interrupt); isRunning(pid) {
		t.Errorf("the program was interrupted and exited, its linked command (pid %d) is still running", pid)
	}
}

// A terminal stops a process that reads it from outside its foreground job,
// so a linked command given os.Stdin never got what was typed.
func TestCommandOnTheTerminalCanReadIt(t *testing.T) {
	answer := filepath.Join(t.TempDir(), "answer")
	script := `read line; echo "got:$line" > ` + answer
	runOnTerminal(t, "exec", script, "hello\n")
	if text, _ := os.ReadFile(answer); string(text) != "got:hello\n" {
		t.Fatalf("the test is wrong: a command run with os/exec read %q from the terminal", text)
	}
	os.Remove(answer)
	if _, took := runOnTerminal(t, "link", script, "hello\n"); took >= promptly {
		t.Errorf("the call took %v, it waited for its context to end", took)
	}
	if text, _ := os.ReadFile(answer); string(text) != "got:hello\n" {
		t.Errorf("a linked command given os.Stdin read %q from the terminal, want %q", text, "got:hello\n")
	}
}

// A command on the terminal is in the group of its caller, which must not be
// killed with it, so a cancelled call kills the command alone.
func TestCancelledCallKillsACommandOnTheTerminal(t *testing.T) {
	pid, took := runOnTerminal(t, "cancel", "exec sleep 30", "")
	if isRunning(pid) {
		t.Errorf("the call was cancelled, its command (pid %d) is still running", pid)
	}
	if took >= promptly {
		t.Errorf("the cancelled call took %v to return", took)
	}
}

// A command that is given nothing of the terminal still has a process group
// of its own, and what it started dies with it when its call is cancelled,
// wherever the program that calls it runs.
func TestCancelledCallOffTheTerminalKillsItsProcessGroup(t *testing.T) {
	started := filepath.Join(t.TempDir(), "started")
	_, took := runOnTerminal(t, "captured", "sh -c 'echo $$ > "+started+"; exec sleep 30' & wait", "")
	text, err := os.ReadFile(started)
	if err != nil {
		t.Fatal(err)
	}
	child, err := strconv.Atoi(strings.TrimSpace(string(text)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syscall.Kill(child, syscall.SIGKILL) })
	if isRunning(child) {
		t.Errorf("the call was cancelled, what its command started (pid %d) is still running", child)
	}
	if took >= promptly {
		t.Errorf("the cancelled call took %v to return", took)
	}
}
