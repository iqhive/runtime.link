package rest_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
)

type watchedItem struct {
	N int `json:"n"`
}

type watching struct {
	api.Specification

	Watch func(context.Context) (<-chan watchedItem, error) `rest:"GET /watch"`
}

// watchEvents returns what a client linked to a server of events reads from the
// stream that the server writes.
func watchEvents(t *testing.T, stream string) []int {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, stream)
	}))
	defer srv.Close()
	client := api.Import[watching](rest.API, srv.URL, srv.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	items, err := client.Watch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got []int
	for item := range items {
		got = append(got, item.N)
	}
	return got
}

// TestClientReadsAnEventOverSeveralLines checks that the data lines of an event
// are read as the one value they are. Each line was tried as a value of its
// own, so an event written over several was dropped without a word.
func TestClientReadsAnEventOverSeveralLines(t *testing.T) {
	for name, c := range map[string]struct {
		stream string
		want   []int
	}{
		"one line to an event": {
			"data: {\"n\": 1}\n\ndata: {\"n\": 2}\n\n", []int{1, 2},
		},
		"an event over three lines": {
			"data: {\"n\": 1}\n\ndata: {\ndata: \"n\": 2\ndata: }\n\ndata: {\"n\": 3}\n\n", []int{1, 2, 3},
		},
		"no space after the colon": {
			"data:{\ndata:\"n\": 4}\n\n", []int{4},
		},
		"carriage returns": {
			"data: {\r\ndata: \"n\": 5}\r\n\r\ndata: {\"n\": 6}\r\n\r\n", []int{5, 6},
		},
		"comments and other fields": {
			": keep-alive\n\nevent: item\nid: 7\ndata: {\"n\": 7}\nretry: 10\n\n:\n\n", []int{7},
		},
		"a data line with nothing in it": {
			"data: {\"n\":\ndata\ndata:\ndata: 8}\n\n", []int{8},
		},
		"an event that is no value is passed over": {
			"data: {\"n\": 1}\n\ndata: {\ndata: nonsense\n\ndata: {\"n\": 9}\n\n", []int{1, 9},
		},
		"lines that are no part of an event": {
			"data: {\"n\": 1}\ndatum: 5\ndatabase\n\n", []int{1},
		},
		"an event the stream ends in the middle of": {
			"data: {\"n\": 1}\n\ndata: {\"n\": 2}\n", []int{1},
		},
		"an event the stream ends a line into": {
			"data: {\"n\": 1}\n\ndata: {\"n\": 2", []int{1},
		},
		"nothing at all": {"", nil},
	} {
		if got := watchEvents(t, c.stream); !slices.Equal(got, c.want) {
			t.Errorf("%s: read %v, want %v", name, got, c.want)
		}
	}
}

// TestClientStopsAtAnEventThatIsTooLarge checks that the lines of one event
// are bounded together, as each of them is: the client holds all of them.
func TestClientStopsAtAnEventThatIsTooLarge(t *testing.T) {
	const line = "data:                                                                                                    \n"
	var stream strings.Builder
	stream.WriteString("data: {\"n\": 1}\n\n")
	stream.WriteString("data: {\"n\":\n")
	// Enough lines of space to pass the 16 MiB a single line may be.
	for range (17 << 20) / len(line) {
		stream.WriteString(line)
	}
	stream.WriteString("data: 2}\n\ndata: {\"n\": 3}\n\n")
	if got := watchEvents(t, stream.String()); !slices.Equal(got, []int{1}) {
		t.Errorf("read %v, want [1] and the end of the stream", got)
	}
}

// TestNilChannelIsAStreamWithNothingInIt checks that a function which returns a
// nil channel and no error is one that worked to its client as well: the 204 it
// is answered with was reported as a failure of the call.
func TestNilChannelIsAStreamWithNothingInIt(t *testing.T) {
	impl := watching{
		Watch: func(context.Context) (<-chan watchedItem, error) { return nil, nil },
	}
	client, _ := linked(t, impl)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	items, err := client.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	select {
	case item, open := <-items:
		if open {
			t.Errorf("the stream of a nil channel held %+v", item)
		}
	case <-ctx.Done():
		t.Fatal("the stream of a nil channel was never closed")
	}

	h, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	for _, accept := range []string{"text/event-stream", "application/json", ""} {
		if rec := serve(t, h, "GET", "/watch", "", "Accept", accept); rec.Code != http.StatusNoContent {
			t.Errorf("GET /watch taking %q: %d %q", accept, rec.Code, rec.Body)
		}
	}
}

// TestAStreamThatIsRefusedIsStillAnError checks the answers next to the 204,
// which are not streams.
func TestAStreamThatIsRefusedIsStillAnError(t *testing.T) {
	for _, status := range []int{200, 202, 205, 400, 404, 500} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		client := api.Import[watching](rest.API, srv.URL, srv.Client())
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if _, err := client.Watch(ctx); err == nil {
			t.Errorf("a stream answered with %d is no error", status)
		}
		cancel()
		srv.Close()
	}
}
