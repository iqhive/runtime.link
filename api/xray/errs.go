package xray

import (
	"bufio"
	"errors"
	"fmt"
	"runtime"
	"strconv"
)

type errorWithTrace struct {
	error
	trace
}

func (err errorWithTrace) Error() string { return err.error.Error() }

func (err errorWithTrace) Format(f fmt.State, verb rune) {
	var trace []error
	for e := err.error; e != nil; e = errors.Unwrap(e) {
		trace = append(trace, e)
	}
	var traceback = bufio.NewWriter(f)
	traceback.WriteString("Error: ")
	traceback.WriteString(err.error.Error())
	traceback.WriteRune('\n')
	traceback.WriteRune('\n')
	for i := len(trace) - 1; i >= 0; i-- {
		e := trace[i]
		switch e := e.(type) {
		case traceable:
			function, file, line, ok := e.Source()
			if ok {
				traceback.WriteString(function)
				traceback.WriteRune('(')
				traceback.WriteRune(')')
				traceback.WriteRune('\n')
				traceback.WriteRune('\t')
				traceback.WriteString(file)
				traceback.WriteRune(':')
				traceback.WriteString(strconv.Itoa(line))
				traceback.WriteRune('\n')
			}
		}
	}
	function, file, line, ok := err.Source()
	if ok {
		traceback.WriteString(function)
		traceback.WriteRune('(')
		traceback.WriteRune(')')
		traceback.WriteRune('\n')
		traceback.WriteRune('\t')
		traceback.WriteString(file)
		traceback.WriteRune(':')
		traceback.WriteString(strconv.Itoa(line))
		traceback.WriteRune('\n')
	}
	traceback.Flush()
}

func (err errorWithTrace) Unwrap() error { return err.error }

// New wraps the provided error with the caller's file and line number.
func New(err error) error {
	if err == nil {
		return nil
	}
	return errorWithTrace{
		trace: newTrace(runtime.Caller(1)),
		error: err,
	}
}

// Error wraps the provided error with the caller's file and line number.
// Skipping the provided number of frames.
func Error(err error, skip int) error {
	if err == nil {
		return nil
	}
	return errorWithTrace{
		trace: newTrace(runtime.Caller(skip + 1)),
		error: err,
	}
}

type traceable interface {
	Source() (fn string, file string, line int, ok bool)
}

type trace struct {
	pc   uintptr
	file string
	line int
	ok   bool
}

func (e *trace) StackTrace() []uintptr {
	return []uintptr{e.pc}
}

// newTrace creates a new Trace, use the values returned by runtime.Caller
func newTrace(pc uintptr, file string, line int, ok bool) trace {
	return trace{pc, file, line, ok}
}

// Source implements the traceable interface.
func (t trace) Source() (fn string, file string, line int, ok bool) {
	return runtime.FuncForPC(t.pc).Name(), t.file, t.line, t.ok
}
