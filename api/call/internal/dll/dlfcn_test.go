//go:build cgo && !windows

package dll

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// The message is per thread, so it's lost whenever the runtime moves
// the goroutine between the failing call and the read of the message.
func TestErrorMessageSurvivesThreadSwitches(t *testing.T) {
	table, err := Open("libc.so.6")
	if err != nil {
		t.Skip("requires glibc: ", err)
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	var wg sync.WaitGroup
	for range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(deadline) {
				_, err := Sym(table, "definitely_not_a_symbol")
				if err == nil || !strings.Contains(err.Error(), "definitely_not_a_symbol") {
					t.Errorf("error = %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestFailedOpenExplainsItself(t *testing.T) {
	_, err := Open("libdoes_not_exist.so")
	if err == nil || !strings.Contains(err.Error(), "libdoes_not_exist.so") {
		t.Errorf("error = %v", err)
	}
}
