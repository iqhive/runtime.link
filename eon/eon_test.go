package eon_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"runtime.link/eon"
)

func TestAge(t *testing.T) {
	var ctx = context.Background()

	var timer, group = eon.New[int, int](nil)

	timer.When(context.Background(), func(ctx context.Context, t time.Time, id int, item int) error {
		fmt.Println(item)
		return nil
	})

	timer.Wait(ctx, 1, time.Now().Add(time.Millisecond), 22)

	group.Wait()
}
