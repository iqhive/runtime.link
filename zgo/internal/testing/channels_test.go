package main

import "testing"

func TestChannels(t *testing.T) {
	var ch = make(chan int, 1)
	ch <- 1
	if <-ch != 1 {
		t.FailNow()
	}
}
