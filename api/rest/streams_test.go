package rest_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api"
)

type window struct {
	From int `json:"from"`
}

type watched struct {
	api.Specification

	Watch  func(context.Context, window) (<-chan int, error)         `rest:"GET /watch"`
	Since  func(context.Context, string, window) (<-chan int, error) `rest:"GET /watch/{id=%v}"`
	Lines  func(context.Context, []string) (<-chan string, error)    `rest:"GET(text/plain) /lines"`
	Values func(context.Context) (<-chan string, error)              `rest:"GET /values"`
}

// TestClientSendsTheBodyOfAChannelFunction checks that the body argument of a
// function that returns a channel is sent, as it is for any other GET. The
// client opened the stream with no body, so the server refused every call.
func TestClientSendsTheBodyOfAChannelFunction(t *testing.T) {
	from := func(start int) (<-chan int, error) {
		values := make(chan int, 2)
		values <- start
		values <- start + 1
		close(values)
		return values, nil
	}
	client, log := linked(t, watched{
		Watch: func(_ context.Context, w window) (<-chan int, error) { return from(w.From) },
		Since: func(_ context.Context, id string, w window) (<-chan int, error) { return from(len(id) + w.From) },
		Lines: func(_ context.Context, lines []string) (<-chan string, error) {
			values := make(chan string, len(lines))
			for _, line := range lines {
				values <- line
			}
			close(values)
			return values, nil
		},
		Values: func(context.Context) (<-chan string, error) {
			values := make(chan string)
			close(values)
			return values, nil
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := client.Watch(ctx, window{From: 7})
	if err != nil {
		t.Fatalf("Watch(from 7) sent the body %q: %v", log.last(t).body, err)
	}
	if got := collect(values); !reflect.DeepEqual(got, []int{7, 8}) {
		t.Errorf("Watch(from 7) = %v", got)
	}
	values, err = client.Since(ctx, "abc", window{From: 1})
	if err != nil {
		t.Fatalf("Since(abc, from 1) sent %s with the body %q: %v", log.last(t).path, log.last(t).body, err)
	}
	if got := collect(values); !reflect.DeepEqual(got, []int{4, 5}) {
		t.Errorf("Since(abc, from 1) = %v", got)
	}
	lines, err := client.Lines(ctx, []string{"a", "b"})
	if err != nil {
		t.Fatalf("Lines(a, b) sent the body %q: %v", log.last(t).body, err)
	}
	if got := collect(lines); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("Lines(a, b) = %q", got)
	}
	// A function without a body argument is sent none, as before.
	none, err := client.Values(ctx)
	if err != nil || log.last(t).body != "" || len(collect(none)) != 0 {
		t.Errorf("Values sent the body %q: %v", log.last(t).body, err)
	}
}

func collect[T any](values <-chan T) []T {
	var got []T
	for value := range values {
		got = append(got, value)
	}
	return got
}
