package rest_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/xyz"
)

type nullTexts struct {
	api.Specification

	Maybe func(context.Context, xyz.Maybe[string]) (string, error) `rest:"POST(text/plain) /maybe"`
	Raw   func(context.Context, json.RawMessage) (string, error)   `rest:"POST(text/plain) /raw"`
}

// TestTextBodyOfAValueWhoseJSONIsNull pins what a client sends as a text/plain
// body for a value that writes itself as the JSON null, and what is sent for the
// values of the same types that are no string: the text of their JSON, as it
// always was. Only the quotes of a JSON string are taken off.
func TestTextBodyOfAValueWhoseJSONIsNull(t *testing.T) {
	client, log := linked(t, nullTexts{
		Maybe: func(context.Context, xyz.Maybe[string]) (string, error) { return "", nil },
		Raw:   func(context.Context, json.RawMessage) (string, error) { return "", nil },
	})
	ctx := context.Background()
	if _, err := client.Maybe(ctx, xyz.Maybe[string]{}); err != nil || log.last(t).body != "null" {
		t.Errorf("a Maybe that holds nothing was sent as %q, %v; want the text null", log.last(t).body, err)
	}
	for _, c := range []struct {
		raw  json.RawMessage
		want string
	}{
		{nil, "null"},
		{json.RawMessage(`null`), "null"},
		{json.RawMessage(`{"a":1}`), `{"a":1}`},
		{json.RawMessage(`[1,2]`), `[1,2]`},
		{json.RawMessage(`5`), "5"},
		{json.RawMessage(`true`), "true"},
	} {
		if _, err := client.Raw(ctx, c.raw); err != nil || log.last(t).body != c.want {
			t.Errorf("the raw JSON %q was sent as %q, %v; want %q", c.raw, log.last(t).body, err, c.want)
		}
	}
}
