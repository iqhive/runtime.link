package main

import (
	"testing"
)

func mark_done(done chan bool) {
	done <- true
}

func TestGoroutines(t *testing.T) {
	done := make(chan bool)
	go mark_done(done)
	<-done
}

func TestChannels(t *testing.T) {
	var ch = make(chan int, 1)
	ch <- 1
	if <-ch != 1 {
		t.FailNow()
	}
}
